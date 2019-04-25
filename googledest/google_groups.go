package googledest

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	admin "google.golang.org/api/admin/directory/v1"

	"github.com/silinternational/personnel-sync"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
)

type GoogleGroupsConfig struct {
	DelegatedAdminEmail string
	GroupEmail          string
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
}

func NewGoogleGroupsDesination(destinationConfig personnel_sync.DestinationConfig) (personnel_sync.Destination, error) {
	var googleGroups GoogleGroups
	// Unmarshal ExtraJSON into GoogleGroupsConfig struct
	err := json.Unmarshal(destinationConfig.ExtraJSON, &googleGroups.GoogleGroupsConfig)
	if err != nil {
		return &GoogleGroups{}, err
	}

	// Initialize AdminService object
	err = googleGroups.initGoogleAdminService()
	if err != nil {
		return &GoogleGroups{}, err
	}

	return &googleGroups, nil
}

func (g *GoogleGroups) ListUsers() ([]personnel_sync.Person, error) {
	membersHolder, err := g.AdminService.Members.List(g.GoogleGroupsConfig.GroupEmail).Do()
	if err != nil {
		return []personnel_sync.Person{}, fmt.Errorf("unable to get members of group %s: %s", g.GoogleGroupsConfig.GroupEmail, err.Error())
	}

	membersList := membersHolder.Members
	var members []personnel_sync.Person

	for _, nextMember := range membersList {
		members = append(members, personnel_sync.Person{
			CompareValue: nextMember.Email,
			Attributes: map[string]string{
				"Email": nextMember.Email,
			},
		})
	}

	return members, nil
}

func (g *GoogleGroups) ApplyChangeSet(changes personnel_sync.ChangeSet) personnel_sync.ChangeResults {

	var results personnel_sync.ChangeResults
	var wg sync.WaitGroup
	errLog := make(chan string, 10000)

	for _, cp := range changes.Create {
		wg.Add(1)
		go g.AddMember(cp, &results.Created, &wg, errLog)
	}

	for _, dp := range changes.Delete {
		wg.Add(1)
		go g.RemoveMember(dp, &results.Deleted, &wg, errLog)
	}

	wg.Wait()
	close(errLog)
	for msg := range errLog {
		results.Errors = append(results.Errors, msg)
	}

	return results
}

func (g *GoogleGroups) AddMember(person personnel_sync.Person, counter *uint64, wg *sync.WaitGroup, errLog chan string) {
	defer wg.Done()

	newMember := admin.Member{
		Role:  "MEMBER",
		Email: person.CompareValue,
	}

	_, err := g.AdminService.Members.Insert(g.GoogleGroupsConfig.GroupEmail, &newMember).Do()
	if err != nil {
		errLog <- fmt.Sprintf("unable to insert %s in Google group %s: %s", person.CompareValue, g.GoogleGroupsConfig.GroupEmail, err.Error())
		return
	}

	atomic.AddUint64(counter, 1)
}

func (g *GoogleGroups) RemoveMember(person personnel_sync.Person, counter *uint64, wg *sync.WaitGroup, errLog chan string) {
	defer wg.Done()

	err := g.AdminService.Members.Delete(g.GoogleGroupsConfig.GroupEmail, person.CompareValue).Do()
	if err != nil {
		errLog <- fmt.Sprintf("unable to delete %s from Google group %s: %s", person.CompareValue, g.GoogleGroupsConfig.GroupEmail, err.Error())
		return
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
