package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/silinternational/personnel-sync/v4/alert"
	"github.com/silinternational/personnel-sync/v4/restapi"

	"github.com/silinternational/personnel-sync/v4/webhelpdesk"

	"github.com/silinternational/personnel-sync/v4/google"

	personnel_sync "github.com/silinternational/personnel-sync/v4"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(0)
	log.Printf("Personnel sync started at %s", time.Now().UTC().Format(time.RFC1123Z))

	appConfig, err := personnel_sync.LoadConfig("")
	if err != nil {
		msg := fmt.Sprintf("Unable to load config, error: %s", err)
		log.Println(msg)
		alert.SendEmail(appConfig.Alert, msg)
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
		err = errors.New("unrecognized source type")
	}

	if err != nil {
		msg := fmt.Sprintf("Unable to initialize %s source, error: %s", appConfig.Source.Type, err)
		log.Println(msg)
		alert.SendEmail(appConfig.Alert, msg)
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
	case personnel_sync.DestinationTypeRestAPI:
		destination, err = restapi.NewRestAPIDestination(appConfig.Destination)
	case personnel_sync.DestinationTypeWebHelpDesk:
		destination, err = webhelpdesk.NewWebHelpDeskDestination(appConfig.Destination)
	default:
		err = errors.New("unrecognized destination type")
	}

	if err != nil {
		msg := fmt.Sprintf("Unable to initialize %s destination, error: %s", appConfig.Destination.Type, err)
		log.Println(msg)
		alert.SendEmail(appConfig.Alert, msg)
		os.Exit(1)
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
			msg := fmt.Sprintf("Error setting source set on syncSet %s: %s", syncSet.Name, err)
			syncSetLogger.Println(msg)
			errors = append(errors, msg)
		}

		err = destination.ForSet(syncSet.Destination)
		if err != nil {
			msg := fmt.Sprintf("Error setting destination set on syncSet %s: %s", syncSet.Name, err)
			syncSetLogger.Println(msg)
			errors = append(errors, msg)
		}

		if err := personnel_sync.SyncPeople(syncSetLogger, source, destination, appConfig); err != nil {
			msg := fmt.Sprintf("Sync failed with error on syncSet %s: %s", syncSet.Name, err)
			syncSetLogger.Println(msg)
			errors = append(errors, msg)
		}
	}

	if len(errors) > 0 {
		alert.SendEmail(appConfig.Alert, fmt.Sprintf("Sync error(s):\n%s", strings.Join(errors, "\n")))
	}

	log.Printf("Personnel sync completed at %s", time.Now().UTC().Format(time.RFC1123Z))
	os.Exit(0)
}
