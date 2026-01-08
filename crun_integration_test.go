//go:build linux && cgo && integration

package crun

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Integration tests require:
// - libcrun installed
// - Root privileges (or user namespaces for rootless)
// - A minimal rootfs (busybox)
//
// Run with: sudo go test -v -tags=integration ./...

func skipIfNotRoot(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Test requires root privileges")
	}
}

func testRootfs(t *testing.T) string {
	// Check for a busybox rootfs in common locations
	paths := []string{
		"/var/lib/containers/test-rootfs",
		"/tmp/test-rootfs",
		os.Getenv("TEST_ROOTFS"),
	}

	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(p, "bin/sh")); err == nil {
			return p
		}
	}

	t.Skip("No test rootfs found. Set TEST_ROOTFS env var or create /tmp/test-rootfs with busybox")
	return ""
}

func testRuntimeContext(t *testing.T) *RuntimeContext {
	stateRoot := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateRoot, 0755); err != nil {
		t.Fatalf("Failed to create state root: %v", err)
	}

	rc, err := NewRuntimeContext(RuntimeConfig{
		Bundle:    t.TempDir(),
		StateRoot: stateRoot,
	})
	if err != nil {
		t.Fatalf("Failed to create RuntimeContext: %v", err)
	}

	t.Cleanup(func() {
		rc.Close()
	})

	return rc
}

func TestIntegration_CreateStartDelete(t *testing.T) {
	skipIfNotRoot(t)
	rootfs := testRootfs(t)
	rc := testRuntimeContext(t)

	spec, err := NewSpec(false,
		WithRootPath(rootfs),
		WithContainerTTY(false),
		WithArgs("/bin/sh", "-c", "exit 0"),
	)
	if err != nil {
		t.Fatalf("Failed to create spec: %v", err)
	}
	defer spec.Close()

	ctr, err := rc.Create("test-create-start", spec, CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	// Check state after create
	state, err := ctr.State()
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}
	if state.Status != StatusCreated {
		t.Errorf("Status = %q, want %q", state.Status, StatusCreated)
	}

	// Start the container
	if err := ctr.Start(); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Delete the container
	if err := ctr.Delete(true); err != nil {
		t.Fatalf("Failed to delete container: %v", err)
	}
}

func TestIntegration_Run(t *testing.T) {
	skipIfNotRoot(t)
	rootfs := testRootfs(t)
	rc := testRuntimeContext(t)

	spec, err := NewSpec(false,
		WithRootPath(rootfs),
		WithContainerTTY(false),
		WithArgs("/bin/sh", "-c", "echo hello"),
	)
	if err != nil {
		t.Fatalf("Failed to create spec: %v", err)
	}
	defer spec.Close()

	var stdout bytes.Buffer
	result, err := rc.RunWithIO("test-run", spec, &IOConfig{
		Stdout: &stdout,
	})
	if err != nil {
		t.Fatalf("Failed to run container: %v", err)
	}

	exitCode, err := result.Wait()
	if err != nil {
		t.Fatalf("Failed to wait for container: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	if got := strings.TrimSpace(stdout.String()); got != "hello" {
		t.Errorf("Expected stdout 'hello', got %q", got)
	}

	// Container should have exited, delete it
	if err := result.Container.Delete(true); err != nil {
		t.Fatalf("Failed to delete container: %v", err)
	}
}

func TestIntegration_List(t *testing.T) {
	skipIfNotRoot(t)
	rootfs := testRootfs(t)
	rc := testRuntimeContext(t)

	// Initially empty
	containers, err := rc.List()
	if err != nil {
		t.Fatalf("Failed to list containers: %v", err)
	}
	if len(containers) != 0 {
		t.Errorf("Expected 0 containers, got %d", len(containers))
	}

	// Create a container
	spec, err := NewSpec(false,
		WithRootPath(rootfs),
		WithContainerTTY(false),
		WithArgs("/bin/sleep", "30"),
	)
	if err != nil {
		t.Fatalf("Failed to create spec: %v", err)
	}
	defer spec.Close()

	ctr, err := rc.Create("test-list", spec, CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	defer ctr.Delete(true)

	// Should have one container now
	containers, err = rc.List()
	if err != nil {
		t.Fatalf("Failed to list containers: %v", err)
	}
	if len(containers) != 1 {
		t.Errorf("Expected 1 container, got %d", len(containers))
	}
	if containers[0].ID != "test-list" {
		t.Errorf("Container ID = %q, want test-list", containers[0].ID)
	}
}

func TestIntegration_Kill(t *testing.T) {
	skipIfNotRoot(t)
	rootfs := testRootfs(t)
	rc := testRuntimeContext(t)

	spec, err := NewSpec(false,
		WithRootPath(rootfs),
		WithContainerTTY(false),
		WithArgs("/bin/sleep", "300"),
	)
	if err != nil {
		t.Fatalf("Failed to create spec: %v", err)
	}
	defer spec.Close()

	ctr, err := rc.Create("test-kill", spec, CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	defer ctr.Delete(true)

	if err := ctr.Start(); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Container should be running
	running, err := ctr.IsRunning()
	if err != nil {
		t.Fatalf("Failed to check if running: %v", err)
	}
	if !running {
		t.Error("Container should be running")
	}

	// Kill it
	if err := ctr.Kill(SIGTERM); err != nil {
		t.Fatalf("Failed to kill container: %v", err)
	}
}

func TestIntegration_ContainerNotFound(t *testing.T) {
	skipIfNotRoot(t)
	rc := testRuntimeContext(t)

	ctr := rc.Get("nonexistent-container")
	_, err := ctr.State()

	if err == nil {
		t.Fatal("Expected error for nonexistent container")
	}

	if !errors.Is(err, ErrContainerNotFound) {
		t.Errorf("Expected ErrContainerNotFound, got %v", err)
	}
}

func TestIntegration_UpdateResources(t *testing.T) {
	skipIfNotRoot(t)
	rootfs := testRootfs(t)
	rc := testRuntimeContext(t)

	spec, err := NewSpec(false,
		WithRootPath(rootfs),
		WithContainerTTY(false),
		WithArgs("/bin/sleep", "300"),
	)
	if err != nil {
		t.Fatalf("Failed to create spec: %v", err)
	}
	defer spec.Close()

	ctr, err := rc.Create("test-update", spec, CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	defer ctr.Delete(true)

	if err := ctr.Start(); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Update memory limit
	memLimit := int64(256 * 1024 * 1024)
	err = ctr.UpdateResources(&specs.LinuxResources{
		Memory: &specs.LinuxMemory{
			Limit: &memLimit,
		},
	})
	if err != nil {
		t.Errorf("Failed to update resources: %v", err)
	}
}

func TestIntegration_PauseUnpause(t *testing.T) {
	skipIfNotRoot(t)
	rootfs := testRootfs(t)
	rc := testRuntimeContext(t)

	spec, err := NewSpec(false,
		WithRootPath(rootfs),
		WithContainerTTY(false),
		WithArgs("/bin/sleep", "300"),
	)
	if err != nil {
		t.Fatalf("Failed to create spec: %v", err)
	}
	defer spec.Close()

	ctr, err := rc.Create("test-pause", spec, CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	defer ctr.Delete(true)

	if err := ctr.Start(); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Pause
	if err := ctr.Pause(); err != nil {
		t.Fatalf("Failed to pause container: %v", err)
	}

	state, err := ctr.State()
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}
	if state.Status != StatusPaused {
		t.Errorf("Status = %q, want %q", state.Status, StatusPaused)
	}

	// Unpause
	if err := ctr.Unpause(); err != nil {
		t.Fatalf("Failed to unpause container: %v", err)
	}

	state, err = ctr.State()
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}
	if state.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", state.Status, StatusRunning)
	}
}

func TestIntegration_PIDs(t *testing.T) {
	skipIfNotRoot(t)
	rootfs := testRootfs(t)

	// Try with systemd cgroup manager for proper cgroup tracking
	stateRoot := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateRoot, 0755); err != nil {
		t.Fatalf("Failed to create state root: %v", err)
	}

	rc, err := NewRuntimeContext(RuntimeConfig{
		Bundle:        t.TempDir(),
		StateRoot:     stateRoot,
		SystemdCgroup: true,
	})
	if err != nil {
		t.Fatalf("Failed to create RuntimeContext: %v", err)
	}
	defer rc.Close()

	spec, err := NewSpec(false,
		WithRootPath(rootfs),
		WithContainerTTY(false),
		WithArgs("/bin/sleep", "300"),
	)
	if err != nil {
		t.Fatalf("Failed to create spec: %v", err)
	}
	defer spec.Close()

	ctr, err := rc.Create("test-pids", spec, CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	defer ctr.Delete(true)

	if err := ctr.Start(); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Get state to verify init PID - this always works
	state, err := ctr.State()
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}
	if state.Pid <= 0 {
		t.Fatalf("Expected valid init PID, got %d", state.Pid)
	}

	// Verify the process exists in /proc
	if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(state.Pid))); err != nil {
		t.Fatalf("Init process %d not found in /proc: %v", state.Pid, err)
	}

	// Get PIDs from cgroup - requires proper cgroup setup
	pids, err := ctr.PIDs(true)
	if err != nil {
		t.Skipf("PIDs() not available (cgroup error): %v", err)
	}

	if len(pids) == 0 {
		t.Skip("PIDs() returned empty - cgroup tracking not available in this environment")
	}

	// Verify init PID is in the list
	found := false
	for _, p := range pids {
		if p == state.Pid {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Init PID %d not found in cgroup PIDs %v", state.Pid, pids)
	}
}

func TestIntegration_SpecOptions(t *testing.T) {
	skipIfNotRoot(t)
	rootfs := testRootfs(t)
	rc := testRuntimeContext(t)

	memLimit := int64(128 * 1024 * 1024)
	cpuShares := uint64(256)

	spec, err := NewSpec(false,
		WithRootPath(rootfs),
		WithContainerTTY(false),
		WithArgs("/bin/sh", "-c", "echo $FOO && exit 0"),
		WithEnv("FOO", "bar"),
		WithMemoryLimit(memLimit),
		WithCPUShares(cpuShares),
		WithHostname("testhost"),
		WithAnnotation("test.key", "test.value"),
	)
	if err != nil {
		t.Fatalf("Failed to create spec: %v", err)
	}
	defer spec.Close()

	var stdout bytes.Buffer
	result, err := rc.RunWithIO("test-spec-options", spec, &IOConfig{
		Stdout: &stdout,
	})
	if err != nil {
		t.Fatalf("Failed to run container: %v", err)
	}

	exitCode, err := result.Wait()
	if err != nil {
		t.Fatalf("Failed to wait for container: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	if got := strings.TrimSpace(stdout.String()); got != "bar" {
		t.Errorf("Expected stdout 'bar', got %q", got)
	}

	defer result.Container.Delete(true)
}

func TestIntegration_ParallelContainers(t *testing.T) {
	skipIfNotRoot(t)
	rootfs := testRootfs(t)
	rc := testRuntimeContext(t)

	const numContainers = 5
	var wg sync.WaitGroup
	errChan := make(chan error, numContainers)
	outputs := make(chan string, numContainers)

	for i := 0; i < numContainers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			spec, err := NewSpec(false,
				WithRootPath(rootfs),
				WithContainerTTY(false),
				WithArgs("/bin/sh", "-c", fmt.Sprintf("echo container-%d", idx)),
			)
			if err != nil {
				errChan <- fmt.Errorf("container %d: failed to create spec: %w", idx, err)
				return
			}
			defer spec.Close()

			var stdout bytes.Buffer
			result, err := rc.RunWithIO(
				fmt.Sprintf("test-parallel-%d", idx),
				spec,
				&IOConfig{Stdout: &stdout},
			)
			if err != nil {
				errChan <- fmt.Errorf("container %d: failed to run: %w", idx, err)
				return
			}

			exitCode, err := result.Wait()
			if err != nil {
				errChan <- fmt.Errorf("container %d: failed to wait: %w", idx, err)
				return
			}
			if exitCode != 0 {
				errChan <- fmt.Errorf("container %d: exited with %d", idx, exitCode)
				return
			}

			outputs <- stdout.String()
			_ = result.Container.Delete(true)
		}(i)
	}

	wg.Wait()
	close(errChan)
	close(outputs)

	for err := range errChan {
		t.Errorf("parallel container error: %v", err)
	}

	outputSet := make(map[string]bool)
	for out := range outputs {
		outputSet[strings.TrimSpace(out)] = true
	}

	for i := 0; i < numContainers; i++ {
		expected := fmt.Sprintf("container-%d", i)
		if !outputSet[expected] {
			t.Errorf("missing output for container %d", i)
		}
	}
}

func TestIntegration_ContainerCrash(t *testing.T) {
	skipIfNotRoot(t)
	rootfs := testRootfs(t)
	rc := testRuntimeContext(t)

	// Get initial zombie count
	initialZombies := countZombieProcesses(t)

	// Test 1: Container exits with non-zero code
	t.Run("NonZeroExit", func(t *testing.T) {
		spec, err := NewSpec(false,
			WithRootPath(rootfs),
			WithContainerTTY(false),
			WithArgs("/bin/sh", "-c", "exit 42"),
		)
		if err != nil {
			t.Fatalf("Failed to create spec: %v", err)
		}
		defer spec.Close()

		var stderr bytes.Buffer
		result, err := rc.RunWithIO("test-crash-exit", spec, &IOConfig{
			Stderr: &stderr,
		})
		if err != nil {
			t.Fatalf("Failed to run container: %v", err)
		}

		exitCode, err := result.Wait()
		if err != nil {
			t.Fatalf("Failed to wait: %v", err)
		}
		if exitCode != 42 {
			t.Errorf("Expected exit code 42, got %d", exitCode)
		}

		_ = result.Container.Delete(true)
	})

	// Test 2: Container with command not found (error case)
	t.Run("CommandNotFound", func(t *testing.T) {
		spec, err := NewSpec(false,
			WithRootPath(rootfs),
			WithContainerTTY(false),
			WithArgs("/nonexistent/command"),
		)
		if err != nil {
			t.Fatalf("Failed to create spec: %v", err)
		}
		defer spec.Close()

		result, err := rc.RunWithIO("test-crash-notfound", spec, &IOConfig{})
		if err != nil {
			// Error during setup is also acceptable
			t.Logf("Run failed (expected): %v", err)
			return
		}

		exitCode, err := result.Wait()
		if err != nil {
			t.Logf("Wait failed (expected for command not found): %v", err)
		} else if exitCode == 0 {
			t.Errorf("Expected non-zero exit code for command not found, got 0")
		} else {
			t.Logf("Got expected non-zero exit code: %d", exitCode)
		}

		_ = result.Container.Delete(true)
	})

	// Test 3: Rapid successive crashes to stress test cleanup
	t.Run("RapidCrashes", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			spec, err := NewSpec(false,
				WithRootPath(rootfs),
				WithContainerTTY(false),
				WithArgs("/bin/sh", "-c", fmt.Sprintf("exit %d", (i%255)+1)),
			)
			if err != nil {
				t.Fatalf("iteration %d: failed to create spec: %v", i, err)
			}

			result, err := rc.RunWithIO(fmt.Sprintf("test-rapid-%d", i), spec, &IOConfig{})
			if err != nil {
				spec.Close()
				t.Fatalf("iteration %d: failed to run: %v", i, err)
			}

			exitCode, err := result.Wait()
			if err != nil {
				spec.Close()
				t.Fatalf("iteration %d: failed to wait: %v", i, err)
			}
			expected := (i % 255) + 1
			if exitCode != expected {
				t.Errorf("iteration %d: expected exit code %d, got %d", i, expected, exitCode)
			}

			_ = result.Container.Delete(true)
			spec.Close()
		}
	})

	// Test 4: Multiple crashes in parallel
	t.Run("ParallelCrashes", func(t *testing.T) {
		const numCrashes = 5
		var wg sync.WaitGroup

		for i := 0; i < numCrashes; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				spec, err := NewSpec(false,
					WithRootPath(rootfs),
					WithContainerTTY(false),
					WithArgs("/bin/sh", "-c", fmt.Sprintf("exit %d", idx+1)),
				)
				if err != nil {
					t.Errorf("container %d: failed to create spec: %v", idx, err)
					return
				}
				defer spec.Close()

				result, err := rc.RunWithIO(
					fmt.Sprintf("test-crash-parallel-%d", idx),
					spec,
					&IOConfig{},
				)
				if err != nil {
					t.Errorf("container %d: failed to run: %v", idx, err)
					return
				}

				exitCode, err := result.Wait()
				if err != nil {
					t.Errorf("container %d: failed to wait: %v", idx, err)
					return
				}
				if exitCode != idx+1 {
					t.Errorf("container %d: expected exit code %d, got %d", idx, idx+1, exitCode)
				}

				_ = result.Container.Delete(true)
			}(i)
		}

		wg.Wait()
	})

	// Give a moment for any zombie processes to appear
	time.Sleep(100 * time.Millisecond)

	// Check for zombie processes
	finalZombies := countZombieProcesses(t)
	newZombies := finalZombies - initialZombies
	if newZombies > 0 {
		t.Errorf("Found %d new zombie processes after container crashes", newZombies)
	}
}

// countZombieProcesses counts zombie processes owned by the current process
func countZombieProcesses(t *testing.T) int {
	t.Helper()
	myPid := os.Getpid()
	count := 0

	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Fatalf("Failed to read /proc: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		statPath := filepath.Join("/proc", entry.Name(), "stat")
		data, err := os.ReadFile(statPath)
		if err != nil {
			continue
		}

		// Parse /proc/[pid]/stat - format: pid (comm) state ppid ...
		statStr := string(data)
		// Find closing paren for comm field
		closeParenIdx := strings.LastIndex(statStr, ")")
		if closeParenIdx == -1 || closeParenIdx+2 >= len(statStr) {
			continue
		}

		fields := strings.Fields(statStr[closeParenIdx+2:])
		if len(fields) < 2 {
			continue
		}

		state := fields[0]
		ppid, _ := strconv.Atoi(fields[1])

		// Check if zombie and our child
		if state == "Z" && ppid == myPid {
			count++
			t.Logf("Found zombie process: PID %d, PPID %d", pid, myPid)
		}
	}

	return count
}

func TestIntegration_Terminal(t *testing.T) {
	skipIfNotRoot(t)
	rootfs := testRootfs(t)

	// Create console socket
	socketPath := filepath.Join(t.TempDir(), "console.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create console socket: %v", err)
	}

	// Channel to receive PTY master fd and keep connection alive
	ptyReceived := make(chan net.Conn, 1)
	listenerClosed := make(chan struct{})

	// Start goroutine to accept the PTY master fd
	go func() {
		defer close(listenerClosed)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		// Keep connection open - send it to main goroutine
		ptyReceived <- conn
	}()

	stateRoot := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateRoot, 0755); err != nil {
		listener.Close()
		t.Fatalf("Failed to create state root: %v", err)
	}

	rc, err := NewRuntimeContext(RuntimeConfig{
		Bundle:        t.TempDir(),
		StateRoot:     stateRoot,
		ConsoleSocket: socketPath,
	})
	if err != nil {
		listener.Close()
		t.Fatalf("Failed to create RuntimeContext: %v", err)
	}
	defer rc.Close()

	spec, err := NewSpec(false,
		WithRootPath(rootfs),
		WithContainerTTY(true), // Enable terminal
		WithArgs("/bin/sh", "-c", "exit 0"),
	)
	if err != nil {
		listener.Close()
		t.Fatalf("Failed to create spec: %v", err)
	}
	defer spec.Close()

	ctr, err := rc.Create("test-terminal", spec, CreateOptions{})
	if err != nil {
		listener.Close()
		t.Fatalf("Failed to create container with terminal: %v", err)
	}
	defer ctr.Delete(true)

	// Verify we received the PTY fd
	var conn net.Conn
	select {
	case conn = <-ptyReceived:
		defer conn.Close()
	case <-time.After(5 * time.Second):
		listener.Close()
		t.Fatal("Timeout waiting for PTY master fd")
	}

	// Close listener now that we have the connection
	listener.Close()
	<-listenerClosed

	// Start the container
	if err := ctr.Start(); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}
}
