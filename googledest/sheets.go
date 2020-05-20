package googledest

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"

	personnel_sync "github.com/silinternational/personnel-sync"
)

// MaxColumns specifies the maximum number of columns (fields) copied into the sheet
const MaxColumns = 40

type GoogleSheets struct {
	DestinationConfig personnel_sync.DestinationConfig
	GoogleConfig      GoogleConfig
	Client            http.Client
	SheetsSyncSet     SheetsSyncSet
	lastRow           int
}

type SheetsSyncSet struct {
	DisableAdd    bool
	DisableUpdate bool
	DisableDelete bool
	SheetID       string
}

func getClient(auth GoogleAuth, adminEmail string, scopes ...string) (http.Client, error) {
	googleAuthJson, err := json.Marshal(auth)
	if err != nil {
		return http.Client{}, fmt.Errorf("unable to marshal google auth data into json, error: %s", err.Error())
	}

	config, err := google.JWTConfigFromJSON(googleAuthJson, scopes...)
	if err != nil {
		return http.Client{}, fmt.Errorf("unable to parse client secret file to config: %s", err)
	}

	ctx := context.Background()
	config.Subject = adminEmail
	client := config.Client(ctx)

	return *client, nil
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

	// Initialize AdminService object
	s.Client, err = getClient(
		s.GoogleConfig.GoogleAuth,
		s.GoogleConfig.DelegatedAdminEmail,
		sheets.SpreadsheetsScope,
	)
	if err != nil {
		return &GoogleSheets{}, err
	}

	return &s, nil
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

	g.readSheet()

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

	if !g.SheetsSyncSet.DisableAdd {
		for _, person := range changes.Create {
			wg.Add(1)
			go g.addRow(person, &results.Created, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
	}

	wg.Wait()

	return results
}

func (g *GoogleSheets) addRow(
	person personnel_sync.Person,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- personnel_sync.EventLogItem) {

	defer wg.Done()

	srv, err := sheets.New(&g.Client)
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("Unable to retrieve Sheets client, error: %v", err),
		}
		return
	}

	if g.lastRow == 0 {
		headerRow := make([]interface{}, MaxColumns)
		i := 0
		for key := range person.Attributes {
			headerRow[i] = key
			i++
		}
		for ; i < MaxColumns; i++ {
			headerRow[i] = ""
		}
		v := &sheets.ValueRange{
			Values: [][]interface{}{headerRow},
		}

		_, err := srv.Spreadsheets.Values.Update(
			g.SheetsSyncSet.SheetID,
			"Sheet1!A1:ZZ",
			v).ValueInputOption("RAW").Do()
		if err != nil {
			eventLog <- personnel_sync.EventLogItem{
				Event:   "error",
				Message: fmt.Sprintf("Unable to add row to sheet, error: %v", err),
			}
			return
		}

		g.lastRow++
	}

	newRow := make([]interface{}, MaxColumns)
	i := 0
	for _, val := range person.Attributes {
		newRow[i] = val
		i++
	}
	for ; i < MaxColumns; i++ {
		newRow[i] = ""
	}
	v := &sheets.ValueRange{
		Values: [][]interface{}{newRow},
	}

	newRowRange := fmt.Sprintf("Sheet1!A%d:ZZ", g.lastRow+1)
	fmt.Printf(newRowRange)
	_, err = srv.Spreadsheets.Values.Update(
		g.SheetsSyncSet.SheetID,
		newRowRange,
		v).ValueInputOption("RAW").Do()
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("Unable to add row to sheet, error: %v", err),
		}
		return
	}
	g.lastRow++

	eventLog <- personnel_sync.EventLogItem{
		Event:   "AddMember",
		Message: person.CompareValue,
	}

	atomic.AddUint64(counter, 1)
}

func (g *GoogleSheets) readSheet() {
	srv, err := sheets.New(&g.Client)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client, error: %v", err)
	}

	spreadsheetID := g.SheetsSyncSet.SheetID
	readRange := "Sheet1!A1:E"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet, error: %v", err)
	}
	if len(resp.Values) > 0 {
		for _, row := range resp.Values {
			for _, col := range row {
				fmt.Printf("%s, ", col)
			}
			fmt.Printf("\n")
		}
	}
}
