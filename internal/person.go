package internal

import (
	"fmt"
	"strings"
)

type Person struct {
	CompareValue   string
	ID             string
	Attributes     map[string]string
	DisableChanges bool
}

func (p *Person) Matches(filters Filters) (bool, error) {
	var missingAttributes []string
	match := true
	for _, f := range filters {
		value, ok := p.Attributes[f.Attribute]
		if !ok {
			missingAttributes = append(missingAttributes, f.Attribute)
		}
		if !f.Matches(value) {
			match = false
		}
	}
	if len(missingAttributes) > 0 {
		return false, fmt.Errorf("attribute(s) %s not present in person %s",
			strings.Join(missingAttributes, ", "), p.CompareValue)
	}
	return match, nil
}
