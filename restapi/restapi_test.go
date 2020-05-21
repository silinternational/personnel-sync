package restapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	personnel_sync "github.com/silinternational/personnel-sync/v3"
)

func TestRestAPI_ListUsers(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	mux.HandleFunc("/workday", func(w http.ResponseWriter, req *http.Request) {
		body := `{
  "Report_Entry": [
    {
      "Employee_Number": "10013",
      "First_Name": "Mickey",
      "Last_Name": "Mouse",
      "Display_Name": "Mickey Mouse",
      "Username": "MICKEY_MOUSE",
      "Email": "mickey_mouse@acme.com",
      "Personal_Email": "mickey_mouse@mousemail.com",
      "Account_Locked__Disabled_or_Expired": "0",
      "requireMfa": "0",
      "Company": "Disney"
    },
	{
      "Employee_Number": "10011",
      "First_Name": "Donald",
      "Last_Name": "Duck",
      "Display_Name": "Donald Duck",
      "Username": "DONALD_DUCK",
      "Email": "donald_duck@acme.com",
      "Personal_Email": "donald_duck@duckmail.com",
      "Account_Locked__Disabled_or_Expired": "0",
      "requireMfa": "0",
      "Company": "Disney"
    }
  ]
}`
		w.WriteHeader(200)
		w.Header().Set("content-type", "application/json")
		_, _ = fmt.Fprintf(w, body)
	})

	mux.HandleFunc("/other", func(w http.ResponseWriter, req *http.Request) {
		body := `[
    {
      "employeeID": "10013",
      "first": "Mickey",
      "last": "Mouse",
      "display": "Mickey Mouse",
      "username": "MICKEY_MOUSE",
      "email": "mickey_mouse@acme.com"
    },
	{
      "employeeID": "10011",
      "first": "Donald",
      "last": "Duck",
      "display": "Donald Duck",
      "username": "DONALD_DUCK",
      "email": "donald_duck@acme.com"
    }
]`
		w.WriteHeader(200)
		w.Header().Set("content-type", "application/json")
		_, _ = fmt.Fprintf(w, body)
	})

	mux.HandleFunc("/sfdc", func(w http.ResponseWriter, req *http.Request) {
		body := `{
  "totalSize": 2,
  "done": true,
  "records": [
    {
      "attributes": {
        "type": "fHCM2__Team_Member__c",
        "url": "/services/data/v20.0/sobjects/fHCM2__Team_Member__c/a1H1U737901ULOwUAO"
      },
      "Name": "Mickey Mouse",
      "fHCM2__User__r": {
        "attributes": {
          "type": "User",
          "url": "/services/data/v20.0/sobjects/User/0051U579303drCrQAI"
        },
        "Email": "mickey_mouse@acme.com"
      }
    },
    {
      "attributes": {
        "type": "fHCM2__Team_Member__c",
        "url": "/services/data/v20.0/sobjects/fHCM2__Team_Member__c/a1H1U50361ULZbUAO"
      },
      "Name": "Donald Duck",
      "fHCM2__User__r": {
        "attributes": {
          "type": "User",
          "url": "/services/data/v20.0/sobjects/User/0051U773763dqt3QAA"
        },
        "Email": "donald_duck@acme.com"
      }
    }
  ]
}`
		w.WriteHeader(200)
		w.Header().Set("content-type", "application/json")
		_, _ = fmt.Fprintf(w, body)
	})

	tests := []struct {
		name         string
		sourceConfig personnel_sync.SourceConfig
		desiredAttrs []string
		want         []personnel_sync.Person
		wantErr      bool
	}{
		{
			name: "workday-like results",
			sourceConfig: personnel_sync.SourceConfig{
				Type: personnel_sync.SourceTypeRestAPI,
				ExtraJSON: []byte(fmt.Sprintf(`{
		  "Method": "GET",
		  "BaseURL": "%s/workday",
		  "ResultsJSONContainer": "Report_Entry",
		  "AuthType": "basic",
		  "Username": "username",
		  "Password": "password",
		  "CompareAttribute": "Email"
		}`, server.URL)),
			},
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
			want: []personnel_sync.Person{
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
			sourceConfig: personnel_sync.SourceConfig{
				Type: personnel_sync.SourceTypeRestAPI,
				ExtraJSON: []byte(fmt.Sprintf(`{
      "Method": "GET",
      "BaseURL": "%s/other",
      "AuthType": "basic",
      "Username": "username",
      "Password": "password",
      "CompareAttribute": "email",
      "ResultsJSONContainer": ""
    }`, server.URL)),
			},
			desiredAttrs: []string{
				"employeeID",
				"first",
				"last",
				"display",
				"username",
				"email",
			},
			want: []personnel_sync.Person{
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
			sourceConfig: personnel_sync.SourceConfig{
				Type: personnel_sync.SourceTypeRestAPI,
				ExtraJSON: []byte(fmt.Sprintf(`{
      "Method": "GET",
      "BaseURL": "%s/sfdc",
      "AuthType": "basic",
      "Username": "username",
      "Password": "password",
      "CompareAttribute": "fHCM2__User__r.Email",
      "ResultsJSONContainer": "records"
    }`, server.URL)),
			},
			desiredAttrs: []string{
				"fHCM2__User__r.Email",
			},
			want: []personnel_sync.Person{
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
