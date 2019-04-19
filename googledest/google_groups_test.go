package googledest

import (
	"reflect"
	"testing"

	"github.com/silinternational/personnel-sync"
	admin "google.golang.org/api/admin/directory/v1"
)

func TestGoogleGroups_ApplyChangeSet(t *testing.T) {
	t.Skip("Skipping test because it requires integration with Google")
	t.SkipNow()

	testConfig, err := personnel_sync.LoadConfig("")
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
			name: "expect two created, one deleted",
			want: personnel_sync.ChangeResults{
				Created: uint64(0),
				Deleted: uint64(1),
			},
			fields: fields{
				DestinationConfig: testConfig.Destination,
			},
			args: args{
				changes: personnel_sync.ChangeSet{
					Delete: []personnel_sync.Person{},
					Create: []personnel_sync.Person{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := NewGoogleGroupsDesination(tt.fields.DestinationConfig)
			if err != nil {
				t.Errorf("Failed to get new googleGroups instance, error: %s", err.Error())
				t.FailNow()
			}
			if got := g.ApplyChangeSet(tt.args.changes); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GoogleGroups.ApplyChangeSet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGoogleGroups_ListUsers(t *testing.T) {
	t.Skip("Skipping test because it requires integration with Google")
	t.SkipNow()

	testConfig, err := personnel_sync.LoadConfig("")
	if err != nil {
		t.Errorf("Failed to load test config, error: %s", err.Error())
		t.FailNow()
	}

	type fields struct {
		DestinationConfig  personnel_sync.DestinationConfig
		GoogleGroupsConfig GoogleGroupsConfig
		AdminService       admin.Service
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
			want:    []personnel_sync.Person{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := NewGoogleGroupsDesination(tt.fields.DestinationConfig)
			if err != nil {
				t.Errorf("Failed to get new googleGroups instance, error: %s", err.Error())
				t.FailNow()
			}
			got, err := g.ListUsers()
			if (err != nil) != tt.wantErr {
				t.Errorf("GoogleGroups.ListUsers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GoogleGroups.ListUsers() = %v, want %v", got, tt.want)
			}
		})
	}
}
