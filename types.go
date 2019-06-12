package personnel_sync

import "encoding/json"

type Person struct {
	CompareValue   string
	Attributes     map[string]string
	DisableChanges bool
}

type AttributeMap struct {
	Source        string
	Destination   string
	Required      bool
	CaseSensitive bool
}

type SourceConfig struct {
	Type      string
	ExtraJSON json.RawMessage
}

type DestinationConfig struct {
	Type      string
	ExtraJSON json.RawMessage
}

type RuntimeConfig struct {
	DryRunMode                            bool
	FailIfSinglePersonMissingCompareValue bool
}

type AppConfig struct {
	Runtime      RuntimeConfig
	Source       SourceConfig
	Destination  DestinationConfig
	AttributeMap []AttributeMap
	SyncSets     []SyncSet
}

type SyncSet struct {
	Name        string
	Source      json.RawMessage
	Destination json.RawMessage
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
	ForSet(syncSetJson json.RawMessage) error
	ListUsers() ([]Person, error)
	ApplyChangeSet(changes ChangeSet, activityLog chan<- EventLogItem) ChangeResults
}

type Source interface {
	ForSet(syncSetJson json.RawMessage) error
	ListUsers() ([]Person, error)
}
