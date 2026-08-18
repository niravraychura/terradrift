// Package validation provides safe, typed option-validation errors.
package validation

import "fmt"

// Error identifies an invalid public option without retaining its value.
type Error struct {
	Field string
	Err   error
}

// Error implements error.
func (err *Error) Error() string {
	return fmt.Sprintf("invalid %s: %v", err.Field, err.Err)
}

// Unwrap returns the underlying validation cause.
func (err *Error) Unwrap() error { return err.Err }

// New returns a typed invalid-option error.
func New(field string, err error) error {
	return &Error{Field: field, Err: err}
}
