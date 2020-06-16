package restapi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/Jeffail/gabs"

	personnel_sync "github.com/silinternational/personnel-sync/v3"
)

const AuthTypeBasic = "basic"
const AuthTypeBearer = "bearer"
const AuthTypeSalesforceOauth = "SalesforceOauth"

type RestAPI struct {
	Method               string
	BaseURL              string
	Paths                []string
	ResultsJSONContainer string
	AuthType             string
	Username             string
	Password             string
	ClientID             string
	ClientSecret         string
	CompareAttribute     string
}

type SetConfig struct {
	Paths []string
}

// NewRestAPISource unmarshals the sourceConfig's ExtraJson into a restApi struct
func NewRestAPISource(sourceConfig personnel_sync.SourceConfig) (personnel_sync.Source, error) {
	var restAPI RestAPI
	// Unmarshal ExtraJSON into GoogleGroupsConfig struct
	err := json.Unmarshal(sourceConfig.ExtraJSON, &restAPI)
	if err != nil {
		return &RestAPI{}, err
	}

	if restAPI.AuthType == AuthTypeSalesforceOauth {
		token, err := restAPI.getSalesforceOauthToken()
		if err != nil {
			log.Println(err)
			return &RestAPI{}, err
		}

		restAPI.Password = token
	}

	return &restAPI, nil
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
		return fmt.Errorf("paths is empty in sync set")
	}

	for i, p := range setConfig.Paths {
		if p == "" {
			return fmt.Errorf("a path in sync set sources is blank")
		}
		if !strings.HasPrefix(p, "/") {
			setConfig.Paths[i] = "/" + p
		}
	}

	r.Paths = setConfig.Paths

	return nil
}

func (r *RestAPI) ListUsersInSource(desiredAttrs []string) ([]personnel_sync.Person, error) {
	errLog := make(chan string, 1000)
	people := make(chan personnel_sync.Person, 20000)
	var wg sync.WaitGroup

	for _, p := range r.Paths {
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
		return []personnel_sync.Person{}, fmt.Errorf("errors listing users: %s", strings.Join(errs, ","))
	}

	var results []personnel_sync.Person

	for person := range people {
		results = append(results, person)
	}

	return results, nil
}

// ListUsersInSource makes an http request and uses the response to populate
// and return a slice of Person instances
func (r *RestAPI) listUsersForPath(
	desiredAttrs []string,
	path string,
	wg *sync.WaitGroup,
	people chan<- personnel_sync.Person,
	errLog chan<- string) {

	defer wg.Done()

	client := &http.Client{}
	apiURL := fmt.Sprintf("%s%s", r.BaseURL, path)
	req, err := http.NewRequest(r.Method, apiURL, nil)
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
		log.Println(err)
		log.Println(err)
		errLog <- err.Error()
	}

	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("error reading response body: %s", err.Error())
		errLog <- err.Error()
	}

	jsonParsed, err := gabs.ParseJSON(bodyText)
	if err != nil {
		log.Printf("error parsing json results: %s", err.Error())
		log.Printf("response body: %s", string(bodyText))
		errLog <- err.Error()
	}

	var peopleList []*gabs.Container
	if r.ResultsJSONContainer != "" {
		// Get children records based on ResultsJSONContainer from config
		peopleList, err = jsonParsed.S(r.ResultsJSONContainer).Children()
	} else {
		// Root level should contain array of children records
		peopleList, err = jsonParsed.Children()
	}

	if err != nil {
		log.Printf("error getting results children: %s\n", err.Error())
		errLog <- err.Error()
	}

	results := getPersonsFromResults(peopleList, r.CompareAttribute, desiredAttrs)

	for _, person := range results {
		people <- person
	}
}

func getPersonsFromResults(peopleList []*gabs.Container, compareAttr string, desiredAttrs []string) []personnel_sync.Person {
	var sourcePeople []personnel_sync.Person

	for _, person := range peopleList {
		peep := personnel_sync.Person{
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

			switch val.(type) {
			case string:
				peep.Attributes[sourceKey] = val.(string)
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
				log.Printf("unsupported data type, sourceKey=%s, type %T", sourceKey, val)
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

type SalesforceAuthResponse struct {
	ID          string `json:"id"`
	IssuedAt    string `json:"issued_at"`
	InstanceURL string `json:"instance_url"`
	Signature   string `json:"signature"`
	AccessToken string `json:"access_token"`
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
