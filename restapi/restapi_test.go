package restapi

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Jeffail/gabs"

	psync "github.com/silinternational/personnel-sync/v3"
)

func TestRestAPI_ListUsers(t *testing.T) {
	server := getTestServer()

	tests := []struct {
		name         string
		sourceConfig psync.SourceConfig
		syncSet      string
		desiredAttrs []string
		want         []psync.Person
		wantErr      bool
	}{
		{
			name: "workday-like results",
			sourceConfig: psync.SourceConfig{
				Type: psync.SourceTypeRestAPI,
				ExtraJSON: []byte(fmt.Sprintf(`{
		  "Method": "GET",
		  "BaseURL": "%s",
		  "ResultsJSONContainer": "Report_Entry",
		  "AuthType": "basic",
		  "Username": "username",
		  "Password": "password",
		  "CompareAttribute": "Email"
		}`, server.URL)),
			},
			syncSet: `{"Paths":["/workday"]}`,
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
			want: []psync.Person{
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
			sourceConfig: psync.SourceConfig{
				Type: psync.SourceTypeRestAPI,
				ExtraJSON: []byte(fmt.Sprintf(`{
		 "Method": "GET",
		 "BaseURL": "%s",
		 "AuthType": "basic",
		 "Username": "username",
		 "Password": "password",
		 "CompareAttribute": "email",
		 "ResultsJSONContainer": ""
		}`, server.URL)),
			},
			syncSet: `{"Paths":["/other"]}`,
			desiredAttrs: []string{
				"employeeID",
				"first",
				"last",
				"display",
				"username",
				"email",
			},
			want: []psync.Person{
				{
					CompareValue: "mickey_mouse@acme.com",
					Attributes: map[string]string{
						"employeeID": "10013",
						"first":      "Mickey",
						"last":       "Mouse",
						"display":    "Mickey Mouse",
						"username":   "MICKEY_MOUSE",
						"email":      "mickey_mouse@acme.com",
					},
				},
				{
					CompareValue: "donald_duck@acme.com",
					Attributes: map[string]string{
						"employeeID": "10011",
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
			sourceConfig: psync.SourceConfig{
				Type: psync.SourceTypeRestAPI,
				ExtraJSON: []byte(fmt.Sprintf(`{
		 "Method": "GET",
		 "BaseURL": "%s",
		 "AuthType": "basic",
		 "Username": "username",
		 "Password": "password",
		 "CompareAttribute": "fHCM2__User__r.Email",
		 "ResultsJSONContainer": "records"
		}`, server.URL)),
			},
			syncSet: `{"Paths":["/sfdc"]}`,
			desiredAttrs: []string{
				"fHCM2__User__r.Email",
			},
			want: []psync.Person{
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
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RestAPI.ListUsers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRestAPI_listUsersForPath(t *testing.T) {
	server := getTestServer()
	type args struct {
		desiredAttrs []string
		path         string
	}
	tests := []struct {
		name string
		r    RestAPI
		args args
		want []psync.Person
	}{
		{
			name: "Workday",
			r: RestAPI{
				Method:               http.MethodGet,
				BaseURL:              server.URL,
				ResultsJSONContainer: "Report_Entry",
				AuthType:             AuthTypeBasic,
				Username:             "username",
				Password:             "password",
				CompareAttribute:     "Email",
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
			want: []psync.Person{
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errLog := make(chan string, 1000)
			people := make(chan psync.Person, 20000)
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

			var results []psync.Person

			for person := range people {
				results = append(results, person)
			}

			if !reflect.DeepEqual(results, tt.want) {
				t.Errorf("RestAPI.listUsersForPath() = %v, want %v", results, tt.want)
			}
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
		want         []psync.Person
	}{
		{
			name:         "compareAttr not present",
			peopleList:   []*gabs.Container{person1},
			compareAttr:  "field",
			desiredAttrs: []string{"field1"},
			want:         []psync.Person{},
		},
		{
			name:         "no match in desiredAttrs",
			peopleList:   []*gabs.Container{person1},
			compareAttr:  "field1",
			desiredAttrs: []string{"field"},
			want:         []psync.Person{},
		},
		{
			name:         "empty person list",
			peopleList:   []*gabs.Container{},
			compareAttr:  "field1",
			desiredAttrs: []string{"field1"},
			want:         []psync.Person{},
		},
		{
			name:         "one field",
			peopleList:   []*gabs.Container{person1},
			compareAttr:  "field1",
			desiredAttrs: []string{"field1"},
			want: []psync.Person{
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
			want: []psync.Person{
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
			if got := getPersonsFromResults(tt.peopleList, tt.compareAttr, tt.desiredAttrs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPersonsFromResults() = %#v, want %#v", got, tt.want)
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
