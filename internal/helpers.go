package internal

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
