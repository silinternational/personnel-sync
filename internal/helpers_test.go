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

func TestJoinUrlPath(t *testing.T) {
	tests := []struct {
		name    string
		testURL string
		path    string
		want    string
	}{
		{
			name:    "no slashes",
			testURL: "https://example.org",
			path:    "myPath",
			want:    "https://example.org/myPath",
		},
		{
			name:    "all slashes",
			testURL: "https://example.org///",
			path:    "///myPath",
			want:    "https://example.org/myPath",
		},
		{
			name:    "slashes on url",
			testURL: "https://example.org///",
			path:    "myPath",
			want:    "https://example.org/myPath",
		},
		{
			name:    "slashes on path",
			testURL: "https://example.org",
			path:    "///myPath",
			want:    "https://example.org/myPath",
		},
		{
			name:    "one slash on path",
			testURL: "https://example.org",
			path:    "/myPath",
			want:    "https://example.org/myPath",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JoinUrlPath(tt.testURL, tt.path)

			require.Equal(t, tt.want, got)
		})
	}
}
