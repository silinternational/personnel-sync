package googledest

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	personnel_sync "github.com/silinternational/personnel-sync"
)

// MaxColumns specifies the maximum number of columns (fields) copied into the sheet
const MaxColumns = 40

type GoogleSheets struct {
	DestinationConfig personnel_sync.DestinationConfig
	GoogleConfig      GoogleConfig
	Service           *sheets.Service
	SheetsSyncSet     SheetsSyncSet
	header            map[string]int // map from field name to column number, zero-based (A=0)
	rowsInSheet       int            // the total number of rows of data in the sheet, including the header row
}

type SheetsSyncSet struct {
	SheetID string
}

func NewGoogleSheetsDestination(destinationConfig personnel_sync.DestinationConfig) (personnel_sync.Destination, error) {
	var s GoogleSheets
	// Unmarshal ExtraJSON into GoogleConfig struct
	err := json.Unmarshal(destinationConfig.ExtraJSON, &s.GoogleConfig)
	if err != nil {
		return &GoogleSheets{}, err
	}

	// Defaults
	if s.GoogleConfig.BatchSize <= 0 {
		s.GoogleConfig.BatchSize = DefaultBatchSize
	}
	if s.GoogleConfig.BatchDelaySeconds <= 0 {
		s.GoogleConfig.BatchDelaySeconds = DefaultBatchDelaySeconds
	}

	// Initialize Sheets Service object
	s.Service, err = initSheetsService(
		s.GoogleConfig.GoogleAuth,
		s.GoogleConfig.DelegatedAdminEmail,
		sheets.SpreadsheetsScope,
	)
	if err != nil {
		return &GoogleSheets{}, err
	}

	return &s, nil
}

func initSheetsService(auth GoogleAuth, adminEmail string, scopes ...string) (*sheets.Service, error) {
	googleAuthJson, err := json.Marshal(auth)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal google auth data into json, error: %s", err.Error())
	}

	config, err := google.JWTConfigFromJSON(googleAuthJson, scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config, error: %s", err)
	}

	config.Subject = adminEmail

	ctx := context.Background()
	svc, err := sheets.NewService(ctx, option.WithHTTPClient(config.Client(ctx)))
	if err != nil {
		return nil, fmt.Errorf("unable to create sheets service, error: %s", err)
	}
	return svc, nil
}

func (g *GoogleSheets) GetIDField() string {
	return "email"
}

func (g *GoogleSheets) ForSet(syncSetJson json.RawMessage) error {
	var syncSetConfig SheetsSyncSet
	err := json.Unmarshal(syncSetJson, &syncSetConfig)
	if err != nil {
		return err
	}

	g.SheetsSyncSet = syncSetConfig

	return nil
}

func (g *GoogleSheets) ListUsers() ([]personnel_sync.Person, error) {
	var members []personnel_sync.Person

	if err := g.readSheet(); err != nil {
		return members, err
	}

	// To start with, let's just ignore the existing content and overwrite the entire sheet

	return members, nil
}

func (g *GoogleSheets) ApplyChangeSet(
	changes personnel_sync.ChangeSet,
	eventLog chan<- personnel_sync.EventLogItem) personnel_sync.ChangeResults {

	var results personnel_sync.ChangeResults
	var wg sync.WaitGroup

	// One minute per batch
	batchTimer := personnel_sync.NewBatchTimer(g.GoogleConfig.BatchSize, g.GoogleConfig.BatchDelaySeconds)

	for i, person := range changes.Create {
		wg.Add(1)
		go g.addRow(i, person, &results.Created, &wg, eventLog)
		batchTimer.WaitOnBatch()
	}

	g.clearExtraRows(len(changes.Create), eventLog)

	wg.Wait()

	return results
}

func (g *GoogleSheets) addRow(
	n int,
	person personnel_sync.Person,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- personnel_sync.EventLogItem) {

	defer wg.Done()

	newRow := make([]interface{}, MaxColumns)
	for i := 0; i < MaxColumns; i++ {
		newRow[i] = ""
	}
	for field, val := range person.Attributes {
		if col, ok := g.header[field]; ok {
			newRow[col] = val
		}
	}
	v := &sheets.ValueRange{
		Values: [][]interface{}{newRow},
	}

	_, err := g.Service.Spreadsheets.Values.Update(
		g.SheetsSyncSet.SheetID,
		fmt.Sprintf("Sheet1!A%d:ZZ", n+2),
		v).ValueInputOption("RAW").Do()
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("Unable to add row to sheet, error: %v", err),
		}
		return
	}

	eventLog <- personnel_sync.EventLogItem{
		Event:   "AddRow",
		Message: person.CompareValue,
	}

	atomic.AddUint64(counter, 1)
}

func (g *GoogleSheets) readSheet() error {
	readRange := "Sheet1!A1:ZZ"
	resp, err := g.Service.Spreadsheets.Values.Get(g.SheetsSyncSet.SheetID, readRange).Do()
	if err != nil {
		return fmt.Errorf("unable to retrieve data from sheet, error: %v", err)
	}
	if len(resp.Values) < 1 {
		return fmt.Errorf("no header row found in sheet")
	}
	g.header = make(map[string]int, len(resp.Values[0]))
	for i, v := range resp.Values[0] {
		field := fmt.Sprintf("%v", v)
		if _, ok := g.header[field]; !ok {
			g.header[field] = i
		}
	}
	g.rowsInSheet = len(resp.Values)
	return nil
}

func (g *GoogleSheets) clearExtraRows(n int, eventLog chan<- personnel_sync.EventLogItem) {
	if g.rowsInSheet <= n+1 {
		return
	}

	var emptyCells [][]interface{}
	for i := 0; i < g.rowsInSheet-(n+1); i++ {
		row := make([]interface{}, MaxColumns)
		for j := 0; j < MaxColumns; j++ {
			row[j] = ""
		}
		emptyCells = append(emptyCells, row)

	}
	v := &sheets.ValueRange{
		Values: emptyCells,
	}

	newRowRange := fmt.Sprintf("Sheet1!A%d:ZZ", n+2)
	fmt.Printf(newRowRange)
	_, err := g.Service.Spreadsheets.Values.
		Update(g.SheetsSyncSet.SheetID, newRowRange, v).
		ValueInputOption("RAW").Do()
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("unable to clear extra rows, error: %v", err),
		}
		return
	}

	g.rowsInSheet = n + 1
}
