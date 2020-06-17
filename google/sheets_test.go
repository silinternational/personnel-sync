package google

import (
	"encoding/json"
	"reflect"
	"testing"

	personnel_sync "github.com/silinternational/personnel-sync/v3"
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
		destinationConfig personnel_sync.DestinationConfig
		want              GoogleSheets
		wantErr           bool
	}{
		{
			name: "test 1",
			destinationConfig: personnel_sync.DestinationConfig{
				Type:          personnel_sync.DestinationTypeGoogleSheets,
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
			destinationConfig: personnel_sync.DestinationConfig{
				Type:          personnel_sync.DestinationTypeGoogleGroups,
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
		sourceConfig personnel_sync.SourceConfig
		want         GoogleSheets
		wantErr      bool
	}{
		{
			name: "test 1",
			sourceConfig: personnel_sync.SourceConfig{
				Type:      personnel_sync.SourceTypeGoogleSheets,
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
			sourceConfig: personnel_sync.SourceConfig{
				Type: personnel_sync.SourceTypeRestAPI,
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

func TestGoogleSheets_getHeader(t *testing.T) {
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
			g := GoogleSheets{}
			if got := g.getHeader(tt.data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getHeader() = %v, want %v", got, tt.want)
			}
		})
	}
}
