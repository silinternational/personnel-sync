package personnel_sync

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strings"
)

const DefaultConfigFile = "./config.json"
const DestinationTypeGoogleGroups = "GoogleGroups"
const DestinationTypeWebHelpDesk = "WebHelpDesk"
const SourceTypeRestAPI = "RestAPI"
const CaseSensitive = true
const CaseInsensitive = false

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

	if config.Source.Type == "" {
		log.Printf("configuration appears to be missing a Source configuration")
		return AppConfig{}, err
	}

	if config.Destination.Type == "" {
		log.Printf("configuration appears to be missing a Destination configuration")
		return AppConfig{}, err
	}

	if len(config.AttributeMap) == 0 {
		log.Printf("configuration appears to be missing an AttributeMap")
		return AppConfig{}, err
	}

	log.Printf("Configuration loaded. Source type: %s, Destination type: %s\n", config.Source.Type, config.Destination.Type)
	log.Printf("%v Sync sets found:\n", len(config.SyncSets))
	counter := 1
	for _, syncSet := range config.SyncSets {
		log.Printf("  %v) %s\n", counter, syncSet.Name)
		counter++
	}

	return config, nil
}

func RemapToDestinationAttributes(sourcePersons []Person, attributeMap []AttributeMap) ([]Person, error) {
	var peopleForDestination []Person

	for _, person := range sourcePersons {
		attrs := map[string]string{}

		// Build attrs with only attributes from destination map, disable changes on person missing a required attribute
		disableChanges := false
		for _, attrMap := range attributeMap {
			if value, ok := person.Attributes[attrMap.Source]; ok {
				attrs[attrMap.Destination] = value
			} else if attrMap.Required {
				jsonAttrs, _ := json.Marshal(attrs)
				log.Printf("user missing attribute %s. Rest of data: %s", attrMap.Source, jsonAttrs)
				disableChanges = true
			}
		}

		peopleForDestination = append(peopleForDestination, Person{
			CompareValue:   person.CompareValue,
			Attributes:     attrs,
			DisableChanges: disableChanges,
		})

	}

	return peopleForDestination, nil
}

func IsPersonInList(compareValue string, peopleList []Person) bool {
	for _, person := range peopleList {
		if strings.ToLower(person.CompareValue) == strings.ToLower(compareValue) {
			return true
		}
	}

	return false
}

const PersonIsNotInList = int(0)
const PersonIsInList = int(1)
const PersonIsInListButDifferent = int(2)

func PersonStatusInList(sourcePerson Person, peopleList []Person, attributeMap []AttributeMap) int {
	caseSensitivityList := getCaseSensitivitySourceAttributeList(attributeMap)

	for _, person := range peopleList {
		if stringsAreEqual(person.CompareValue, sourcePerson.CompareValue, CaseInsensitive) {
			for key, val := range sourcePerson.Attributes {
				if !stringsAreEqual(val, person.Attributes[key], caseSensitivityList[key]) {
					log.Printf("Attribute %s not equal for user %s. Case Sensitive: %v, Source: %s, Destination: %s \n",
						key, sourcePerson.CompareValue, caseSensitivityList[key], val, person.Attributes[key])
					return PersonIsInListButDifferent
				}
			}
			return PersonIsInList
		}
	}

	return PersonIsNotInList
}

func stringsAreEqual(val1, val2 string, caseSensitive bool) bool {
	if caseSensitive {
		return val1 == val2
	}

	return strings.ToLower(val1) == strings.ToLower(val2)
}

func getCaseSensitivitySourceAttributeList(attributeMap []AttributeMap) map[string]bool {
	results := map[string]bool{}

	for _, attrMap := range attributeMap {
		results[attrMap.Destination] = attrMap.CaseSensitive
	}

	return results
}

func GenerateChangeSet(sourcePeople, destinationPeople []Person, attributeMap []AttributeMap) ChangeSet {
	var changeSet ChangeSet

	// Find users who need to be created
	for _, sp := range sourcePeople {
		// If user was missing a required attribute, don't change their record
		if sp.DisableChanges {
			continue
		}

		personInDestinationStatus := PersonStatusInList(sp, destinationPeople, attributeMap)
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

func SyncPeople(source Source, destination Destination, attributeMap []AttributeMap, dryRun bool) ChangeResults {
	sourcePeople, err := source.ListUsers()
	if err != nil {
		return ChangeResults{
			Errors: []string{
				err.Error(),
			},
		}
	}
	log.Printf("    Found %v people in source", len(sourcePeople))

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
	log.Printf("    Found %v people in destination", len(destinationPeople))

	changeSet := GenerateChangeSet(sourcePeople, destinationPeople, attributeMap)

	printChangeSet(changeSet)

	// If in DryRun mode only print out ChangeSet plans and return mocked change results based on plans
	if dryRun {
		return ChangeResults{
			Created: uint64(len(changeSet.Create)),
			Updated: uint64(len(changeSet.Update)),
			Deleted: uint64(len(changeSet.Delete)),
		}
	}

	return destination.ApplyChangeSet(changeSet)
}

func printChangeSet(changeSet ChangeSet) {
	log.Printf("ChangeSet Plans: Create %v, Update %v, Delete %v\n", len(changeSet.Create), len(changeSet.Update), len(changeSet.Delete))

	log.Println("Users to be created...")
	c := 1
	for _, user := range changeSet.Create {
		log.Printf("  %v) %s", c, user.CompareValue)
		c++
	}

	log.Println("Users to be updated...")
	u := 1
	for _, user := range changeSet.Update {
		log.Printf("  %v) %s", u, user.CompareValue)
		u++
	}

	log.Println("Users to be deleted...")
	d := 1
	for _, user := range changeSet.Delete {
		log.Printf("  %v) %s", d, user.CompareValue)
		d++
	}
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

func (e *EmptyDestination) ForSet(syncSetJson json.RawMessage) error {
	return nil
}

func (e *EmptyDestination) ListUsers() ([]Person, error) {
	return []Person{}, nil
}

func (e *EmptyDestination) ApplyChangeSet(changes ChangeSet) ChangeResults {
	return ChangeResults{}
}

type EmptySource struct{}

func (e *EmptySource) ForSet(syncSetJson json.RawMessage) error {
	return nil
}

func (e *EmptySource) ListUsers() ([]Person, error) {
	return []Person{}, nil
}
