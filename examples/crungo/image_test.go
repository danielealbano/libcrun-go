//go:build linux && cgo

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple image name",
			input:    "alpine",
			expected: "index.docker.io/library/alpine:latest",
		},
		{
			name:     "image with tag",
			input:    "alpine:3.19",
			expected: "index.docker.io/library/alpine:3.19",
		},
		{
			name:     "image with user",
			input:    "nginx/nginx",
			expected: "index.docker.io/nginx/nginx:latest",
		},
		{
			name:     "full registry path",
			input:    "docker.io/library/alpine:latest",
			expected: "index.docker.io/library/alpine:latest",
		},
		{
			name:     "ghcr registry",
			input:    "ghcr.io/owner/repo:v1.0",
			expected: "ghcr.io/owner/repo:v1.0",
		},
		{
			name:     "quay registry",
			input:    "quay.io/coreos/etcd:v3.5.0",
			expected: "quay.io/coreos/etcd:v3.5.0",
		},
		{
			name:     "image with digest",
			input:    "alpine@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expected: "index.docker.io/library/alpine@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid characters",
			input:   "alpine:tag with spaces",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseImageRef(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseImageRef(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("ParseImageRef(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestEnsurePasswd(t *testing.T) {
	// Create a temporary directory to act as rootfs
	rootfs, err := os.MkdirTemp("", "test-rootfs-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(rootfs)

	// Test creating passwd in empty rootfs
	if err := ensurePasswd(rootfs); err != nil {
		t.Fatalf("ensurePasswd() error = %v", err)
	}

	// Verify /etc/passwd was created
	passwdPath := filepath.Join(rootfs, "etc", "passwd")
	content, err := os.ReadFile(passwdPath)
	if err != nil {
		t.Fatalf("failed to read passwd file: %v", err)
	}

	// Check content contains root entry
	if !contains(string(content), "root:x:0:0:root:/root:") {
		t.Errorf("passwd content missing root entry: %s", content)
	}

	// Test that calling again doesn't fail (idempotent)
	if err := ensurePasswd(rootfs); err != nil {
		t.Fatalf("ensurePasswd() second call error = %v", err)
	}
}

func TestEnsurePasswdExisting(t *testing.T) {
	// Create a temporary directory with existing /etc/passwd
	rootfs, err := os.MkdirTemp("", "test-rootfs-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(rootfs)

	// Create existing /etc/passwd
	etcDir := filepath.Join(rootfs, "etc")
	if err := os.MkdirAll(etcDir, 0755); err != nil {
		t.Fatalf("failed to create /etc: %v", err)
	}
	existingContent := "custom:x:1000:1000:custom:/home/custom:/bin/bash\n"
	if err := os.WriteFile(filepath.Join(etcDir, "passwd"), []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to create existing passwd: %v", err)
	}

	// Call ensurePasswd - should not overwrite
	if err := ensurePasswd(rootfs); err != nil {
		t.Fatalf("ensurePasswd() error = %v", err)
	}

	// Verify content was NOT overwritten
	content, err := os.ReadFile(filepath.Join(etcDir, "passwd"))
	if err != nil {
		t.Fatalf("failed to read passwd: %v", err)
	}

	if string(content) != existingContent {
		t.Errorf("passwd was overwritten: got %q, want %q", string(content), existingContent)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

