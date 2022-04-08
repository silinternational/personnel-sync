package internal

import (
	"net/url"
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
// Otherwise, it adds each param to the url as `&param[0]=param[1]` (in alphabetical order)
func AddParamsToURL(inURL string, params [][2]string) string {
	if len(params) == 0 {
		return inURL
	}

	parsed, _ := url.Parse(inURL)
	q, _ := url.ParseQuery(parsed.RawQuery)

	for _, p := range params {
		q.Add(p[0], p[1])
	}

	parsed.RawQuery = q.Encode()

	return parsed.String()
}
