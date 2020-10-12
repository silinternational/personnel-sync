package webhelpdesk

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log/syslog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	personnel_sync "github.com/silinternational/personnel-sync/v5"
)

const DefaultBatchSize = 50
const DefaultBatchDelaySeconds = 60
const DefaultListClientsPageLimit = 100
const ClientsAPIPath = "/ra/Clients"

// In WebHelpDesk the basic user is called a "Client", so this is not an API Client
type User struct {
	ID        int    `json:"id,omitempty"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Username  string `json:"username"`
}

type WebHelpDesk struct {
	URL                  string
	Username             string
	Password             string
	ListClientsPageLimit int
	BatchSize            int
	BatchDelaySeconds    int
}

func NewWebHelpDeskDestination(destinationConfig personnel_sync.DestinationConfig) (personnel_sync.Destination, error) {
	var webHelpDesk WebHelpDesk

	err := json.Unmarshal(destinationConfig.ExtraJSON, &webHelpDesk)
	if err != nil {
		return &webHelpDesk, err
	}

	// Set defaults for batch size per minute and page limit if not provided in ExtraJSON
	if webHelpDesk.BatchSize <= 0 {
		webHelpDesk.BatchSize = DefaultBatchSize
	}
	if webHelpDesk.BatchDelaySeconds <= 0 {
		webHelpDesk.BatchDelaySeconds = DefaultBatchDelaySeconds
	}

	if webHelpDesk.ListClientsPageLimit == 0 {
		webHelpDesk.ListClientsPageLimit = DefaultListClientsPageLimit
	}

	return &webHelpDesk, nil
}

func (w *WebHelpDesk) ForSet(syncSetJson json.RawMessage) error {
	// unused in WebHelpDesk
	return nil
}

func (w *WebHelpDesk) ListUsers(desiredAttrs []string) ([]personnel_sync.Person, error) {
	var allClients []User
	page := 1

	for {
		additionalParams := map[string]string{
			"limit": fmt.Sprintf("%v", w.ListClientsPageLimit),
			"page":  fmt.Sprintf("%v", page),
		}

		listUsersResp, err := w.makeHttpRequest(ClientsAPIPath, http.MethodGet, "", additionalParams)
		if err != nil {
			return []personnel_sync.Person{}, err
		}

		var whdClients []User
		err = json.Unmarshal(listUsersResp, &whdClients)
		if err != nil {
			return []personnel_sync.Person{}, err
		}

		// Append the new users to the master list of users
		allClients = append(allClients, whdClients...)

		// If this batch of users is fewer than the normal number returned per page, we're done
		if len(whdClients) < w.ListClientsPageLimit {
			break
		}

		page++
	}

	var users []personnel_sync.Person
	for _, nextClient := range allClients {
		users = append(users, personnel_sync.Person{
			CompareValue: nextClient.Username,
			Attributes: map[string]string{
				"id":        strconv.Itoa(nextClient.ID),
				"email":     nextClient.Email,
				"firstName": nextClient.FirstName,
				"lastName":  nextClient.LastName,
				"username":  nextClient.Username,
			},
		})
	}

	return users, nil
}

func (w *WebHelpDesk) ApplyChangeSet(
	changes personnel_sync.ChangeSet,
	eventLog chan<- personnel_sync.EventLogItem) personnel_sync.ChangeResults {

	var results personnel_sync.ChangeResults
	var wg sync.WaitGroup

	// One minute per batch
	batchTimer := personnel_sync.NewBatchTimer(w.BatchSize, w.BatchDelaySeconds)

	for _, cp := range changes.Create {
		wg.Add(1)
		go w.CreateUser(cp, &results.Created, &wg, eventLog)
		batchTimer.WaitOnBatch()
	}

	for _, dp := range changes.Update {
		wg.Add(1)
		go w.UpdateUser(dp, &results.Updated, &wg, eventLog)
		batchTimer.WaitOnBatch()
	}

	// WHD API does not support deactivating or deleting users

	wg.Wait()

	return results
}

func (w *WebHelpDesk) CreateUser(
	person personnel_sync.Person,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- personnel_sync.EventLogItem) {

	defer wg.Done()

	newClient, err := getWebHelpDeskClientFromPerson(person)
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("unable to create user, unable to convert string to int, error: %s", err.Error())}
		return
	}

	jsonBody, err := json.Marshal(newClient)
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("unable to create user, unable to marshal json, error: %s", err.Error())}
		return
	}

	_, err = w.makeHttpRequest(ClientsAPIPath, http.MethodPost, string(jsonBody), map[string]string{})
	if err != nil {
		// Since WebHelpDesk APIs are garbage, just ignore errors, but don't count as a newly created user
		eventLog <- personnel_sync.EventLogItem{
			Level: syslog.LOG_ERR,
			Message: fmt.Sprintf("unable to create user (person=%v, client=%v), error calling api: %s",
				person, newClient, err.Error())}
		return
	}

	eventLog <- personnel_sync.EventLogItem{
		Level:   syslog.LOG_INFO,
		Message: "CreateUser " + person.CompareValue,
	}

	atomic.AddUint64(counter, 1)
}

func (w *WebHelpDesk) UpdateUser(
	person personnel_sync.Person,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- personnel_sync.EventLogItem) {

	defer wg.Done()

	newClient, err := getWebHelpDeskClientFromPerson(person)
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("unable to update user, unable to convert string to int, error: %s", err.Error())}
		return
	}

	jsonBody, err := json.Marshal(newClient)
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("unable to update user, unable to marshal json, error: %s", err.Error())}
		return
	}

	updatePath := fmt.Sprintf("%s/%v", ClientsAPIPath, newClient.ID)
	_, err = w.makeHttpRequest(updatePath, http.MethodPut, string(jsonBody), map[string]string{})
	if err != nil {
		// Since WebHelpDesk APIs are garbage, just ignore errors, but don't count as a newly created user
		eventLog <- personnel_sync.EventLogItem{
			Level: syslog.LOG_ERR,
			Message: fmt.Sprintf("unable to update user (person=%+v, client=%+v), error calling api, error: %s",
				person, newClient, err.Error())}
		return
	}

	eventLog <- personnel_sync.EventLogItem{
		Level:   syslog.LOG_INFO,
		Message: "UpdateUser " + person.CompareValue,
	}

	atomic.AddUint64(counter, 1)
}

func (w *WebHelpDesk) makeHttpRequest(path, method, body string, additionalQueryParams map[string]string) ([]byte, error) {
	// Create client and request
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{Transport: tr}
	req, err := http.NewRequest(method, w.URL+path, strings.NewReader(body))
	if err != nil {
		return []byte{}, err
	}

	// Add authentication query string parameters
	q := req.URL.Query()
	q.Add("username", w.Username)
	q.Add("apiKey", w.Password)
	for key, value := range additionalQueryParams {
		q.Add(key, value)
	}
	req.URL.RawQuery = q.Encode()

	// do request
	resp, err := client.Do(req)
	if err != nil {
		return []byte{}, err
	}

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 204 {
		return []byte{}, fmt.Errorf("error returned from API. status: %v, body: %s", resp.StatusCode, responseBody)
	}

	return responseBody, nil
}

func getWebHelpDeskClientFromPerson(person personnel_sync.Person) (User, error) {
	newClient := User{
		FirstName: person.Attributes["firstName"],
		LastName:  person.Attributes["lastName"],
		Username:  person.Attributes["username"],
		Email:     person.Attributes["email"],
	}

	// if id attribute isn't present, default to a zero
	if person.ID != "" {
		intId, err := strconv.Atoi(person.ID)
		if err != nil {
			return User{}, err
		}
		newClient.ID = intId
	}

	return newClient, nil
}
