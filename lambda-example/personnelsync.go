package main

import (
	"fmt"
	"log"
	"os"
	"time"

	personnel_sync "github.com/silinternational/personnel-sync/v4"
	"github.com/silinternational/personnel-sync/v4/google"
	"github.com/silinternational/personnel-sync/v4/restapi"
	"github.com/silinternational/personnel-sync/v4/webhelpdesk"

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
		log.Println("Unable to load config, error: ", err.Error())
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
		log.Printf("Unable to initialize %s source, error: %s", appConfig.Source.Type, err.Error())
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
		log.Println("Unable to load config, error: ", err.Error())
		return err
	}

	maxNameLength := appConfig.MaxSyncSetNameLength()

	// Iterate through SyncSets and process changes
	for i, syncSet := range appConfig.SyncSets {
		prefix := fmt.Sprintf("[%-*s] ", maxNameLength, syncSet.Name)
		syncSetLogger := log.New(os.Stdout, prefix, 0)
		syncSetLogger.Printf("(%v/%v) Beginning sync set", i+1, len(appConfig.SyncSets))

		// Apply SyncSet configs (excluding source/destination as appropriate)
		err = source.ForSet(syncSet.Source)
		if err != nil {
			syncSetLogger.Printf("Error setting source set: %s", err.Error())
		}

		err = destination.ForSet(syncSet.Destination)
		if err != nil {
			syncSetLogger.Printf("Error setting destination set: %s", err.Error())
		}

		if err := personnel_sync.SyncPeople(syncSetLogger, source, destination, appConfig); err != nil {
			syncSetLogger.Printf("Failed with error: %s", err)
		}
	}

	return nil
}
