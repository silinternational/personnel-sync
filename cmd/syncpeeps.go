package main

import (
	"log"
	"os"
	"time"

	"github.com/silinternational/personnel-sync/v3/restapi"

	"github.com/silinternational/personnel-sync/v3/webhelpdesk"

	"github.com/silinternational/personnel-sync/v3/google"

	personnel_sync "github.com/silinternational/personnel-sync/v3"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(0)
	log.Printf("Personnel sync started at %s", time.Now().UTC().Format(time.RFC1123Z))

	appConfig, err := personnel_sync.LoadConfig("")
	if err != nil {
		log.Println("Unable to load config, error: ", err.Error())
		os.Exit(1)
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
		log.Println("Unable to initialize source, error: ", err.Error())
		os.Exit(1)
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
	case personnel_sync.DestinationTypeWebHelpDesk:
		destination, err = webhelpdesk.NewWebHelpDeskDestination(appConfig.Destination)
	default:
		destination = &personnel_sync.EmptyDestination{}
	}

	if err != nil {
		log.Println("Unable to load config, error: ", err.Error())
		os.Exit(1)
	}

	// Iterate through SyncSets and process changes
	for i, syncSet := range appConfig.SyncSets {
		log.Printf("\n\n%v/%v: Beginning sync set: %s\n", i+1, len(appConfig.SyncSets), syncSet.Name)

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
			os.Exit(1)
		}
	}

	log.Printf("Personnel sync completed at %s", time.Now().UTC().Format(time.RFC1123Z))
	os.Exit(0)
}
