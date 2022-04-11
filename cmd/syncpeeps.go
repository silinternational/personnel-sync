package main

import (
	"os"

	personnel_sync "github.com/silinternational/personnel-sync/v6"
)

func main() {
	configFile := ""
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}
	if err := personnel_sync.RunSync(configFile); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
