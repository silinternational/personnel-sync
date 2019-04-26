package main

import (
	"log"
	"os"

	"github.com/silinternational/personnel-sync/restapi"

	"github.com/silinternational/personnel-sync/webhelpdesk"

	"github.com/silinternational/personnel-sync/googledest"

	"github.com/silinternational/personnel-sync"
)

func main() {
	appConfig, err := personnel_sync.LoadConfig("")
	if err != nil {
		log.Println("Unable to load config, error: ", err.Error())
		os.Exit(1)
	}

	for i, syncSet := range appConfig.SyncSets {
		log.Printf("%v/%v: Beginning sync set: %s. Source type: %s, Destination type: %s\n",
			i, len(appConfig.SyncSets), syncSet.Name, syncSet.Source.Type, syncSet.Destination.Type)

		// Instantiate Source
		var source personnel_sync.Source
		switch syncSet.Source.Type {
		case personnel_sync.SourceTypeRestAPI:
			source, err = restapi.NewRestAPISource(syncSet.Source)
		default:
			source = &personnel_sync.EmptySource{}
		}

		// Instantiate Destination
		var destination personnel_sync.Destination
		switch syncSet.Destination.Type {
		case personnel_sync.DestinationTypeGoogleGroups:
			destination, err = googledest.NewGoogleGroupsDesination(syncSet.Destination)
			if err != nil {
				log.Println("Unable to load config, error: ", err.Error())
				os.Exit(1)
			}
		case personnel_sync.DestinationTypeWebHelpDesk:
			destination, err = webhelpdesk.NewWebHelpDeskDesination(syncSet.Destination)
			if err != nil {
				log.Println("Unable to load config, error: ", err.Error())
				os.Exit(1)
			}
		default:
			destination = &personnel_sync.EmptyDestination{}
		}

		// Perform sync and get results
		changeResults := personnel_sync.SyncPeople(source, destination, syncSet.DestinationAttributeMap)

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

	os.Exit(0)
}
