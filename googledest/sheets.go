package googledest

import (
	"encoding/json"
	"fmt"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	sync "github.com/silinternational/personnel-sync/v3"
)

const DefaultSheetName = "Sheet1"

type GoogleSheets struct {
	DestinationConfig sync.DestinationConfig
	GoogleConfig      GoogleConfig
	Service           *sheets.Service
	SheetsSyncSet     SheetsSyncSet
}

type SheetsSyncSet struct {
	SheetID   string
	SheetName string
}

func NewGoogleSheetsDestination(destinationConfig sync.DestinationConfig) (sync.Destination, error) {
	var s GoogleSheets

	err := json.Unmarshal(destinationConfig.ExtraJSON, &s.GoogleConfig)
	if err != nil {
		return &GoogleSheets{}, err
	}

	s.Service, err = initSheetsService(
		s.GoogleConfig.GoogleAuth,
		s.GoogleConfig.DelegatedAdminEmail,
		sheets.SpreadsheetsScope,
	)
	if err != nil {
		return &GoogleSheets{}, err
	}

	s.DestinationConfig = destinationConfig

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
	return ""
}

func (g *GoogleSheets) ForSet(syncSetJson json.RawMessage) error {
	var syncSetConfig SheetsSyncSet
	err := json.Unmarshal(syncSetJson, &syncSetConfig)
	if err != nil {
		return err
	}

	g.SheetsSyncSet = syncSetConfig

	// Defaults
	if g.SheetsSyncSet.SheetName == "" {
		g.SheetsSyncSet.SheetName = DefaultSheetName
	}

	return nil
}

func (g *GoogleSheets) ListUsers() ([]sync.Person, error) {
	var members []sync.Person

	// To keep it simple, ignore the existing content and overwrite the entire sheet

	return members, nil
}

func (g *GoogleSheets) ApplyChangeSet(
	changes sync.ChangeSet,
	eventLog chan<- sync.EventLogItem) sync.ChangeResults {

	if g.DestinationConfig.DisableAdd || g.DestinationConfig.DisableDelete || g.DestinationConfig.DisableUpdate {
		eventLog <- sync.EventLogItem{
			Event:   "ApplyChangeSet",
			Message: fmt.Sprintf("Sync is disabled, no action taken"),
		}
		return sync.ChangeResults{}
	}

	sheetData, err := g.readSheet()
	if err != nil {
		eventLog <- sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("Unable to read sheet, error: %v", err),
		}
	}
	g.clearSheet(sheetData, eventLog)
	g.updateSheet(g.getHeader(sheetData), changes.Create, eventLog)

	return sync.ChangeResults{Created: uint64(len(changes.Create))}
}

func (g *GoogleSheets) readSheet() ([][]interface{}, error) {
	readRange := fmt.Sprintf("%s!A1:ZZ", g.SheetsSyncSet.SheetName)
	resp, err := g.Service.Spreadsheets.Values.Get(g.SheetsSyncSet.SheetID, readRange).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve data from sheet, error: %v", err)
	}
	if len(resp.Values) < 1 {
		return nil, fmt.Errorf("no header row found in sheet")
	}
	return resp.Values, nil
}

func (g *GoogleSheets) getHeader(data [][]interface{}) map[string]int {
	header := make(map[string]int, len(data[0]))
	for i, v := range data[0] {
		field := fmt.Sprintf("%v", v)
		if _, ok := header[field]; !ok {
			header[field] = i
		}
	}
	return header
}

func (g *GoogleSheets) clearSheet(data [][]interface{}, eventLog chan<- sync.EventLogItem) {
	for i, row := range data {
		if i == 0 {
			continue
		}
		for j := range row {
			data[i][j] = ""
		}
	}
	v := &sheets.ValueRange{
		Values: data,
	}

	updateRange := fmt.Sprintf("%s!A1", g.SheetsSyncSet.SheetName)
	_, err := g.Service.Spreadsheets.Values.
		Update(g.SheetsSyncSet.SheetID, updateRange, v).
		ValueInputOption("RAW").Do()
	if err != nil {
		eventLog <- sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("unable to clear sheet, error: %v", err),
		}
		return
	}
}

func (g *GoogleSheets) updateSheet(header map[string]int, persons []sync.Person, eventLog chan<- sync.EventLogItem) {
	table := make([][]interface{}, len(persons))
	for i, person := range persons {
		row := make([]interface{}, len(header))
		for field, val := range person.Attributes {
			if col, ok := header[field]; ok {
				row[col] = val
			}
		}
		table[i] = row
	}
	v := &sheets.ValueRange{
		Values: table,
	}

	updateRange := fmt.Sprintf("%s!A2:ZZ", g.SheetsSyncSet.SheetName)
	_, err := g.Service.Spreadsheets.Values.
		Update(g.SheetsSyncSet.SheetID, updateRange, v).
		ValueInputOption("RAW").Do()
	if err != nil {
		eventLog <- sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("Unable to update sheet, error: %v", err),
		}
	}
}
