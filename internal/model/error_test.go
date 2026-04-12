package model

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestErrorCode_String(t *testing.T) {
	tests := []struct {
		code ErrorCode
		want string
	}{
		{ErrorCodeInvalidArgument, "INVALID_ARGUMENT"},
		{ErrorCodeNotFound, "NOT_FOUND"},
		{ErrorCodeAlreadyExists, "ALREADY_EXISTS"},
		{ErrorCodeUnauthenticated, "UNAUTHENTICATED"},
		{ErrorCodePermissionDenied, "PERMISSION_DENIED"},
		{ErrorCodeInternal, "INTERNAL"},
		{ErrorCode(999), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.code.String(); got != tt.want {
			t.Errorf("ErrorCode(%d).String() = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestErrorCode_HTTPStatus(t *testing.T) {
	tests := []struct {
		code ErrorCode
		want int
	}{
		{ErrorCodeInvalidArgument, http.StatusBadRequest},
		{ErrorCodeNotFound, http.StatusNotFound},
		{ErrorCodeAlreadyExists, http.StatusConflict},
		{ErrorCodeUnauthenticated, http.StatusUnauthorized},
		{ErrorCodePermissionDenied, http.StatusForbidden},
		{ErrorCodeInternal, http.StatusInternalServerError},
		{ErrorCode(999), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		if got := tt.code.HTTPStatus(); got != tt.want {
			t.Errorf("ErrorCode(%d).HTTPStatus() = %d, want %d", tt.code, got, tt.want)
		}
	}
}

func TestErrorCode_MarshalJSON(t *testing.T) {
	tests := []struct {
		code ErrorCode
		want string
	}{
		{ErrorCodeInvalidArgument, `"INVALID_ARGUMENT"`},
		{ErrorCodeNotFound, `"NOT_FOUND"`},
		{ErrorCodeInternal, `"INTERNAL"`},
	}
	for _, tt := range tests {
		data, err := json.Marshal(tt.code)
		if err != nil {
			t.Fatalf("Marshal ErrorCode(%d): %v", tt.code, err)
		}
		if string(data) != tt.want {
			t.Errorf("Marshal ErrorCode(%d) = %s, want %s", tt.code, data, tt.want)
		}
	}
}

func TestErrorCode_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		input string
		want  ErrorCode
	}{
		{`"INVALID_ARGUMENT"`, ErrorCodeInvalidArgument},
		{`"NOT_FOUND"`, ErrorCodeNotFound},
		{`"ALREADY_EXISTS"`, ErrorCodeAlreadyExists},
		{`"UNAUTHENTICATED"`, ErrorCodeUnauthenticated},
		{`"PERMISSION_DENIED"`, ErrorCodePermissionDenied},
		{`"INTERNAL"`, ErrorCodeInternal},
		{`"SOMETHING_ELSE"`, ErrorCodeInternal},
	}
	for _, tt := range tests {
		var c ErrorCode
		if err := json.Unmarshal([]byte(tt.input), &c); err != nil {
			t.Fatalf("Unmarshal %s: %v", tt.input, err)
		}
		if c != tt.want {
			t.Errorf("Unmarshal %s = %d, want %d", tt.input, c, tt.want)
		}
	}
}

func TestErrorCode_UnmarshalJSON_InvalidType(t *testing.T) {
	var c ErrorCode
	if err := json.Unmarshal([]byte(`123`), &c); err == nil {
		t.Error("expected error unmarshaling non-string JSON")
	}
}

func TestErrorCode_RoundTrip(t *testing.T) {
	codes := []ErrorCode{
		ErrorCodeInvalidArgument,
		ErrorCodeNotFound,
		ErrorCodeAlreadyExists,
		ErrorCodeUnauthenticated,
		ErrorCodePermissionDenied,
		ErrorCodeInternal,
	}
	for _, code := range codes {
		data, err := json.Marshal(code)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var got ErrorCode
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if got != code {
			t.Errorf("round-trip ErrorCode %d: got %d", code, got)
		}
	}
}

func TestAppError_Error(t *testing.T) {
	e := &AppError{Code: ErrorCodeNotFound, Message: "user not found"}
	if e.Error() != "user not found" {
		t.Errorf("got %q, want %q", e.Error(), "user not found")
	}
}

func TestAppError_Constructors(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string) *AppError
		code ErrorCode
	}{
		{"NewNotFound", NewNotFound, ErrorCodeNotFound},
		{"NewAlreadyExists", NewAlreadyExists, ErrorCodeAlreadyExists},
		{"NewInvalidArgument", NewInvalidArgument, ErrorCodeInvalidArgument},
		{"NewUnauthenticated", NewUnauthenticated, ErrorCodeUnauthenticated},
		{"NewPermissionDenied", NewPermissionDenied, ErrorCodePermissionDenied},
		{"NewInternal", NewInternal, ErrorCodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := tt.fn("test message")
			if e.Code != tt.code {
				t.Errorf("Code = %v, want %v", e.Code, tt.code)
			}
			if e.Message != "test message" {
				t.Errorf("Message = %q, want %q", e.Message, "test message")
			}
			if e.Details != nil {
				t.Errorf("Details should be nil, got %v", e.Details)
			}
		})
	}
}

func TestAppError_JSON(t *testing.T) {
	e := &AppError{
		Code:    ErrorCodeNotFound,
		Message: "not found",
		Details: map[string]string{"id": "123"},
	}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got AppError
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Code != ErrorCodeNotFound {
		t.Errorf("Code = %v, want %v", got.Code, ErrorCodeNotFound)
	}
	if got.Message != "not found" {
		t.Errorf("Message = %q, want %q", got.Message, "not found")
	}
}
