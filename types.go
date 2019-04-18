package personnel_sync

import "encoding/json"

type Person struct {
	ID         string
	Attributes []PersonAttribute
}

type PersonAttribute struct {
	Name  string
	Value string
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
}

type DestinationConfig struct {
	Type      string
	URL       string
	Username  string
	Password  string
	ExtraJSON json.RawMessage
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
