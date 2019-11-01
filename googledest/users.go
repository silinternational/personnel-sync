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
	googleUsers.AdminService, err = initGoogleAdminService(
		googleUsers.GoogleUsersConfig.GoogleAuth,
		googleUsers.GoogleUsersConfig.DelegatedAdminEmail,
		admin.AdminDirectoryUserScope,
	)
	if err != nil {
		return &GoogleUsers{}, err
	}

	return &googleUsers, nil
}

func (g *GoogleUsers) GetIDField() string {
	return "id"
}

func (g *GoogleUsers) ForSet(syncSetJson json.RawMessage) error {
	// sync sets not implemented for this destination
	return nil
}

// getStringFromInterface gets a string from an interface{}, and assigns it to a map
func getStringFromInterface(i interface{}, m map[string]string, key string) {
	if value, ok := i.(string); ok {
		m[key] = value
	}
}

// findFirstMatchingType iterates through a slice of interfaces until it finds a matching key. The underlying type
// of the given interface must be `[]map[string]interface{}`. If `findType` is empty, the first element in the
// slice is returned.
func findFirstMatchingType(in interface{}, findType string) map[string]interface{} {
	if sliceOfInterfaces, ok := in.([]interface{}); ok {
		for _, i := range sliceOfInterfaces {
			if m, ok := i.(map[string]interface{}); ok {
				if dataType, ok := m["type"].(string); findType == "" || (ok && dataType == findType) {
					return m
				}
			}
		}
	}
	return nil
}

func extractData(user admin.User) personnel_sync.Person {
	newPerson := personnel_sync.Person{
		CompareValue: user.PrimaryEmail,
		Attributes: map[string]string{
			"email": strings.ToLower(user.PrimaryEmail),
		},
	}

	if found := findFirstMatchingType(user.ExternalIds, "organization"); found != nil {
		getStringFromInterface(found["value"], newPerson.Attributes, "id")
	}

	if found := findFirstMatchingType(user.Locations, "desk"); found != nil {
		getStringFromInterface(found["area"], newPerson.Attributes, "area")
		getStringFromInterface(found["buildingId"], newPerson.Attributes, "building")
	}

	if found := findFirstMatchingType(user.Organizations, ""); found != nil {
		getStringFromInterface(found["costCenter"], newPerson.Attributes, "costCenter")
		getStringFromInterface(found["department"], newPerson.Attributes, "department")
		getStringFromInterface(found["title"], newPerson.Attributes, "title")
	}

	if found := findFirstMatchingType(user.Phones, "work"); found != nil {
		getStringFromInterface(found["value"], newPerson.Attributes, "phone")
	}

	if found := findFirstMatchingType(user.Relations, "manager"); found != nil {
		getStringFromInterface(found["value"], newPerson.Attributes, "manager")
	}

	if user.Name != nil {
		newPerson.Attributes["familyName"] = user.Name.FamilyName
		newPerson.Attributes["givenName"] = user.Name.GivenName
	}

	return newPerson
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
		return nil, fmt.Errorf("unable to get users: %s", err)
	}

	var people []personnel_sync.Person
	for _, nextUser := range usersList {
		if nextUser != nil {
			people = append(people, extractData(*nextUser))
		}
	}
	return people, nil
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

func newUserForUpdate(person personnel_sync.Person) admin.User {
	newName := admin.UserName{
		GivenName:  person.Attributes["givenName"],
		FamilyName: person.Attributes["familyName"],
	}

	id := admin.UserExternalId{
		Type:  "organization",
		Value: person.Attributes["id"],
	}

	location := admin.UserLocation{
		Type:       "desk",
		Area:       person.Attributes["area"],
		BuildingId: person.Attributes["building"],
	}

	organization := admin.UserOrganization{
		CostCenter: person.Attributes["costCenter"],
		Department: person.Attributes["department"],
		Title:      person.Attributes["title"],
	}

	phone := admin.UserPhone{
		Type:  "work",
		Value: person.Attributes["phone"],
	}

	relation := admin.UserRelation{
		Type:  "manager",
		Value: person.Attributes["manager"],
	}

	return admin.User{
		Name:          &newName,
		ExternalIds:   []admin.UserExternalId{id},
		Locations:     []admin.UserLocation{location},
		Organizations: []admin.UserOrganization{organization},
		Phones:        []admin.UserPhone{phone},
		Relations:     []admin.UserRelation{relation},
	}
}

func (g *GoogleUsers) updateUser(
	person personnel_sync.Person,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- personnel_sync.EventLogItem) {

	defer wg.Done()

	newUser := newUserForUpdate(person)

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
