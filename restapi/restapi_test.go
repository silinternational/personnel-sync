package restapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	personnel_sync "github.com/silinternational/personnel-sync"
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
    },
	{
      "First_Name": "Missing",
      "Last_Name": "Email Field",
      "Username": "MISSING_EMAIL_FIELD"
    }
  ]
}`
		w.WriteHeader(200)
		w.Header().Set("content-type", "application/json")
		fmt.Fprintf(w, body)
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
    },
	{
      "first": "Missing",
      "last": "Email Field",
      "username": "MISSING_EMAIL_FIELD"
    }
]`
		w.WriteHeader(200)
		w.Header().Set("content-type", "application/json")
		fmt.Fprintf(w, body)
	})

	tests := []struct {
		name         string
		sourceConfig personnel_sync.SourceConfig
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewRestAPISource(tt.sourceConfig)
			if err != nil {
				t.Errorf("Failed to get new RestAPI, error: %s", err.Error())
				t.FailNow()
			}

			got, err := r.ListUsers()
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
