//go:build linux

package crun

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSignalConstants(t *testing.T) {
	tests := []struct {
		sig  Signal
		want string
	}{
		{SIGTERM, "SIGTERM"},
		{SIGKILL, "SIGKILL"},
		{SIGINT, "SIGINT"},
		{SIGHUP, "SIGHUP"},
		{SIGUSR1, "SIGUSR1"},
		{SIGUSR2, "SIGUSR2"},
		{SIGSTOP, "SIGSTOP"},
		{SIGCONT, "SIGCONT"},
	}

	for _, tt := range tests {
		if string(tt.sig) != tt.want {
			t.Errorf("Signal %v = %q, want %q", tt.sig, string(tt.sig), tt.want)
		}
	}
}

func TestContainerStatusConstants(t *testing.T) {
	tests := []struct {
		status ContainerStatus
		want   string
	}{
		{StatusCreating, "creating"},
		{StatusCreated, "created"},
		{StatusRunning, "running"},
		{StatusStopped, "stopped"},
		{StatusPaused, "paused"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("ContainerStatus %v = %q, want %q", tt.status, string(tt.status), tt.want)
		}
	}
}

func TestContainerStateUnmarshal(t *testing.T) {
	jsonData := `{
		"ociVersion": "1.0.0",
		"id": "test-container",
		"status": "running",
		"pid": 1234,
		"bundle": "/var/lib/containers/test",
		"annotations": {"key": "value"},
		"created": "2024-01-15T10:30:00Z"
	}`

	var state ContainerState
	if err := json.Unmarshal([]byte(jsonData), &state); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if state.OciVersion != "1.0.0" {
		t.Errorf("OciVersion = %q, want %q", state.OciVersion, "1.0.0")
	}
	if state.ID != "test-container" {
		t.Errorf("ID = %q, want %q", state.ID, "test-container")
	}
	if state.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", state.Status, StatusRunning)
	}
	if state.Pid != 1234 {
		t.Errorf("Pid = %d, want %d", state.Pid, 1234)
	}
	if state.Bundle != "/var/lib/containers/test" {
		t.Errorf("Bundle = %q, want %q", state.Bundle, "/var/lib/containers/test")
	}
	if state.Annotations["key"] != "value" {
		t.Errorf("Annotations[key] = %q, want %q", state.Annotations["key"], "value")
	}
	expectedTime, _ := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
	if !state.Created.Equal(expectedTime) {
		t.Errorf("Created = %v, want %v", state.Created, expectedTime)
	}
}

