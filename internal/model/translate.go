package model

// TranslateFunc translates a key for a given language code.
// Services use this to produce localized messages without depending on i18n directly.
type TranslateFunc func(lang, key string) string
