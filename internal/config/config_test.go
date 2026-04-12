package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.Server.Addr != ":8080" {
		t.Errorf("Server.Addr = %s, want :8080", cfg.Server.Addr)
	}
	if cfg.Server.DisableUI {
		t.Error("DisableUI should default to false")
	}
	if cfg.Server.DisableAPI {
		t.Error("DisableAPI should default to false")
	}
	if cfg.Server.DisableRegistration {
		t.Error("DisableRegistration should default to false")
	}
	if cfg.Session.DurationHours != 720 {
		t.Errorf("Session.DurationHours = %d, want 720", cfg.Session.DurationHours)
	}
	if cfg.Session.SecureCookie {
		t.Error("SecureCookie should default to false")
	}
	if cfg.Argon2.Time != 1 {
		t.Errorf("Argon2.Time = %d, want 1", cfg.Argon2.Time)
	}
	if cfg.Argon2.Memory != 65536 {
		t.Errorf("Argon2.Memory = %d, want 65536", cfg.Argon2.Memory)
	}
	if cfg.Argon2.Threads != 4 {
		t.Errorf("Argon2.Threads = %d, want 4", cfg.Argon2.Threads)
	}
	if cfg.Argon2.KeyLength != 32 {
		t.Errorf("Argon2.KeyLength = %d, want 32", cfg.Argon2.KeyLength)
	}
	if cfg.Argon2.SaltLen != 16 {
		t.Errorf("Argon2.SaltLen = %d, want 16", cfg.Argon2.SaltLen)
	}
	if cfg.TMDB.APIKey != "" {
		t.Errorf("TMDB.APIKey should default to empty, got %s", cfg.TMDB.APIKey)
	}
	if cfg.TMDB.DefaultRegion != "NL" {
		t.Errorf("TMDB.DefaultRegion = %s, want NL", cfg.TMDB.DefaultRegion)
	}
	if cfg.TMDB.MetadataSyncInterval != "24h" {
		t.Errorf("TMDB.MetadataSyncInterval = %s, want 24h", cfg.TMDB.MetadataSyncInterval)
	}
	if cfg.TMDB.KeywordScanInterval != "24h" {
		t.Errorf("TMDB.KeywordScanInterval = %s, want 24h", cfg.TMDB.KeywordScanInterval)
	}
}

func TestSessionDuration(t *testing.T) {
	cfg := Config{Session: SessionConfig{DurationHours: 48}}
	want := 48 * time.Hour
	if got := cfg.SessionDuration(); got != want {
		t.Errorf("SessionDuration() = %v, want %v", got, want)
	}
}

func TestMetadataSyncDuration(t *testing.T) {
	cfg := Config{TMDB: TMDBConfig{MetadataSyncInterval: "12h"}}
	want := 12 * time.Hour
	if got := cfg.MetadataSyncDuration(); got != want {
		t.Errorf("MetadataSyncDuration() = %v, want %v", got, want)
	}
}

func TestMetadataSyncDuration_Invalid(t *testing.T) {
	cfg := Config{TMDB: TMDBConfig{MetadataSyncInterval: "garbage"}}
	want := 24 * time.Hour
	if got := cfg.MetadataSyncDuration(); got != want {
		t.Errorf("MetadataSyncDuration() with invalid input = %v, want %v (fallback)", got, want)
	}
}

func TestKeywordScanDuration(t *testing.T) {
	cfg := Config{TMDB: TMDBConfig{KeywordScanInterval: "6h"}}
	want := 6 * time.Hour
	if got := cfg.KeywordScanDuration(); got != want {
		t.Errorf("KeywordScanDuration() = %v, want %v", got, want)
	}
}

func TestKeywordScanDuration_Invalid(t *testing.T) {
	cfg := Config{TMDB: TMDBConfig{KeywordScanInterval: "garbage"}}
	want := 24 * time.Hour
	if got := cfg.KeywordScanDuration(); got != want {
		t.Errorf("KeywordScanDuration() with invalid input = %v, want %v (fallback)", got, want)
	}
}

func TestDecodeTOML_AllFields(t *testing.T) {
	input := `
[server]
addr = ":9090"
disable_ui = true
disable_api = true
disable_registration = true

[database]
path = "/tmp/test.db"

[session]
duration_hours = 24
secure_cookie = true

[argon2]
time = 3
memory = 131072
threads = 8
key_length = 64
salt_length = 32

[tmdb]
api_key = "test-key-123"
default_region = "US"
metadata_sync_interval = "12h"
keyword_scan_interval = "6h"
`
	cfg := Defaults()
	if err := decodeTOML(input, &cfg); err != nil {
		t.Fatalf("decodeTOML() error: %v", err)
	}
	if cfg.Server.Addr != ":9090" {
		t.Errorf("Server.Addr = %s, want :9090", cfg.Server.Addr)
	}
	if !cfg.Server.DisableUI {
		t.Error("Server.DisableUI should be true")
	}
	if !cfg.Server.DisableAPI {
		t.Error("Server.DisableAPI should be true")
	}
	if !cfg.Server.DisableRegistration {
		t.Error("Server.DisableRegistration should be true")
	}
	if cfg.DB.Path != "/tmp/test.db" {
		t.Errorf("DB.Path = %s, want /tmp/test.db", cfg.DB.Path)
	}
	if cfg.Session.DurationHours != 24 {
		t.Errorf("Session.DurationHours = %d, want 24", cfg.Session.DurationHours)
	}
	if !cfg.Session.SecureCookie {
		t.Error("Session.SecureCookie should be true")
	}
	if cfg.Argon2.Time != 3 {
		t.Errorf("Argon2.Time = %d, want 3", cfg.Argon2.Time)
	}
	if cfg.Argon2.Memory != 131072 {
		t.Errorf("Argon2.Memory = %d, want 131072", cfg.Argon2.Memory)
	}
	if cfg.Argon2.Threads != 8 {
		t.Errorf("Argon2.Threads = %d, want 8", cfg.Argon2.Threads)
	}
	if cfg.Argon2.KeyLength != 64 {
		t.Errorf("Argon2.KeyLength = %d, want 64", cfg.Argon2.KeyLength)
	}
	if cfg.Argon2.SaltLen != 32 {
		t.Errorf("Argon2.SaltLen = %d, want 32", cfg.Argon2.SaltLen)
	}
	if cfg.TMDB.APIKey != "test-key-123" {
		t.Errorf("TMDB.APIKey = %s, want test-key-123", cfg.TMDB.APIKey)
	}
	if cfg.TMDB.DefaultRegion != "US" {
		t.Errorf("TMDB.DefaultRegion = %s, want US", cfg.TMDB.DefaultRegion)
	}
	if cfg.TMDB.MetadataSyncInterval != "12h" {
		t.Errorf("TMDB.MetadataSyncInterval = %s, want 12h", cfg.TMDB.MetadataSyncInterval)
	}
	if cfg.TMDB.KeywordScanInterval != "6h" {
		t.Errorf("TMDB.KeywordScanInterval = %s, want 6h", cfg.TMDB.KeywordScanInterval)
	}
}

func TestDecodeTOML_Comments(t *testing.T) {
	input := `
# This is a full-line comment
[server]
addr = ":7070"
`
	cfg := Defaults()
	if err := decodeTOML(input, &cfg); err != nil {
		t.Fatalf("decodeTOML() error: %v", err)
	}
	if cfg.Server.Addr != ":7070" {
		t.Errorf("Server.Addr = %s, want :7070", cfg.Server.Addr)
	}
}

func TestDecodeTOML_InlineComments_UnquotedValues(t *testing.T) {
	input := `
[session]
duration_hours = 48  # comment
`
	cfg := Defaults()
	if err := decodeTOML(input, &cfg); err != nil {
		t.Fatalf("decodeTOML() error: %v", err)
	}
	if cfg.Session.DurationHours != 48 {
		t.Errorf("DurationHours = %d, want 48", cfg.Session.DurationHours)
	}
}

func TestDecodeTOML_InlineComments_QuotedValues(t *testing.T) {
	input := `
[server]
addr = ":7070"  # inline comment
`
	cfg := Defaults()
	if err := decodeTOML(input, &cfg); err != nil {
		t.Fatalf("decodeTOML() error: %v", err)
	}
	if cfg.Server.Addr != ":7070" {
		t.Errorf("Server.Addr = %q, want :7070", cfg.Server.Addr)
	}
}

func TestDecodeTOML_QuotedValues(t *testing.T) {
	input := `
[database]
path = "/path/with spaces/db.sqlite"
`
	cfg := Defaults()
	if err := decodeTOML(input, &cfg); err != nil {
		t.Fatalf("decodeTOML() error: %v", err)
	}
	if cfg.DB.Path != "/path/with spaces/db.sqlite" {
		t.Errorf("DB.Path = %s, want /path/with spaces/db.sqlite", cfg.DB.Path)
	}
}

func TestDecodeTOML_InvalidSyntax(t *testing.T) {
	input := `
[server]
this line has no equals sign
`
	cfg := Defaults()
	err := decodeTOML(input, &cfg)
	if err == nil {
		t.Error("decodeTOML() should return error for invalid syntax")
	}
}

func TestDecodeTOML_Empty(t *testing.T) {
	cfg := Defaults()
	if err := decodeTOML("", &cfg); err != nil {
		t.Fatalf("decodeTOML() should handle empty input: %v", err)
	}
	if cfg.Server.Addr != ":8080" {
		t.Errorf("Server.Addr = %s, want :8080 (default)", cfg.Server.Addr)
	}
}

func TestDecodeTOML_PartialConfig(t *testing.T) {
	input := `
[session]
duration_hours = 48
`
	cfg := Defaults()
	if err := decodeTOML(input, &cfg); err != nil {
		t.Fatalf("decodeTOML() error: %v", err)
	}
	if cfg.Session.DurationHours != 48 {
		t.Errorf("Session.DurationHours = %d, want 48", cfg.Session.DurationHours)
	}
	if cfg.Server.Addr != ":8080" {
		t.Errorf("Server.Addr = %s, want :8080 (default)", cfg.Server.Addr)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	got := expandHome("~/foo/bar")
	want := filepath.Join(home, "foo/bar")
	if got != want {
		t.Errorf("expandHome(~/foo/bar) = %s, want %s", got, want)
	}
}

func TestExpandHome_NoTilde(t *testing.T) {
	got := expandHome("/absolute/path")
	if got != "/absolute/path" {
		t.Errorf("expandHome should not modify paths without ~/: got %s", got)
	}
}
