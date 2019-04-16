package personnel_sync

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

type Source struct {
	URL                  string
	Method               string
	Username             string
	Password             string
	ResultsJSONContainer string
	IDAttribute          string
}

type Destination struct {
	Type     string
	URL      string
	Username string
	Password string
	Extra    string
}

type Runtime struct {
	FailIfSinglePersonMissingRequiredAttribute bool
}

type AppConfig struct {
	Runtime                 Runtime
	Source                  Source
	Destination             Destination
	DestinationAttributeMap []DestinationAttributeMap
}
