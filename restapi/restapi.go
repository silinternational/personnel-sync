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

	"github.com/Jeffail/gabs"

	"github.com/silinternational/personnel-sync"
)

const AuthTypeBasic = "basic"
const AuthTypeBearer = "bearer"
const AuthTypeSalesforceOauth = "SalesforceOauth"

type RestAPI struct {
	Method               string
	BaseURL              string
	Path                 string
	ResultsJSONContainer string
	AuthType             string
	Username             string
	Password             string
	ClientID             string
	ClientSecret         string
	CompareAttribute     string
}

type SetConfig struct {
	Path string
}

// NewRestAPISource unmarshals the sourceConfig's ExtraJson into a restApi struct
func NewRestAPISource(sourceConfig personnel_sync.SourceConfig) (personnel_sync.Source, error) {
	var restAPI RestAPI
	// Unmarshal ExtraJSON into GoogleGroupsConfig struct
	err := json.Unmarshal(sourceConfig.ExtraJSON, &restAPI)
	if err != nil {
		return &RestAPI{}, err
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

	if setConfig.Path == "" {
		return fmt.Errorf("path is empty in sync set")
	}
	if !strings.HasPrefix(setConfig.Path, "/") {
		setConfig.Path = "/" + setConfig.Path
	}

	r.Path = setConfig.Path

	return nil
}

// ListUsers makes an http request and uses the response to populate
// and return a slice of Person instances
func (r *RestAPI) ListUsers(desiredAttrs []string) ([]personnel_sync.Person, error) {
	client := &http.Client{}
	apiURL := fmt.Sprintf("%s%s", r.BaseURL, r.Path)
	req, err := http.NewRequest(r.Method, apiURL, nil)
	if err != nil {
		log.Println(err)
		return []personnel_sync.Person{}, err
	}

	switch r.AuthType {
	case AuthTypeBasic:
		req.SetBasicAuth(r.Username, r.Password)
	case AuthTypeBearer:
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.Password))
	case AuthTypeSalesforceOauth:
		token, err := r.getSalesforceOauthToken()
		if err != nil {
			return []personnel_sync.Person{}, err
		}
		newApiUrl := fmt.Sprintf("%s%s", r.BaseURL, r.Path)
		req.URL, err = url.Parse(newApiUrl)
		if err != nil {
			return []personnel_sync.Person{}, fmt.Errorf("unable to change api url after Salesforce auth")
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return []personnel_sync.Person{}, err
	}

	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("error reading response body: %s", err.Error())
		return []personnel_sync.Person{}, err
	}

	jsonParsed, err := gabs.ParseJSON(bodyText)
	if err != nil {
		log.Printf("error parsing json results: %s", err.Error())
		log.Printf("response body: %s", bodyText)
		return []personnel_sync.Person{}, err
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
		return []personnel_sync.Person{}, err
	}

	return getPersonsFromResults(peopleList, r.CompareAttribute, desiredAttrs), nil
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

			peep.Attributes[sourceKey] = person.Path(sourceKey).Data().(string)
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
