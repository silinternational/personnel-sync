package restapi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/Jeffail/gabs"

	"github.com/silinternational/personnel-sync"
)

const AuthTypeBasic = "basic"
const AuthTypeBearer = "bearer"

type RestAPI struct {
	Method               string
	BaseURL              string
	Path                 string
	ResultsJSONContainer string
	AuthType             string
	Username             string
	Password             string
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
func (r *RestAPI) ListUsers() ([]personnel_sync.Person, error) {
	client := &http.Client{}
	url := fmt.Sprintf("%s%s", r.BaseURL, r.Path)
	req, err := http.NewRequest(r.Method, url, nil)
	if err != nil {
		log.Println(err)
		return []personnel_sync.Person{}, err
	}

	if r.AuthType == AuthTypeBasic {
		req.SetBasicAuth(r.Username, r.Password)
	} else if r.AuthType == AuthTypeBearer {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer: %s", r.Password))
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return []personnel_sync.Person{}, err
	}

	bodyText, err := ioutil.ReadAll(resp.Body)
	jsonParsed, err := gabs.ParseJSON(bodyText)
	if err != nil {
		log.Printf("error parsing json results: %s", err.Error())
		return []personnel_sync.Person{}, err
	}

	// sourcePeople will hold array of Person(s) from source API
	var sourcePeople []personnel_sync.Person

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

	// Iterate through people in list from source to convert to Persons
	for _, person := range peopleList {
		sourcePerson, err := r.getPersonFromContainer(person)
		if err != nil {
			log.Println(err)
			return []personnel_sync.Person{}, err
		}

		sourcePeople = append(sourcePeople, sourcePerson)
	}

	return sourcePeople, nil
}

func (r *RestAPI) getPersonFromContainer(personContainer *gabs.Container) (personnel_sync.Person, error) {
	// Get map of attribute name to gabs container
	personAttributes, err := personContainer.ChildrenMap()
	if err != nil {
		return personnel_sync.Person{}, err
	}

	// Convert map of attributes to simple map of string to string
	attrs := map[string]string{}
	for key, value := range personAttributes {
		attrVal := value.Data().(string)
		if strings.ToLower(key) == strings.ToLower(r.CompareAttribute) {
			attrVal = strings.ToLower(attrVal)
		}
		attrs[key] = attrVal
	}

	compareValue, ok := personAttributes[r.CompareAttribute]
	if !ok {
		msg := fmt.Sprintf("ListUsers failed, user missing CompareValue (%s), have attributes: %s",
			r.CompareAttribute, personAttributes)

		return personnel_sync.Person{}, fmt.Errorf(msg)
	}

	sourcePerson :=  personnel_sync.Person{
		CompareValue: strings.ToLower(compareValue.Data().(string)),
		Attributes:   attrs,
	}

	return sourcePerson, nil
}