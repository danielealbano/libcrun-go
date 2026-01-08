//go:build linux

package crun

import "time"

// Signal represents a signal to send to a container process.
type Signal string

// Standard signals for container operations.
const (
	SIGTERM Signal = "SIGTERM"
	SIGKILL Signal = "SIGKILL"
	SIGINT  Signal = "SIGINT"
	SIGHUP  Signal = "SIGHUP"
	SIGUSR1 Signal = "SIGUSR1"
	SIGUSR2 Signal = "SIGUSR2"
	SIGSTOP Signal = "SIGSTOP"
	SIGCONT Signal = "SIGCONT"
)

// ContainerStatus represents the state of a container.
type ContainerStatus string

// Container status values as defined by OCI runtime spec.
const (
	StatusCreating ContainerStatus = "creating"
	StatusCreated  ContainerStatus = "created"
	StatusRunning  ContainerStatus = "running"
	StatusStopped  ContainerStatus = "stopped"
	StatusPaused   ContainerStatus = "paused"
)

// ContainerState represents the state of a container as returned by libcrun.
type ContainerState struct {
	OciVersion  string            `json:"ociVersion"`
	ID          string            `json:"id"`
	Status      ContainerStatus   `json:"status"`
	Pid         int               `json:"pid"`
	Bundle      string            `json:"bundle"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Created     time.Time         `json:"created,omitempty"`
}

