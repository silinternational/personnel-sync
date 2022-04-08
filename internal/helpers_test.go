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
			got := AddStringToSlice(tt.newString, tt.slice)
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

func TestAddParamsToURL(t *testing.T) {
	tests := []struct {
		name            string
		testURL         string
		params          [][2]string
		wantErrContains string
		want            string
	}{
		{
			name:            "bad url",
			testURL:         "https://example.org:1111portplusotherstuff",
			params:          [][2]string{{"size", "1"}},
			wantErrContains: "error parsing url in AddParamsToURL",
		},
		{
			name:            "missing param key",
			testURL:         "https://example.org",
			params:          [][2]string{{"", "NoKey"}, {"count", "10"}},
			wantErrContains: "missing param key for index 0. Has value: NoKey",
		},
		{
			name:            "missing param value",
			testURL:         "https://example.org",
			params:          [][2]string{{"count", "10"}, {"NoVal", ""}},
			wantErrContains: "missing param value for index 1. Has key: NoVal",
		},
		{
			name:    "no params at all",
			testURL: "https://example.org",
			params:  [][2]string{},
			want:    "https://example.org",
		},
		{
			name:    "no new params",
			testURL: "https://example.org?size=1&limit=2",
			params:  [][2]string{},
			want:    "https://example.org?size=1&limit=2",
		},
		{
			name:    "no starting params",
			testURL: "https://example.org",
			params:  [][2]string{{"size", "2"}},
			want:    "https://example.org?size=2",
		},
		{
			name:    "params everywhere",
			testURL: "https://example.org?alpha=111&bravo=222",
			params:  [][2]string{{"size", "3"}, {"limit", "4"}},
			want:    "https://example.org?alpha=111&bravo=222&limit=4&size=3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AddParamsToURL(tt.testURL, tt.params)

			if tt.wantErrContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErrContains, "incorrect error")
				return
			}

			require.Equal(t, tt.want, got)
		})
	}
}
