package i18n

import "testing"

func TestT_EnglishKey(t *testing.T) {
	got := T(LangEN, "nav.library")
	if got != "Library" {
		t.Errorf("T(en, nav.library) = %q, want Library", got)
	}
}

func TestT_GermanKey(t *testing.T) {
	got := T(LangDE, "nav.library")
	if got != "Bibliothek" {
		t.Errorf("T(de, nav.library) = %q, want Bibliothek", got)
	}
}

func TestT_DutchKey(t *testing.T) {
	got := T(LangNL, "nav.library")
	if got != "Bibliotheek" {
		t.Errorf("T(nl, nav.library) = %q, want Bibliotheek", got)
	}
}

func TestT_FallbackToEnglish(t *testing.T) {
	got := T(Lang("xx"), "nav.library")
	if got != "Library" {
		t.Errorf("T(xx, nav.library) = %q, want Library (English fallback)", got)
	}
}

func TestT_UnknownKey_ReturnsKey(t *testing.T) {
	got := T(LangEN, "nonexistent.key")
	if got != "nonexistent.key" {
		t.Errorf("T(en, nonexistent.key) = %q, want nonexistent.key", got)
	}
}

func TestParseLang_Simple(t *testing.T) {
	tests := []struct {
		input string
		want  Lang
	}{
		{"de", LangDE},
		{"nl", LangNL},
		{"en", LangEN},
		{"fr", LangEN}, // unsupported → English
		{"", LangEN},
	}
	for _, tt := range tests {
		if got := ParseLang(tt.input); got != tt.want {
			t.Errorf("ParseLang(%q) = %s, want %s", tt.input, got, tt.want)
		}
	}
}

func TestParseLang_WithRegion(t *testing.T) {
	tests := []struct {
		input string
		want  Lang
	}{
		{"de-DE", LangDE},
		{"de_AT", LangDE},
		{"nl-NL", LangNL},
		{"en-US", LangEN},
		{"en_GB", LangEN},
	}
	for _, tt := range tests {
		if got := ParseLang(tt.input); got != tt.want {
			t.Errorf("ParseLang(%q) = %s, want %s", tt.input, got, tt.want)
		}
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	tests := []struct {
		header string
		want   Lang
	}{
		{"de-DE,de;q=0.9,en;q=0.8", LangDE},
		{"nl,en;q=0.5", LangNL},
		{"en-US,en;q=0.9", LangEN},
		{"fr-FR,fr;q=0.9,en;q=0.8", LangEN}, // fr unsupported, falls through
		{"", LangEN},
	}
	for _, tt := range tests {
		if got := ParseAcceptLanguage(tt.header); got != tt.want {
			t.Errorf("ParseAcceptLanguage(%q) = %s, want %s", tt.header, got, tt.want)
		}
	}
}

func TestRegister_MergesTranslations(t *testing.T) {
	Register(Lang("test"), map[string]string{"key1": "val1"})
	Register(Lang("test"), map[string]string{"key2": "val2"})

	if got := T(Lang("test"), "key1"); got != "val1" {
		t.Errorf("key1 = %q, want val1", got)
	}
	if got := T(Lang("test"), "key2"); got != "val2" {
		t.Errorf("key2 = %q, want val2", got)
	}
}

func TestSupportedLangs(t *testing.T) {
	if len(SupportedLangs) != 3 {
		t.Errorf("len(SupportedLangs) = %d, want 3", len(SupportedLangs))
	}
}
