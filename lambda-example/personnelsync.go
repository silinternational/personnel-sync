package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	personnel_sync "github.com/silinternational/personnel-sync/v5"
	"github.com/silinternational/personnel-sync/v5/alert"
	"github.com/silinternational/personnel-sync/v5/google"
	"github.com/silinternational/personnel-sync/v5/restapi"
	"github.com/silinternational/personnel-sync/v5/webhelpdesk"

	"github.com/aws/aws-lambda-go/lambda"
)

type LambdaConfig struct {
	ConfigPath string
}

func main() {
	lambda.Start(handler)
}

func handler(lambdaConfig LambdaConfig) error {
	// Log to stdout and remove leading date/time stamps from each log entry (Cloudwatch Logs will add these)
	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	now := time.Now().UTC()
	log.Printf("Personnel sync started at %s", now.Format(time.RFC1123Z))

	appConfig, err := personnel_sync.LoadConfig(lambdaConfig.ConfigPath)
	if err != nil {
		msg := fmt.Sprintf("Unable to load config, error: %s", err)
		log.Println(msg)
		alert.SendEmail(appConfig.Alert, msg)
		return err
	}

	// Instantiate Source
	var source personnel_sync.Source
	switch appConfig.Source.Type {
	case personnel_sync.SourceTypeRestAPI:
		source, err = restapi.NewRestAPISource(appConfig.Source)
	case personnel_sync.SourceTypeGoogleSheets:
		source, err = google.NewGoogleSheetsSource(appConfig.Source)
	default:
		source = &personnel_sync.EmptySource{}
	}

	if err != nil {
		msg := fmt.Sprintf("Unable to initialize %s source, error: %s", appConfig.Source.Type, err)
		log.Println(msg)
		alert.SendEmail(appConfig.Alert, msg)
		return err
	}

	// Instantiate Destination
	var destination personnel_sync.Destination
	switch appConfig.Destination.Type {
	case personnel_sync.DestinationTypeGoogleContacts:
		destination, err = google.NewGoogleContactsDestination(appConfig.Destination)
	case personnel_sync.DestinationTypeGoogleGroups:
		destination, err = google.NewGoogleGroupsDestination(appConfig.Destination)
	case personnel_sync.DestinationTypeGoogleSheets:
		destination, err = google.NewGoogleSheetsDestination(appConfig.Destination)
	case personnel_sync.DestinationTypeGoogleUsers:
		destination, err = google.NewGoogleUsersDestination(appConfig.Destination)
	case personnel_sync.DestinationTypeRestAPI:
		destination, err = restapi.NewRestAPIDestination(appConfig.Destination)
	case personnel_sync.DestinationTypeWebHelpDesk:
		destination, err = webhelpdesk.NewWebHelpDeskDestination(appConfig.Destination)
	default:
		destination = &personnel_sync.EmptyDestination{}
	}

	if err != nil {
		msg := fmt.Sprintf("Unable to initialize %s destination, error: %s", appConfig.Destination.Type, err)
		log.Println(msg)
		alert.SendEmail(appConfig.Alert, msg)
		return err
	}

	maxNameLength := appConfig.MaxSyncSetNameLength()
	var errors []string

	// Iterate through SyncSets and process changes
	for i, syncSet := range appConfig.SyncSets {
		prefix := fmt.Sprintf("[%-*s] ", maxNameLength, syncSet.Name)
		syncSetLogger := log.New(os.Stdout, prefix, 0)
		syncSetLogger.Printf("(%v/%v) Beginning sync set", i+1, len(appConfig.SyncSets))

		// Apply SyncSet configs (excluding source/destination as appropriate)
		err = source.ForSet(syncSet.Source)
		if err != nil {
			msg := fmt.Sprintf(`Error setting source set on syncSet "%s": %s`, syncSet.Name, err)
			syncSetLogger.Println(msg)
			errors = append(errors, msg)
		}

		err = destination.ForSet(syncSet.Destination)
		if err != nil {
			msg := fmt.Sprintf(`Error setting destination set on syncSet "%s": %s`, syncSet.Name, err)
			syncSetLogger.Println(msg)
			errors = append(errors, msg)
		}

		if err := personnel_sync.SyncPeople(syncSetLogger, source, destination, appConfig); err != nil {
			msg := fmt.Sprintf(`Sync failed with error on syncSet "%s": %s`, syncSet.Name, err)
			syncSetLogger.Println(msg)
			errors = append(errors, msg)
		}
	}

	if len(errors) > 0 {
		alert.SendEmail(appConfig.Alert, fmt.Sprintf("Sync error(s):\n%s", strings.Join(errors, "\n")))
	}

	return nil
}
