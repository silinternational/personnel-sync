package google

import (
	"encoding/json"
	"reflect"
	"testing"

	sync "github.com/silinternational/personnel-sync/v3"
)

func TestNewGoogleSheetsDestination(t *testing.T) {
	const extraJSON = `{
      "DelegatedAdminEmail": "delegated-admin@example.com",
      "Domain": "example.com",
      "GoogleAuth": {
        "type": "service_account",
        "project_id": "abc-theme-123456",
        "private_key_id": "abc123",
        "private_key": "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
        "client_email": "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
        "client_id": "123456789012345678901",
        "auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "token_uri": "https://oauth2.googleapis.com/token",
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com"
      }
	}`

	tests := []struct {
		name              string
		destinationConfig sync.DestinationConfig
		want              GoogleSheets
		wantErr           bool
	}{
		{
			name: "test 1",
			destinationConfig: sync.DestinationConfig{
				Type:          sync.DestinationTypeGoogleSheets,
				DisableAdd:    true,
				DisableDelete: true,
				DisableUpdate: true,
				ExtraJSON:     json.RawMessage(extraJSON),
			},
			want: GoogleSheets{
				GoogleConfig: GoogleConfig{
					DelegatedAdminEmail: "delegated-admin@example.com",
					Domain:              "example.com",
					GoogleAuth: GoogleAuth{
						Type:                    "service_account",
						ProjectID:               "abc-theme-123456",
						PrivateKeyID:            "abc123",
						PrivateKey:              "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
						ClientEmail:             "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
						ClientID:                "123456789012345678901",
						AuthURI:                 "https://accounts.google.com/o/oauth2/auth",
						TokenURI:                "https://oauth2.googleapis.com/token",
						AuthProviderX509CertURL: "https://www.googleapis.com/oauth2/v1/certs",
						ClientX509CertURL:       "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "wrong type",
			destinationConfig: sync.DestinationConfig{
				Type:          sync.DestinationTypeGoogleGroups,
				DisableAdd:    true,
				DisableDelete: true,
				DisableUpdate: true,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewGoogleSheetsDestination(tt.destinationConfig)
			if tt.wantErr {
				if err == nil {
					t.Errorf("error expected, but didn't happen")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %s", err)
				return
			}
			g := got.(*GoogleSheets)
			if !reflect.DeepEqual(g.GoogleConfig, tt.want.GoogleConfig) {
				t.Errorf("incorrect GoogleConfig \ngot: %#v, \nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestNewGoogleSheetsSource(t *testing.T) {
	const extraJSON = `{
      "DelegatedAdminEmail": "delegated-admin@example.com",
      "Domain": "example.com",
      "GoogleAuth": {
        "type": "service_account",
        "project_id": "abc-theme-123456",
        "private_key_id": "abc123",
        "private_key": "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
        "client_email": "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
        "client_id": "123456789012345678901",
        "auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "token_uri": "https://oauth2.googleapis.com/token",
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com"
      }
	}`

	tests := []struct {
		name         string
		sourceConfig sync.SourceConfig
		want         GoogleSheets
		wantErr      bool
	}{
		{
			name: "test 1",
			sourceConfig: sync.SourceConfig{
				Type:      sync.SourceTypeGoogleSheets,
				ExtraJSON: json.RawMessage(extraJSON),
			},
			want: GoogleSheets{
				GoogleConfig: GoogleConfig{
					DelegatedAdminEmail: "delegated-admin@example.com",
					Domain:              "example.com",
					GoogleAuth: GoogleAuth{
						Type:                    "service_account",
						ProjectID:               "abc-theme-123456",
						PrivateKeyID:            "abc123",
						PrivateKey:              "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
						ClientEmail:             "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
						ClientID:                "123456789012345678901",
						AuthURI:                 "https://accounts.google.com/o/oauth2/auth",
						TokenURI:                "https://oauth2.googleapis.com/token",
						AuthProviderX509CertURL: "https://www.googleapis.com/oauth2/v1/certs",
						ClientX509CertURL:       "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "wrong type",
			sourceConfig: sync.SourceConfig{
				Type: sync.SourceTypeRestAPI,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewGoogleSheetsSource(tt.sourceConfig)
			if tt.wantErr {
				if err == nil {
					t.Errorf("error expected, but didn't happen")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %s", err)
				return
			}
			g := got.(*GoogleSheets)
			if !reflect.DeepEqual(g.GoogleConfig, tt.want.GoogleConfig) {
				t.Errorf("incorrect GoogleConfig \ngot: %#v, \nwant: %#v", got, tt.want)
			}
		})
	}
}

func Test_getPersonsFromSheetData(t *testing.T) {
	tests := []struct {
		name         string
		sheetData    [][]interface{}
		desiredAttrs []string
		want         []sync.Person
	}{
		{
			name:         "empty sheet data",
			sheetData:    [][]interface{}{},
			desiredAttrs: []string{"a", "b"},
			want:         []sync.Person{},
		},
		{
			name: "only a header row",
			sheetData: [][]interface{}{
				{"a", "b", "c"},
			},
			desiredAttrs: []string{"a", "b"},
			want:         []sync.Person{},
		},
		{
			name: "no desired attributes",
			sheetData: [][]interface{}{
				{"a", "b", "c"},
				{"valueA", "valueB", "valueC"},
			},
			desiredAttrs: []string{},
			want: []sync.Person{{
				Attributes:     map[string]string{},
				DisableChanges: false,
			}},
		},
		{
			name: "one row",
			sheetData: [][]interface{}{
				{"a", "b", "c"},
				{"valueA", "valueB", "valueC"},
			},
			desiredAttrs: []string{"a", "b"},
			want: []sync.Person{{
				ID: "",
				Attributes: map[string]string{
					"a": "valueA",
					"b": "valueB",
				},
				DisableChanges: false,
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPersonsFromSheetData(tt.sheetData, tt.desiredAttrs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPersonsFromSheetData() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestGoogleSheets_getHeaderFromSheetData(t *testing.T) {
	tests := []struct {
		name string
		data [][]interface{}
		want map[string]int
	}{
		{
			name: "empty sheet data",
			data: [][]interface{}{},
			want: map[string]int{},
		},
		{
			name: "one column",
			data: [][]interface{}{
				{"a"},
			},
			want: map[string]int{
				"a": 0,
			},
		},
		{
			name: "two columns, separated by a an empty column",
			data: [][]interface{}{
				{"a", "", "c"},
			},
			want: map[string]int{
				"a": 0,
				"":  1,
				"c": 2,
			},
		},
		{
			name: "two columns with same value",
			data: [][]interface{}{
				{"a", "a"},
			},
			want: map[string]int{
				"a": 0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getHeaderFromSheetData(tt.data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getHeaderFromSheetData() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_makeSheetDataFromPersons(t *testing.T) {
	tests := []struct {
		name    string
		header  map[string]int
		persons []sync.Person
		want    [][]interface{}
	}{
		{
			name:    "empty input",
			header:  map[string]int{},
			persons: []sync.Person{},
			want:    [][]interface{}{},
		},
		{
			name:   "empty header",
			header: map[string]int{},
			persons: []sync.Person{
				{
					CompareValue:   "",
					ID:             "",
					Attributes:     map[string]string{"a": "valueA"},
					DisableChanges: false,
				},
			},
			want: [][]interface{}{},
		},
		{
			name:    "empty persons list",
			header:  map[string]int{"a": 0},
			persons: []sync.Person{},
			want:    [][]interface{}{},
		},
		{
			name:   "2 persons, 2 attributes",
			header: map[string]int{"a": 0, "b": 1},
			persons: []sync.Person{
				{
					CompareValue:   "",
					ID:             "",
					Attributes:     map[string]string{"a": "valueA1", "b": "valueB1"},
					DisableChanges: false,
				},
				{
					CompareValue:   "",
					ID:             "",
					Attributes:     map[string]string{"a": "valueA2", "b": "valueB2"},
					DisableChanges: false,
				},
			},
			want: [][]interface{}{
				{"valueA1", "valueB1"},
				{"valueA2", "valueB2"},
			},
		},
		{
			name:   "extra header column",
			header: map[string]int{"a": 0, "b": 1, "c": 2},
			persons: []sync.Person{
				{
					CompareValue:   "",
					ID:             "",
					Attributes:     map[string]string{"a": "valueA1", "b": "valueB1"},
					DisableChanges: false,
				},
				{
					CompareValue:   "",
					ID:             "",
					Attributes:     map[string]string{"a": "valueA2", "b": "valueB2"},
					DisableChanges: false,
				},
			},
			want: [][]interface{}{
				{"valueA1", "valueB1"},
				{"valueA2", "valueB2"},
			},
		},
		{
			name:   "unused attribute",
			header: map[string]int{"a": 0, "b": 1},
			persons: []sync.Person{
				{
					CompareValue:   "",
					ID:             "",
					Attributes:     map[string]string{"a": "valueA1", "b": "valueB1", "c": "valueC1"},
					DisableChanges: false,
				},
			},
			want: [][]interface{}{
				{"valueA1", "valueB1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := makeSheetDataFromPersons(tt.header, tt.persons); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeSheetDataFromPersons() = %v, want %v", got, tt.want)
			}
		})
	}
}
