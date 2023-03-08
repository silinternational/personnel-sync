package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadConfig(t *testing.T) {
	goodConfig := NewConfig()
	goodConfig.Source.Type = "RestAPI"
	goodConfig.Destination.Type = "RestAPI"
	goodConfig.AttributeMap = []AttributeMap{{}}

	tests := []struct {
		name    string
		config  string
		want    Config
		wantErr bool
	}{
		{
			name:    "successful unmarshal",
			config:  `{"Source": {"Type": "RestAPI"},"Destination": {"Type": "RestAPI"},"AttributeMap":[{}]}`,
			want:    goodConfig,
			wantErr: false,
		},
		{
			name:    "json parse error",
			config:  `{"Runtime":{"Verbosity":"should be a number"}}`,
			wantErr: true,
		},
		{
			name:    "missing Source",
			config:  `{"Destination": {"Type": "RestAPI"},"AttributeMap":[{}]}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadConfig([]byte(tt.config))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{
			name:    "default config, missing some things",
			config:  NewConfig(),
			wantErr: "missing",
		},
		{
			name: "missing Source",
			config: Config{
				Destination:  DestinationConfig{Type: "RestAPI"},
				AttributeMap: []AttributeMap{{Required: false}},
			},
			wantErr: "missing a Source",
		},
		{
			name: "missing Destination",
			config: Config{
				Source:       SourceConfig{Type: "RestAPI"},
				AttributeMap: []AttributeMap{{Required: false}},
			},
			wantErr: "missing a Destination",
		},
		{
			name: "missing AttributeMap",
			config: Config{
				Destination:  DestinationConfig{Type: "RestAPI"},
				Source:       SourceConfig{Type: "RestAPI"},
				AttributeMap: []AttributeMap{},
			},
			wantErr: "missing an AttributeMap",
		},
		{
			name: "no error",
			config: Config{
				Destination:  DestinationConfig{Type: "RestAPI"},
				Source:       SourceConfig{Type: "RestAPI"},
				AttributeMap: []AttributeMap{{Required: false}},
			},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
