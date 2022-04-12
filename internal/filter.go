package internal

import (
	"fmt"
	"regexp"
)

type Filters []Filter

type Filter struct {
	Attribute          string
	Expression         string
	compiledExpression *regexp.Regexp
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
