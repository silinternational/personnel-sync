package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddStringToSlice(t *testing.T) {
	tests := []struct {
		name      string
		slice     []string
		newString string
		want      []string
	}{
		{
			name:      "nil",
			slice:     nil,
			newString: "new",
			want:      []string{"new"},
		},
		{
			name:      "empty",
			slice:     []string{},
			newString: "new",
			want:      []string{"new"},
		},
		{
			name:      "not empty",
			slice:     []string{"old"},
			newString: "new",
			want:      []string{"old", "new"},
		},
		{
			name:      "duplicate",
			slice:     []string{"old"},
			newString: "old",
			want:      []string{"old"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AddStringToSlice(tt.slice, tt.newString)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIsStringInSlice(t *testing.T) {
	type testData struct {
		name     string
		needle   string
		haystack []string
		want     bool
	}

	allTestData := []testData{
		{
			name:     "empty haystack",
			needle:   "no",
			haystack: []string{},
			want:     false,
		},
		{
			name:     "not in haystack",
			needle:   "no",
			haystack: []string{"really", "are you sure"},
			want:     false,
		},
		{
			name:     "in one element haystack",
			needle:   "yes",
			haystack: []string{"yes"},
			want:     true,
		},
		{
			name:     "in longer haystack",
			needle:   "yes",
			haystack: []string{"one", "two", "three", "yes"},
			want:     true,
		},
	}

	for i, td := range allTestData {
		t.Run(td.name, func(t *testing.T) {
			got := IsStringInSlice(td.needle, td.haystack)
			require.Equal(t, td.want, got, "incorrect value for test %v", i)
		})
	}
}
