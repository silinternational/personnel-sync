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

func (g *GoogleUsers) extractData(user admin.User) personnel_sync.Person {
	var id string
	if externalIDs, ok := user.ExternalIds.([]interface{}); ok && len(externalIDs) > 0 {
		if id0, ok := externalIDs[0].(map[string]interface{}); ok {
			if value, ok := id0["value"].(string); ok {
				id = value
			}
		}
	}

	var area, building string
	if locations, ok := user.Locations.([]interface{}); ok && len(locations) > 0 {
		if loc0, ok := locations[0].(map[string]interface{}); ok {
			if a, ok := loc0["area"].(string); ok {
				area = a
			}
			if b, ok := loc0["buildingId"].(string); ok {
				building = b
			}
		}
	}

	var costCenter, department, title string
	if organizations, ok := user.Organizations.([]interface{}); ok && len(organizations) > 0 {
		if org0, ok := organizations[0].(map[string]interface{}); ok {
			if c, ok := org0["costCenter"].(string); ok {
				costCenter = c
			}
			if d, ok := org0["department"].(string); ok {
				department = d
			}
			if t, ok := org0["title"].(string); ok {
				title = t
			}
		}
	}

	var phone string
	if phones, ok := user.Phones.([]interface{}); ok && len(phones) > 0 {
		// should we take only a certain type of phone? (home, work?)
		if phone0, ok := phones[0].(map[string]interface{}); ok {
			if value, ok := phone0["value"].(string); ok {
				phone = value
			}
		}
	}

	var manager string
	if relations, ok := user.Relations.([]interface{}); ok && len(relations) > 0 {
		for i := range relations {
			if mgr, ok := relations[i].(map[string]interface{}); ok {
				if t, ok := mgr["type"].(string); ok && t == "manager" {
					if value, ok := mgr["value"].(string); ok {
						manager = value
						break
					}
				}
			}
		}
	}

	var familyName, givenName string
	if user.Name != nil {
		familyName = user.Name.FamilyName
		givenName = user.Name.GivenName
	}

	newPerson := personnel_sync.Person{
		CompareValue: user.PrimaryEmail,
		Attributes: map[string]string{
			"email":      strings.ToLower(user.PrimaryEmail),
			"familyName": familyName,
			"givenName":  givenName,
			"id":         id,
			"area":       area,
			"building":   building,
			"costCenter": costCenter,
			"department": department,
			"title":      title,
			"phone":      phone,
			"manager":    manager,
		},
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
			people = append(people, g.extractData(*nextUser))
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

	newUser := admin.User{
		Name:          &newName,
		ExternalIds:   []admin.UserExternalId{id},
		Locations:     []admin.UserLocation{location},
		Organizations: []admin.UserOrganization{organization},
		Phones:        []admin.UserPhone{phone},
		Relations:     []admin.UserRelation{relation},
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
