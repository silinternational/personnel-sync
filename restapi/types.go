package restapi

import "github.com/silinternational/personnel-sync/v5/internal"

type RestAPI struct {
	Method               string // DEPRECATED
	ListMethod           string
	CreateMethod         string
	UpdateMethod         string
	DeleteMethod         string
	IDAttribute          string
	BaseURL              string
	ResultsJSONContainer string
	AuthType             string
	Username             string
	Password             string
	ClientID             string
	ClientSecret         string
	CompareAttribute     string
	UserAgent            string
	BatchSize            int
	BatchDelaySeconds    int
	destinationConfig    internal.DestinationConfig
	setConfig            SetConfig
	Pagination           Pagination
}

type SetConfig struct {
	Paths      []string
	CreatePath string
	UpdatePath string
	DeletePath string
}

type Pagination struct {
	Scheme        string // if specified, must be 'query'
	PageNumberKey string // query string key for the page number
	PageSizeKey   string // number of items per page, default is 100
	FirstPage     int    // number of the first page, default is 1
	PageLimit     int    // maximum number of pages to request, default is 1000
	PageSize      int    // page size, default is 100
}
