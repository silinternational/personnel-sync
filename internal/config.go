package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"text/template"

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
		return nil, err
	}
	return data, err
}

// ReadConfig parses raw json config data into a Config struct
func ReadConfig(data []byte) (Config, error) {
	config := NewConfig()
	if d, err := substituteEnvVars(data); err == nil {
		if err = json.Unmarshal(d, &config); err != nil {
			return config, fmt.Errorf("config error: %w", err)
		}
	} else {
		return config, err
	}

	if err := config.Validate(); err != nil {
		return config, err
	}

	log.Printf("Configuration loaded. Source type: %s, Destination type: %s\n", config.Source.Type, config.Destination.Type)
	log.Printf("%v Sync sets found:\n", len(config.SyncSets))

	for i, syncSet := range config.SyncSets {
		log.Printf("  %v) %s\n", i+1, syncSet.Name)
	}

	return config, nil
}

func (c *Config) Validate() error {
	if c.Source.Type == "" {
		return errors.New("configuration appears to be missing a Source configuration")
	}

	if c.Destination.Type == "" {
		return errors.New("configuration appears to be missing a Destination configuration")
	}

	if len(c.AttributeMap) == 0 {
		return errors.New("configuration appears to be missing an AttributeMap")
	}
	return nil
}

func (c *Config) MaxSyncSetNameLength() int {
	maxLength := 0
	for _, set := range c.SyncSets {
		if maxLength < len(set.Name) {
			maxLength = len(set.Name)
		}
	}
	return maxLength
}

func substituteEnvVars(c []byte) ([]byte, error) {
	t, err := template.New("config").Option("missingkey=error").Parse(string(c))
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err = t.Execute(&b, getEnvMap()); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func getEnvMap() map[string]string {
	e := os.Environ()
	vars := make(map[string]string, len(e))
	for _, s := range e {
		spl := strings.SplitN(s, "=", 2)
		k, v := spl[0], spl[1]
		vars[k] = v
	}
	return vars
}
