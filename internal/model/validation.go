package model

import (
	"fmt"
	"strings"
)

// FieldError represents a single validation error on a named field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors collects one or more field-level validation errors.
type ValidationErrors struct {
	errors []FieldError
}

// NewValidationErrors creates an empty ValidationErrors.
func NewValidationErrors() *ValidationErrors {
	return &ValidationErrors{}
}

// Add appends a field error.
func (v *ValidationErrors) Add(field, message string) {
	v.errors = append(v.errors, FieldError{Field: field, Message: message})
}

// HasErrors reports whether any validation errors have been recorded.
func (v *ValidationErrors) HasErrors() bool {
	return len(v.errors) > 0
}

// Fields returns the collected field errors.
func (v *ValidationErrors) Fields() []FieldError {
	return v.errors
}

// Error implements the error interface.
func (v *ValidationErrors) Error() string {
	if len(v.errors) == 0 {
		return "validation failed"
	}
	msgs := make([]string, len(v.errors))
	for i, e := range v.errors {
		msgs[i] = fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return "validation failed: " + strings.Join(msgs, "; ")
}

// OrNil returns nil if there are no errors, or the receiver otherwise.
func (v *ValidationErrors) OrNil() error {
	if !v.HasErrors() {
		return nil
	}
	return v
}
