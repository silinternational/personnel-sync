package internal

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"

	"github.com/silinternational/personnel-sync/v6/alert"
)

type Config struct {
	Runtime      RuntimeConfig
	Source       SourceConfig
	Destination  DestinationConfig
	Alert        alert.Config
	AttributeMap []AttributeMap
	SyncSets     []SyncSet
}

func NewConfig() Config {
	return Config{
		Runtime: RuntimeConfig{
			Verbosity: DefaultVerbosity,
		},
	}
}

// LoadConfig looks for a config file if one is provided. Otherwise, it looks for
// a config file based on the CONFIG_PATH env var.  If that is not set, it gets
// the default config file ("./config.json").
func LoadConfig(configFile string) ([]byte, error) {
	if configFile == "" {
		configFile = os.Getenv("CONFIG_PATH")
		if configFile == "" {
			configFile = DefaultConfigFile
		}
	}

	log.Printf("Using config file: %s\n", configFile)

	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Printf("unable to read application config file %s, error: %s\n", configFile, err.Error())
		return nil, err
	}
	return data, err
}

// ReadConfig parses raw json config data into a Config struct
func ReadConfig(data []byte) (Config, error) {
	config := NewConfig()
	err := json.Unmarshal(data, &config)
	if err != nil {
		log.Printf("unable to unmarshal application configuration file data, error: %s\n", err.Error())
		return config, err
	}

	if config.Source.Type == "" {
		return config, errors.New("configuration appears to be missing a Source configuration")
	}

	if config.Destination.Type == "" {
		return config, errors.New("configuration appears to be missing a Destination configuration")
	}

	if len(config.AttributeMap) == 0 {
		return config, errors.New("configuration appears to be missing an AttributeMap")
	}

	log.Printf("Configuration loaded. Source type: %s, Destination type: %s\n", config.Source.Type, config.Destination.Type)
	log.Printf("%v Sync sets found:\n", len(config.SyncSets))

	for i, syncSet := range config.SyncSets {
		log.Printf("  %v) %s\n", i+1, syncSet.Name)
	}

	return config, nil
}

func (a *Config) MaxSyncSetNameLength() int {
	maxLength := 0
	for _, set := range a.SyncSets {
		if maxLength < len(set.Name) {
			maxLength = len(set.Name)
		}
	}
	return maxLength
}
