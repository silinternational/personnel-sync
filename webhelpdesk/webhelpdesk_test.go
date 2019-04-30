package webhelpdesk

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/silinternational/personnel-sync/restapi"

	"github.com/silinternational/personnel-sync"
)

func TestWebHelpDesk_ListUsers(t *testing.T) {

	fixtures := map[string][]User{
		"page1": {
			{
				ID:               1,
				FirstName:        "c1",
				LastName:         "c1",
				Email:            "c1@c1.com",
				EmploymentStatus: "c1",
				Username:         "c1",
			},
			{
				ID:               2,
				FirstName:        "c2",
				LastName:         "c2",
				Email:            "c2@c2.com",
				EmploymentStatus: "c2",
				Username:         "c2",
			},
			{
				ID:               3,
				FirstName:        "c3",
				LastName:         "c3",
				Email:            "c3@c3.com",
				EmploymentStatus: "c3",
				Username:         "c3",
			},
			{
				ID:               4,
				FirstName:        "c4",
				LastName:         "c4",
				Email:            "c4@c4.com",
				EmploymentStatus: "c4",
				Username:         "c4",
			},
			{
				ID:               5,
				FirstName:        "c5",
				LastName:         "c5",
				Email:            "c5@c5.com",
				EmploymentStatus: "c5",
				Username:         "c5",
			},
		},
		"page2": {
			{
				ID:               6,
				FirstName:        "c6",
				LastName:         "c6",
				Email:            "c6@c6.com",
				EmploymentStatus: "c6",
				Username:         "c6",
			},
			{
				ID:               7,
				FirstName:        "c7",
				LastName:         "c7",
				Email:            "c7@c7.com",
				EmploymentStatus: "c7",
				Username:         "c7",
			},
			{
				ID:               8,
				FirstName:        "c8",
				LastName:         "c8",
				Email:            "c8@c8.com",
				EmploymentStatus: "c8",
				Username:         "c8",
			},
			{
				ID:               9,
				FirstName:        "c9",
				LastName:         "c9",
				Email:            "c9@c9.com",
				EmploymentStatus: "c9",
				Username:         "c9",
			},
		},
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	mux.HandleFunc("/ra/Clients", func(w http.ResponseWriter, req *http.Request) {
		v := req.URL.Query()
		page := v.Get("page")

		key := fmt.Sprintf("page%s", page)

		jsonBytes, err := json.Marshal(fixtures[key])
		if err != nil {
			t.Errorf("Unable to marshal fixture results, error: %s", err.Error())
			t.FailNow()
		}

		w.WriteHeader(200)
		w.Header().Set("content-type", "application/json")
		fmt.Fprintf(w, string(jsonBytes))
	})

	whdConfig := WebHelpDesk{
		URL:                  server.URL,
		Password:             "alala",
		Username:             "bkbkb",
		ListClientsPageLimit: 5,
	}

	extraJson, err := json.Marshal(whdConfig)
	if err != nil {
		t.Errorf("Error marshalling whdConfig to json: %s", err.Error())
	}

	type fields struct {
		DestinationConfig personnel_sync.DestinationConfig
	}
	tests := []struct {
		name    string
		fields  fields
		want    []personnel_sync.Person
		wantErr bool
	}{
		{
			name: "all results",
			fields: fields{
				DestinationConfig: personnel_sync.DestinationConfig{
					Type:      personnel_sync.DestinationTypeWebHelpDesk,
					ExtraJSON: extraJson,
				},
			},
			want: []personnel_sync.Person{
				{
					CompareValue: "c1@c1.com",
					Attributes: map[string]string{
						"id":               "1",
						"email":            "c1@c1.com",
						"firstName":        "c1",
						"lastName":         "c1",
						"username":         "c1",
						"employmentStatus": "c1",
					},
				},
				{
					CompareValue: "c2@c2.com",
					Attributes: map[string]string{
						"id":               "2",
						"email":            "c2@c2.com",
						"firstName":        "c2",
						"lastName":         "c2",
						"username":         "c2",
						"employmentStatus": "c2",
					},
				},
				{
					CompareValue: "c3@c3.com",
					Attributes: map[string]string{
						"id":               "3",
						"email":            "c3@c3.com",
						"firstName":        "c3",
						"lastName":         "c3",
						"username":         "c3",
						"employmentStatus": "c3",
					},
				},
				{
					CompareValue: "c4@c4.com",
					Attributes: map[string]string{
						"id":               "4",
						"email":            "c4@c4.com",
						"firstName":        "c4",
						"lastName":         "c4",
						"username":         "c4",
						"employmentStatus": "c4",
					},
				},
				{
					CompareValue: "c5@c5.com",
					Attributes: map[string]string{
						"id":               "5",
						"email":            "c5@c5.com",
						"firstName":        "c5",
						"lastName":         "c5",
						"username":         "c5",
						"employmentStatus": "c5",
					},
				},
				{
					CompareValue: "c6@c6.com",
					Attributes: map[string]string{
						"id":               "6",
						"email":            "c6@c6.com",
						"firstName":        "c6",
						"lastName":         "c6",
						"username":         "c6",
						"employmentStatus": "c6",
					},
				},
				{
					CompareValue: "c7@c7.com",
					Attributes: map[string]string{
						"id":               "7",
						"email":            "c7@c7.com",
						"firstName":        "c7",
						"lastName":         "c7",
						"username":         "c7",
						"employmentStatus": "c7",
					},
				},
				{
					CompareValue: "c8@c8.com",
					Attributes: map[string]string{
						"id":               "8",
						"email":            "c8@c8.com",
						"firstName":        "c8",
						"lastName":         "c8",
						"username":         "c8",
						"employmentStatus": "c8",
					},
				},
				{
					CompareValue: "c9@c9.com",
					Attributes: map[string]string{
						"id":               "9",
						"email":            "c9@c9.com",
						"firstName":        "c9",
						"lastName":         "c9",
						"username":         "c9",
						"employmentStatus": "c9",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, err := NewWebHelpDeskDesination(tt.fields.DestinationConfig)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
			got, err := w.ListUsers()
			if (err != nil) != tt.wantErr {
				t.Errorf("WebHelpDesk.ListUsers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WebHelpDesk.ListUsers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateChangeSet(t *testing.T) {
	testConfig, err := personnel_sync.LoadConfig("./config.json")
	if err != nil {
		t.Errorf("Failed to load test config, error: %s", err.Error())
		t.FailNow()
	}

	whd, err := NewWebHelpDeskDesination(testConfig.Destination)
	if err != nil {
		t.Errorf("Failed to get new whd client, error: %s", err.Error())
		t.FailNow()
	}

	users, err := whd.ListUsers()
	if err != nil {
		t.Errorf("Failed to list whd users, error: %s", err.Error())
		t.FailNow()
	}

	source, err := restapi.NewRestAPISource(testConfig.Source)
	if err != nil {
		t.Error(err)
	}

	sourcePeople, err := source.ListUsers()
	log.Printf("found %v people in source", len(sourcePeople))

	changeSet := personnel_sync.GenerateChangeSet(sourcePeople, users)

	log.Printf("ChangeSet ready %v to be created, %v to be deleted", len(changeSet.Create), len(changeSet.Delete))
}

func TestWebHelpDesk_CreateUser(t *testing.T) {
	testConfig, err := personnel_sync.LoadConfig("./config.json")
	if err != nil {
		t.Errorf("Failed to load test config, error: %s", err.Error())
		t.FailNow()
	}

	whd, err := NewWebHelpDeskDesination(testConfig.Destination)
	if err != nil {
		t.Errorf("Failed to get new whd client, error: %s", err.Error())
		t.FailNow()
	}

	personToCreate := personnel_sync.Person{
		Attributes: map[string]string{
			"firstName":        "testing123456",
			"lastName":         "test-for-phillip",
			"email":            "phillip_shipley+testing123456@sil.org",
			"username":         "phillip_shipley+testing123456@sil.org",
			"employmentStatus": "123456",
		},
	}

	changeSet := personnel_sync.ChangeSet{
		Create: []personnel_sync.Person{
			personToCreate,
		},
	}

	changeResults := whd.ApplyChangeSet(changeSet)
	log.Println(changeResults)

	if changeResults.Created != 1 {
		t.Errorf("Unable to create user, number of users created was %v", changeResults.Created)
		log.Println("Errors creating user:")
		for _, msg := range changeResults.Errors {
			log.Println(msg)
		}
	}
}
