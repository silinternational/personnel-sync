package google

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/syslog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/silinternational/personnel-sync/v5/internal"

	"golang.org/x/net/context"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"
)

type GoogleUsers struct {
	DestinationConfig internal.DestinationConfig
	BatchSize         int
	BatchDelaySeconds int
	GoogleConfig      GoogleConfig
	AdminService      admin.Service
}

func NewGoogleUsersDestination(destinationConfig internal.DestinationConfig) (internal.Destination, error) {
	var googleUsers GoogleUsers
	// Unmarshal ExtraJSON into GoogleConfig struct
	err := json.Unmarshal(destinationConfig.ExtraJSON, &googleUsers.GoogleConfig)
	if err != nil {
		return &GoogleUsers{}, err
	}

	// Defaults
	if googleUsers.BatchSize <= 0 {
		googleUsers.BatchSize = DefaultBatchSize
	}
	if googleUsers.BatchDelaySeconds <= 0 {
		googleUsers.BatchDelaySeconds = DefaultBatchDelaySeconds
	}

	googleUsers.DestinationConfig = destinationConfig

	// Initialize AdminService object
	googleUsers.AdminService, err = initGoogleAdminService(
		googleUsers.GoogleConfig.GoogleAuth,
		googleUsers.GoogleConfig.DelegatedAdminEmail,
		admin.AdminDirectoryUserScope,
	)
	if err != nil {
		return &GoogleUsers{}, err
	}

	return &googleUsers, nil
}

func (g *GoogleUsers) ForSet(syncSetJson json.RawMessage) error {
	// sync sets not implemented for this destination
	return nil
}

func extractData(user admin.User) internal.Person {
	attributes := map[string]string{"email": strings.ToLower(user.PrimaryEmail)}

	if found := findFirstMatchingType(user.ExternalIds, "organization"); found != nil {
		setStringFromInterface(found["value"], attributes, "id")
	}

	if found := findFirstMatchingType(user.Locations, "desk"); found != nil {
		setStringFromInterface(found["area"], attributes, "area")
	}

	if found := findFirstMatchingType(user.Organizations, ""); found != nil {
		setStringFromInterface(found["costCenter"], attributes, "costCenter")
		setStringFromInterface(found["department"], attributes, "department")
		setStringFromInterface(found["title"], attributes, "title")
	}

	//if found := findFirstMatchingType(user.Phones, "work"); found != nil {
	//	setStringFromInterface(found["value"], attributes, "phone")
	//}

	if found := findFirstMatchingType(user.Relations, "manager"); found != nil {
		setStringFromInterface(found["value"], attributes, "manager")
	}

	if user.Name != nil {
		attributes["familyName"] = user.Name.FamilyName
		attributes["givenName"] = user.Name.GivenName
	}

	for schemaKey, schemaVal := range user.CustomSchemas {
		var schema map[string]string
		_ = json.Unmarshal(schemaVal, &schema)
		for propertyKey, propertyVal := range schema {
			attributes[schemaKey+"."+propertyKey] = propertyVal
		}
	}

	attributes = mergeAttributeMaps(attributes, getPhoneNumbersFromUser(user))

	return internal.Person{CompareValue: user.PrimaryEmail, Attributes: attributes}
}

func getPhoneNumbersFromUser(user admin.User) map[string]string {
	attributes := map[string]string{}

	phones, ok := user.Phones.([]interface{})
	if !ok {
		return attributes
	}

	for _, phoneAsInterface := range phones {
		phone := phoneAsInterface.(map[string]interface{})

		phoneType, ok := phone["type"].(string)
		if !ok {
			continue
		}
		val, ok := phone["value"].(string)
		if !ok {
			continue
		}
		custom, _ := phone["customType"].(string)
		primary, ok := phone["primary"].(bool)

		//i := 0
		//key := phoneKey(phoneType, custom, primary, i)
		//_, ok = attributes[key]
		//for ok {
		//	i++
		//	key = phoneKey(phoneType, custom, primary, i)
		//	_, ok = attributes[key]
		//}
		key := phoneKeyR(phoneType, custom, primary, attributes)
		attributes[key] = val
	}

	return attributes
}

func phoneKey(phoneType, custom string, primary bool, i int) string {
	key := "phone" + delim + phoneType
	if i > 0 {
		key += "~" + strconv.Itoa(i)
	}
	if phoneType == "custom" && custom != "" {
		key += delim + custom
	}
	if primary {
		key += delim + "primary"
	}
	return key
}

func phoneKeyR(phoneType, custom string, primary bool, attributes map[string]string) string {
	key := "phone" + delim + phoneType

	if strings.HasPrefix(phoneType, "custom") && custom != "" {
		key += delim + custom
	}
	if primary {
		key += delim + "primary"
	}
	_, ok := attributes[key]
	if ok {
		split := strings.SplitN(phoneType, "~", 2)
		i := 1
		if len(split) > 1 {
			n, _ := strconv.Atoi(split[1])
			i = n + 1
		}

		return phoneKeyR(fmt.Sprintf("%s~%d", split[0], i), custom, primary, attributes)
	}

	return key
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

func (g *GoogleUsers) ListUsers(desiredAttrs []string) ([]internal.Person, error) {
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

	var people []internal.Person
	for _, nextUser := range usersList {
		if nextUser != nil {
			people = append(people, extractData(*nextUser))
		}
	}
	return people, nil
}

func (g *GoogleUsers) ApplyChangeSet(
	changes internal.ChangeSet,
	eventLog chan<- internal.EventLogItem) internal.ChangeResults {

	var results internal.ChangeResults
	var wg sync.WaitGroup

	// One minute per batch
	batchTimer := internal.NewBatchTimer(g.BatchSize, g.BatchDelaySeconds)

	if !g.DestinationConfig.DisableUpdate {
		for _, toUpdate := range changes.Update {
			wg.Add(1)
			go g.updateUser(toUpdate, &results.Updated, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
	}

	wg.Wait()

	return results
}

func newUserForUpdate(person internal.Person, oldUser admin.User) (admin.User, error) {
	user := admin.User{}
	var err error
	var organization admin.UserOrganization
	isOrgModified := false

	phones := getPhoneNumbersFromUser(oldUser)

	for key, val := range person.Attributes {
		switch beforeComma(key) {
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
			phones[key] = val

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

	user.Phones, err = attributesToUserPhones(phones)
	if err != nil {
		return admin.User{}, err
	}

	if isOrgModified {
		// NOTICE: this will overwrite any and all existing Organizations
		user.Organizations = []admin.UserOrganization{organization}
	}

	return user, nil
}

func beforeComma(s string) string {
	split := strings.SplitN(s, ",", 2)
	return split[0]
}

func (g *GoogleUsers) updateUser(
	person internal.Person,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- internal.EventLogItem) {

	defer wg.Done()

	email := person.Attributes["email"]

	oldUser, err := g.getUser(person.CompareValue)
	if err != nil {
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("unable to get old user %s, %s", email, err.Error()),
		}
		return
	}

	newUser, err2 := newUserForUpdate(person, oldUser)
	if err2 != nil {
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("unable to prepare update for %s in Users: %s", email, err2.Error()),
		}
		return
	}

	_, err3 := g.AdminService.Users.Update(email, &newUser).Do()
	if err3 != nil {
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("unable to update %s in Users: %s", email, err3.Error()),
		}
		return
	}

	eventLog <- internal.EventLogItem{
		Level:   syslog.LOG_INFO,
		Message: "UpdateUser " + email,
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

func attributesToUserPhones(phones map[string]string) ([]admin.UserPhone, error) {
	userPhones := []admin.UserPhone{}

	for key, val := range phones {
		split := strings.Split(key, ",")
		if split[0] != "phone" {
			return userPhones, fmt.Errorf("phone key doesn't start with 'phone': %s", key)
		}
		if len(split) < 2 {
			// for backward compatibility
			userPhones = append(userPhones, admin.UserPhone{Type: "work", Value: val})
			continue
		}
		phoneType := strings.TrimRight(split[1], "~0123456789")
		custom := ""
		primary := false
		if len(split) > 2 {
			nPrimary := 2
			if phoneType == "custom" {
				custom = split[2]
				nPrimary = 3
			}
			if len(split) > nPrimary {
				primary = true
			}
		}
		userPhones = append(userPhones, admin.UserPhone{Type: phoneType, CustomType: custom, Primary: primary, Value: val})
	}

	return userPhones, nil
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
