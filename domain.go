package personnel_sync

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"reflect"
)

const DefaultConfigFile = "./config.json"
const DestinationTypeGoogleGroups = "GoogleGroups"
const DestinationTypeWebHelpDesk = "WebHelpDesk"
const SourceTypeRestAPI = "RestAPI"

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

	log.Printf("Configuration loaded. Sync sets found:\n")
	for _, syncSet := range config.SyncSets {
		log.Printf("Name: %s, Source Type: %s, Destination Type: %s", syncSet.Name, syncSet.Source.Type, syncSet.Destination.Type)
	}

	return config, nil
}

func RemapToDestinationAttributes(sourcePersons []Person, attributeMap []DestinationAttributeMap) ([]Person, error) {
	var peopleForDestination []Person

	for _, person := range sourcePersons {
		attrs := map[string]string{}
		var attrNames []string

		// Get simple one-dimensional array of destination attribute names
		desiredAttrs := GetDesiredAttributeNames(attributeMap)

		// Iterate through attributes of person and build []PersonAttribute with only attributes wanted in destination
		for name, value := range person.Attributes {
			if ok, _ := InArray(name, desiredAttrs); ok {
				attrs[name] = value
				attrNames = append(attrNames, name)
			}
		}

		// Check if all required attributes are present in results
		if ok, missing := HasAllRequiredAttributes(person.Attributes, attributeMap); !ok {
			jsonAttrs, _ := json.Marshal(attrs)
			log.Printf("user missing attribute %s. Rest of data: %s", missing, jsonAttrs)
			continue
		}

		peopleForDestination = append(peopleForDestination, Person{})

	}

	return []Person{}, nil
}

// HasAllRequiredAttributes checks if a person has all required attributes, if not it will return false and the
// name of the first missing required attribute
func HasAllRequiredAttributes(personAttributes map[string]string, attributeMap []DestinationAttributeMap) (bool, string) {
	// Build simple array of attributes present
	var hasAttrs []string
	for attrName := range personAttributes {
		hasAttrs = append(hasAttrs, attrName)
	}

	// Check if all required attributes are present in results
	requiredAttrs := GetRequiredAttributeNames(attributeMap)
	for _, reqAttr := range requiredAttrs {
		if ok, _ := InArray(reqAttr, hasAttrs); !ok {
			return false, reqAttr
		}
	}

	return true, ""
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

func SyncPeople(source Source, destination Destination, attributeMap []DestinationAttributeMap) ChangeResults {
	sourcePeople, err := source.ListUsers()
	if err != nil {
		return ChangeResults{
			Errors: []string{
				err.Error(),
			},
		}
	}

	// remap source people to destination attributes for comparison
	sourcePeople, err = RemapToDestinationAttributes(sourcePeople, attributeMap)
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

type EmptySource struct{}

func (e *EmptySource) ListUsers() ([]Person, error) {
	return []Person{}, nil
}

// todo
// create interface for destinations
// get destination user list
// calculate change set (create/update/delete)
// apply change set (goroutines?)
