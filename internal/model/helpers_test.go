package model

import "testing"

// assertValidationField checks that err is a *ValidationErrors containing
// an error for the given field name.
func assertValidationField(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected validation error for %q, got nil", field)
	}
	ve, ok := err.(*ValidationErrors)
	if !ok {
		t.Fatalf("expected *ValidationErrors, got %T", err)
	}
	for _, fe := range ve.Fields() {
		if fe.Field == field {
			return
		}
	}
	t.Errorf("expected validation error for field %q, got: %v", field, ve)
}
