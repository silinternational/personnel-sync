package main

import (
	"log"
	"os"
	"time"

	personnel_sync "github.com/silinternational/personnel-sync/v3"
	"github.com/silinternational/personnel-sync/v3/googledest"
	"github.com/silinternational/personnel-sync/v3/restapi"
	"github.com/silinternational/personnel-sync/v3/webhelpdesk"

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
		if err != nil {
			log.Println("Unable to initialize RestAPI source, error: ", err.Error())
			return err
		}
	default:
		source = &personnel_sync.EmptySource{}
	}

	// Instantiate Destination
	var destination personnel_sync.Destination
	switch appConfig.Destination.Type {
	case personnel_sync.DestinationTypeGoogleContacts:
		destination, err = googledest.NewGoogleContactsDestination(appConfig.Destination)
	case personnel_sync.DestinationTypeGoogleGroups:
		destination, err = googledest.NewGoogleGroupsDestination(appConfig.Destination)
	case personnel_sync.DestinationTypeGoogleSheets:
		destination, err = googledest.NewGoogleSheetsDestination(appConfig.Destination)
	case personnel_sync.DestinationTypeGoogleUsers:
		destination, err = googledest.NewGoogleUsersDestination(appConfig.Destination)
	case personnel_sync.DestinationTypeWebHelpDesk:
		destination, err = webhelpdesk.NewWebHelpDeskDestination(appConfig.Destination)
	default:
		destination = &personnel_sync.EmptyDestination{}
	}

	if err != nil {
		log.Println("Unable to load config, error: ", err.Error())
		return err
	}

	// Iterate through SyncSets and process changes
	for i, syncSet := range appConfig.SyncSets {
		log.Printf("%v/%v: Beginning sync set: %s\n", i+1, len(appConfig.SyncSets), syncSet.Name)

		// Apply SyncSet configs (excluding source/destination as appropriate)
		err = source.ForSet(syncSet.Source)
		if err != nil {
			log.Printf("Error setting source set: %s", err.Error())
		}

		err = destination.ForSet(syncSet.Destination)
		if err != nil {
			log.Printf("Error setting destination set: %s", err.Error())
		}

		// Perform sync and get results
		changeResults := personnel_sync.SyncPeople(source, destination, appConfig)

		log.Printf("Sync results: %v users added, %v users updated, %v users removed, %v errors\n",
			changeResults.Created, changeResults.Updated, changeResults.Deleted, len(changeResults.Errors))

		if len(changeResults.Errors) > 0 {
			log.Println("Errors:")
			for _, msg := range changeResults.Errors {
				log.Printf("  %s\n", msg)
			}
			return err
		}
	}

	return nil
}
