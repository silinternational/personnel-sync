package googledest

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	admin "google.golang.org/api/admin/directory/v1"

	personnel_sync "github.com/silinternational/personnel-sync"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
)

type GoogleUsersConfig struct {
	DelegatedAdminEmail string
	GoogleAuth          GoogleAuth
}

type GoogleUsers struct {
	GoogleUsersConfig  GoogleUsersConfig
	AdminService       admin.Service
	BatchSizePerMinute int
}

func NewGoogleUsersDestination(destinationConfig personnel_sync.DestinationConfig) (personnel_sync.Destination, error) {
	var googleUsers GoogleUsers
	// Unmarshal ExtraJSON into GoogleUsersConfig struct
	err := json.Unmarshal(destinationConfig.ExtraJSON, &googleUsers.GoogleUsersConfig)
	if err != nil {
		return &GoogleUsers{}, err
	}

	// Defaults
	if googleUsers.BatchSizePerMinute <= 0 {
		googleUsers.BatchSizePerMinute = DefaultBatchSizePerMinute
	}

	// Initialize AdminService object
	err = googleUsers.initGoogleAdminService()
	if err != nil {
		return &GoogleUsers{}, err
	}

	return &googleUsers, nil
}

// GetGoogleAdminService authenticates with the Google API and returns an admin.Service
//  that has the scopes to manage Users
//  Authentication requires an email address that matches an actual GMail user (e.g. a machine account)
func (g *GoogleUsers) initGoogleAdminService() error {
	googleAuthJson, err := json.Marshal(g.GoogleUsersConfig.GoogleAuth)
	if err != nil {
		return fmt.Errorf("unable to marshal google auth data into json, error: %s", err.Error())
	}

	config, err := google.JWTConfigFromJSON(googleAuthJson, admin.AdminDirectoryUserScope)
	if err != nil {
		return fmt.Errorf("unable to parse client secret file to config: %s", err)
	}

	config.Subject = g.GoogleUsersConfig.DelegatedAdminEmail
	client := config.Client(context.Background())

	adminService, err := admin.New(client)
	if err != nil {
		return fmt.Errorf("unable to retrieve directory Service: %s", err)
	}

	g.AdminService = *adminService

	return nil
}

func (g *GoogleUsers) GetIDField() string {
	return "id"
}

func (g *GoogleUsers) ForSet(syncSetJson json.RawMessage) error {
	// sync sets not implemented for this destination
	return nil
}

func (g *GoogleUsers) ListUsers() ([]personnel_sync.Person, error) {
	var usersList []*admin.User
	usersListCall := g.AdminService.Users.List()
	usersListCall.Customer("my_customer")
	err := usersListCall.Pages(context.TODO(), func(users *admin.Users) error {
		usersList = append(usersList, users.Users...)
		return nil
	})
	if err != nil {
		return []personnel_sync.Person{}, fmt.Errorf("unable to get users: %s", err)
	}

	var users []personnel_sync.Person

	for _, nextUser := range usersList {
		newPerson := personnel_sync.Person{
			CompareValue: nextUser.PrimaryEmail,
			Attributes: map[string]string{
				"email":      strings.ToLower(nextUser.PrimaryEmail),
				"familyName": nextUser.Name.FamilyName,
				"givenName":  nextUser.Name.GivenName,
				"fullName":   nextUser.Name.FullName,
			},
		}

		users = append(users, newPerson)
	}

	return users, nil
}

func (g *GoogleUsers) ApplyChangeSet(
	changes personnel_sync.ChangeSet,
	eventLog chan<- personnel_sync.EventLogItem) personnel_sync.ChangeResults {

	var results personnel_sync.ChangeResults
	var wg sync.WaitGroup

	// One minute per batch
	batchTimer := personnel_sync.NewBatchTimer(g.BatchSizePerMinute, int(60))

	for _, toUpdate := range changes.Update {
		wg.Add(1)
		go g.updateUser(toUpdate, &results.Updated, &wg, eventLog)
		batchTimer.WaitOnBatch()
	}

	wg.Wait()

	return results
}

func (g *GoogleUsers) updateUser(
	person personnel_sync.Person,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- personnel_sync.EventLogItem) {

	defer wg.Done()

	newName := admin.UserName{
		GivenName:  person.Attributes["givenName"],
		FamilyName: person.Attributes["familyName"],
	}

	newUser := admin.User{
		Name: &newName,
	}

	email := person.Attributes["email"]

	_, err := g.AdminService.Users.Update(email, &newUser).Do()
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("unable to update %s in Users: %s", email, err.Error())}
		return
	}

	eventLog <- personnel_sync.EventLogItem{
		Event:   "UpdateUser",
		Message: email,
	}

	atomic.AddUint64(counter, 1)
}
