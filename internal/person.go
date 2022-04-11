package internal

type Person struct {
	CompareValue   string
	ID             string
	Attributes     map[string]string
	DisableChanges bool
}

func (p *Person) Matches(filters Filters) bool {
	for _, f := range filters {
		value := p.Attributes[f.Attribute]
		if value == "" {
			return false
		}
		if !f.compiledExpression.MatchString(value) {
			return false
		}
	}
	return true
}
