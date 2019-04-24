package personnel_sync

import "encoding/json"

type Person struct {
	CompareValue string
	Attributes   map[string]string
}

type DestinationAttributeMap struct {
	SourceName      string
	DestinationName string
	Required        bool
}

type SourceConfig struct {
	URL                  string
	Method               string
	Username             string
	Password             string
	ResultsJSONContainer string
	IDAttribute          string
	CompareAttribute     string
}

type DestinationConfig struct {
	Type             string
	URL              string
	Username         string
	Password         string
	CompareAttribute string
	ExtraJSON        json.RawMessage
}

type RuntimeConfig struct {
	FailIfSinglePersonMissingRequiredAttribute bool
}

type AppConfig struct {
	Runtime                 RuntimeConfig
	Source                  SourceConfig
	Destination             DestinationConfig
	DestinationAttributeMap []DestinationAttributeMap
}

type ChangeSet struct {
	Create []Person
	Update []Person
	Delete []Person
}

type ChangeResults struct {
	Created uint64
	Updated uint64
	Deleted uint64
	Errors  []string
}

type Destination interface {
	ListUsers() ([]Person, error)
	ApplyChangeSet(changes ChangeSet) ChangeResults
}
