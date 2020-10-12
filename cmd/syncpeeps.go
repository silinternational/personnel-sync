package main

import (
	"os"

	"github.com/silinternational/personnel-sync/v5"
)

func main() {
	if err := personnel_sync.RunSync(""); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
