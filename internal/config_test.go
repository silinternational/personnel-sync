package internal

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadConfig(t *testing.T) {
	goodConfig := NewConfig()
	goodConfig.Source.Type = "RestAPI"
	goodConfig.Destination.Type = "RestAPI"
	goodConfig.AttributeMap = []AttributeMap{{}}

	err := os.Setenv("SOURCE_PASSWORD", "srcPw")
	require.NoError(t, err, "problem with test setup")
	err = os.Setenv("DESTINATION_PASSWORD", "destPw")
	require.NoError(t, err, "problem with test setup")

	substConfig := goodConfig
	substConfig.Source.ExtraJSON = json.RawMessage(`{"Password":"srcPw"}`)
	substConfig.Destination.ExtraJSON = json.RawMessage(`{"Password":"destPw"}`)

	tests := []struct {
		name    string
		config  string
		want    Config
		wantErr bool
	}{
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
		{
			name:    "successful unmarshal",
			config:  `{"Source": {"Type": "RestAPI"},"Destination": {"Type": "RestAPI"},"AttributeMap":[{}]}`,
			want:    goodConfig,
			wantErr: false,
		},
		{
			name: "successful with env variable substitution",
			config: `{
	"Source": {
		"Type": "RestAPI",
		"ExtraJSON": {"Password":"{{.SOURCE_PASSWORD}}"}
	},
	"Destination": { 
		"Type": "RestAPI",
		"ExtraJSON": {"Password":"{{.DESTINATION_PASSWORD}}"}
	},
	"AttributeMap": [{}]
}`,
			want:    substConfig,
			wantErr: false,
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

func Test_substituteEnvVars(t *testing.T) {
	const tokenValue = "abc123!@#"
	err := os.Setenv("token", tokenValue)
	require.NoError(t, err, "problem with test setup")

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "invalid template",
			input:   `{"token":"{{invalid}}"}`,
			wantErr: true,
		},
		{
			name:    "undefined variable",
			input:   `{"token":"{{.undefined}}"}`,
			wantErr: true,
		},
		{
			name:    "no substitutions",
			input:   "{}",
			want:    "{}",
			wantErr: false,
		},
		{
			name:    "simple substitution",
			input:   `{"token":"{{.token}}"}`,
			want:    `{"token":"` + tokenValue + `"}`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := substituteEnvVars([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, string(got))
		})
	}
}

func Test_getEnvMap(t *testing.T) {
	err := os.Setenv("test_key1", "test_value1")
	require.NoError(t, err, "problem with test setup")
	err = os.Setenv(" test_key2 ", " test_value2 ")
	require.NoError(t, err, "problem with test setup")

	want := map[string]string{"test_key1": "test_value1", "test_key2": "test_value2"}
	got := getEnvMap()

	require.Equal(t, want["test_key1"], got["test_key1"])
	require.Equal(t, want["test_key2"], got["test_key2"])
}
