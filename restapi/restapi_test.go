package restapi

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Jeffail/gabs/v2"
	"github.com/stretchr/testify/require"

	"github.com/silinternational/personnel-sync/v5/internal"
)

func TestRestAPI_ForSet(t *testing.T) {
	tests := []struct {
		name       string
		syncSet    string
		wantConfig RestAPI
		wantErr    string
	}{
		{
			name:    "invalid json",
			syncSet: `{"Paths":["/resource"],}`,
			wantErr: "json unmarshal error on set config",
		},
		{
			name:    "no path",
			syncSet: `{"Paths":[]}`,
			wantErr: "paths is empty in sync set",
		},
		{
			name:    "empty path",
			syncSet: `{"Paths":[""]}`,
			wantErr: "a path in sync set sources is blank",
		},
		{
			name:    "simple",
			syncSet: `{"Paths":["/path"]}`,
			wantConfig: RestAPI{
				destinationConfig: internal.DestinationConfig{
					DisableUpdate: true,
					DisableDelete: true,
				},
				setConfig: SetConfig{
					Paths: []string{"/path"},
				},
			},
		},
		{
			name:    "no leading slash",
			syncSet: `{"Paths":["path"]}`,
			wantConfig: RestAPI{
				destinationConfig: internal.DestinationConfig{
					DisableUpdate: true,
					DisableDelete: true,
				},
				setConfig: SetConfig{
					Paths: []string{"/path"},
				},
			},
		},
		{
			name:    "invalid UpdatePath",
			syncSet: `{"Paths":["/resource"],"UpdatePath":"/resource","DeletePath":"/resource/{id}"}`,
			wantErr: "invalid UpdatePath",
		},
		{
			name:    "invalid DeletePath",
			syncSet: `{"Paths":["/resource"],"UpdatePath":"/resource/{id}","DeletePath":"/resource"}`,
			wantErr: "invalid DeletePath",
		},
		{
			name:    "with UpdatePath and DeletePath",
			syncSet: `{"Paths":["/resource"],"UpdatePath":"/resource/{id}","DeletePath":"/resource/{id}"}`,
			wantConfig: RestAPI{
				setConfig: SetConfig{
					Paths:      []string{"/resource"},
					UpdatePath: "/resource/{id}",
					DeletePath: "/resource/{id}",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RestAPI{}
			err := r.ForSet([]byte(tt.syncSet))
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr, "error doesn't contain '%s'", tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantConfig, *r, "incorrect config produced by ForSet")
		})
	}
}

func TestRestAPI_ListUsers(t *testing.T) {
	server := getTestServer()
	endpoints := getFakeEndpoints()
	workday := endpoints[EndpointListWorkday]
	other := endpoints[EndpointListOther]
	salesforce := endpoints[EndpointListSalesforce]

	tests := []struct {
		name         string
		sourceConfig internal.SourceConfig
		syncSet      string
		desiredAttrs []string
		want         []internal.Person
		wantErr      bool
		errMsg       string
	}{
		{
			name: "workday-like results",
			sourceConfig: internal.SourceConfig{
				Type: internal.SourceTypeRestAPI,
				ExtraJSON: []byte(fmt.Sprintf(extraJSONtemplate,
					workday.method,
					server.URL,
					workday.resultsContainer,
					workday.authType,
					workday.username,
					workday.password,
					workday.compareAttr,
					workday.idAttr,
				)),
			},
			syncSet: `{"Paths":["` + workday.path + `"]}`,
			desiredAttrs: []string{
				"Employee_Number",
				"First_Name",
				"Last_Name",
				"Display_Name",
				"Username",
				"Email",
				"Personal_Email",
				"Account_Locked__Disabled_or_Expired",
				"requireMfa",
				"Company",
			},
			want: []internal.Person{
				{
					CompareValue: "mickey_mouse@acme.com",
					Attributes: map[string]string{
						"Employee_Number":                     "10013",
						"First_Name":                          "Mickey",
						"Last_Name":                           "Mouse",
						"Display_Name":                        "Mickey Mouse",
						"Username":                            "MICKEY_MOUSE",
						"Email":                               "mickey_mouse@acme.com",
						"Personal_Email":                      "mickey_mouse@mousemail.com",
						"Account_Locked__Disabled_or_Expired": "0",
						"requireMfa":                          "0",
						"Company":                             "Disney",
					},
				},
				{
					CompareValue: "donald_duck@acme.com",
					Attributes: map[string]string{
						"Employee_Number":                     "10011",
						"First_Name":                          "Donald",
						"Last_Name":                           "Duck",
						"Display_Name":                        "Donald Duck",
						"Username":                            "DONALD_DUCK",
						"Email":                               "donald_duck@acme.com",
						"Personal_Email":                      "donald_duck@duckmail.com",
						"Account_Locked__Disabled_or_Expired": "0",
						"requireMfa":                          "0",
						"Company":                             "Disney",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "other results",
			sourceConfig: internal.SourceConfig{
				Type: internal.SourceTypeRestAPI,
				ExtraJSON: []byte(fmt.Sprintf(extraJSONtemplate,
					other.method,
					server.URL,
					other.resultsContainer,
					other.authType,
					other.username,
					other.password,
					other.compareAttr,
					other.idAttr,
				)),
			},
			syncSet: `{"Paths":["` + other.path + `"]}`,
			desiredAttrs: []string{
				"first",
				"last",
				"display",
				"username",
				"email",
			},
			want: []internal.Person{
				{
					ID:           "10000013",
					CompareValue: "mickey_mouse@acme.com",
					Attributes: map[string]string{
						"employeeID": "10000013",
						"first":      "Mickey",
						"last":       "Mouse",
						"display":    "Mickey Mouse",
						"username":   "MICKEY_MOUSE",
						"email":      "mickey_mouse@acme.com",
					},
				},
				{
					ID:           "10000011",
					CompareValue: "donald_duck@acme.com",
					Attributes: map[string]string{
						"employeeID": "10000011",
						"first":      "Donald",
						"last":       "Duck",
						"display":    "Donald Duck",
						"username":   "DONALD_DUCK",
						"email":      "donald_duck@acme.com",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "sfdc results",
			sourceConfig: internal.SourceConfig{
				Type: internal.SourceTypeRestAPI,
				ExtraJSON: []byte(fmt.Sprintf(extraJSONtemplate,
					salesforce.method,
					server.URL,
					salesforce.resultsContainer,
					AuthTypeBearer,
					salesforce.username,
					salesforce.password,
					salesforce.compareAttr,
					salesforce.idAttr,
				)),
			},
			syncSet: `{"Paths":["` + salesforce.path + `"]}`,
			desiredAttrs: []string{
				"fHCM2__User__r.Email",
			},
			want: []internal.Person{
				{
					CompareValue: "mickey_mouse@acme.com",
					Attributes: map[string]string{
						"fHCM2__User__r.Email": "mickey_mouse@acme.com",
					},
				},
				{
					CompareValue: "donald_duck@acme.com",
					Attributes: map[string]string{
						"fHCM2__User__r.Email": "donald_duck@acme.com",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "auth error",
			sourceConfig: internal.SourceConfig{
				Type: internal.SourceTypeRestAPI,
				ExtraJSON: []byte(fmt.Sprintf(extraJSONtemplate,
					other.method,
					server.URL,
					other.resultsContainer,
					other.authType,
					other.username,
					other.password+"bad",
					other.compareAttr,
					other.idAttr,
				)),
			},
			syncSet: `{"Paths":["` + other.path + `"]}`,
			desiredAttrs: []string{
				"employeeID",
				"first",
				"last",
				"display",
				"username",
				"email",
			},
			want:    []internal.Person{},
			wantErr: true,
			errMsg:  "Not Authorized",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewRestAPISource(tt.sourceConfig)
			if err != nil {
				t.Errorf("Failed to get new RestAPI, error: %s", err.Error())
				t.FailNow()
			}
			err = r.ForSet([]byte(tt.syncSet))
			if err != nil {
				t.Errorf("ForSet error: %s", err)
				t.FailNow()
			}
			got, err := r.ListUsers(tt.desiredAttrs)
			if (err != nil) != tt.wantErr {
				t.Errorf("RestAPI.ListUsers() error = %v, wantErr %v", err, tt.wantErr)
				t.FailNow()
			}

			if err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf(`Unexpected error message "%s", expected: "%v"`, err, tt.errMsg)
				t.FailNow()
			}

			require.Equal(t, tt.want, got)
		})
	}
}

func TestRestAPI_listUsersForPath(t *testing.T) {
	server := getTestServer()
	endpoints := getFakeEndpoints()
	workday := endpoints[EndpointListWorkday]
	other := endpoints[EndpointListOther]

	type args struct {
		desiredAttrs []string
		path         string
	}
	tests := []struct {
		name string
		r    RestAPI
		args args
		want []internal.Person
	}{
		{
			name: "Workday",
			r: RestAPI{
				ListMethod:           workday.method,
				BaseURL:              server.URL,
				ResultsJSONContainer: workday.resultsContainer,
				AuthType:             workday.authType,
				Username:             workday.username,
				Password:             workday.password,
				CompareAttribute:     workday.compareAttr,
			},
			args: args{
				desiredAttrs: []string{
					"Employee_Number",
					"First_Name",
					"Last_Name",
					"Display_Name",
					"Username",
					"Email",
					"Personal_Email",
					"Account_Locked__Disabled_or_Expired",
					"requireMfa",
					"Company",
				},
				path: "/workday",
			},
			want: []internal.Person{
				{
					CompareValue: "mickey_mouse@acme.com",
					Attributes: map[string]string{
						"Employee_Number":                     "10013",
						"First_Name":                          "Mickey",
						"Last_Name":                           "Mouse",
						"Display_Name":                        "Mickey Mouse",
						"Username":                            "MICKEY_MOUSE",
						"Email":                               "mickey_mouse@acme.com",
						"Personal_Email":                      "mickey_mouse@mousemail.com",
						"Account_Locked__Disabled_or_Expired": "0",
						"requireMfa":                          "0",
						"Company":                             "Disney",
					},
				},
				{
					CompareValue: "donald_duck@acme.com",
					Attributes: map[string]string{
						"Employee_Number":                     "10011",
						"First_Name":                          "Donald",
						"Last_Name":                           "Duck",
						"Display_Name":                        "Donald Duck",
						"Username":                            "DONALD_DUCK",
						"Email":                               "donald_duck@acme.com",
						"Personal_Email":                      "donald_duck@duckmail.com",
						"Account_Locked__Disabled_or_Expired": "0",
						"requireMfa":                          "0",
						"Company":                             "Disney",
					},
				},
			},
		},
		{
			name: "test ID and number handling",
			r: RestAPI{
				ListMethod:           other.method,
				BaseURL:              server.URL,
				ResultsJSONContainer: other.resultsContainer,
				AuthType:             other.authType,
				Username:             other.username,
				Password:             other.password,
				CompareAttribute:     other.compareAttr,
				IDAttribute:          other.idAttr,
			},
			args: args{
				desiredAttrs: []string{
					"employeeID",
					"display",
					"email",
				},
				path: "/other/list",
			},
			want: []internal.Person{
				{
					ID:           "10000013",
					CompareValue: "mickey_mouse@acme.com",
					Attributes: map[string]string{
						"employeeID": "10000013",
						"email":      "mickey_mouse@acme.com",
						"display":    "Mickey Mouse",
					},
				},
				{
					ID:           "10000011",
					CompareValue: "donald_duck@acme.com",
					Attributes: map[string]string{
						"employeeID": "10000011",
						"email":      "donald_duck@acme.com",
						"display":    "Donald Duck",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errLog := make(chan string, 1000)
			people := make(chan internal.Person, 20000)
			var wg sync.WaitGroup

			wg.Add(1)
			go tt.r.listUsersForPath(tt.args.desiredAttrs, tt.args.path, &wg, people, errLog)

			wg.Wait()
			close(people)
			close(errLog)

			if len(errLog) > 0 {
				var errs []string
				for msg := range errLog {
					errs = append(errs, msg)
				}
				t.Errorf("errors listing users: %s", strings.Join(errs, ","))
				t.FailNow()
			}

			var results []internal.Person

			for person := range people {
				results = append(results, person)
			}

			require.Equal(t, tt.want, results)
		})
	}
}

func Test_getPersonsFromResults(t *testing.T) {
	person1 := gabs.New()
	_, _ = person1.Set("value1", "field1")
	_, _ = person1.Set("value2", "field2")

	person2 := gabs.New()
	_, _ = person2.Set("p2value1", "field1")

	tests := []struct {
		name         string
		peopleList   []*gabs.Container
		compareAttr  string
		desiredAttrs []string
		want         []internal.Person
	}{
		{
			name:         "compareAttr not present",
			peopleList:   []*gabs.Container{person1},
			compareAttr:  "field",
			desiredAttrs: []string{"field1"},
			want:         []internal.Person{},
		},
		{
			name:         "no match in desiredAttrs",
			peopleList:   []*gabs.Container{person1},
			compareAttr:  "field1",
			desiredAttrs: []string{"field"},
			want:         []internal.Person{},
		},
		{
			name:         "empty person list",
			peopleList:   []*gabs.Container{},
			compareAttr:  "field1",
			desiredAttrs: []string{"field1"},
			want:         []internal.Person{},
		},
		{
			name:         "one field",
			peopleList:   []*gabs.Container{person1},
			compareAttr:  "field1",
			desiredAttrs: []string{"field1"},
			want: []internal.Person{
				{
					CompareValue: "value1",
					Attributes:   map[string]string{"field1": "value1"},
				},
			},
		},
		{
			name:         "two persons, two fields",
			peopleList:   []*gabs.Container{person1, person2},
			compareAttr:  "field1",
			desiredAttrs: []string{"field1", "field2"},
			want: []internal.Person{
				{
					CompareValue: "value1",
					Attributes:   map[string]string{"field1": "value1", "field2": "value2"},
				},
				{
					CompareValue: "p2value1",
					Attributes:   map[string]string{"field1": "p2value1"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := RestAPI{CompareAttribute: tt.compareAttr}
			if got := r.getPersonsFromResults(tt.peopleList, tt.desiredAttrs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPersonsFromResults() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func Test_attributesToJSON(t *testing.T) {
	tests := []struct {
		name string
		attr map[string]string
		want string
	}{
		{
			name: "1",
			attr: map[string]string{
				"field":        "value",
				"parent.child": "child_value",
			},
			want: `{"field":"value","parent":{"child":"child_value"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := attributesToJSON(tt.attr); got != tt.want {
				t.Errorf("attributesToJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRestAPI_httpRequest(t *testing.T) {
	server := getTestServer()
	endpoints := getFakeEndpoints()

	tests := []struct {
		name    string
		restAPI RestAPI
		verb    string
		url     string
		body    string
		headers map[string]string
		want    string
		wantErr bool
	}{
		{
			name: "basic auth",
			restAPI: RestAPI{
				AuthType: endpoints[EndpointListWorkday].authType,
				Username: endpoints[EndpointListWorkday].username,
				Password: endpoints[EndpointListWorkday].password,
			},
			verb: endpoints[EndpointListWorkday].method,
			url:  server.URL + endpoints[EndpointListWorkday].path,
			want: endpoints[EndpointListWorkday].responseBody,
		},
		{
			name: "basic auth fail",
			restAPI: RestAPI{
				AuthType: endpoints[EndpointListWorkday].authType,
				Username: endpoints[EndpointListWorkday].username,
				Password: "bad password",
			},
			verb:    endpoints[EndpointListWorkday].method,
			url:     server.URL + endpoints[EndpointListWorkday].path,
			wantErr: true,
		},
		{
			name: "bearer token",
			restAPI: RestAPI{
				AuthType: endpoints[EndpointListOther].authType,
				Password: endpoints[EndpointListOther].password,
			},
			verb: endpoints[EndpointListOther].method,
			url:  server.URL + endpoints[EndpointListOther].path,
			want: endpoints[EndpointListOther].responseBody,
		},
		{
			name: "bearer token fail",
			restAPI: RestAPI{
				AuthType: endpoints[EndpointListOther].authType,
				Password: "bad token",
			},
			verb:    endpoints[EndpointListOther].method,
			url:     server.URL + endpoints[EndpointListOther].path,
			wantErr: true,
		},
		{
			name: "salesforce",
			restAPI: RestAPI{
				AuthType: endpoints[EndpointListSalesforce].authType,
				Password: endpoints[EndpointListSalesforce].password,
			},
			verb: endpoints[EndpointListSalesforce].method,
			url:  server.URL + endpoints[EndpointListSalesforce].path,
			want: endpoints[EndpointListSalesforce].responseBody,
		},
		{
			name: "salesforce fail",
			restAPI: RestAPI{
				AuthType: endpoints[EndpointListSalesforce].authType,
				Password: "bad token",
			},
			verb:    endpoints[EndpointListSalesforce].method,
			url:     server.URL + endpoints[EndpointListSalesforce].path,
			wantErr: true,
		},
		{
			name: "bearer create",
			restAPI: RestAPI{
				AuthType: endpoints[EndpointCreateOther].authType,
				Password: endpoints[EndpointCreateOther].password,
			},
			verb: endpoints[EndpointCreateOther].method,
			url:  server.URL + endpoints[EndpointCreateOther].path,
			body: `{"email":"test@example.com","id":"1234"}`,
			want: endpoints[EndpointCreateOther].responseBody,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.restAPI.httpRequest(tt.verb, tt.url, tt.body, tt.headers)
			if (err != nil) != tt.wantErr {
				t.Errorf("httpRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("httpRequest() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parsePathTemplate(t *testing.T) {
	tests := []struct {
		name         string
		pathTemplate string
		wantPath     string
		wantErr      bool
	}{
		{
			name:         "no field name",
			pathTemplate: "/contacts",
			wantErr:      true,
		},
		{
			name:         "has a field name",
			pathTemplate: "/contacts/{someFieldName}",
			wantPath:     "/contacts/{id}",
		},
		{
			name:         "no leading slash",
			pathTemplate: "contacts/{someOtherFieldName}",
			wantPath:     "/contacts/{id}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := parsePathTemplate(tt.pathTemplate)
			if tt.wantErr {
				require.Error(t, err, "expected error but did not get one")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantPath, path)
		})
	}
}
