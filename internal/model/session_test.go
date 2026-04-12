package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSession_StructFields(t *testing.T) {
	now := time.Now()
	s := Session{
		ID:         "sess-1",
		UserID:     "user-1",
		Token:      "secret-token",
		CreatedAt:  now,
		ExpiresAt:  now.Add(24 * time.Hour),
		LastSeenAt: now,
	}

	if s.ID != "sess-1" {
		t.Errorf("ID = %q, want %q", s.ID, "sess-1")
	}
	if s.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", s.UserID, "user-1")
	}
	if s.Token != "secret-token" {
		t.Errorf("Token = %q, want %q", s.Token, "secret-token")
	}
	if !s.ExpiresAt.After(s.CreatedAt) {
		t.Error("ExpiresAt should be after CreatedAt")
	}
}

func TestSession_TokenHiddenFromJSON(t *testing.T) {
	s := Session{
		ID:     "sess-1",
		UserID: "user-1",
		Token:  "secret-token",
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := m["token"]; ok {
		t.Error("Token should not appear in JSON output (json:\"-\" tag)")
	}
}
