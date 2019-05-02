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

func NewRestAPISource(sourceConfig personnel_sync.SourceConfig) (personnel_sync.Source, error) {
	var restAPI RestAPI
	// Unmarshal ExtraJSON into GoogleGroupsConfig struct
	err := json.Unmarshal(sourceConfig.ExtraJSON, &restAPI)
	if err != nil {
		return &RestAPI{}, err
	}

	return &restAPI, nil
}

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

		// Get map of attribute name to gabs container
		personAttributes, err := person.ChildrenMap()
		if err != nil {
			log.Println(err)
			return []personnel_sync.Person{}, err
		}

		// Convert map of attributes to simple map of string to string
		attrs := map[string]string{}
		for key, value := range personAttributes {
			attrs[key] = value.Data().(string)
		}

		compareValue, ok := personAttributes[r.CompareAttribute]
		if !ok {
			msg := fmt.Sprintf("ListUsers failed, user missing CompareValue (%s), have attributes: %s",
				r.CompareAttribute, personAttributes)

			log.Println(msg)

			return []personnel_sync.Person{}, fmt.Errorf(msg)
		} else {
			// Append person to sourcePeople array to be returned from function
			sourcePeople = append(sourcePeople, personnel_sync.Person{
				CompareValue: compareValue.Data().(string),
				Attributes:   attrs,
			})
		}

	}

	return sourcePeople, nil
}
