package model

import (
	"strings"
	"testing"
)

func TestUserValidate_Valid(t *testing.T) {
	u := User{Username: "alice", Password: "securepassword"}
	if err := u.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestUserValidate_EmptyUsername(t *testing.T) {
	u := User{Username: "", Password: "securepassword"}
	assertValidationField(t, u.Validate(), "username")
}

func TestUserValidate_WhitespaceUsername(t *testing.T) {
	u := User{Username: "   ", Password: "securepassword"}
	assertValidationField(t, u.Validate(), "username")
}

func TestUserValidate_EmptyPassword(t *testing.T) {
	u := User{Username: "alice", Password: ""}
	assertValidationField(t, u.Validate(), "password")
}

func TestUserValidate_ShortPassword(t *testing.T) {
	u := User{Username: "alice", Password: "short"}
	assertValidationField(t, u.Validate(), "password")
}

func TestUserValidate_ExactMinPassword(t *testing.T) {
	u := User{Username: "alice", Password: strings.Repeat("a", 8)}
	if err := u.Validate(); err != nil {
		t.Fatalf("8-char password should be valid, got %v", err)
	}
}

func TestUserValidate_InvalidRole(t *testing.T) {
	u := User{Username: "alice", Password: "securepassword", Role: "superadmin"}
	assertValidationField(t, u.Validate(), "role")
}

func TestUserValidate_ValidRoles(t *testing.T) {
	for _, role := range ValidUserRoles {
		u := User{Username: "alice", Password: "securepassword", Role: role}
		if err := u.Validate(); err != nil {
			t.Errorf("role %q should be valid, got %v", role, err)
		}
	}
}

func TestUserValidate_EmptyRoleAllowed(t *testing.T) {
	u := User{Username: "alice", Password: "securepassword", Role: ""}
	if err := u.Validate(); err != nil {
		t.Fatalf("empty role should be allowed (set by service), got %v", err)
	}
}

func TestUserValidate_MultipleErrors(t *testing.T) {
	u := User{}
	err := u.Validate()
	ve, ok := err.(*ValidationErrors)
	if !ok {
		t.Fatalf("expected *ValidationErrors, got %T", err)
	}
	if len(ve.Fields()) < 2 {
		t.Errorf("expected at least 2 field errors, got %d", len(ve.Fields()))
	}
}

func TestUserSettings_Validate_Valid(t *testing.T) {
	s := &UserSettings{Locale: "en", Theme: "dark", Region: "NL"}
	if err := s.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestUserSettings_Validate_Locale(t *testing.T) {
	for _, loc := range []string{"en", "de", "nl"} {
		s := &UserSettings{Locale: loc, Theme: "dark"}
		if err := s.Validate(); err != nil {
			t.Errorf("locale %q should be valid, got: %v", loc, err)
		}
	}
	s := &UserSettings{Locale: "fr", Theme: "dark"}
	assertValidationField(t, s.Validate(), "locale")
}

func TestUserSettings_Validate_Theme(t *testing.T) {
	for _, theme := range []string{"light", "dark", "oled"} {
		s := &UserSettings{Locale: "en", Theme: theme}
		if err := s.Validate(); err != nil {
			t.Errorf("theme %q should be valid, got: %v", theme, err)
		}
	}
	s := &UserSettings{Locale: "en", Theme: "sepia"}
	assertValidationField(t, s.Validate(), "theme")
}

func TestUserSettings_Validate_Region(t *testing.T) {
	s := &UserSettings{Locale: "en", Theme: "dark", Region: "US"}
	if err := s.Validate(); err != nil {
		t.Fatalf("2-letter region should be valid, got %v", err)
	}

	s = &UserSettings{Locale: "en", Theme: "dark", Region: ""}
	if err := s.Validate(); err != nil {
		t.Fatalf("empty region should be valid, got %v", err)
	}

	s = &UserSettings{Locale: "en", Theme: "dark", Region: "USA"}
	assertValidationField(t, s.Validate(), "region")
}

func TestUserSettings_Validate_MultipleErrors(t *testing.T) {
	s := &UserSettings{Locale: "xx", Theme: "xx", Region: "XXX"}
	err := s.Validate()
	ve, ok := err.(*ValidationErrors)
	if !ok {
		t.Fatalf("expected *ValidationErrors, got %T", err)
	}
	if len(ve.Fields()) < 3 {
		t.Errorf("expected at least 3 field errors, got %d", len(ve.Fields()))
	}
}
