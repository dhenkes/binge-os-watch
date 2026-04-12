package model

import "testing"

func TestNewValidationErrors(t *testing.T) {
	v := NewValidationErrors()
	if v == nil {
		t.Fatal("NewValidationErrors returned nil")
	}
	if v.HasErrors() {
		t.Error("new ValidationErrors should have no errors")
	}
	if len(v.Fields()) != 0 {
		t.Error("new ValidationErrors should have empty Fields()")
	}
}

func TestValidationErrors_Add(t *testing.T) {
	v := NewValidationErrors()
	v.Add("field1", "msg1")

	if !v.HasErrors() {
		t.Error("HasErrors should return true after Add")
	}
	if len(v.Fields()) != 1 {
		t.Fatalf("expected 1 field error, got %d", len(v.Fields()))
	}
	if v.Fields()[0].Field != "field1" || v.Fields()[0].Message != "msg1" {
		t.Errorf("unexpected field error: %+v", v.Fields()[0])
	}
}

func TestValidationErrors_AddMultiple(t *testing.T) {
	v := NewValidationErrors()
	v.Add("a", "err a")
	v.Add("b", "err b")
	v.Add("c", "err c")

	if len(v.Fields()) != 3 {
		t.Fatalf("expected 3 field errors, got %d", len(v.Fields()))
	}
}

func TestValidationErrors_Error_Empty(t *testing.T) {
	v := NewValidationErrors()
	if v.Error() != "validation failed" {
		t.Errorf("unexpected error string: %s", v.Error())
	}
}

func TestValidationErrors_Error_WithFields(t *testing.T) {
	v := NewValidationErrors()
	v.Add("name", "must not be empty")
	v.Add("age", "must be positive")

	want := "validation failed: name: must not be empty; age: must be positive"
	if v.Error() != want {
		t.Errorf("got %q, want %q", v.Error(), want)
	}
}

func TestValidationErrors_OrNil_NoErrors(t *testing.T) {
	v := NewValidationErrors()
	if v.OrNil() != nil {
		t.Error("OrNil should return nil when no errors")
	}
}

func TestValidationErrors_OrNil_WithErrors(t *testing.T) {
	v := NewValidationErrors()
	v.Add("x", "bad")

	err := v.OrNil()
	if err == nil {
		t.Fatal("OrNil should return non-nil when errors exist")
	}

	ve, ok := err.(*ValidationErrors)
	if !ok {
		t.Fatal("OrNil should return *ValidationErrors")
	}
	if len(ve.Fields()) != 1 {
		t.Errorf("expected 1 field error, got %d", len(ve.Fields()))
	}
}
