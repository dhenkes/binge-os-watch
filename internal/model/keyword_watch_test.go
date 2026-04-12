package model

import "testing"

func TestKeywordWatchValidate_Valid(t *testing.T) {
	kw := KeywordWatch{Keyword: "baki", MediaTypes: "movie,tv"}
	if err := kw.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestKeywordWatchValidate_EmptyKeyword(t *testing.T) {
	kw := KeywordWatch{Keyword: "", MediaTypes: "movie"}
	assertValidationField(t, kw.Validate(), "keyword")
}

func TestKeywordWatchValidate_WhitespaceKeyword(t *testing.T) {
	kw := KeywordWatch{Keyword: "   ", MediaTypes: "movie"}
	assertValidationField(t, kw.Validate(), "keyword")
}

func TestKeywordWatchValidate_InvalidMediaType(t *testing.T) {
	kw := KeywordWatch{Keyword: "test", MediaTypes: "movie,podcast"}
	assertValidationField(t, kw.Validate(), "media_types")
}

func TestKeywordWatchValidate_SingleMovieType(t *testing.T) {
	kw := KeywordWatch{Keyword: "test", MediaTypes: "movie"}
	if err := kw.Validate(); err != nil {
		t.Fatalf("single 'movie' type should be valid, got %v", err)
	}
}

func TestKeywordWatchValidate_SingleTVType(t *testing.T) {
	kw := KeywordWatch{Keyword: "test", MediaTypes: "tv"}
	if err := kw.Validate(); err != nil {
		t.Fatalf("single 'tv' type should be valid, got %v", err)
	}
}

func TestKeywordWatchValidate_EmptyMediaTypesAllowed(t *testing.T) {
	kw := KeywordWatch{Keyword: "test", MediaTypes: ""}
	if err := kw.Validate(); err != nil {
		t.Fatalf("empty media_types should be allowed (defaults in service), got %v", err)
	}
}

func TestKeywordWatchValidate_AllInvalid(t *testing.T) {
	kw := KeywordWatch{Keyword: "test", MediaTypes: "podcast,anime"}
	assertValidationField(t, kw.Validate(), "media_types")
}
