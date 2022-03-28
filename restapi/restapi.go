package restapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"log/syslog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Jeffail/gabs/v2"

	internal "github.com/silinternational/personnel-sync/v5/internal"
)

const (
	AuthTypeBasic            = "basic"
	AuthTypeBearer           = "bearer"
	AuthTypeSalesforceOauth  = "SalesforceOauth"
	AuthTypeZohoOauth        = "ZohoOauth"
	DefaultBatchSize         = 10
	DefaultBatchDelaySeconds = 3
)

type RestAPI struct {
	Method               string // DEPRECATED
	ListMethod           string
	CreateMethod         string
	BaseURL              string
	AuthURL              string
	QueryURL             string
	ResultsJSONContainer string
	AuthType             string
	Username             string
	Password             string
	ClientID             string
	ClientSecret         string
	CompareAttribute     string
	UserAgent            string
	BatchSize            int
	BatchDelaySeconds    int
	destinationConfig    internal.DestinationConfig
	setConfig            SetConfig
}

type SetConfig struct {
	Paths      []string
	CreatePath string
}

// NewRestAPISource unmarshals the sourceConfig's ExtraJson into a RestApi struct
func NewRestAPISource(sourceConfig internal.SourceConfig) (internal.Source, error) {
	restAPI, err := readConfig(sourceConfig.ExtraJSON)
	return &restAPI, err
}

// NewRestAPIDestination unmarshals the destinationConfig's ExtraJson into a RestApi struct
func NewRestAPIDestination(destinationConfig internal.DestinationConfig) (internal.Destination, error) {
	restAPI, err := readConfig(destinationConfig.ExtraJSON)
	restAPI.destinationConfig = destinationConfig
	return &restAPI, err
}

func readConfig(config []byte) (RestAPI, error) {
	var restAPI RestAPI
	if err := json.Unmarshal(config, &restAPI); err != nil {
		return RestAPI{}, err
	}

	restAPI.setDefaults()

	if restAPI.ClientSecret != "" && restAPI.ClientID != "" {
		token, err := restAPI.getOauthToken()
		if err != nil {
			log.Println(err)
			return RestAPI{}, err
		}
		restAPI.Password = token
	}

	return restAPI, nil
}

// ForSet sets this RestAPI structs Path value to the one in the
// umarshalled syncSetJson.
// It ensures the resulting Path attribute includes an initial "/"
func (r *RestAPI) ForSet(syncSetJson json.RawMessage) error {
	var setConfig SetConfig
	err := json.Unmarshal(syncSetJson, &setConfig)
	if err != nil {
		return err
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

	r.setConfig = setConfig

	return nil
}

// ListUsers makes an http request and uses the response to populate
// and return a slice of Person instances
func (r *RestAPI) ListUsers(desiredAttrs []string) ([]internal.Person, error) {
	errLog := make(chan string, 1000)
	people := make(chan internal.Person, 20000)
	var wg sync.WaitGroup

	for _, p := range r.setConfig.Paths {
		wg.Add(1)
		go r.listUsersForPath(desiredAttrs, p, &wg, people, errLog)
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

	var results []internal.Person

	for person := range people {
		results = append(results, person)
	}

	return results, nil
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
			go r.addContact(toCreate, &results.Created, &wg, eventLog)
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
	errLog chan<- string) {

	defer wg.Done()

	client := &http.Client{}
	apiURL := fmt.Sprintf("%s%s", r.BaseURL, path)
	req, err := http.NewRequest(r.ListMethod, apiURL, nil)
	if err != nil {
		log.Println(err)
		errLog <- err.Error()
	}

	switch r.AuthType {
	case AuthTypeBasic:
		req.SetBasicAuth(r.Username, r.Password)
	default:
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.Password))
	}

	resp, err := client.Do(req)
	if err != nil {
		errLog <- "error issuing http request, " + err.Error()
		return
	}

	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errLog <- "error reading response body: " + err.Error()
		return
	}

	if resp.StatusCode > 299 {
		msg := fmt.Sprintf("response status code: %d response body: %s", resp.StatusCode, bodyText)
		log.Print(msg)
		errLog <- msg
		return
	}

	jsonParsed, err := gabs.ParseJSON(bodyText)
	if err != nil {
		log.Printf("error parsing json results: %s", err.Error())
		log.Printf("response body: %s", string(bodyText))
		errLog <- err.Error()
		return
	}

	var peopleList []*gabs.Container
	if r.ResultsJSONContainer != "" {
		// Get children records based on ResultsJSONContainer from config
		peopleList = jsonParsed.S(r.ResultsJSONContainer).Children()
	} else {
		// Root level should contain array of children records
		peopleList = jsonParsed.Children()
	}

	results := getPersonsFromResults(peopleList, r.CompareAttribute, desiredAttrs)

	for _, person := range results {
		people <- person
	}
}

func getPersonsFromResults(peopleList []*gabs.Container, compareAttr string, desiredAttrs []string) []internal.Person {
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

			if sourceKey == compareAttr {
				peep.CompareValue = person.Path(sourceKey).Data().(string)
			}
		}

		// If person is missing a compare value, do not append them to list
		if peep.CompareValue == "" {
			continue
		}

		sourcePeople = append(sourcePeople, peep)
	}

	return sourcePeople
}

type AuthResponse struct {
	// common properties
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`

	// Salesforce response properties
	ID               string `json:"id"`
	IssuedAt         string `json:"issued_at"`
	InstanceURL      string `json:"instance_url"`
	Signature        string `json:"signature"`
	ErrorDescription string `json:"error_description"`
}

func (r *RestAPI) getOauthToken() (string, error) {
	// Body params
	data := url.Values{}
	data.Set("client_id", r.ClientID)
	data.Set("client_secret", r.ClientSecret)

	switch r.AuthType {
	case AuthTypeSalesforceOauth:
		data.Set("grant_type", "password")
		data.Set("username", r.Username)
		data.Set("password", r.Password)
		r.AuthURL = r.BaseURL // added for backward compatibility
	case AuthTypeZohoOauth:
		data.Set("grant_type", "refresh_token")
		data.Set("refresh_token", r.Password)
	}

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, r.AuthURL, strings.NewReader(data.Encode()))
	if err != nil {
		log.Println(err)
		return "", fmt.Errorf("failed to get OAuth token, %s", err)
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

	var authResponse AuthResponse
	err = json.Unmarshal(bodyText, &authResponse)
	if err != nil {
		log.Printf("Unable to parse auth response, status: %v, err: %s. body: %s", resp.StatusCode, err.Error(), string(bodyText))
		return "", err
	}
	if authResponse.Error != "" {
		return "", fmt.Errorf("auth error: %s, %s\n",
			authResponse.Error, authResponse.ErrorDescription)
	}

	if authResponse.InstanceURL != "" {
		// Update BaseURL to Salesforce instance URL
		r.BaseURL = strings.TrimSuffix(authResponse.InstanceURL, "/")
	} else {
		r.BaseURL = r.QueryURL
	}

	return authResponse.AccessToken, nil
}

func (r *RestAPI) setDefaults() {
	// migrate from `Method` to `ListMethod`
	if r.ListMethod == "" {
		r.ListMethod = r.Method
	}
	// if neither was set, use the default
	if r.ListMethod == "" {
		r.ListMethod = http.MethodGet
	}
	if r.CreateMethod == "" {
		r.CreateMethod = http.MethodPost
	}
	if r.BatchSize <= 0 {
		r.BatchSize = DefaultBatchSize
	}
	if r.BatchDelaySeconds <= 0 {
		r.BatchDelaySeconds = DefaultBatchDelaySeconds
	}
	if r.UserAgent == "" {
		r.UserAgent = "personnel-sync"
	}
}

func (r *RestAPI) addContact(p internal.Person, n *uint64, wg *sync.WaitGroup, eventLog chan<- internal.EventLogItem) {
	defer wg.Done()

	apiURL := fmt.Sprintf("%s%s", r.BaseURL, r.setConfig.CreatePath)
	headers := map[string]string{"Content-Type": "application/json"}
	responseBody, err := r.httpRequest(r.CreateMethod, apiURL, attributesToJSON(p.Attributes), headers)
	if err != nil {
		eventLog <- internal.EventLogItem{
			Level: syslog.LOG_ERR,
			Message: fmt.Sprintf("addContact %s httpRequest error %s, response: %s", p.CompareValue, err,
				responseBody),
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

func (r *RestAPI) updateContact(p internal.Person, n *uint64, wg *sync.WaitGroup, eventLog chan<- internal.EventLogItem) {
	wg.Done()
}

func (r *RestAPI) deleteContact(p internal.Person, n *uint64, wg *sync.WaitGroup, eventLog chan<- internal.EventLogItem) {
	wg.Done()
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
	default:
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
