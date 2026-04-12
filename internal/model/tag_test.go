package model

import "testing"

func TestTagValidate_Valid(t *testing.T) {
	tag := Tag{Name: "sci-fi"}
	if err := tag.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestTagValidate_EmptyName(t *testing.T) {
	tag := Tag{Name: ""}
	assertValidationField(t, tag.Validate(), "name")
}

func TestTagValidate_WhitespaceName(t *testing.T) {
	tag := Tag{Name: "   "}
	assertValidationField(t, tag.Validate(), "name")
}
