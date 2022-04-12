package internal

import "fmt"

type Person struct {
	CompareValue   string
	ID             string
	Attributes     map[string]string
	DisableChanges bool
}

func (p *Person) Matches(filters Filters) (bool, error) {
	for _, f := range filters {
		value, ok := p.Attributes[f.Attribute]
		if !ok {
			return false, fmt.Errorf("attribute %s not present in person %s", f.Attribute, p.CompareValue)
		}
		if !f.compiledExpression.MatchString(value) {
			return false, nil
		}
	}
	return true, nil
}
