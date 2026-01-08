//go:build linux

package crun

import "strings"

// ErrorCode represents specific error types from libcrun operations.
type ErrorCode int

// Error codes for libcrun operations.
const (
	ErrUnknown ErrorCode = iota
	ErrNotFound
	ErrAlreadyExists
	ErrInvalidSpec
	ErrPermissionDenied
	ErrContainerRunning
	ErrContainerNotRunning
)

// Sentinel errors for errors.Is() checks.
var (
	ErrContainerNotFound    = &Error{Code: ErrNotFound, Message: "container not found"}
	ErrContainerExists      = &Error{Code: ErrAlreadyExists, Message: "container already exists"}
	ErrInvalidContainerSpec = &Error{Code: ErrInvalidSpec, Message: "invalid container spec"}
)

// Error wraps libcrun errors with structured error codes.
type Error struct {
	Code    ErrorCode
	Message string
	Status  int   // errno value
	cause   error // underlying error
}

func (e *Error) Error() string { return e.Message }

func (e *Error) Unwrap() error { return e.cause }

func (e *Error) Is(target error) bool {
	if t, ok := target.(*Error); ok {
		return e.Code == t.Code
	}
	return false
}

// classifyError attempts to determine the error code from the error message.
func classifyError(msg string, status int) ErrorCode {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "not found") || strings.Contains(lower, "does not exist"):
		return ErrNotFound
	case strings.Contains(lower, "already exists"):
		return ErrAlreadyExists
	case strings.Contains(lower, "invalid") || strings.Contains(lower, "parse"):
		return ErrInvalidSpec
	case strings.Contains(lower, "permission") || status == 1 || status == 13: // EPERM, EACCES
		return ErrPermissionDenied
	case strings.Contains(lower, "not running"):
		return ErrContainerNotRunning
	case strings.Contains(lower, "running"):
		return ErrContainerRunning
	default:
		return ErrUnknown
	}
}

