package internal

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilter_Matches(t *testing.T) {
	re, err := regexp.Compile("abc123")
	require.NoError(t, err, "test configuration might be bad")

	positiveFilter := Filter{
		Expression:         "abc123",
		Exclude:            false,
		compiledExpression: re,
	}
	excludeFilter := Filter{
		Expression:         "abc123",
		Exclude:            true,
		compiledExpression: re,
	}

	tests := []struct {
		name   string
		value  string
		filter Filter
		want   bool
	}{
		{
			name:   "positive match",
			value:  "abc123",
			filter: positiveFilter,
			want:   true,
		},
		{
			name:   "exclude match",
			value:  "xyz789",
			filter: excludeFilter,
			want:   true,
		},
		{
			name:   "positive mismatch",
			value:  "xyz789",
			filter: positiveFilter,
			want:   false,
		},
		{
			name:   "exclude mismatch",
			value:  "abc123",
			filter: excludeFilter,
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.Matches(tt.value)
			require.Equal(t, tt.want, got)
		})
	}
}

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
