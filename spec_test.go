//go:build linux && cgo

package crun

import (
	"strings"
	"testing"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func TestSpec(t *testing.T) {
	// Test rootless spec generation
	js, err := Spec(true)
	if err != nil {
		t.Fatalf("Spec(true) failed: %v", err)
	}
	if js == "" {
		t.Error("Spec(true) returned empty string")
	}
	if !strings.Contains(js, "ociVersion") {
		t.Error("Spec output should contain ociVersion")
	}

	// Test privileged spec generation
	js, err = Spec(false)
	if err != nil {
		t.Fatalf("Spec(false) failed: %v", err)
	}
	if js == "" {
		t.Error("Spec(false) returned empty string")
	}
}

func TestLoadContainerSpecFromJSON(t *testing.T) {
	// Get a valid spec JSON from libcrun
	js, err := Spec(true)
	if err != nil {
		t.Fatalf("Spec(true) failed: %v", err)
	}

	// Load it as a ContainerSpec
	spec, err := LoadContainerSpecFromJSON(js)
	if err != nil {
		t.Fatalf("LoadContainerSpecFromJSON failed: %v", err)
	}
	defer spec.Close()

	if spec.c == nil {
		t.Error("ContainerSpec.c should not be nil")
	}
}

func TestLoadContainerSpecFromJSONInvalid(t *testing.T) {
	// Test with invalid JSON
	_, err := LoadContainerSpecFromJSON("not valid json")
	if err == nil {
		t.Error("LoadContainerSpecFromJSON should fail with invalid JSON")
	}
}

func TestNewContainerSpec(t *testing.T) {
	sp := &specs.Spec{
		Version: "1.0.0",
		Root: &specs.Root{
			Path: "/tmp/rootfs",
		},
		Process: &specs.Process{
			Args: []string{"/bin/sh"},
			Cwd:  "/",
		},
	}

	spec, err := NewContainerSpec(sp)
	if err != nil {
		t.Fatalf("NewContainerSpec failed: %v", err)
	}
	defer spec.Close()

	if spec.c == nil {
		t.Error("ContainerSpec.c should not be nil")
	}
}

func TestContainerSpecClose(t *testing.T) {
	js, err := Spec(true)
	if err != nil {
		t.Fatalf("Spec(true) failed: %v", err)
	}

	spec, err := LoadContainerSpecFromJSON(js)
	if err != nil {
		t.Fatalf("LoadContainerSpecFromJSON failed: %v", err)
	}

	// Close should not error
	if err := spec.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	// Second close should be idempotent
	if err := spec.Close(); err != nil {
		t.Errorf("Second Close() failed: %v", err)
	}

	// Close on nil should not panic
	var nilSpec *ContainerSpec
	if err := nilSpec.Close(); err != nil {
		t.Errorf("Close() on nil failed: %v", err)
	}
}

