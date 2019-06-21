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

const DefaultBatchSizePerMinute = 50
const RoleMember = "MEMBER"
const RoleOwner = "OWNER"
const RoleManager = "MANAGER"

type GoogleGroupsConfig struct {
	DelegatedAdminEmail string
	GoogleAuth          GoogleAuth
}

type GoogleAuth struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri"`
	TokenURI                string `json:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	ClientX509CertURL       string `json:"client_x509_cert_url"`
}

type GoogleGroups struct {
	DestinationConfig  personnel_sync.DestinationConfig
	GoogleGroupsConfig GoogleGroupsConfig
	AdminService       admin.Service
	GroupSyncSet       GroupSyncSet
	BatchSizePerMinute int
}

type GroupSyncSet struct {
	GroupEmail  string
	Owners      []string
	Managers    []string
	ExtraOwners []string
}

func NewGoogleGroupsDestination(destinationConfig personnel_sync.DestinationConfig) (personnel_sync.Destination, error) {
	var googleGroups GoogleGroups
	// Unmarshal ExtraJSON into GoogleGroupsConfig struct
	err := json.Unmarshal(destinationConfig.ExtraJSON, &googleGroups.GoogleGroupsConfig)
	if err != nil {
		return &GoogleGroups{}, err
	}

	// Defaults
	if googleGroups.BatchSizePerMinute <= 0 {
		googleGroups.BatchSizePerMinute = DefaultBatchSizePerMinute
	}

	// Initialize AdminService object
	err = googleGroups.initGoogleAdminService()
	if err != nil {
		return &GoogleGroups{}, err
	}

	return &googleGroups, nil
}

func (g *GoogleGroups) GetIDField() string {
	return "email"
}

func (g *GoogleGroups) ForSet(syncSetJson json.RawMessage) error {
	var syncSetConfig GroupSyncSet
	err := json.Unmarshal(syncSetJson, &syncSetConfig)
	if err != nil {
		return err
	}

	if syncSetConfig.GroupEmail == "" {
		return fmt.Errorf("GroupEmail missing from sync set json")
	}

	g.GroupSyncSet = syncSetConfig

	return nil
}

func (g *GoogleGroups) ListUsers() ([]personnel_sync.Person, error) {
	var membersList []*admin.Member
	membersListCall := g.AdminService.Members.List(g.GroupSyncSet.GroupEmail)
	err := membersListCall.Pages(context.TODO(), func(members *admin.Members) error {
		membersList = append(membersList, members.Members...)
		return nil
	})
	if err != nil {
		return []personnel_sync.Person{}, fmt.Errorf("unable to get members of group %s: %s", g.GroupSyncSet.GroupEmail, err.Error())
	}

	var members []personnel_sync.Person

	for _, nextMember := range membersList {
		// Do not include ExtraOwners in list to prevent inclusion in delete list
		if isExtraOwner, _ := personnel_sync.InArray(nextMember.Email, g.GroupSyncSet.ExtraOwners); isExtraOwner {
			continue
		}

		members = append(members, personnel_sync.Person{
			CompareValue: nextMember.Email,
			Attributes: map[string]string{
				"Email": strings.ToLower(nextMember.Email),
			},
		})
	}

	return members, nil
}

func (g *GoogleGroups) ApplyChangeSet(
	changes personnel_sync.ChangeSet,
	eventLog chan<- personnel_sync.EventLogItem) personnel_sync.ChangeResults {

	var results personnel_sync.ChangeResults
	var wg sync.WaitGroup

	// key = email, value = role
	toBeCreated := map[string]string{}
	for _, person := range changes.Create {
		toBeCreated[person.CompareValue] = RoleMember
	}

	// Update Owner / Manager roles
	for _, owner := range g.GroupSyncSet.Owners {
		if _, ok := toBeCreated[owner]; ok {
			toBeCreated[owner] = RoleOwner
		}
	}
	for _, manager := range g.GroupSyncSet.Managers {
		if _, ok := toBeCreated[manager]; ok {
			toBeCreated[manager] = RoleManager
		}
	}

	// Add any ExtraOwners to Create list since they are not in the source people
	for _, owner := range g.GroupSyncSet.ExtraOwners {
		toBeCreated[owner] = RoleOwner
	}

	// One minute per batch
	batchTimer := personnel_sync.NewBatchTimer(g.BatchSizePerMinute, int(60))

	for email, role := range toBeCreated {
		wg.Add(1)
		go g.addMember(email, role, &results.Created, &wg, eventLog)
		batchTimer.WaitOnBatch()
	}

	for _, dp := range changes.Delete {
		// Do not delete ExtraOwners
		if isExtraOwner, _ := personnel_sync.InArray(dp.CompareValue, g.GroupSyncSet.ExtraOwners); isExtraOwner {
			continue
		}

		wg.Add(1)
		go g.removeMember(dp.CompareValue, &results.Deleted, &wg, eventLog)
		batchTimer.WaitOnBatch()
	}

	wg.Wait()

	return results
}

func (g *GoogleGroups) addMember(
	email, role string,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- personnel_sync.EventLogItem) {

	defer wg.Done()

	newMember := admin.Member{
		Role:  role,
		Email: email,
	}

	_, err := g.AdminService.Members.Insert(g.GroupSyncSet.GroupEmail, &newMember).Do()
	if err != nil && !strings.Contains(err.Error(), "409") { // error code 409 is for existing user
		eventLog <- personnel_sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("unable to insert %s in Google group %s: %s", email, g.GroupSyncSet.GroupEmail, err.Error())}
		return
	}

	eventLog <- personnel_sync.EventLogItem{
		Event:   "AddMember",
		Message: email,
	}

	atomic.AddUint64(counter, 1)
}

func (g *GoogleGroups) removeMember(
	email string,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- personnel_sync.EventLogItem) {

	defer wg.Done()

	err := g.AdminService.Members.Delete(g.GroupSyncSet.GroupEmail, email).Do()
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("unable to delete %s from Google group %s: %s", email, g.GroupSyncSet.GroupEmail, err.Error())}
		return
	}

	eventLog <- personnel_sync.EventLogItem{
		Event:   "RemoveMember",
		Message: email,
	}

	atomic.AddUint64(counter, 1)
}

// GetGoogleAdminService authenticates with the Google API and returns an admin.Service
//  that has the scopes for Group and GroupMember
//  Authentication requires an email address that matches an actual GMail user (e.g. a machine account)
func (g *GoogleGroups) initGoogleAdminService() error {
	googleAuthJson, err := json.Marshal(g.GoogleGroupsConfig.GoogleAuth)
	if err != nil {
		return fmt.Errorf("unable to marshal google auth data into json, error: %s", err.Error())
	}

	config, err := google.JWTConfigFromJSON(googleAuthJson, admin.AdminDirectoryGroupScope, admin.AdminDirectoryGroupMemberScope)
	if err != nil {
		return fmt.Errorf("unable to parse client secret file to config: %s", err)
	}

	config.Subject = g.GoogleGroupsConfig.DelegatedAdminEmail
	client := config.Client(context.Background())

	adminService, err := admin.New(client)
	if err != nil {
		return fmt.Errorf("unable to retrieve directory Service: %s", err)
	}

	g.AdminService = *adminService

	return nil
}
