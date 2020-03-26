package googledest

import (
	"encoding/json"
	"encoding/xml"
	"reflect"
	"strings"
	"testing"

	personnel_sync "github.com/silinternational/personnel-sync"
)

func TestNewGoogleContactsDestination(t *testing.T) {
	const extraJSON = `{
      "BatchSize": 5,
      "BatchDelaySeconds": 1,
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
		want              GoogleContacts
		wantErr           bool
	}{
		{
			name: "test 1",
			destinationConfig: personnel_sync.DestinationConfig{
				Type:      personnel_sync.DestinationTypeGoogleContacts,
				ExtraJSON: json.RawMessage(extraJSON),
			},
			want: GoogleContacts{
				GoogleContactsConfig: GoogleContactsConfig{
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
					BatchSize:         5,
					BatchDelaySeconds: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "wrong type",
			destinationConfig: personnel_sync.DestinationConfig{
				Type: personnel_sync.DestinationTypeGoogleGroups,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewGoogleContactsDestination(tt.destinationConfig)
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
			g := got.(*GoogleContacts)
			if !reflect.DeepEqual(g.GoogleContactsConfig, tt.want.GoogleContactsConfig) {
				t.Errorf("incorrect GoogleContactsConfig \ngot: %#v, \nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestGoogleContacts_extractPersonsFromResponse(t *testing.T) {
	tests := []struct {
		name     string
		contacts []Contact
		want     []personnel_sync.Person
		wantErr  bool
	}{
		{
			name: "no data",
			want: []personnel_sync.Person{},
		},
		{
			name: "one contact, all fields",
			contacts: []Contact{
				{
					XMLName: xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "entry"},
					Links: []Link{
						{
							XMLName: xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "link"},
							Rel:     "http://schemas.google.com/contacts/2008/rel#photo",
							Href:    "https://www.google.com/m8/feeds/photos/media/example.org/8a415cec04b8a4b8",
						},
						{
							XMLName: xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "link"},
							Rel:     "self",
							Href:    "https://www.google.com/m8/feeds/contacts/example.org/full/204e599dcd6d3605",
						},
						{
							XMLName: xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "link"},
							Rel:     "edit",
							Href:    "https://www.google.com/m8/feeds/contacts/example.org/full/204e599dcd6d3605/1585068827106000",
						},
					},
					Etag:  "686897696a7c876b7e",
					Title: "Alfred E. Newman",
					Name: Name{
						XMLName:    xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "name"},
						FullName:   "Alfred E. Newman",
						GivenName:  "Alfred",
						FamilyName: "Newman",
					},
					Emails: []Email{
						{
							XMLName: xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "email"},
							Address: "alfred@example.com", Primary: true,
						},
					},
					Organization: Organization{
						XMLName:        xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "organization"},
						Name:           "Mad Magazine",
						Title:          "Mascot",
						JobDescription: "Photo ops",
						Department:     "Marketing",
					},
				},
			},
			want: []personnel_sync.Person{
				{
					CompareValue: "alfred@example.com",
					ID:           "https://www.google.com/m8/feeds/contacts/example.org/full/204e599dcd6d3605",
					Attributes: map[string]string{
						"email":          "alfred@example.com",
						"fullName":       "Alfred E. Newman",
						"givenName":      "Alfred",
						"familyName":     "Newman",
						"id":             "https://www.google.com/m8/feeds/contacts/example.org/full/204e599dcd6d3605",
						"organization":   "Mad Magazine",
						"title":          "Mascot",
						"jobDescription": "Photo ops",
						"department":     "Marketing",
					},
					DisableChanges: false,
				},
			},
		},
		{
			name: "multiple contacts",
			contacts: []Contact{
				{
					Links:  []Link{{Rel: "self", Href: "https://www.google.com/m8/feeds/contacts/example.org/full/204e599dcd6d3605"}},
					Emails: []Email{{Address: "alfred@example.com", Primary: true}},
				},
				{
					Links:  []Link{{Rel: "self", Href: "https://www.google.com/m8/feeds/contacts/example.org/full/8f47da821e4824d8"}},
					Emails: []Email{{Address: "ironman@example.com", Primary: true}},
				},
			},
			want: []personnel_sync.Person{
				{
					CompareValue: "alfred@example.com",
					ID:           "https://www.google.com/m8/feeds/contacts/example.org/full/204e599dcd6d3605",
					Attributes: map[string]string{
						"email":          "alfred@example.com",
						"fullName":       "",
						"givenName":      "",
						"familyName":     "",
						"id":             "https://www.google.com/m8/feeds/contacts/example.org/full/204e599dcd6d3605",
						"organization":   "",
						"title":          "",
						"jobDescription": "",
						"department":     "",
					},
					DisableChanges: false,
				},
				{
					CompareValue: "ironman@example.com",
					ID:           "https://www.google.com/m8/feeds/contacts/example.org/full/8f47da821e4824d8",
					Attributes: map[string]string{
						"email":          "ironman@example.com",
						"fullName":       "",
						"givenName":      "",
						"familyName":     "",
						"id":             "https://www.google.com/m8/feeds/contacts/example.org/full/8f47da821e4824d8",
						"organization":   "",
						"title":          "",
						"jobDescription": "",
						"department":     "",
					},
					DisableChanges: false,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GoogleContacts{}
			got, err := g.extractPersonsFromResponse(tt.contacts)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractPersonsFromResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractPersonsFromResponse() \ngot: %#v, \nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestGoogleContacts_createBody(t *testing.T) {
	tests := []struct {
		name   string
		person personnel_sync.Person
		want   string
	}{
		{
			name:   "fullName",
			person: personnel_sync.Person{Attributes: map[string]string{"fullName": "Fred J. Smith"}},
			want:   "<gd:fullName>Fred J. Smith</gd:fullName>",
		},
		{
			name:   "givenName",
			person: personnel_sync.Person{Attributes: map[string]string{"givenName": "Fred"}},
			want:   "<gd:givenName>Fred</gd:givenName>",
		},
		{
			name:   "familyName",
			person: personnel_sync.Person{Attributes: map[string]string{"familyName": "Smith"}},
			want:   "<gd:familyName>Smith</gd:familyName>",
		},
		{
			name:   "email",
			person: personnel_sync.Person{Attributes: map[string]string{"email": "fred@example.com"}},
			want:   "<gd:email rel='http://schemas.google.com/g/2005#work' primary='true' address='fred@example.com'/>",
		},
		{
			name:   "organization",
			person: personnel_sync.Person{Attributes: map[string]string{"organization": "Acme, Inc."}},
			want:   "<gd:orgName>Acme, Inc.</gd:orgName>",
		},
		{
			name:   "department",
			person: personnel_sync.Person{Attributes: map[string]string{"department": "Operations"}},
			want:   "<gd:orgDepartment>Operations</gd:orgDepartment>",
		},
		{
			name:   "title",
			person: personnel_sync.Person{Attributes: map[string]string{"title": "VP of Operations"}},
			want:   "<gd:orgTitle>VP of Operations</gd:orgTitle>",
		},
		{
			name:   "jobDescription",
			person: personnel_sync.Person{Attributes: map[string]string{"jobDescription": "does important stuff"}},
			want:   "<gd:orgJobDescription>does important stuff</gd:orgJobDescription>",
		},
	}
	for _, tt := range tests {
		g := GoogleContacts{}
		t.Run(tt.name, func(t *testing.T) {
			body := g.createBody(tt.person)
			if !strings.Contains(body, tt.want) {
				t.Errorf(`no "%v" in body: \n%v`, tt.want, body)
			}
			if !strings.HasPrefix(body, `<atom:entry xmlns:atom='http://www.w3.org/2005/Atom' xmlns:gd='http://schemas.google.com/g/2005'>`) {
				t.Errorf("missing <atom:entry> tag")
			}
			if !strings.Contains(body, `<atom:category scheme='http://schemas.google.com/g/2005#kind' term='http://schemas.google.com/contact/2008#contact' />`) {
				t.Errorf("missing <atom:entry> tag")
			}
		})
	}
}
