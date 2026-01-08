//go:build linux

package crun_test

import (
	"errors"
	"fmt"
	"os"

	crun "github.com/danielealbano/libcrun-go"
)

// ExampleNewRuntimeContext demonstrates creating a runtime context for container operations.
func ExampleNewRuntimeContext() {
	rc, err := crun.NewRuntimeContext(crun.RuntimeConfig{
		StateRoot: "/run/crun", // where container state is stored
	})
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer rc.Close()

	fmt.Println("RuntimeContext created successfully")
	// Output: RuntimeContext created successfully
}

// ExampleNewSpec demonstrates creating a container spec with functional options.
func ExampleNewSpec() {
	// Create a rootless container spec with various options
	spec, err := crun.NewSpec(true, // rootless mode
		crun.WithRootPath("/path/to/rootfs"),
		crun.WithArgs("/bin/echo", "hello", "world"),
		crun.WithEnv("MY_VAR", "my_value"),
		crun.WithEnv("PATH", "/usr/bin:/bin"),
		crun.WithMemoryLimit(256*1024*1024), // 256MB
		crun.WithCPUShares(1024),
		crun.WithPidsLimit(100),
		crun.WithHostname("my-container"),
		crun.WithCwd("/"),
	)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer spec.Close()

	fmt.Println("ContainerSpec created successfully")
	// Output: ContainerSpec created successfully
}

// ExampleRuntimeContext_RunWithIO demonstrates running a container with I/O streams.
func ExampleRuntimeContext_RunWithIO() {
	rc, err := crun.NewRuntimeContext(crun.RuntimeConfig{
		StateRoot: "/run/crun",
	})
	if err != nil {
		fmt.Println("Error creating context:", err)
		return
	}
	defer rc.Close()

	spec, err := crun.NewSpec(true,
		crun.WithRootPath("/path/to/rootfs"),
		crun.WithArgs("/bin/echo", "hello from container"),
	)
	if err != nil {
		fmt.Println("Error creating spec:", err)
		return
	}
	defer spec.Close()

	// Run with stdout/stderr captured
	result, err := rc.RunWithIO("example-container", spec, &crun.IOConfig{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		fmt.Println("Error running container:", err)
		return
	}

	// Wait for container to finish
	exitCode, err := result.Wait()
	if err != nil {
		fmt.Println("Error waiting:", err)
		return
	}

	fmt.Printf("Container exited with code: %d\n", exitCode)
}

// ExampleContainer_State demonstrates getting container state information.
func ExampleContainer_State() {
	rc, err := crun.NewRuntimeContext(crun.RuntimeConfig{
		StateRoot: "/run/crun",
	})
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer rc.Close()

	// Get handle to an existing container
	container := rc.Get("my-container-id")

	// Get container state
	state, err := container.State()
	if err != nil {
		if errors.Is(err, crun.ErrContainerNotFound) {
			fmt.Println("Container not found")
			return
		}
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Container %s is %s (PID: %d)\n", state.ID, state.Status, state.Pid)
}

// Example_errorHandling demonstrates using errors.Is() for error classification.
func Example_errorHandling() {
	rc, err := crun.NewRuntimeContext(crun.RuntimeConfig{
		StateRoot: "/run/crun",
	})
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer rc.Close()

	// Try to get state of non-existent container
	container := rc.Get("nonexistent-container")
	_, err = container.State()

	// Use errors.Is() to check error types
	switch {
	case errors.Is(err, crun.ErrContainerNotFound):
		fmt.Println("Container does not exist")
	case errors.Is(err, crun.ErrContainerExists):
		fmt.Println("Container already exists")
	case errors.Is(err, crun.ErrInvalidContainerSpec):
		fmt.Println("Invalid container specification")
	case err != nil:
		fmt.Println("Other error:", err)
	default:
		fmt.Println("Success")
	}
}
