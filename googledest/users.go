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

func extractData(user admin.User) personnel_sync.Person {
	newPerson := personnel_sync.Person{
		CompareValue: user.PrimaryEmail,
		Attributes: map[string]string{
			"email": strings.ToLower(user.PrimaryEmail),
		},
	}

	if found := findFirstMatchingType(user.ExternalIds, "organization"); found != nil {
		setStringFromInterface(found["value"], newPerson.Attributes, "id")
	}

	if found := findFirstMatchingType(user.Locations, "desk"); found != nil {
		setStringFromInterface(found["area"], newPerson.Attributes, "area")
		setStringFromInterface(found["buildingId"], newPerson.Attributes, "building")
	}

	if found := findFirstMatchingType(user.Organizations, ""); found != nil {
		setStringFromInterface(found["costCenter"], newPerson.Attributes, "costCenter")
		setStringFromInterface(found["department"], newPerson.Attributes, "department")
		setStringFromInterface(found["title"], newPerson.Attributes, "title")
	}

	if found := findFirstMatchingType(user.Phones, "work"); found != nil {
		setStringFromInterface(found["value"], newPerson.Attributes, "phone")
	}

	if found := findFirstMatchingType(user.Relations, "manager"); found != nil {
		setStringFromInterface(found["value"], newPerson.Attributes, "manager")
	}

	if user.Name != nil {
		newPerson.Attributes["familyName"] = user.Name.FamilyName
		newPerson.Attributes["givenName"] = user.Name.GivenName
	}

	return newPerson
}

// findFirstMatchingType iterates through a slice of interfaces until it finds a matching key. The underlying type
// of the given interface must be `[]map[string]interface{}`. If `findType` is empty, the first element in the
// slice is returned.
func findFirstMatchingType(in interface{}, findType string) map[string]interface{} {
	sliceOfInterfaces, ok := in.([]interface{})
	if !ok {
		return nil
	}
	for _, i := range sliceOfInterfaces {
		if i2 := isMatchingType(i, findType); i2 != nil {
			return i2
		}
	}
	return nil
}

// isMatchingType returns the value of `i`, cast to `map[string]interface{}` if it contains an entry with key 'type'
// and value equal to `findType`. If `findType` is empty, the first element in the slice is returned.
func isMatchingType(i interface{}, findType string) map[string]interface{} {
	if m, ok := i.(map[string]interface{}); ok {
		if findType == "" {
			return m
		}
		if recordType, ok := m["type"].(string); ok && recordType == findType {
			return m
		}
	}
	return nil
}

// setStringFromInterface gets a string from an interface{}, and assigns it to a map
func setStringFromInterface(i interface{}, m map[string]string, key string) {
	if value, ok := i.(string); ok {
		m[key] = value
	}
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
