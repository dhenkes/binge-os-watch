// Package i18n provides translations for user-facing content.
// It imports no other internal packages.
//
// Translations are registered per language in separate files (en.go, etc.).
package i18n

import "strings"

type Lang string

const (
	LangEN Lang = "en"
	LangDE Lang = "de"
	LangNL Lang = "nl"
)

var SupportedLangs = []Lang{LangEN, LangDE, LangNL}

var translations = map[Lang]map[string]string{}

// Register adds or merges a translation map for a language.
// Called by init() in each language file.
func Register(lang Lang, m map[string]string) {
	if translations[lang] == nil {
		translations[lang] = make(map[string]string, len(m))
	}
	for k, v := range m {
		translations[lang][k] = v
	}
}

// T looks up a translation key for a given language.
// Falls back to English, then to the key itself.
func T(lang Lang, key string) string {
	if m, ok := translations[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	if v, ok := translations[LangEN][key]; ok {
		return v
	}
	return key
}

func ParseLang(s string) Lang {
	s = strings.TrimSpace(strings.ToLower(s))
	if i := strings.IndexAny(s, "-_"); i > 0 {
		s = s[:i]
	}
	switch s {
	case "de":
		return LangDE
	case "nl":
		return LangNL
	default:
		return LangEN
	}
}

func ParseAcceptLanguage(header string) Lang {
	for _, part := range strings.Split(header, ",") {
		tag := strings.TrimSpace(part)
		if i := strings.Index(tag, ";"); i > 0 {
			tag = tag[:i]
		}
		lang := ParseLang(tag)
		if lang != LangEN || strings.HasPrefix(strings.ToLower(strings.TrimSpace(tag)), "en") {
			return lang
		}
	}
	return LangEN
}
