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
	// If specified, must be "pages" for a page based request or "items" for an item based request.
	// If not specified, no pagination is attempted
	Scheme string

	// Item based values
	FirstItemIndex int    // index of first item to fetch, default is 0
	ItemKey        string // query string key for the item index to start at

	// Page based values
	FirstPage     int    // number of the first page, default is 1
	PageNumberKey string // query string key for the page number

	// Shared values (for either Item based or Page based)
	PageLimit           int    // maximum number of pages to request, default is 1000
	PageSize            int    // page size, default is 100 items per page
	PageSizeKey         string // query string key the number of items per page
	QueryStartDelimiter string // defaults to "?" but can be set to "&"
}
