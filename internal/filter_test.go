package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilters_Validate(t *testing.T) {
	tests := []struct {
		name    string
		f       Filters
		wantErr bool
	}{
		{
			name:    "empty list",
			f:       nil,
			wantErr: false,
		},
		{
			name: "single filter",
			f: Filters{Filter{
				Expression: "expr",
			}},
			wantErr: false,
		},
		{
			name: "two filters",
			f: Filters{
				Filter{Expression: "expr1"},
				Filter{Expression: "expr2"},
			},
			wantErr: false,
		},
		{
			name: "two filters, second is invalid",
			f: Filters{
				Filter{Expression: "expr1"},
				Filter{Expression: "("},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.f.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
