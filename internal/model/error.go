package model

import (
	"encoding/json"
	"net/http"
)

// ErrorCode represents an AEP-inspired error code.
type ErrorCode int

const (
	ErrorCodeInvalidArgument ErrorCode = iota
	ErrorCodeNotFound
	ErrorCodeAlreadyExists
	ErrorCodeUnauthenticated
	ErrorCodePermissionDenied
	ErrorCodeInternal
)

// String returns the canonical string name for the error code.
func (c ErrorCode) String() string {
	switch c {
	case ErrorCodeInvalidArgument:
		return "INVALID_ARGUMENT"
	case ErrorCodeNotFound:
		return "NOT_FOUND"
	case ErrorCodeAlreadyExists:
		return "ALREADY_EXISTS"
	case ErrorCodeUnauthenticated:
		return "UNAUTHENTICATED"
	case ErrorCodePermissionDenied:
		return "PERMISSION_DENIED"
	case ErrorCodeInternal:
		return "INTERNAL"
	default:
		return "UNKNOWN"
	}
}

// HTTPStatus maps the error code to an HTTP status code.
func (c ErrorCode) HTTPStatus() int {
	switch c {
	case ErrorCodeInvalidArgument:
		return http.StatusBadRequest
	case ErrorCodeNotFound:
		return http.StatusNotFound
	case ErrorCodeAlreadyExists:
		return http.StatusConflict
	case ErrorCodeUnauthenticated:
		return http.StatusUnauthorized
	case ErrorCodePermissionDenied:
		return http.StatusForbidden
	case ErrorCodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// MarshalJSON serializes ErrorCode as its string representation.
func (c ErrorCode) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

// UnmarshalJSON deserializes a string into an ErrorCode.
func (c *ErrorCode) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "INVALID_ARGUMENT":
		*c = ErrorCodeInvalidArgument
	case "NOT_FOUND":
		*c = ErrorCodeNotFound
	case "ALREADY_EXISTS":
		*c = ErrorCodeAlreadyExists
	case "UNAUTHENTICATED":
		*c = ErrorCodeUnauthenticated
	case "PERMISSION_DENIED":
		*c = ErrorCodePermissionDenied
	case "INTERNAL":
		*c = ErrorCodeInternal
	default:
		*c = ErrorCodeInternal
	}
	return nil
}

// AppError is the standard API error response shape.
type AppError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details any       `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	return e.Message
}

func NewNotFound(message string) *AppError {
	return &AppError{Code: ErrorCodeNotFound, Message: message}
}

func NewAlreadyExists(message string) *AppError {
	return &AppError{Code: ErrorCodeAlreadyExists, Message: message}
}

func NewInvalidArgument(message string) *AppError {
	return &AppError{Code: ErrorCodeInvalidArgument, Message: message}
}

func NewUnauthenticated(message string) *AppError {
	return &AppError{Code: ErrorCodeUnauthenticated, Message: message}
}

func NewPermissionDenied(message string) *AppError {
	return &AppError{Code: ErrorCodePermissionDenied, Message: message}
}

func NewInternal(message string) *AppError {
	return &AppError{Code: ErrorCodeInternal, Message: message}
}
