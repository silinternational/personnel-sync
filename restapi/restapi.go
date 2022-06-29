package restapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"log/syslog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Jeffail/gabs/v2"

	"github.com/silinternational/personnel-sync/v6/internal"
)

const (
	AuthTypeBasic            = "basic"
	AuthTypeBearer           = "bearer"
	AuthTypeSalesforceOauth  = "SalesforceOauth"
	DefaultBatchSize         = 10
	DefaultBatchDelaySeconds = 3
	PaginationSchemeItems    = "items"
	PaginationSchemePages    = "pages"
)

// NewRestAPISource unmarshals the sourceConfig's ExtraJson into a RestApi struct
func NewRestAPISource(sourceConfig internal.SourceConfig) (internal.Source, error) {
	restAPI := New()

	// Unmarshal ExtraJSON into RestAPI struct
	if err := json.Unmarshal(sourceConfig.ExtraJSON, &restAPI); err != nil {
		return &RestAPI{}, err
	}

	if restAPI.AuthType == AuthTypeSalesforceOauth {
		if token, err := restAPI.getSalesforceOauthToken(); err != nil {
			log.Println(err)
			return &RestAPI{}, err
		} else {
			restAPI.Password = token
		}
	}

	if err := restAPI.validateConfig(); err != nil {
		return &restAPI, fmt.Errorf("invalid configuration: %w", err)
	}
	return &restAPI, nil
}

// NewRestAPIDestination unmarshals the destinationConfig's ExtraJson into a RestApi struct
func NewRestAPIDestination(destinationConfig internal.DestinationConfig) (internal.Destination, error) {
	restAPI := New()

	// Unmarshal ExtraJSON into GoogleGroupsConfig struct
	if err := json.Unmarshal(destinationConfig.ExtraJSON, &restAPI); err != nil {
		return &RestAPI{}, err
	}

	restAPI.destinationConfig = destinationConfig

	if err := restAPI.validateConfig(); err != nil {
		return &restAPI, fmt.Errorf("invalid configuration: %w", err)
	}
	return &restAPI, nil
}

// ForSet sets this RestAPI struct's Path values to those in the umarshalled syncSetJson.
// It ensures the resulting Path attributes include an initial "/"
func (r *RestAPI) ForSet(syncSetJson json.RawMessage) error {
	var setConfig SetConfig
	if err := json.Unmarshal(syncSetJson, &setConfig); err != nil {
		return fmt.Errorf("json unmarshal error on set config: %w", err)
	}

	if len(setConfig.Paths) == 0 {
		return errors.New("paths is empty in sync set")
	}

	for i, p := range setConfig.Paths {
		if p == "" {
			return errors.New("a path in sync set sources is blank")
		}
		if !strings.HasPrefix(p, "/") {
			setConfig.Paths[i] = "/" + p
		}
	}

	if setConfig.UpdatePath == "" {
		r.destinationConfig.DisableUpdate = true
	} else {
		if path, err := parsePathTemplate(setConfig.UpdatePath); err != nil {
			return fmt.Errorf("invalid UpdatePath: %w", err)
		} else {
			setConfig.UpdatePath = path
		}
	}

	if setConfig.DeletePath == "" {
		r.destinationConfig.DisableDelete = true
	} else {
		if path, err := parsePathTemplate(setConfig.DeletePath); err != nil {
			return fmt.Errorf("invalid DeletePath: %w", err)
		} else {
			setConfig.DeletePath = path
		}
	}

	r.setConfig = setConfig
	return nil
}

// ListUsers makes http requests and uses the responses to populate
// and return a slice of Person instances
func (r *RestAPI) ListUsers(desiredAttrs []string) ([]internal.Person, error) {
	errLog := make(chan string, 1000)
	people := make(chan internal.Person, 20000)
	var wg sync.WaitGroup

	attributesToRead := internal.AddStringToSlice(r.IDAttribute, desiredAttrs)
	for _, f := range r.Filters {
		attributesToRead = internal.AddStringToSlice(f.Attribute, attributesToRead)
	}
	for _, p := range r.setConfig.Paths {
		wg.Add(1)
		go r.listUsersForPath(attributesToRead, p, &wg, people, errLog)
	}

	wg.Wait()
	close(people)
	close(errLog)

	if len(errLog) > 0 {
		var errs []string
		for msg := range errLog {
			errs = append(errs, msg)
		}
		return []internal.Person{}, fmt.Errorf("errors listing users from %s: %s", r.BaseURL, strings.Join(errs, ","))
	}

	return r.filterPeople(people)
}

func (r *RestAPI) ApplyChangeSet(changes internal.ChangeSet, eventLog chan<- internal.EventLogItem) internal.ChangeResults {
	var results internal.ChangeResults
	var wg sync.WaitGroup

	batchTimer := internal.NewBatchTimer(r.BatchSize, r.BatchDelaySeconds)

	if r.destinationConfig.DisableAdd {
		log.Println("Contact creation is disabled.")
	} else {
		for _, toCreate := range changes.Create {
			wg.Add(1)
			go r.addPerson(toCreate, &results.Created, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
	}

	if r.destinationConfig.DisableUpdate {
		log.Println("Update is disabled.")
	} else {
		for _, toUpdate := range changes.Update {
			wg.Add(1)
			go r.updatePerson(toUpdate, &results.Updated, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
	}

	if r.destinationConfig.DisableDelete {
		log.Println("Deletion is disabled.")
	} else {
		for _, toUpdate := range changes.Delete {
			wg.Add(1)
			go r.deletePerson(toUpdate, &results.Deleted, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
	}

	wg.Wait()

	return results
}

func (r *RestAPI) listUsersForPath(
	desiredAttrs []string,
	path string,
	wg *sync.WaitGroup,
	people chan<- internal.Person,
	errLog chan<- string,
) {
	defer wg.Done()

	scheme := r.Pagination.Scheme // too long, otherwise

	if scheme == "" {
		apiURL := r.BaseURL + path
		p := r.requestPage(desiredAttrs, apiURL, errLog)
		for _, pp := range p {
			people <- pp
		}
		return
	}

	if scheme != PaginationSchemeItems && scheme != PaginationSchemePages {
		msg := fmt.Sprintf("invalid pagination scheme (%s), must be %s or %s",
			r.Pagination.Scheme, PaginationSchemeItems, PaginationSchemePages)
		log.Println(msg)
		errLog <- msg
		return
	}

	for i := r.Pagination.FirstIndex; i <= r.Pagination.PageLimit; i++ {
		nextIndex := i
		if scheme == PaginationSchemeItems {
			nextIndex = r.Pagination.FirstIndex + i*r.Pagination.PageSize
		}

		apiURL, err := internal.AddParamsToURL(
			internal.JoinUrlPath(r.BaseURL, path),
			[][2]string{
				{r.Pagination.NumberKey, fmt.Sprintf("%d", nextIndex)},
				{r.Pagination.PageSizeKey, fmt.Sprintf("%d", r.Pagination.PageSize)},
			},
		)
		if err != nil {
			log.Println(err)
			errLog <- err.Error()
			return
		}

		p := r.requestPage(desiredAttrs, apiURL, errLog)
		if len(p) == 0 {
			break
		}
		for _, pp := range p {
			people <- pp
		}
	}
}

func (r *RestAPI) requestPage(desiredAttrs []string, url string, errLog chan<- string) []internal.Person {
	client := &http.Client{}
	req, err := http.NewRequest(r.ListMethod, url, nil)
	if err != nil {
		log.Println(err)
		errLog <- err.Error()
	}

	switch r.AuthType {
	case AuthTypeBasic:
		req.SetBasicAuth(r.Username, r.Password)
	case AuthTypeBearer, AuthTypeSalesforceOauth:
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.Password))
	}

	resp, err := client.Do(req)
	if err != nil {
		errLog <- "error issuing http request, " + err.Error()
		return nil
	}

	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errLog <- "error reading response body: " + err.Error()
		return nil
	}

	if resp.StatusCode > 299 {
		msg := fmt.Sprintf("response status code: %d url: %s response body: %s", resp.StatusCode, url, bodyText)
		log.Print(msg)
		errLog <- msg
		return nil
	}

	// by default, json package uses float64 for all numbers -- UseNumber() makes it use the json.Number type
	dec := json.NewDecoder(bytes.NewReader(bodyText))
	dec.UseNumber()
	jsonParsed, err := gabs.ParseJSONDecoder(dec)
	if err != nil {
		log.Printf("error parsing json results: %s", err.Error())
		log.Printf("response body: %s", string(bodyText))
		errLog <- err.Error()
		return nil
	}

	var peopleList []*gabs.Container
	if r.ResultsJSONContainer != "" {
		// Get children records based on ResultsJSONContainer from config
		peopleList = jsonParsed.Path(r.ResultsJSONContainer).Children()
	} else {
		// Root level should contain array of children records
		peopleList = jsonParsed.Children()
	}

	return r.getPersonsFromResults(peopleList, desiredAttrs)
}

func (r *RestAPI) getPersonsFromResults(peopleList []*gabs.Container, desiredAttrs []string) []internal.Person {
	sourcePeople := make([]internal.Person, 0)

	for _, person := range peopleList {
		peep := internal.Person{
			Attributes: map[string]string{},
		}

		for _, sourceKey := range desiredAttrs {
			if !person.ExistsP(sourceKey) {
				continue
			}

			val := person.Path(sourceKey).Data()
			if val == nil {
				continue
			}

			switch v := val.(type) {
			case []interface{}:
				if len(val.([]interface{})) > 0 {
					firstValue := val.([]interface{})[0]
					if firstValue == nil {
						continue
					}

					var ok bool
					if peep.Attributes[sourceKey], ok = firstValue.(string); !ok {
						log.Printf("not a string, sourceKey=%s: %+v, type %T", sourceKey, firstValue, firstValue)
					}
				}
			default:
				peep.Attributes[sourceKey] = fmt.Sprintf("%v", v)
			}
		}

		peep.CompareValue = peep.Attributes[r.CompareAttribute]
		// If person is missing a compare value, do not append them to list
		if peep.CompareValue == "" {
			continue
		}

		peep.ID = peep.Attributes[r.IDAttribute]

		sourcePeople = append(sourcePeople, peep)
	}

	return sourcePeople
}

type SalesforceAuthResponse struct {
	ID          string `json:"id"`
	IssuedAt    string `json:"issued_at"`
	InstanceURL string `json:"instance_url"`
	Signature   string `json:"signature"`
	AccessToken string `json:"access_token"`
}

type SalesforceErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func (r *RestAPI) getSalesforceOauthToken() (string, error) {
	// Body params
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("username", r.Username)
	data.Set("password", r.Password)
	data.Set("client_id", r.ClientID)
	data.Set("client_secret", r.ClientSecret)

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, r.BaseURL, strings.NewReader(data.Encode()))
	if err != nil {
		log.Println(err)
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len(data.Encode())))

	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return "", err
	}

	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("error reading response body: %s", err.Error())
		return "", err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		var errorResponse SalesforceErrorResponse
		err = json.Unmarshal(bodyText, &errorResponse)
		if err != nil {
			log.Printf("Unable to parse error response, status: %v, err: %s. body: %s",
				resp.StatusCode, err.Error(), string(bodyText))
			return "", err
		}
		return "", fmt.Errorf("Salesforce auth error: %s, %s",
			errorResponse.Error, errorResponse.ErrorDescription)
	}

	var authResponse SalesforceAuthResponse
	err = json.Unmarshal(bodyText, &authResponse)
	if err != nil {
		log.Printf("Unable to parse auth response, status: %v, err: %s. body: %s", resp.StatusCode, err.Error(), string(bodyText))
		return "", err
	}

	// Update BaseUrl to instance url
	r.BaseURL = strings.TrimSuffix(authResponse.InstanceURL, "/")

	return authResponse.AccessToken, nil
}

func New() RestAPI {
	return RestAPI{
		ListMethod:           http.MethodGet,
		CreateMethod:         http.MethodPost,
		UpdateMethod:         http.MethodPut,
		DeleteMethod:         http.MethodDelete,
		IDAttribute:          "id",
		ResultsJSONContainer: "",
		UserAgent:            "personnel-sync",
		BatchSize:            DefaultBatchSize,
		BatchDelaySeconds:    DefaultBatchDelaySeconds,
		destinationConfig:    internal.DestinationConfig{},
		Pagination: Pagination{
			Scheme:      "",
			FirstIndex:  1,
			NumberKey:   "page",
			PageLimit:   1000,
			PageSize:    100,
			PageSizeKey: "page_size",
		},
	}
}

func (r *RestAPI) validateConfig() error {
	if r.BatchSize <= 0 {
		r.BatchSize = DefaultBatchSize
	}
	if r.BatchDelaySeconds <= 0 {
		r.BatchDelaySeconds = DefaultBatchDelaySeconds
	}
	return r.Filters.Validate()
}

func (r *RestAPI) addPerson(p internal.Person, n *uint64, wg *sync.WaitGroup, eventLog chan<- internal.EventLogItem) {
	defer wg.Done()

	apiURL := fmt.Sprintf("%s%s", r.BaseURL, r.setConfig.CreatePath)
	headers := map[string]string{"Content-Type": "application/json"}
	reqBody := attributesToJSON(p.Attributes)
	responseBody, err := r.httpRequest(r.CreateMethod, apiURL, reqBody, headers)
	if err != nil {
		eventLog <- internal.EventLogItem{
			Level: syslog.LOG_ERR,
			Message: fmt.Sprintf("addPerson '%s' httpRequest error '%s', url: %s, request: %s, response: %s",
				p.CompareValue, err, apiURL, reqBody, responseBody),
		}
		return
	}

	eventLog <- internal.EventLogItem{
		Level:   syslog.LOG_INFO,
		Message: "AddContact " + p.CompareValue,
	}

	atomic.AddUint64(n, 1)
}

func attributesToJSON(attr map[string]string) string {
	jsonObj := gabs.New()
	for field, value := range attr {
		if _, err := jsonObj.SetP(value, field); err != nil {
			log.Printf("error setting field %s for REST API add, %s", field, err)
		}
	}
	return jsonObj.String()
}

func (r *RestAPI) updatePerson(p internal.Person, n *uint64, wg *sync.WaitGroup, eventLog chan<- internal.EventLogItem) {
	defer wg.Done()

	updatePath := strings.Replace(r.setConfig.UpdatePath, "{id}", p.ID, 1)
	apiURL := fmt.Sprintf("%s%s", r.BaseURL, updatePath)
	headers := map[string]string{"Content-Type": "application/json"}
	reqBody := attributesToJSON(p.Attributes)
	responseBody, err := r.httpRequest(r.UpdateMethod, apiURL, reqBody, headers)
	if err != nil {
		eventLog <- internal.EventLogItem{
			Level: syslog.LOG_ERR,
			Message: fmt.Sprintf("updatePerson '%s' httpRequest error '%s', url: %s, request: %s, response: %s",
				p.CompareValue, err, apiURL, reqBody, responseBody),
		}
		return
	}

	eventLog <- internal.EventLogItem{
		Level:   syslog.LOG_INFO,
		Message: "UpdateContact " + p.CompareValue,
	}

	atomic.AddUint64(n, 1)
}

func (r *RestAPI) deletePerson(p internal.Person, n *uint64, wg *sync.WaitGroup, eventLog chan<- internal.EventLogItem) {
	defer wg.Done()

	deletePath := strings.Replace(r.setConfig.DeletePath, "{id}", p.ID, 1)
	apiURL := fmt.Sprintf("%s%s", r.BaseURL, deletePath)
	headers := map[string]string{"Content-Type": "application/json"}
	responseBody, err := r.httpRequest(r.DeleteMethod, apiURL, "", headers)
	if err != nil {
		eventLog <- internal.EventLogItem{
			Level: syslog.LOG_ERR,
			Message: fmt.Sprintf("deletePerson '%s' httpRequest error '%s', url: %s,  response: %s",
				p.CompareValue, err, apiURL, responseBody),
		}
		return
	}

	eventLog <- internal.EventLogItem{
		Level:   syslog.LOG_INFO,
		Message: "DeleteContact " + p.CompareValue,
	}

	atomic.AddUint64(n, 1)
}

func (r *RestAPI) httpRequest(verb, url, body string, headers map[string]string) (string, error) {
	var req *http.Request
	var err error
	if body == "" {
		req, err = http.NewRequest(verb, url, nil)
	} else {
		req, err = http.NewRequest(verb, url, strings.NewReader(body))
	}
	if err != nil {
		return "", err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", r.UserAgent)

	switch r.AuthType {
	case AuthTypeBasic:
		req.SetBasicAuth(r.Username, r.Password)
	case AuthTypeBearer, AuthTypeSalesforceOauth:
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.Password))
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read http response body: %s", err)
	}
	bodyString := string(bodyBytes)

	if resp.StatusCode >= 400 {
		return bodyString, errors.New(resp.Status)
	}

	return bodyString, nil
}

// parsePathTemplate verifies that the path has a bracketed id, and returns an error if it does not. It also normalizes
// the ID field to "id" and adds a leading slash if necessary.
func parsePathTemplate(pathTemplate string) (string, error) {
	re := regexp.MustCompile("{([a-zA-Z0-9]+)}")
	matches := re.FindStringSubmatch(pathTemplate)
	if len(matches) != 2 {
		return "", fmt.Errorf("path must contain a field bracketed with {}, e.g. /path/{id}")
	}

	path := re.ReplaceAllString(pathTemplate, "{id}")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path, nil
}

func (r *RestAPI) filterPeople(people chan internal.Person) ([]internal.Person, error) {
	var results []internal.Person

	for person := range people {
		if match, err := person.Matches(r.Filters); err != nil {
			return results, fmt.Errorf("filter failure: %w", err)
		} else if match {
			results = append(results, person)
		}
	}

	return results, nil
}
