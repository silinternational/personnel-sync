package googledest

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"

	personnel_sync "github.com/silinternational/personnel-sync"
)

type GoogleSheets struct {
	DestinationConfig personnel_sync.DestinationConfig
	GoogleConfig      GoogleConfig
	Client            http.Client
	SheetsSyncSet     SheetsSyncSet
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

	srv, err := sheets.New(&g.Client)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	spreadsheetID := g.SheetsSyncSet.SheetID
	readRange := "Sheet1!A2:E"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	if len(resp.Values) == 0 {
		fmt.Println("No data found.")
	} else {
		fmt.Println("Name, Major:")
		for _, row := range resp.Values {
			// Print columns A and E, which correspond to indices 0 and 4.
			fmt.Printf("%s, %s\n", row[0], row[4])
		}
	}

	//var membersList []*admin.Member
	//membersListCall := g.AdminService.Members.List(g.SheetsSyncSet.GroupEmail)
	//err := membersListCall.Pages(context.TODO(), func(members *admin.Members) error {
	//	membersList = append(membersList, members.Members...)
	//	return nil
	//})
	//if err != nil {
	//	return []personnel_sync.Person{}, fmt.Errorf("unable to get members of group %s: %s", g.SheetsSyncSet.GroupEmail, err.Error())
	//}

	var members []personnel_sync.Person
	//
	//for _, nextMember := range membersList {
	//	// Do not include Extra Managers or ExtraOwners in list to prevent inclusion in delete list
	//	if isExtraManager, _ := personnel_sync.InArray(nextMember.Email, g.SheetsSyncSet.ExtraManagers); isExtraManager {
	//		continue
	//	}
	//	if isExtraOwner, _ := personnel_sync.InArray(nextMember.Email, g.SheetsSyncSet.ExtraOwners); isExtraOwner {
	//		continue
	//	}
	//	if isExtraMember, _ := personnel_sync.InArray(nextMember.Email, g.SheetsSyncSet.ExtraMembers); isExtraMember {
	//		continue
	//	}
	//
	//	members = append(members, personnel_sync.Person{
	//		CompareValue: nextMember.Email,
	//		Attributes: map[string]string{
	//			"Email": strings.ToLower(nextMember.Email),
	//		},
	//	})
	//}

	return members, nil
}

func (g *GoogleSheets) ApplyChangeSet(
	changes personnel_sync.ChangeSet,
	eventLog chan<- personnel_sync.EventLogItem) personnel_sync.ChangeResults {

	var results personnel_sync.ChangeResults
	//var wg sync.WaitGroup
	//
	//// key = email, value = role
	//toBeCreated := map[string]string{}
	//for _, person := range changes.Create {
	//	toBeCreated[person.CompareValue] = RoleMember
	//}
	//
	//// Update Owner / Manager roles
	//for _, owner := range g.SheetsSyncSet.Owners {
	//	if _, ok := toBeCreated[owner]; ok {
	//		toBeCreated[owner] = RoleOwner
	//	}
	//}
	//for _, manager := range g.SheetsSyncSet.Managers {
	//	if _, ok := toBeCreated[manager]; ok {
	//		toBeCreated[manager] = RoleManager
	//	}
	//}
	//
	//// Add any ExtraManagers, ExtraOwners, and ExtraMembers to Create list since they are not in the source people
	//for _, manager := range g.SheetsSyncSet.ExtraManagers {
	//	toBeCreated[manager] = RoleManager
	//}
	//for _, owner := range g.SheetsSyncSet.ExtraOwners {
	//	toBeCreated[owner] = RoleOwner
	//}
	//for _, member := range g.SheetsSyncSet.ExtraMembers {
	//	toBeCreated[member] = RoleMember
	//}
	//
	//// One minute per batch
	//batchTimer := personnel_sync.NewBatchTimer(g.BatchSize, g.BatchDelaySeconds)
	//
	//if !g.SheetsSyncSet.DisableAdd {
	//	for email, role := range toBeCreated {
	//		wg.Add(1)
	//		go g.addMember(email, role, &results.Created, &wg, eventLog)
	//		batchTimer.WaitOnBatch()
	//	}
	//}
	//
	//if !g.SheetsSyncSet.DisableDelete {
	//	for _, dp := range changes.Delete {
	//		// Do not delete ExtraManagers, ExtraOwners, or ExtraMembers
	//		if isExtraManager, _ := personnel_sync.InArray(dp.CompareValue, g.SheetsSyncSet.ExtraManagers); isExtraManager {
	//			continue
	//		}
	//		if isExtraOwner, _ := personnel_sync.InArray(dp.CompareValue, g.SheetsSyncSet.ExtraOwners); isExtraOwner {
	//			continue
	//		}
	//		if isExtraMember, _ := personnel_sync.InArray(dp.CompareValue, g.SheetsSyncSet.ExtraMembers); isExtraMember {
	//			continue
	//		}
	//		wg.Add(1)
	//		go g.removeMember(dp.CompareValue, &results.Deleted, &wg, eventLog)
	//		batchTimer.WaitOnBatch()
	//	}
	//}
	//
	//wg.Wait()

	return results
}

func (g *GoogleSheets) addMember(
	email, role string,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- personnel_sync.EventLogItem) {

	defer wg.Done()

	//newMember := admin.Member{
	//	Role:  role,
	//	Email: email,
	//}
	//
	//_, err := g.AdminService.Members.Insert(g.SheetsSyncSet.GroupEmail, &newMember).Do()
	//if err != nil && !strings.Contains(err.Error(), "409") { // error code 409 is for existing user
	//	eventLog <- personnel_sync.EventLogItem{
	//		Event:   "error",
	//		Message: fmt.Sprintf("unable to insert %s in Google group %s: %s", email, g.SheetsSyncSet.GroupEmail, err.Error())}
	//	return
	//}
	//
	//eventLog <- personnel_sync.EventLogItem{
	//	Event:   "AddMember",
	//	Message: email,
	//}
	//
	//atomic.AddUint64(counter, 1)
}

func (g *GoogleSheets) removeMember(
	email string,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- personnel_sync.EventLogItem) {

	defer wg.Done()

	//err := g.AdminService.Members.Delete(g.SheetsSyncSet.GroupEmail, email).Do()
	//if err != nil {
	//	eventLog <- personnel_sync.EventLogItem{
	//		Event:   "error",
	//		Message: fmt.Sprintf("unable to delete %s from Google group %s: %s", email, g.SheetsSyncSet.GroupEmail, err.Error())}
	//	return
	//}
	//
	//eventLog <- personnel_sync.EventLogItem{
	//	Event:   "RemoveMember",
	//	Message: email,
	//}
	//
	//atomic.AddUint64(counter, 1)
}
