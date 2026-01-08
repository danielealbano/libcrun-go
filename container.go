//go:build linux

package crun

import (
	"encoding/json"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Container represents a running or created container with lifecycle methods.
type Container struct {
	ID      string
	runtime *RuntimeContext
}

// Start starts a previously created container.
func (c *Container) Start() error {
	return c.runtime.startContainer(c.ID)
}

// Kill sends a signal to the container's init process.
func (c *Container) Kill(sig Signal) error {
	return c.runtime.killContainer(c.ID, sig)
}

// Delete removes the container.
func (c *Container) Delete(force bool) error {
	return c.runtime.deleteContainer(c.ID, force)
}

// State returns the current state of the container.
func (c *Container) State() (*ContainerState, error) {
	jsonStr, err := c.runtime.containerStateJSON(c.ID)
	if err != nil {
		return nil, err
	}
	var state ContainerState
	if err := json.Unmarshal([]byte(jsonStr), &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// StateJSON returns the raw JSON state of the container.
func (c *Container) StateJSON() (string, error) {
	return c.runtime.containerStateJSON(c.ID)
}

// execConfig holds configuration for exec operations.
type execConfig struct {
	detach   bool
	terminal bool
	cwd      string
}

// ExecOption is a functional option for configuring exec operations.
type ExecOption func(*execConfig)

// WithDetach runs the exec process in detached mode.
func WithDetach() ExecOption {
	return func(c *execConfig) { c.detach = true }
}

// WithExecTTY allocates a pseudo-terminal for the exec process.
func WithExecTTY() ExecOption {
	return func(c *execConfig) { c.terminal = true }
}

// WithWorkingDir sets the working directory for the exec process.
func WithWorkingDir(cwd string) ExecOption {
	return func(c *execConfig) { c.cwd = cwd }
}

// Exec executes a process in the container.
func (c *Container) Exec(proc *specs.Process, opts ...ExecOption) error {
	cfg := &execConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Apply options to the process
	execProc := *proc // copy
	if cfg.terminal {
		execProc.Terminal = true
	}
	if cfg.cwd != "" {
		execProc.Cwd = cfg.cwd
	}

	b, err := json.Marshal(&execProc)
	if err != nil {
		return err
	}
	return c.runtime.execJSON(c.ID, string(b))
}

// UpdateResources updates the container's resource limits.
func (c *Container) UpdateResources(res *specs.LinuxResources) error {
	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	return c.runtime.updateContainer(c.ID, string(b))
}

// Pause pauses/freezes the container.
func (c *Container) Pause() error {
	return c.runtime.pauseContainer(c.ID)
}

// Unpause unpauses/thaws the container.
func (c *Container) Unpause() error {
	return c.runtime.unpauseContainer(c.ID)
}

// KillAll sends a signal to all processes in the container.
func (c *Container) KillAll(sig Signal) error {
	return c.runtime.killAllContainer(c.ID, sig)
}

// IsRunning returns true if the container is currently running.
func (c *Container) IsRunning() (bool, error) {
	return c.runtime.isContainerRunning(c.ID)
}

// PIDs returns the list of process IDs in the container.
// If recurse is true, includes PIDs from child cgroups.
func (c *Container) PIDs(recurse bool) ([]int, error) {
	return c.runtime.containerPIDs(c.ID, recurse)
}

