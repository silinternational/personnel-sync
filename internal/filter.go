package internal

import (
	"fmt"
	"regexp"
)

type Filters []Filter

type Filter struct {
	Attribute          string
	Expression         string
	Exclude            bool
	compiledExpression *regexp.Regexp
	Required           bool
}

func (f Filter) Matches(value string) bool {
	return f.Exclude != f.compiledExpression.MatchString(value)
}

func (f *Filters) Validate() error {
	for i := range *f {
		expression := (*f)[i].Expression
		re, err := regexp.Compile(expression)
		if err != nil {
			return fmt.Errorf("invalid filter expression %s: %w", expression, err)
		}
		(*f)[i].compiledExpression = re
	}
	return nil
}
