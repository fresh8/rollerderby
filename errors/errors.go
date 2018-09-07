package errors

import "strings"

// Errors encapsulates a collection of errors.
type Errors []error

func (errs Errors) String() string {
	var s []string
	for _, err := range errs {
		s = append(s, err.Error())
	}
	return strings.Join(s, "; ")
}
