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
