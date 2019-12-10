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

const DefaultBatchSize = 10
const DefaultBatchDelay = 3
const RoleMember = "MEMBER"
const RoleOwner = "OWNER"
const RoleManager = "MANAGER"

type GoogleGroupsConfig struct {
	DelegatedAdminEmail string
	GoogleAuth          GoogleAuth
}

type GoogleGroups struct {
	DestinationConfig  personnel_sync.DestinationConfig
	GoogleGroupsConfig GoogleGroupsConfig
	AdminService       admin.Service
	GroupSyncSet       GroupSyncSet
	BatchSize          int
	BatchDelay         int
}

type GroupSyncSet struct {
	GroupEmail    string
	Owners        []string
	ExtraOwners   []string
	Managers      []string
	ExtraManagers []string
	ExtraMembers  []string
	DisableAdd    bool
	DisableUpdate bool
	DisableDelete bool
}

func NewGoogleGroupsDestination(destinationConfig personnel_sync.DestinationConfig) (personnel_sync.Destination, error) {
	var googleGroups GoogleGroups
	// Unmarshal ExtraJSON into GoogleGroupsConfig struct
	err := json.Unmarshal(destinationConfig.ExtraJSON, &googleGroups.GoogleGroupsConfig)
	if err != nil {
		return &GoogleGroups{}, err
	}

	// Defaults
	if googleGroups.BatchSize <= 0 {
		googleGroups.BatchSize = DefaultBatchSize
	}
	if googleGroups.BatchDelay <= 0 {
		googleGroups.BatchDelay = DefaultBatchDelay
	}

	// Initialize AdminService object
	googleGroups.AdminService, err = initGoogleAdminService(
		googleGroups.GoogleGroupsConfig.GoogleAuth,
		googleGroups.GoogleGroupsConfig.DelegatedAdminEmail,
		admin.AdminDirectoryGroupScope,
		admin.AdminDirectoryGroupMemberScope,
	)
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
		// Do not include Extra Managers or ExtraOwners in list to prevent inclusion in delete list
		if isExtraManager, _ := personnel_sync.InArray(nextMember.Email, g.GroupSyncSet.ExtraManagers); isExtraManager {
			continue
		}
		if isExtraOwner, _ := personnel_sync.InArray(nextMember.Email, g.GroupSyncSet.ExtraOwners); isExtraOwner {
			continue
		}
		if isExtraMember, _ := personnel_sync.InArray(nextMember.Email, g.GroupSyncSet.ExtraMembers); isExtraMember {
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

	// Add any ExtraManagers, ExtraOwners, and ExtraMembers to Create list since they are not in the source people
	for _, manager := range g.GroupSyncSet.ExtraManagers {
		toBeCreated[manager] = RoleManager
	}
	for _, owner := range g.GroupSyncSet.ExtraOwners {
		toBeCreated[owner] = RoleOwner
	}
	for _, member := range g.GroupSyncSet.ExtraMembers {
		toBeCreated[member] = RoleMember
	}

	// One minute per batch
	batchTimer := personnel_sync.NewBatchTimer(g.BatchSize, g.BatchDelay)

	if !g.GroupSyncSet.DisableAdd {
		for email, role := range toBeCreated {
			wg.Add(1)
			go g.addMember(email, role, &results.Created, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
	}

	if !g.GroupSyncSet.DisableDelete {
		for _, dp := range changes.Delete {
			// Do not delete ExtraManagers, ExtraOwners, or ExtraMembers
			if isExtraManager, _ := personnel_sync.InArray(dp.CompareValue, g.GroupSyncSet.ExtraManagers); isExtraManager {
				continue
			}
			if isExtraOwner, _ := personnel_sync.InArray(dp.CompareValue, g.GroupSyncSet.ExtraOwners); isExtraOwner {
				continue
			}
			if isExtraMember, _ := personnel_sync.InArray(dp.CompareValue, g.GroupSyncSet.ExtraMembers); isExtraMember {
				continue
			}
			wg.Add(1)
			go g.removeMember(dp.CompareValue, &results.Deleted, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
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
