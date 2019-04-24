package personnel_sync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestGetPersonsFromSource(t *testing.T) {
	expectedResultsJson := `{
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

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	mux.HandleFunc("/people", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(200)
		w.Header().Set("content-type", "application/json")
		_, _ = fmt.Fprintf(w, expectedResultsJson)
	})

	sourceConfig := SourceConfig{
		URL:                  server.URL + "/people",
		Username:             "test",
		Password:             "test",
		Method:               "GET",
		ResultsJSONContainer: "Report_Entry",
		IDAttribute:          "Email",
	}

	appConfig := AppConfig{
		Runtime: RuntimeConfig{
			FailIfSinglePersonMissingRequiredAttribute: false,
		},
		Source:      sourceConfig,
		Destination: DestinationConfig{},
		DestinationAttributeMap: []DestinationAttributeMap{
			{
				SourceName:      "First_Name",
				DestinationName: "givenName",
				Required:        true,
			},
			{
				SourceName:      "Last_Name",
				DestinationName: "sn",
				Required:        true,
			},
			{
				SourceName:      "Email",
				DestinationName: "mail",
				Required:        true,
			},
		},
	}

	people, err := GetPersonsFromSource(appConfig)
	if err != nil {
		t.Errorf("Unable to get people from source, error: %s", err.Error())
	}

	validPeopleInTestData := 2
	if len(people) != validPeopleInTestData {
		t.Errorf("Did not get expected number of results. Expected %v, got %v. Results: %v", validPeopleInTestData, len(people), people)
	}

	jsonResults, _ := json.Marshal(people)

	fmt.Println(string(jsonResults))
}

func TestGenerateChangeSet(t *testing.T) {
	type args struct {
		sourcePeople      []Person
		destinationPeople []Person
	}
	tests := []struct {
		name string
		args args
		want ChangeSet
	}{
		{
			name: "creates two, deletes one, updates one",
			want: ChangeSet{
				Create: []Person{
					{
						CompareValue: "1",
					},
					{
						CompareValue: "2",
					},
				},
				Delete: []Person{
					{
						CompareValue: "3",
					},
				},
				Update: []Person{
					{
						CompareValue: "5",
						Attributes: map[string]string{
							"name": "new value",
						},
					},
				},
			},
			args: args{
				sourcePeople: []Person{
					{
						CompareValue: "1",
					},
					{
						CompareValue: "2",
					},
					{
						CompareValue: "4",
					},
					{
						CompareValue: "5",
						Attributes: map[string]string{
							"name": "new value",
						},
					},
				},
				destinationPeople: []Person{
					{
						CompareValue: "3",
					},
					{
						CompareValue: "4",
					},
					{
						CompareValue: "5",
						Attributes: map[string]string{
							"name": "original value",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateChangeSet(tt.args.sourcePeople, tt.args.destinationPeople); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenerateChangeSet() = %v, want %v", got, tt.want)
			}
		})
	}
}
