package personnel_sync

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"

	"github.com/Jeffail/gabs"
)

const DefaultConfigFile = "./config.json"
const DestinationTypeGoogleGroups = "GoogleGroups"
const DestinationTypeWebHelpDesk = "WebHelpDesk"

func LoadConfig(configFile string) (AppConfig, error) {

	if configFile == "" {
		configFile = os.Getenv("CONFIG_PATH")
		if configFile == "" {
			configFile = DefaultConfigFile
		}
	}

	log.Printf("Using config file: %s\n", configFile)

	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Printf("unable to application config file %s, error: %s\n", configFile, err.Error())
		return AppConfig{}, err
	}

	config := AppConfig{}
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Printf("unable to unmarshal application configuration file data, error: %s\n", err.Error())
		return AppConfig{}, err
	}

	log.Printf("Configuration loaded. Source URL: %s, Destination type: %s", config.Source.URL, config.Destination.Type)

	return config, nil
}

func GetPersonsFromSource(config AppConfig) ([]Person, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", config.Source.URL, nil)
	if err != nil {
		log.Println(err)
		return []Person{}, err
	}
	req.SetBasicAuth(config.Source.Username, config.Source.Password)
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return []Person{}, err
	}

	bodyText, err := ioutil.ReadAll(resp.Body)
	jsonParsed, err := gabs.ParseJSON(bodyText)
	if err != nil {
		log.Println(err)
		return []Person{}, err
	}

	// sourcePeople will hold array of Person(s) from source API
	var sourcePeople []Person

	// Get children records based on ResultsJSONContainer from config
	peopleList, err := jsonParsed.S(config.Source.ResultsJSONContainer).Children()
	if err != nil {
		log.Println(err)
		return []Person{}, err
	}

	// Iterate through people in list from source to convert to Persons
	for _, person := range peopleList {

		personAttributes, err := person.ChildrenMap()
		if err != nil {
			log.Println(err)
			return []Person{}, err
		}

		attrs, err := FilterMappedAttributes(personAttributes, config.DestinationAttributeMap)
		if err != nil {
			log.Println(err)
			if config.Runtime.FailIfSinglePersonMissingRequiredAttribute {
				return []Person{}, err
			}
		} else {
			compareValue, ok := attrs[config.Source.CompareAttribute]
			if !ok {
				log.Printf("person id attribute (%s) not found, have attributes: %s\n", config.Source.IDAttribute, attrs)
				if config.Runtime.FailIfSinglePersonMissingRequiredAttribute {
					return []Person{}, err
				}
			} else {
				// Append person to sourcePeople array to be returned from function
				sourcePeople = append(sourcePeople, Person{
					CompareValue: compareValue,
					Attributes:   attrs,
				})
			}
		}
	}

	return sourcePeople, nil
}

func FilterMappedAttributes(personAttributes map[string]*gabs.Container, attributeMap []DestinationAttributeMap) (map[string]string, error) {
	attrs := map[string]string{}
	var attrNames []string

	// Get simple one-dimensional array of destination attribute names
	desiredAttrs := GetDesiredAttributeNames(attributeMap)

	// Iterate through attributes of person and build []PersonAttribute with only attributes wanted in destination
	for name, value := range personAttributes {
		if ok, _ := InArray(name, desiredAttrs); ok {
			attrs[name] = value.Data().(string)
			attrNames = append(attrNames, name)
		}
	}

	// Check if all required attributes are present in results
	requiredAttrs := GetRequiredAttributeNames(attributeMap)
	for _, reqAttr := range requiredAttrs {
		if ok, _ := InArray(reqAttr, attrNames); !ok {
			jsonAttrs, _ := json.Marshal(attrs)
			return map[string]string{}, fmt.Errorf("user missing attribute %s. Rest of data: %s", reqAttr, jsonAttrs)
		}
	}

	return attrs, nil
}

func GetDesiredAttributeNames(attributeMap []DestinationAttributeMap) []string {
	var attrs []string

	for _, attr := range attributeMap {
		attrs = append(attrs, attr.SourceName)
	}

	return attrs
}

func GetRequiredAttributeNames(attributeMap []DestinationAttributeMap) []string {
	var attrs []string

	for _, attr := range attributeMap {
		if attr.Required {
			attrs = append(attrs, attr.SourceName)
		}
	}

	return attrs
}

func GetSourceAttrNameForDestinationAttr(attributeMap []DestinationAttributeMap, destinationAttrName string) string {
	for _, attr := range attributeMap {
		if attr.DestinationName == destinationAttrName {
			return attr.SourceName
		}
	}

	return destinationAttrName
}

func GetDestinationAttrNameForSourceAttr(attributeMap []DestinationAttributeMap, sourceAttrName string) string {
	for _, attr := range attributeMap {
		if attr.SourceName == sourceAttrName {
			return attr.DestinationName
		}
	}

	return sourceAttrName
}

func IsPersonInList(compareValue string, peopleList []Person) bool {
	for _, person := range peopleList {
		if person.CompareValue == compareValue {
			return true
		}
	}

	return false
}

const PersonIsNotInList = int(0)
const PersonIsInList = int(1)
const PersonIsInListButDifferent = int(2)

func PersonStatusInList(compareValue string, attrs map[string]string, peopleList []Person) int {
	for _, person := range peopleList {
		if person.CompareValue == compareValue {
			if !reflect.DeepEqual(attrs, person.Attributes) {
				return PersonIsInListButDifferent
			}
			return PersonIsInList
		}
	}

	return PersonIsNotInList
}

func GenerateChangeSet(sourcePeople, destinationPeople []Person) ChangeSet {
	var changeSet ChangeSet

	// Find users who need to be created
	for _, sp := range sourcePeople {
		personInDestinationStatus := PersonStatusInList(sp.CompareValue, sp.Attributes, destinationPeople)
		switch personInDestinationStatus {
		case PersonIsNotInList:
			changeSet.Create = append(changeSet.Create, sp)
		case PersonIsInListButDifferent:
			changeSet.Update = append(changeSet.Update, sp)
		}
	}

	// Find users who need to be deleted
	for _, dp := range destinationPeople {
		if !IsPersonInList(dp.CompareValue, sourcePeople) {
			changeSet.Delete = append(changeSet.Delete, dp)
		}
	}

	return changeSet
}

func SyncPeople(config AppConfig, destination Destination) ChangeResults {
	sourcePeople, err := GetPersonsFromSource(config)
	if err != nil {
		return ChangeResults{
			Errors: []string{
				err.Error(),
			},
		}
	}

	destinationPeople, err := destination.ListUsers()
	if err != nil {
		return ChangeResults{
			Errors: []string{
				err.Error(),
			},
		}
	}

	changeSet := GenerateChangeSet(sourcePeople, destinationPeople)

	return destination.ApplyChangeSet(changeSet)
}

// This function will search element inside array with any type.
// Will return boolean and index for matched element.
// True and index more than 0 if element is exist.
// needle is element to search, haystack is slice of value to be search.
func InArray(needle interface{}, haystack interface{}) (exists bool, index int) {
	exists = false
	index = -1

	switch reflect.TypeOf(haystack).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(haystack)

		for i := 0; i < s.Len(); i++ {
			if reflect.DeepEqual(needle, s.Index(i).Interface()) == true {
				index = i
				exists = true
				return
			}
		}
	}

	return
}

type EmptyDestination struct{}

func (e *EmptyDestination) ListUsers() ([]Person, error) {
	return []Person{}, nil
}

func (e *EmptyDestination) ApplyChangeSet(changes ChangeSet) ChangeResults {
	return ChangeResults{}
}

// todo
// create interface for destinations
// get destination user list
// calculate change set (create/update/delete)
// apply change set (goroutines?)
