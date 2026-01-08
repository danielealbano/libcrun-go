//go:build linux && cgo

package main

import (
	"testing"
)

func TestParseVolume(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected VolumeSpec
		wantErr  bool
	}{
		{
			name:     "simple volume",
			input:    "./data:/app",
			expected: VolumeSpec{Source: "./data", Dest: "/app", ReadOnly: false},
		},
		{
			name:     "absolute paths",
			input:    "/host/path:/container/path",
			expected: VolumeSpec{Source: "/host/path", Dest: "/container/path", ReadOnly: false},
		},
		{
			name:     "readonly volume",
			input:    "./data:/app:ro",
			expected: VolumeSpec{Source: "./data", Dest: "/app", ReadOnly: true},
		},
		{
			name:     "readonly uppercase",
			input:    "./data:/app:RO",
			expected: VolumeSpec{Source: "./data", Dest: "/app", ReadOnly: true},
		},
		{
			name:     "readwrite explicit",
			input:    "./data:/app:rw",
			expected: VolumeSpec{Source: "./data", Dest: "/app", ReadOnly: false},
		},
		{
			name:    "missing destination",
			input:   "./data",
			wantErr: true,
		},
		{
			name:    "empty source",
			input:   ":/app",
			wantErr: true,
		},
		{
			name:    "empty destination",
			input:   "./data:",
			wantErr: true,
		},
		{
			name:    "relative destination",
			input:   "./data:app",
			wantErr: true,
		},
		{
			name:    "invalid option",
			input:   "./data:/app:invalid",
			wantErr: true,
		},
		{
			name:    "too many colons",
			input:   "./data:/app:ro:extra",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVolume(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseVolume(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("parseVolume(%q) = %+v, want %+v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		wantErr  bool
	}{
		{
			name:     "bytes plain",
			input:    "1024",
			expected: 1024,
		},
		{
			name:     "kilobytes lowercase",
			input:    "1k",
			expected: 1024,
		},
		{
			name:     "kilobytes uppercase",
			input:    "1K",
			expected: 1024,
		},
		{
			name:     "megabytes lowercase",
			input:    "512m",
			expected: 512 * 1024 * 1024,
		},
		{
			name:     "megabytes uppercase",
			input:    "512M",
			expected: 512 * 1024 * 1024,
		},
		{
			name:     "gigabytes lowercase",
			input:    "1g",
			expected: 1024 * 1024 * 1024,
		},
		{
			name:     "gigabytes uppercase",
			input:    "2G",
			expected: 2 * 1024 * 1024 * 1024,
		},
		{
			name:     "explicit bytes suffix",
			input:    "1024b",
			expected: 1024,
		},
		{
			name:     "fractional gigabytes",
			input:    "1.5g",
			expected: int64(1.5 * 1024 * 1024 * 1024),
		},
		{
			name:     "fractional megabytes",
			input:    "0.5m",
			expected: int64(0.5 * 1024 * 1024),
		},
		{
			name:     "with whitespace",
			input:    "  512m  ",
			expected: 512 * 1024 * 1024,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "only whitespace",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "invalid number",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "negative value",
			input:   "-512m",
			wantErr: true,
		},
		{
			name:    "only suffix",
			input:   "m",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMemory(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseMemory(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("parseMemory(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseUser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected UserSpec
		wantErr  bool
	}{
		{
			name:     "uid only",
			input:    "1000",
			expected: UserSpec{UID: 1000, GID: 1000},
		},
		{
			name:     "uid and gid",
			input:    "1000:1000",
			expected: UserSpec{UID: 1000, GID: 1000},
		},
		{
			name:     "different uid and gid",
			input:    "1000:2000",
			expected: UserSpec{UID: 1000, GID: 2000},
		},
		{
			name:     "root user",
			input:    "0",
			expected: UserSpec{UID: 0, GID: 0},
		},
		{
			name:     "root user explicit",
			input:    "0:0",
			expected: UserSpec{UID: 0, GID: 0},
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid uid",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "invalid gid",
			input:   "1000:abc",
			wantErr: true,
		},
		{
			name:    "too many colons",
			input:   "1000:1000:1000",
			wantErr: true,
		},
		{
			name:    "negative uid",
			input:   "-1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUser(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUser(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("parseUser(%q) = %+v, want %+v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseCPUs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		wantErr  bool
	}{
		{
			name:     "half cpu",
			input:    "0.5",
			expected: 50000, // 0.5 * 100000
		},
		{
			name:     "one cpu",
			input:    "1",
			expected: 100000,
		},
		{
			name:     "two cpus",
			input:    "2",
			expected: 200000,
		},
		{
			name:     "fractional cpus",
			input:    "1.5",
			expected: 150000,
		},
		{
			name:     "quarter cpu",
			input:    "0.25",
			expected: 25000,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "zero",
			input:   "0",
			wantErr: true,
		},
		{
			name:    "negative",
			input:   "-1",
			wantErr: true,
		},
		{
			name:    "invalid",
			input:   "abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCPUs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCPUs(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("parseCPUs(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

