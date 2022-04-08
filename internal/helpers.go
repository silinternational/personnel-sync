package internal

import (
	"fmt"
	"net/url"
	"strings"
)

func AddStringToSlice(newString string, slice []string) []string {
	if !IsStringInSlice(newString, slice) {
		slice = append(slice, newString)
	}
	return slice
}

// IsStringInSlice iterates over a slice of strings, looking for the given
// string. If found, true is returned. Otherwise, false is returned.
func IsStringInSlice(needle string, haystack []string) bool {
	for _, hs := range haystack {
		if needle == hs {
			return true
		}
	}

	return false
}

// AddParamsToURL returns the input url string if there are no params to add
// Otherwise, it adds each param pair to the url as `params[n][0]=params[n][1]` (in alphabetical order)
//   with the appropriate delimiter ("?" or "&")
func AddParamsToURL(inURL string, params [][2]string) (string, error) {
	if len(params) == 0 {
		return inURL, nil
	}

	parsed, err := url.Parse(inURL)
	if err != nil {
		return "", fmt.Errorf("error parsing url in AddParamsToURL: %s", err)
	}

	q, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", fmt.Errorf("error parsing query in AddParamsToURL: %s", err)
	}

	for i, p := range params {
		key := p[0]
		val := p[1]
		if key == "" {
			return "", fmt.Errorf("missing param key for index %d. Has value: %s", i, val)
		}
		if val == "" {
			return "", fmt.Errorf("missing param value for index %d. Has key: %s", i, key)
		}
		q.Add(key, val)
	}

	parsed.RawQuery = q.Encode()

	return parsed.String(), nil
}

func JoinUrlPath(inURL, path string) string {
	const slash = "/"
	newURL := strings.TrimRight(inURL, slash)
	newPath := strings.TrimLeft(path, slash)
	return newURL + slash + newPath
}
