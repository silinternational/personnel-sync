package personnel_sync

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strings"
	"time"
)

const DefaultConfigFile = "./config.json"
const DestinationTypeGoogleGroups = "GoogleGroups"
const DestinationTypeGoogleContacts = "GoogleContacts"
const DestinationTypeWebHelpDesk = "WebHelpDesk"
const SourceTypeRestAPI = "RestAPI"

// LoadConfig looks for a config file if one is provided. Otherwise, it looks for
// a config file based on the CONFIG_PATH env var.  If that is not set, it gets
// the default config file ("./config.json").
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

	for i, syncSet := range config.SyncSets {
		log.Printf("  %v) %s\n", i+1, syncSet.Name)
	}

	return config, nil
}

// RemapToDestinationAttributes returns a slice of Person instances that each have
// only the desired attributes based on the destination attribute keys.
// If a required attribute is missing for a Person, then their disableChanges
// value is set to true.
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

// getPersonFromList returns the person if found in peopleList otherwise an empty Person{}
func getPersonFromList(compareValue string, peopleList []Person) Person {
	lowerCompareValue := strings.ToLower(compareValue)

	for _, person := range peopleList {
		if strings.ToLower(person.CompareValue) == lowerCompareValue {
			return person
		}
	}

	return Person{}
}

func personAttributesAreEqual(sp, dp Person, attributeMap []AttributeMap) bool {
	caseSensitivityList := getCaseSensitivitySourceAttributeList(attributeMap)
	for key, val := range sp.Attributes {
		if !stringsAreEqual(val, dp.Attributes[key], caseSensitivityList[key]) {
			log.Printf("Attribute %s not equal for user %s. Case Sensitive: %v, Source: %s, Destination: %s \n",
				key, sp.CompareValue, caseSensitivityList[key], val, dp.Attributes[key])
			return false
		}
	}

	return true
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

// GenerateChangeSet builds the three slice attributes of a ChangeSet
// (Create, Update and Delete) based on whether they are in the slice
//  of destination Person instances.
// It skips all source Person instances that have DisableChanges set to true
func GenerateChangeSet(sourcePeople, destinationPeople []Person, attributeMap []AttributeMap, idField string) ChangeSet {
	var changeSet ChangeSet

	// Find users who need to be created or updated
	for _, sp := range sourcePeople {
		// If user was missing a required attribute, don't change their record
		if sp.DisableChanges {
			continue
		}

		destinationPerson := getPersonFromList(sp.CompareValue, destinationPeople)
		if destinationPerson.CompareValue == "" {
			changeSet.Create = append(changeSet.Create, sp)
			continue
		}

		if !personAttributesAreEqual(sp, destinationPerson, attributeMap) {
			sp.ID = destinationPerson.ID
			changeSet.Update = append(changeSet.Update, sp)
			continue
		}
	}

	// Find users who need to be deleted
	for _, dp := range destinationPeople {
		sourcePerson := getPersonFromList(dp.CompareValue, sourcePeople)
		if sourcePerson.CompareValue == "" {
			changeSet.Delete = append(changeSet.Delete, dp)
		}
	}

	return changeSet
}

// SyncPeople calls a number of functions to do the following ...
//  - it gets the list of people from the source
//  - it remaps their attributes to match the keys used in the destination
//  - it gets the list of people from the destination
//  - it generates the lists of people to change, update and delete
//  - if dryRun is true, it prints those lists, but otherwise makes the associated changes
func SyncPeople(source Source, destination Destination, attributeMap []AttributeMap, dryRun bool) ChangeResults {
	desiredAttrs := GetDesiredAttributes(attributeMap)
	sourcePeople, err := source.ListUsers(desiredAttrs)
	if err != nil {
		return ChangeResults{
			Errors: []string{err.Error()},
		}
	}
	log.Printf("    Found %v people in source", len(sourcePeople))

	// remap source people to destination attributes for comparison
	sourcePeople, err = RemapToDestinationAttributes(sourcePeople, attributeMap)
	if err != nil {
		return ChangeResults{
			Errors: []string{err.Error()},
		}
	}

	destinationPeople, err := destination.ListUsers()
	if err != nil {
		return ChangeResults{
			Errors: []string{err.Error()},
		}
	}
	log.Printf("    Found %v people in destination", len(destinationPeople))

	changeSet := GenerateChangeSet(sourcePeople, destinationPeople, attributeMap, destination.GetIDField())

	// If in DryRun mode only print out ChangeSet plans and return mocked change results based on plans
	if dryRun {
		printChangeSet(changeSet)
		return ChangeResults{
			Created: uint64(len(changeSet.Create)),
			Updated: uint64(len(changeSet.Update)),
			Deleted: uint64(len(changeSet.Delete)),
		}
	}

	// Create a channel to pass activity logs for printing
	eventLog := make(chan EventLogItem, 50)
	go processEventLog(eventLog)

	results := destination.ApplyChangeSet(changeSet, eventLog)
	close(eventLog)

	return results
}

func GetDesiredAttributes(attrMap []AttributeMap) []string {
	var keys []string
	for _, attrMap := range attrMap {
		keys = append(keys, attrMap.Source)
	}

	return keys
}

type EventLogItem struct {
	Event   string
	Message string
}

func processEventLog(eventLog <-chan EventLogItem) {
	for msg := range eventLog {
		log.Printf("%s %s\n", msg.Event, msg.Message)
	}
}

func printChangeSet(changeSet ChangeSet) {
	log.Printf("ChangeSet Plans: Create %v, Update %v, Delete %v\n", len(changeSet.Create), len(changeSet.Update), len(changeSet.Delete))

	log.Println("Users to be created...")
	for i, user := range changeSet.Create {
		log.Printf("  %v) %s", i+1, user.CompareValue)
	}

	log.Println("Users to be updated...")
	for i, user := range changeSet.Update {
		log.Printf("  %v) %s", i+1, user.CompareValue)
	}

	log.Println("Users to be deleted...")
	for i, user := range changeSet.Delete {
		log.Printf("  %v) %s", i+1, user.CompareValue)
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
			if reflect.DeepEqual(needle, s.Index(i).Interface()) {
				index = i
				exists = true
				return
			}
		}
	}

	return
}

type EmptyDestination struct{}

func (e *EmptyDestination) GetIDField() string {
	return "id"
}

func (e *EmptyDestination) ForSet(syncSetJson json.RawMessage) error {
	return nil
}

func (e *EmptyDestination) ListUsers() ([]Person, error) {
	return []Person{}, nil
}

func (e *EmptyDestination) ApplyChangeSet(changes ChangeSet, eventLog chan<- EventLogItem) ChangeResults {
	return ChangeResults{}
}

type EmptySource struct{}

func (e *EmptySource) ForSet(syncSetJson json.RawMessage) error {
	return nil
}

func (e *EmptySource) ListUsers(desiredAttrs []string) ([]Person, error) {
	return []Person{}, nil
}

// Init sets the startTime to the current time,
//    sets the endTime based on secondsPerBatch into the future
func NewBatchTimer(batchSize, secondsPerBatch int) BatchTimer {
	b := BatchTimer{}
	b.Init(batchSize, secondsPerBatch)
	return b
}

// BatchTimer is intended as a time limited batch enforcer
// To create one, call its Init method.
// Then, to use it call its WaitOnBatch method after every call to
//  the associated go routine
type BatchTimer struct {
	startTime       time.Time
	endTime         time.Time
	Counter         int
	SecondsPerBatch int
	BatchSize       int
}

// Init sets the startTime to the current time,
//    sets the endTime based on secondsPerBatch into the future
func (b *BatchTimer) Init(batchSize, secondsPerBatch int) {
	b.startTime = time.Now()
	b.setEndTime()
	b.SecondsPerBatch = secondsPerBatch
	b.BatchSize = batchSize
	b.Counter = 0
}

func (b *BatchTimer) setEndTime() {
	var emptyTime time.Time
	if b.startTime == emptyTime {
		b.startTime = time.Now()
	}
	b.endTime = b.startTime.Add(time.Second * time.Duration(b.SecondsPerBatch))
}

// WaitOnBatch increments the Counter and then
//   if fewer than BatchSize have been dealt with, just returns without doing anything
//   Otherwise, sleeps until the batch time has expired (i.e. current time is past endTime).
//   If this last process occurs, then it ends by resetting the batch's times and counter.
func (b *BatchTimer) WaitOnBatch() {
	b.Counter++
	if b.Counter < b.BatchSize {
		return
	}

	for {
		currTime := time.Now()
		if currTime.After(b.endTime) {
			break
		}
		time.Sleep(time.Second)
	}
	b.Init(b.BatchSize, b.SecondsPerBatch)
}
