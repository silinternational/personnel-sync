package restapi

import "github.com/silinternational/personnel-sync/v6/internal"

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
	// If specified, must be "pages" for a page based request or "items" for an item based request.
	// If not specified, no pagination is attempted.
	Scheme string

	FirstIndex  int    // index of first item/page to fetch, default is 1
	NumberKey   string // query string key for the item index or page number, default is "page"
	PageLimit   int    // index of last page to request, default is 1000
	PageSize    int    // page size, default is 100 items per page
	PageSizeKey string // query string key the number of items per page
}
