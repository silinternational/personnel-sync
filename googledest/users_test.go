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

func Test_newUserForUpdate(t *testing.T) {
	tests := []struct {
		name   string
		person personnel_sync.Person
		want   admin.User
	}{
		{
			name: "basic",
			person: personnel_sync.Person{
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
			want: admin.User{
				ExternalIds: []admin.UserExternalId{{
					Type:  "organization",
					Value: "12345",
				}},
				Locations: []admin.UserLocation{{
					Area:       "An area",
					BuildingId: "A building",
					Type:       "desk",
				}},
				Name: &admin.UserName{
					FamilyName: "Jones",
					GivenName:  "John",
				},
				Organizations: []admin.UserOrganization{{
					CostCenter: "A cost center",
					Department: "A department",
					Title:      "A title",
				}},
				Phones: []admin.UserPhone{{
					Type:  "work",
					Value: "555-1212",
				}},
				Relations: []admin.UserRelation{{
					Type:  "manager",
					Value: "manager@example.com",
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := newUserForUpdate(tt.person, admin.User{}); err != nil {
				t.Errorf("newUserForUpdate() error: %s", err)
			} else if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newUserForUpdate() = %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func Test_updateIDs(t *testing.T) {
	tests := []struct {
		name   string
		newID  string
		oldIDs interface{}
		want   []admin.UserExternalId
	}{
		{
			name:  "organization and custom",
			newID: "12345",
			oldIDs: []interface{}{
				map[string]interface{}{
					"type":  "organization",
					"value": "00000",
				},
				map[string]interface{}{
					"type":       "custom",
					"customType": "foo",
					"value":      "abcdef",
				},
			},
			want: []admin.UserExternalId{
				{
					Type:  "organization",
					Value: "12345",
				},
				{
					Type:       "custom",
					Value:      "abcdef",
					CustomType: "foo",
				},
			},
		},
		{
			name:  "organization only",
			newID: "12345",
			oldIDs: []interface{}{
				map[string]interface{}{
					"type":  "organization",
					"value": "00000",
				},
			},
			want: []admin.UserExternalId{
				{
					Type:  "organization",
					Value: "12345",
				},
			},
		},
		{
			name:  "custom only",
			newID: "12345",
			oldIDs: []interface{}{
				map[string]interface{}{
					"type":       "custom",
					"customType": "foo",
					"value":      "abcdef",
				},
			},
			want: []admin.UserExternalId{
				{
					Type:  "organization",
					Value: "12345",
				},
				{
					Type:       "custom",
					Value:      "abcdef",
					CustomType: "foo",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := updateIDs(tt.newID, tt.oldIDs); err != nil {
				t.Errorf("updateIDs() error: %s", err)
			} else if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updateIDs():\n%+v\nwant:\n%+v", got, tt.want)
			}
		})
	}
}

func Test_updateLocations(t *testing.T) {
	tests := []struct {
		name         string
		newArea      string
		newBuilding  string
		oldLocations interface{}
		want         []admin.UserLocation
	}{
		{
			name:        "desk and custom",
			newArea:     "Area 2",
			newBuilding: "Bldg 2",
			oldLocations: []interface{}{
				map[string]interface{}{
					"type":       "desk",
					"area":       "Area 1",
					"buildingId": "Bldg 1",
				},
				map[string]interface{}{
					"type":         "custom",
					"customType":   "foo",
					"area":         "Area A",
					"buildingId":   "Bldg B",
					"deskCode":     "deskCode",
					"floorName":    "floorName",
					"floorSection": "floorSection",
				},
			},
			want: []admin.UserLocation{
				{
					Type:       "desk",
					Area:       "Area 2",
					BuildingId: "Bldg 2",
				},
				{
					Type:         "custom",
					CustomType:   "foo",
					Area:         "Area A",
					BuildingId:   "Bldg B",
					DeskCode:     "deskCode",
					FloorName:    "floorName",
					FloorSection: "floorSection",
				},
			},
		},
		{
			name:        "desk only",
			newArea:     "Area 2",
			newBuilding: "Bldg 2",
			oldLocations: []interface{}{
				map[string]interface{}{
					"type":       "desk",
					"area":       "Area 1",
					"buildingId": "Bldg 1",
				},
			},
			want: []admin.UserLocation{
				{
					Type:       "desk",
					Area:       "Area 2",
					BuildingId: "Bldg 2",
				},
			},
		},
		{
			name:        "custom only",
			newArea:     "Area 2",
			newBuilding: "Bldg 2",
			oldLocations: []interface{}{
				map[string]interface{}{
					"type":         "custom",
					"customType":   "foo",
					"area":         "Area A",
					"buildingId":   "Bldg B",
					"deskCode":     "deskCode",
					"floorName":    "floorName",
					"floorSection": "floorSection",
				},
			},
			want: []admin.UserLocation{
				{
					Type:       "desk",
					Area:       "Area 2",
					BuildingId: "Bldg 2",
				},
				{
					Type:         "custom",
					CustomType:   "foo",
					Area:         "Area A",
					BuildingId:   "Bldg B",
					DeskCode:     "deskCode",
					FloorName:    "floorName",
					FloorSection: "floorSection",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := updateLocations(tt.newArea, tt.newBuilding, tt.oldLocations); err != nil {
				t.Errorf("updateLocations() error: %s", err)
			} else if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updateLocations():\n%+v\nwant:\n%+v", got, tt.want)
			}
		})
	}
}

func Test_updatePhones(t *testing.T) {
	tests := []struct {
		name      string
		newPhone  string
		oldPhones interface{}
		want      []admin.UserPhone
	}{
		{
			name:     "work and custom",
			newPhone: "555-1212",
			oldPhones: []interface{}{
				map[string]interface{}{
					"type":  "work",
					"value": "222-333-4444",
				},
				map[string]interface{}{
					"type":       "custom",
					"customType": "foo",
					"value":      "999-111-2222",
					"primary":    true,
				},
			},
			want: []admin.UserPhone{
				{
					Type:  "work",
					Value: "555-1212",
				},
				{
					Type:       "custom",
					CustomType: "foo",
					Value:      "999-111-2222",
					Primary:    true,
				},
			},
		},
		{
			name:     "work only",
			newPhone: "555-1212",
			oldPhones: []interface{}{
				map[string]interface{}{
					"type":  "work",
					"value": "222-333-4444",
				},
			},
			want: []admin.UserPhone{
				{
					Type:  "work",
					Value: "555-1212",
				},
			},
		},
		{
			name:     "custom only",
			newPhone: "555-1212",
			oldPhones: []interface{}{
				map[string]interface{}{
					"type":       "custom",
					"customType": "foo",
					"value":      "999-111-2222",
					"primary":    true,
				},
			},
			want: []admin.UserPhone{
				{
					Type:  "work",
					Value: "555-1212",
				},
				{
					Type:       "custom",
					CustomType: "foo",
					Value:      "999-111-2222",
					Primary:    true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := updatePhones(tt.newPhone, tt.oldPhones); err != nil {
				t.Errorf("updatePhones() error: %s", err)
			} else if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updatePhones():\n%+v\nwant:\n%+v", got, tt.want)
			}
		})
	}
}

func Test_updateRelations(t *testing.T) {
	tests := []struct {
		name         string
		newRelation  string
		oldRelations interface{}
		want         []admin.UserRelation
	}{
		{
			name:        "manager and custom",
			newRelation: "new_manager@example.com",
			oldRelations: []interface{}{
				map[string]interface{}{
					"type":  "manager",
					"value": "old_manager@example.com",
				},
				map[string]interface{}{
					"type":       "custom",
					"customType": "foo",
					"value":      "other@example.com",
				},
			},
			want: []admin.UserRelation{
				{
					Type:  "manager",
					Value: "new_manager@example.com",
				},
				{
					Type:       "custom",
					CustomType: "foo",
					Value:      "other@example.com",
				},
			},
		},
		{
			name:        "manager only",
			newRelation: "new_manager@example.com",
			oldRelations: []interface{}{
				map[string]interface{}{
					"type":  "manager",
					"value": "old_manager@example.com",
				},
			},
			want: []admin.UserRelation{
				{
					Type:  "manager",
					Value: "new_manager@example.com",
				},
			},
		},
		{
			name:        "custom only",
			newRelation: "new_manager@example.com",
			oldRelations: []interface{}{
				map[string]interface{}{
					"type":       "custom",
					"customType": "foo",
					"value":      "other@example.com",
				},
			},
			want: []admin.UserRelation{
				{
					Type:  "manager",
					Value: "new_manager@example.com",
				},
				{
					Type:       "custom",
					CustomType: "foo",
					Value:      "other@example.com",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := updateRelations(tt.newRelation, tt.oldRelations); err != nil {
				t.Errorf("updateRelations() error: %s", err)
			} else if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updateRelations():\n%+v\nwant:\n%+v", got, tt.want)
			}
		})
	}
}
