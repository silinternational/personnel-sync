package main

import (
	"log"
	"os"

	"github.com/silinternational/personnel-sync/googledest"

	"github.com/silinternational/personnel-sync"
)

func main() {
	appConfig, err := personnel_sync.LoadConfig("")
	if err != nil {
		log.Println("Unable to load config, error: ", err.Error())
		os.Exit(1)
	}

	var destination personnel_sync.Destination
	if appConfig.Destination.Type == personnel_sync.DestinationTypeGoogleGroups {
		destination, err = googledest.NewGoogleGroupsDesination(appConfig.Destination)
		if err != nil {
			log.Println("Unable to load config, error: ", err.Error())
			os.Exit(1)
		}
	} else {
		destination = &personnel_sync.EmptyDestination{}
	}

	changeResults := personnel_sync.SyncPeople(appConfig, destination)

	log.Printf("Sync results: %v users added, %v users updated, %v users removed, %v errors\n",
		changeResults.Created, changeResults.Updated, changeResults.Deleted, len(changeResults.Errors))

	if len(changeResults.Errors) > 0 {
		log.Println("Errors:")
		for _, msg := range changeResults.Errors {
			log.Printf("  %s\n", msg)
		}
		os.Exit(1)
	}

	os.Exit(0)
}
