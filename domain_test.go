package personnel_sync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

	sourceConfig := Source{
		URL:                  server.URL + "/people",
		Username:             "test",
		Password:             "test",
		Method:               "GET",
		ResultsJSONContainer: "Report_Entry",
		IDAttribute:          "Email",
	}

	appConfig := AppConfig{
		Runtime: Runtime{
			FailIfSinglePersonMissingRequiredAttribute: false,
		},
		Source:      sourceConfig,
		Destination: Destination{},
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
