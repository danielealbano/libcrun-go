//go:build linux && cgo

package main

import (
	"fmt"
	"strconv"
	"strings"
)

// VolumeSpec represents a parsed volume mount specification.
type VolumeSpec struct {
	Source   string
	Dest     string
	ReadOnly bool
}

// parseVolume parses a volume specification string in the format "source:dest[:ro]".
// Examples: "./data:/app", "/host/path:/container/path:ro"
func parseVolume(spec string) (VolumeSpec, error) {
	parts := strings.Split(spec, ":")

	// Handle Windows-style paths (e.g., C:\path) - not fully supported but don't break
	if len(parts) < 2 {
		return VolumeSpec{}, fmt.Errorf("invalid volume spec %q: must be source:dest[:ro]", spec)
	}

	var source, dest string
	var readonly bool

	if len(parts) == 2 {
		source = parts[0]
		dest = parts[1]
	} else if len(parts) == 3 {
		source = parts[0]
		dest = parts[1]
		switch strings.ToLower(parts[2]) {
		case "ro":
			readonly = true
		case "rw":
			readonly = false
		default:
			return VolumeSpec{}, fmt.Errorf("invalid volume option %q: must be 'ro' or 'rw'", parts[2])
		}
	} else {
		return VolumeSpec{}, fmt.Errorf("invalid volume spec %q: too many colons", spec)
	}

	if source == "" {
		return VolumeSpec{}, fmt.Errorf("invalid volume spec %q: source cannot be empty", spec)
	}
	if dest == "" {
		return VolumeSpec{}, fmt.Errorf("invalid volume spec %q: destination cannot be empty", spec)
	}
	if !strings.HasPrefix(dest, "/") {
		return VolumeSpec{}, fmt.Errorf("invalid volume spec %q: destination must be absolute path", spec)
	}

	return VolumeSpec{
		Source:   source,
		Dest:     dest,
		ReadOnly: readonly,
	}, nil
}

// parseMemory parses a memory specification string and returns bytes.
// Supports: plain bytes, k/K (kilobytes), m/M (megabytes), g/G (gigabytes).
// Examples: "512m", "1g", "1073741824"
func parseMemory(spec string) (int64, error) {
	if spec == "" {
		return 0, fmt.Errorf("empty memory specification")
	}

	spec = strings.TrimSpace(spec)
	if spec == "" {
		return 0, fmt.Errorf("empty memory specification")
	}

	// Check for suffix
	lastChar := spec[len(spec)-1]
	var multiplier int64 = 1
	var numStr string

	switch lastChar {
	case 'k', 'K':
		multiplier = 1024
		numStr = spec[:len(spec)-1]
	case 'm', 'M':
		multiplier = 1024 * 1024
		numStr = spec[:len(spec)-1]
	case 'g', 'G':
		multiplier = 1024 * 1024 * 1024
		numStr = spec[:len(spec)-1]
	case 'b', 'B':
		// Explicit bytes suffix (e.g., "1024b")
		multiplier = 1
		numStr = spec[:len(spec)-1]
	default:
		// No suffix, treat as bytes
		numStr = spec
	}

	if numStr == "" {
		return 0, fmt.Errorf("invalid memory specification %q: no number provided", spec)
	}

	// Try parsing as integer first
	value, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		// Try parsing as float (e.g., "1.5g")
		floatVal, floatErr := strconv.ParseFloat(numStr, 64)
		if floatErr != nil {
			return 0, fmt.Errorf("invalid memory specification %q: %v", spec, err)
		}
		value = int64(floatVal * float64(multiplier))
		return value, nil
	}

	if value < 0 {
		return 0, fmt.Errorf("invalid memory specification %q: cannot be negative", spec)
	}

	return value * multiplier, nil
}

// UserSpec represents a parsed user specification.
type UserSpec struct {
	UID uint32
	GID uint32
}

// parseUser parses a user specification string in the format "uid" or "uid:gid".
// Examples: "1000", "1000:1000", "0:0"
func parseUser(spec string) (UserSpec, error) {
	if spec == "" {
		return UserSpec{}, fmt.Errorf("empty user specification")
	}

	parts := strings.Split(spec, ":")

	if len(parts) > 2 {
		return UserSpec{}, fmt.Errorf("invalid user spec %q: too many colons", spec)
	}

	uid, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return UserSpec{}, fmt.Errorf("invalid user spec %q: invalid UID: %v", spec, err)
	}

	var gid uint64
	if len(parts) == 2 {
		gid, err = strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			return UserSpec{}, fmt.Errorf("invalid user spec %q: invalid GID: %v", spec, err)
		}
	} else {
		// If no GID specified, use same as UID
		gid = uid
	}

	return UserSpec{
		UID: uint32(uid),
		GID: uint32(gid),
	}, nil
}

// parseCPUs parses a CPU limit specification and returns the quota in microseconds.
// The period is fixed at 100000 microseconds (100ms).
// Examples: "0.5" (50% of one CPU), "2" (200% = 2 CPUs), "1.5" (150% = 1.5 CPUs)
func parseCPUs(spec string) (int64, error) {
	if spec == "" {
		return 0, fmt.Errorf("empty CPU specification")
	}

	cpus, err := strconv.ParseFloat(spec, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU specification %q: %v", spec, err)
	}

	if cpus <= 0 {
		return 0, fmt.Errorf("invalid CPU specification %q: must be positive", spec)
	}

	// CPU period is 100000 microseconds (100ms)
	// Quota = cpus * period
	const period = 100000
	quota := int64(cpus * period)

	return quota, nil
}

