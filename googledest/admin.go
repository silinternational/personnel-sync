package googledest

import (
	"encoding/json"
	"fmt"

	"google.golang.org/api/option"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
)

type GoogleAuth struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri"`
	TokenURI                string `json:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	ClientX509CertURL       string `json:"client_x509_cert_url"`
}

// initGoogleAdminService authenticates with the Google API and returns an admin.Service
//  that has the requested scopes
func initGoogleAdminService(auth GoogleAuth, adminEmail string, scopes ...string) (admin.Service, error) {
	googleAuthJson, err := json.Marshal(auth)
	if err != nil {
		return admin.Service{}, fmt.Errorf("unable to marshal google auth data into json, error: %s", err.Error())
	}

	config, err := google.JWTConfigFromJSON(googleAuthJson, scopes...)
	if err != nil {
		return admin.Service{}, fmt.Errorf("unable to parse client secret file to config: %s", err)
	}

	ctx := context.TODO()
	config.Subject = adminEmail
	client := config.Client(ctx)

	adminService, err := admin.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return admin.Service{}, fmt.Errorf("unable to retrieve directory Service: %s", err)
	}

	return *adminService, nil
}
