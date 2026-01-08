//go:build linux

package crun

import (
	"errors"
	"testing"
)

func TestErrorIs(t *testing.T) {
	err := &Error{Code: ErrNotFound, Message: "container not found"}

	if !errors.Is(err, ErrContainerNotFound) {
		t.Error("Expected errors.Is(err, ErrContainerNotFound) to be true")
	}

	if errors.Is(err, ErrContainerExists) {
		t.Error("Expected errors.Is(err, ErrContainerExists) to be false")
	}
}

func TestErrorUnwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &Error{Code: ErrNotFound, Message: "wrapper", cause: cause}

	unwrapped := errors.Unwrap(err)
	if unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		msg    string
		status int
		want   ErrorCode
	}{
		{"container not found", 0, ErrNotFound},
		{"container does not exist", 0, ErrNotFound},
		{"container already exists", 0, ErrAlreadyExists},
		{"invalid spec", 0, ErrInvalidSpec},
		{"failed to parse config", 0, ErrInvalidSpec},
		{"permission denied", 0, ErrPermissionDenied},
		{"some error", 1, ErrPermissionDenied},  // EPERM
		{"some error", 13, ErrPermissionDenied}, // EACCES
		{"container is running", 0, ErrContainerRunning},
		{"container is not running", 0, ErrContainerNotRunning},
		{"unknown error", 0, ErrUnknown},
	}

	for _, tt := range tests {
		got := classifyError(tt.msg, tt.status)
		if got != tt.want {
			t.Errorf("classifyError(%q, %d) = %v, want %v", tt.msg, tt.status, got, tt.want)
		}
	}
}
