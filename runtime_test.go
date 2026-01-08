//go:build linux

package crun

import (
	"testing"
)

func TestRuntimeConfigDefaults(t *testing.T) {
	cfg := RuntimeConfig{}

	// Check that zero values are correct defaults
	if cfg.Bundle != "" {
		t.Errorf("Default Bundle = %q, want empty", cfg.Bundle)
	}
	if cfg.SystemdCgroup {
		t.Error("Default SystemdCgroup should be false")
	}
	if cfg.Detach {
		t.Error("Default Detach should be false")
	}
}

func TestSetLogHandler(t *testing.T) {
	// Set a handler
	SetLogHandler(func(entry LogEntry) {
		// Handler would receive logs if libcrun emitted any
		_ = entry
	})

	// Verify handler is set (we can't easily trigger libcrun logs in unit test,
	// but we can at least verify the setup doesn't panic)
	if logHandle == 0 {
		t.Error("Expected logHandle to be set")
	}

	// Clear handler
	SetLogHandler(nil)

	if logHandle != 0 {
		t.Error("Expected logHandle to be cleared")
	}

	// Verify we can set another handler after clearing
	SetLogHandler(func(entry LogEntry) {
		_ = entry
	})
	if logHandle == 0 {
		t.Error("Expected logHandle to be set again")
	}

	// Clean up
	SetLogHandler(nil)
}

func TestSetLogHandlerVerbosityConstants(t *testing.T) {
	// Verify verbosity constants are correctly mapped
	if VerbosityError != 0 {
		t.Errorf("VerbosityError = %d, want 0", VerbosityError)
	}
	if VerbosityWarning != 1 {
		t.Errorf("VerbosityWarning = %d, want 1", VerbosityWarning)
	}
	if VerbosityDebug != 2 {
		t.Errorf("VerbosityDebug = %d, want 2", VerbosityDebug)
	}
}

func TestLogEntry(t *testing.T) {
	entry := LogEntry{
		Errno:     2,
		Message:   "test message",
		Verbosity: VerbosityWarning,
	}

	if entry.Errno != 2 {
		t.Errorf("Errno = %d, want 2", entry.Errno)
	}
	if entry.Message != "test message" {
		t.Errorf("Message = %q, want %q", entry.Message, "test message")
	}
	if entry.Verbosity != VerbosityWarning {
		t.Errorf("Verbosity = %d, want %d", entry.Verbosity, VerbosityWarning)
	}
}

