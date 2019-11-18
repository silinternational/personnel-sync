package googledest

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	personnel_sync "github.com/silinternational/personnel-sync"
	"golang.org/x/net/context"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"
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

	for schemaKey, schemaVal := range user.CustomSchemas {
		var schema map[string]string
		_ = json.Unmarshal(schemaVal, &schema)
		for propertyKey, propertyVal := range schema {
			newPerson.Attributes[schemaKey+"."+propertyKey] = propertyVal
		}
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
	usersListCall.Customer("my_customer") // query all domains in this GSuite
	usersListCall.Projection("full")      // include custom fields
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

func newUserForUpdate(person personnel_sync.Person, oldUser admin.User) (admin.User, error) {
	user := admin.User{}
	var err error
	var organization admin.UserOrganization
	isOrgModified := false

	for key, val := range person.Attributes {
		switch key {
		case "givenName":
			if user.Name == nil {
				user.Name = &admin.UserName{GivenName: val}
			} else {
				user.Name.GivenName = val
			}

		case "familyName":
			if user.Name == nil {
				user.Name = &admin.UserName{FamilyName: val}
			} else {
				user.Name.FamilyName = val
			}

		case "id":
			user.ExternalIds, err = updateIDs(val, oldUser.ExternalIds)
			if err != nil {
				return admin.User{}, err
			}

		case "area":
			user.Locations, err = updateLocations(val, oldUser.Locations)
			if err != nil {
				return admin.User{}, err
			}

		case "costCenter":
			organization.CostCenter = val
			isOrgModified = true

		case "department":
			organization.Department = val
			isOrgModified = true

		case "title":
			organization.Title = val
			isOrgModified = true

		case "phone":
			user.Phones, err = updatePhones(val, oldUser.Phones)
			if err != nil {
				return admin.User{}, err
			}

		case "manager":
			user.Relations, err = updateRelations(val, oldUser.Relations)
			if err != nil {
				return admin.User{}, err
			}

		default:
			keys := strings.SplitN(key, ".", 2)
			if len(keys) < 2 {
				continue
			}

			j, err := json.Marshal(&map[string]string{keys[1]: val})
			if err != nil {
				return admin.User{}, fmt.Errorf("error marshaling custom schema, %s", err)
			}

			user.CustomSchemas = map[string]googleapi.RawMessage{
				keys[0]: j,
			}
		}
	}

	if isOrgModified {
		// NOTICE: this will overwrite any and all existing Organizations
		user.Organizations = []admin.UserOrganization{organization}
	}

	return user, nil
}

func (g *GoogleUsers) updateUser(
	person personnel_sync.Person,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- personnel_sync.EventLogItem) {

	defer wg.Done()

	email := person.Attributes["email"]

	oldUser, err := g.getUser(person.CompareValue)
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("unable to get old user %s, %s", email, err.Error())}
		return
	}

	newUser, err2 := newUserForUpdate(person, oldUser)
	if err2 != nil {
		eventLog <- personnel_sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("unable to prepare update for %s in Users: %s", email, err2.Error())}
		return
	}

	_, err3 := g.AdminService.Users.Update(email, &newUser).Do()
	if err3 != nil {
		eventLog <- personnel_sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("unable to update %s in Users: %s", email, err3.Error())}
		return
	}

	eventLog <- personnel_sync.EventLogItem{
		Event:   "UpdateUser",
		Message: email,
	}

	atomic.AddUint64(counter, 1)
}

func (g *GoogleUsers) getUser(email string) (admin.User, error) {
	userCall := g.AdminService.Users.Get(email)
	user, err := userCall.Do()
	if err != nil || user == nil {
		return admin.User{}, err
	}
	return *user, nil
}

func updateIDs(newID string, oldIDs interface{}) ([]admin.UserExternalId, error) {
	IDs := []admin.UserExternalId{{
		Type:  "organization",
		Value: newID,
	}}

	if oldIDs == nil {
		return IDs, nil
	}

	interfaces, ok := oldIDs.([]interface{})
	if !ok {
		return nil, errors.New("no slice in Google API ExternalIDs")
	}

	for i := range interfaces {
		IDMap, ok := interfaces[i].(map[string]interface{})
		if !ok {
			return nil, errors.New("unexpected data in Google API ID list")
		}

		thisType, ok := IDMap["type"].(string)
		if !ok {
			return nil, errors.New("unexpected data in Google API ID list entry")
		}

		if thisType == "organization" {
			continue
		}

		value, _ := IDMap["value"].(string)
		customType, _ := IDMap["customType"].(string)
		IDs = append(IDs, admin.UserExternalId{
			Type:       thisType,
			CustomType: customType,
			Value:      value,
		})
	}

	return IDs, nil
}

func updateLocations(newArea string, oldLocations interface{}) ([]admin.UserLocation, error) {
	locations := []admin.UserLocation{{
		Type: "desk",
		Area: newArea,
	}}

	if oldLocations == nil {
		return locations, nil
	}

	interfaces, ok := oldLocations.([]interface{})
	if !ok {
		return nil, errors.New("no slice in Google API Locations")
	}

	for i := range interfaces {
		locationMap, ok := interfaces[i].(map[string]interface{})
		if !ok {
			return nil, errors.New("unexpected data in Google API location list")
		}

		thisType, ok := locationMap["type"].(string)
		if !ok {
			return nil, errors.New("unexpected data in Google API location list entry")
		}

		if thisType == "desk" {
			continue
		}

		area, _ := locationMap["area"].(string)
		buildingId, _ := locationMap["buildingId"].(string)
		customType, _ := locationMap["customType"].(string)
		deskCode, _ := locationMap["deskCode"].(string)
		floorName, _ := locationMap["floorName"].(string)
		floorSection, _ := locationMap["floorSection"].(string)
		locations = append(locations, admin.UserLocation{
			Type:         thisType,
			Area:         area,
			BuildingId:   buildingId,
			CustomType:   customType,
			DeskCode:     deskCode,
			FloorName:    floorName,
			FloorSection: floorSection,
		})
	}

	return locations, nil
}

func updatePhones(newPhone string, oldPhones interface{}) ([]admin.UserPhone, error) {
	phones := []admin.UserPhone{{Type: "work", Value: newPhone}}

	if oldPhones == nil {
		return phones, nil
	}

	interfaces, ok := oldPhones.([]interface{})
	if !ok {
		return nil, errors.New("no slice in Google API Phones")
	}

	for i := range interfaces {
		phoneMap, ok := interfaces[i].(map[string]interface{})
		if !ok {
			return nil, errors.New("unexpected data in Google API phone list")
		}

		thisType, ok := phoneMap["type"].(string)
		if !ok {
			return nil, errors.New("unexpected data in Google API phone list entry")
		}

		if thisType == "work" {
			continue
		}

		value, _ := phoneMap["value"].(string)
		customType, _ := phoneMap["customType"].(string)
		primary, _ := phoneMap["primary"].(bool)
		phones = append(phones, admin.UserPhone{
			Type:       thisType,
			Value:      value,
			CustomType: customType,
			Primary:    primary,
		})
	}

	return phones, nil
}

func updateRelations(newRelation string, oldRelations interface{}) ([]admin.UserRelation, error) {
	relations := []admin.UserRelation{{Type: "manager", Value: newRelation}}

	if oldRelations == nil {
		return relations, nil
	}

	interfaces, ok := oldRelations.([]interface{})
	if !ok {
		return nil, errors.New("no slice in Google API Relations")
	}

	for i := range interfaces {
		relationMap, ok := interfaces[i].(map[string]interface{})
		if !ok {
			return nil, errors.New("unexpected data in Google API relation list")
		}

		thisType, ok := relationMap["type"].(string)
		if !ok {
			return nil, errors.New("unexpected data in Google API relation list entry")
		}

		if thisType == "manager" {
			continue
		}

		value, _ := relationMap["value"].(string)
		customType, _ := relationMap["customType"].(string)
		relations = append(relations, admin.UserRelation{
			Type:       thisType,
			Value:      value,
			CustomType: customType,
		})
	}

	return relations, nil
}
