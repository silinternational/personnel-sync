package googledest

import (
	"math/rand"
	"reflect"
	"strconv"
	"testing"

	personnel_sync "github.com/silinternational/personnel-sync"
	admin "google.golang.org/api/admin/directory/v1"
)

func TestGoogleUsers_ListUsers(t *testing.T) {
	t.Skip("Skipping test because it requires integration with Google")
	t.SkipNow()

	testConfig, err := personnel_sync.LoadConfig("../cmd/config.json")
	if err != nil {
		t.Errorf("Failed to load test config, error: %s", err.Error())
		t.FailNow()
	}

	type fields struct {
		DestinationConfig personnel_sync.DestinationConfig
		GoogleUsersConfig GoogleUsersConfig
		AdminService      admin.Service
	}
	tests := []struct {
		name    string
		fields  fields
		want    []personnel_sync.Person
		wantErr bool
	}{
		{
			name: "test listing users",
			fields: fields{
				DestinationConfig: testConfig.Destination,
			},
			want: []personnel_sync.Person{
				{
					CompareValue: "user_one@example.com",
					ID:           "",
					Attributes: map[string]string{
						"email":      "user_one@example.com",
						"familyName": "one",
						"fullName":   "user one",
						"givenName":  "user",
					},
					DisableChanges: false,
				},
				{
					CompareValue: "user_two@example.com",
					ID:           "",
					Attributes: map[string]string{
						"email":      "user_two@example.com",
						"familyName": "two",
						"fullName":   "user two",
						"givenName":  "user",
					},
					DisableChanges: false,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := NewGoogleUsersDestination(tt.fields.DestinationConfig)
			if err != nil {
				t.Errorf("Failed to get new googleUsers instance, error: %s", err.Error())
				t.FailNow()
			}
			got, err := g.ListUsers()
			if (err != nil) != tt.wantErr {
				t.Errorf("GoogleUsers.ListUsers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GoogleUsers.ListUsers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGoogleUsers_ApplyChangeSet(t *testing.T) {
	t.Skip("Skipping test because it requires integration with Google")
	t.SkipNow()

	testConfig, err := personnel_sync.LoadConfig("./config.json")
	if err != nil {
		t.Errorf("Failed to load test config, error: %s", err.Error())
		t.FailNow()
	}

	type fields struct {
		DestinationConfig personnel_sync.DestinationConfig
	}
	type args struct {
		changes personnel_sync.ChangeSet
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   personnel_sync.ChangeResults
	}{
		{
			name: "expect one updated",
			want: personnel_sync.ChangeResults{
				Created: uint64(0),
				Updated: uint64(1),
				Deleted: uint64(0),
			},
			fields: fields{
				DestinationConfig: testConfig.Destination,
			},
			args: args{
				changes: personnel_sync.ChangeSet{
					Create: []personnel_sync.Person{},
					Update: []personnel_sync.Person{
						{
							CompareValue: "user@example.com",
							ID:           "",
							Attributes: map[string]string{
								"email":      "user@example.com",
								"familyName": strconv.Itoa(rand.Intn(1000)),
								"givenName":  "x",
							},
							DisableChanges: false,
						},
					},
					Delete: []personnel_sync.Person{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := NewGoogleUsersDestination(tt.fields.DestinationConfig)
			if err != nil {
				t.Errorf("Failed to get new googleUsers instance, error: %s", err.Error())
				t.FailNow()
			}
			eventLog := make(chan personnel_sync.EventLogItem, 50)
			if got := g.ApplyChangeSet(tt.args.changes, eventLog); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GoogleUsers.ApplyChangeSet() = %v, want %v", got, tt.want)
			}
			close(eventLog)
		})
	}
}

func TestGoogleUsers_extractData(t *testing.T) {
	tests := []struct {
		name string
		user admin.User
		want personnel_sync.Person
	}{
		{
			name: "minimum",
			user: admin.User{
				ExternalIds:   nil,
				Locations:     nil,
				Name:          nil,
				Organizations: nil,
				Phones:        nil,
				PrimaryEmail:  "email@example.com",
				Relations:     nil,
			},
			want: personnel_sync.Person{
				CompareValue: "email@example.com",
				Attributes:   map[string]string{"email": "email@example.com"},
			},
		},
		{
			name: "all supported fields",
			user: admin.User{
				ExternalIds: []interface{}{map[string]interface{}{
					"type":  "organization",
					"value": "12345",
				}},
				Locations: []interface{}{map[string]interface{}{
					"area":       "An area",
					"buildingId": "A building",
					"type":       "desk",
				}},
				Name: &admin.UserName{
					FamilyName: "Jones",
					FullName:   "John Jones",
					GivenName:  "John",
				},
				Organizations: []interface{}{map[string]interface{}{
					"costCenter": "A cost center",
					"department": "A department",
					"title":      "A title",
				}},
				Phones: []interface{}{map[string]interface{}{
					"type":  "work",
					"value": "555-1212",
				}},
				PrimaryEmail: "email@example.com",
				Relations: []interface{}{map[string]interface{}{
					"type":  "manager",
					"value": "manager@example.com",
				}},
			},
			want: personnel_sync.Person{
				CompareValue: "email@example.com",
				Attributes: map[string]string{
					"email":      "email@example.com",
					"familyName": "Jones",
					"givenName":  "John",
					"id":         "12345",
					"area":       "An area",
					"building":   "A building",
					"costCenter": "A cost center",
					"department": "A department",
					"title":      "A title",
					"phone":      "555-1212",
					"manager":    "manager@example.com",
				},
			},
		},
		{
			name: `only "organization" externalIDs`,
			user: admin.User{
				ExternalIds: []interface{}{
					map[string]interface{}{
						"type":  "custom",
						"value": "abc123",
					},
					map[string]interface{}{
						"type":  "organization",
						"value": "12345",
					},
				},
				PrimaryEmail: "email@example.com",
			},
			want: personnel_sync.Person{
				CompareValue: "email@example.com",
				Attributes: map[string]string{
					"email": "email@example.com",
					"id":    "12345",
				},
			},
		},
		{
			name: `only "work" phones`,
			user: admin.User{
				PrimaryEmail: "email@example.com",
				Phones: []interface{}{
					map[string]interface{}{
						"type":  "home",
						"value": "555-1212",
					},
					map[string]interface{}{
						"type":  "work",
						"value": "888-5555",
					},
				},
			},
			want: personnel_sync.Person{
				CompareValue: "email@example.com",
				Attributes: map[string]string{
					"email": "email@example.com",
					"phone": "888-5555",
				},
			},
		},
		{
			name: `only "desk" locations`,
			user: admin.User{
				PrimaryEmail: "email@example.com",
				Locations: []interface{}{
					map[string]interface{}{
						"area":       "Custom area",
						"buildingId": "Custom building",
						"type":       "custom",
					},
					map[string]interface{}{
						"area":       "An area",
						"buildingId": "A building",
						"type":       "desk",
					},
				},
			},
			want: personnel_sync.Person{
				CompareValue: "email@example.com",
				Attributes: map[string]string{
					"email":    "email@example.com",
					"area":     "An area",
					"building": "A building",
				},
			},
		},
		{
			name: "invalid data types",
			user: admin.User{
				ExternalIds: []interface{}{map[string]interface{}{
					"type":  "organization",
					"value": 12345,
				}},
				Locations: []interface{}{map[string]interface{}{
					"type":       "desk",
					"area":       1.0,
					"buildingId": 1,
				}},
				Organizations: []interface{}{map[string]interface{}{
					"costCenter": []string{"A cost center"},
					"department": true,
					"title":      map[string]string{"key": "value"},
				}},
				Phones: []interface{}{map[string]interface{}{
					"type":  "work",
					"value": 5551212,
				}},
				PrimaryEmail: "email@example.com",
				Relations: []interface{}{map[string]interface{}{
					"type":  "manager",
					"value": []string{"manager@example.com"},
				}},
			},
			want: personnel_sync.Person{
				CompareValue: "email@example.com",
				Attributes: map[string]string{
					"email": "email@example.com",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractData(tt.user); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractData() = %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}
