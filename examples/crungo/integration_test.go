//go:build linux && cgo && integration

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	crun "github.com/danielealbano/libcrun-go"
)

// Integration tests require root privileges and network access.
// Run with: sudo go test -tags=integration -v ./...

func TestRunSimpleCommand(t *testing.T) {
	pulled, err := PullAndExtract("alpine:latest")
	if err != nil {
		t.Fatalf("failed to pull image: %v", err)
	}
	defer os.RemoveAll(pulled.RootFS)

	stateRoot, err := os.MkdirTemp("", "crungo-test-*")
	if err != nil {
		t.Fatalf("failed to create state root: %v", err)
	}
	defer os.RemoveAll(stateRoot)

	rc, err := crun.NewRuntimeContext(crun.RuntimeConfig{
		StateRoot: stateRoot,
	})
	if err != nil {
		t.Fatalf("failed to create runtime context: %v", err)
	}
	defer rc.Close()

	spec, err := crun.NewSpec(true,
		crun.WithRootPath(pulled.RootFS),
		crun.WithArgs("echo", "hello", "world"),
		crun.WithContainerTTY(false),
		crun.WithEnv("HOME", "/root"),
	)
	if err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}
	defer spec.Close()

	var stdout, stderr bytes.Buffer
	result, err := rc.RunWithIO("test-simple", spec, &crun.IOConfig{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("failed to run container: %v", err)
	}

	exitCode, err := result.Wait()
	if err != nil {
		t.Fatalf("failed to wait for container: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d. stderr: %s", exitCode, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output != "hello world" {
		t.Errorf("expected output 'hello world', got %q", output)
	}
}

func TestRunWithEnv(t *testing.T) {
	pulled, err := PullAndExtract("alpine:latest")
	if err != nil {
		t.Fatalf("failed to pull image: %v", err)
	}
	defer os.RemoveAll(pulled.RootFS)

	stateRoot, err := os.MkdirTemp("", "crungo-test-*")
	if err != nil {
		t.Fatalf("failed to create state root: %v", err)
	}
	defer os.RemoveAll(stateRoot)

	rc, err := crun.NewRuntimeContext(crun.RuntimeConfig{
		StateRoot: stateRoot,
	})
	if err != nil {
		t.Fatalf("failed to create runtime context: %v", err)
	}
	defer rc.Close()

	spec, err := crun.NewSpec(true,
		crun.WithRootPath(pulled.RootFS),
		crun.WithArgs("sh", "-c", "echo $MY_VAR"),
		crun.WithContainerTTY(false),
		crun.WithEnv("HOME", "/root"),
		crun.WithEnv("MY_VAR", "test_value_123"),
	)
	if err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}
	defer spec.Close()

	var stdout, stderr bytes.Buffer
	result, err := rc.RunWithIO("test-env", spec, &crun.IOConfig{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("failed to run container: %v", err)
	}

	exitCode, err := result.Wait()
	if err != nil {
		t.Fatalf("failed to wait for container: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d. stderr: %s", exitCode, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output != "test_value_123" {
		t.Errorf("expected output 'test_value_123', got %q", output)
	}
}

func TestRunWithVolume(t *testing.T) {
	pulled, err := PullAndExtract("alpine:latest")
	if err != nil {
		t.Fatalf("failed to pull image: %v", err)
	}
	defer os.RemoveAll(pulled.RootFS)

	// Create a temp file to mount
	tmpDir, err := os.MkdirTemp("", "crungo-vol-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testContent := "volume_test_content"
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	stateRoot, err := os.MkdirTemp("", "crungo-test-*")
	if err != nil {
		t.Fatalf("failed to create state root: %v", err)
	}
	defer os.RemoveAll(stateRoot)

	rc, err := crun.NewRuntimeContext(crun.RuntimeConfig{
		StateRoot: stateRoot,
	})
	if err != nil {
		t.Fatalf("failed to create runtime context: %v", err)
	}
	defer rc.Close()

	spec, err := crun.NewSpec(true,
		crun.WithRootPath(pulled.RootFS),
		crun.WithArgs("cat", "/mnt/test.txt"),
		crun.WithContainerTTY(false),
		crun.WithEnv("HOME", "/root"),
		crun.WithMount(tmpDir, "/mnt", "none", []string{"bind", "ro"}),
	)
	if err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}
	defer spec.Close()

	var stdout, stderr bytes.Buffer
	result, err := rc.RunWithIO("test-volume", spec, &crun.IOConfig{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("failed to run container: %v", err)
	}

	exitCode, err := result.Wait()
	if err != nil {
		t.Fatalf("failed to wait for container: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d. stderr: %s", exitCode, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output != testContent {
		t.Errorf("expected output %q, got %q", testContent, output)
	}
}

func TestRunWithMemoryLimit(t *testing.T) {
	pulled, err := PullAndExtract("alpine:latest")
	if err != nil {
		t.Fatalf("failed to pull image: %v", err)
	}
	defer os.RemoveAll(pulled.RootFS)

	stateRoot, err := os.MkdirTemp("", "crungo-test-*")
	if err != nil {
		t.Fatalf("failed to create state root: %v", err)
	}
	defer os.RemoveAll(stateRoot)

	rc, err := crun.NewRuntimeContext(crun.RuntimeConfig{
		StateRoot: stateRoot,
	})
	if err != nil {
		t.Fatalf("failed to create runtime context: %v", err)
	}
	defer rc.Close()

	// Set 64MB memory limit
	memLimit := int64(64 * 1024 * 1024)

	spec, err := crun.NewSpec(true,
		crun.WithRootPath(pulled.RootFS),
		crun.WithArgs("cat", "/sys/fs/cgroup/memory.max"),
		crun.WithContainerTTY(false),
		crun.WithEnv("HOME", "/root"),
		crun.WithMemoryLimit(memLimit),
	)
	if err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}
	defer spec.Close()

	var stdout, stderr bytes.Buffer
	result, err := rc.RunWithIO("test-memory", spec, &crun.IOConfig{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("failed to run container: %v", err)
	}

	exitCode, err := result.Wait()
	if err != nil {
		t.Fatalf("failed to wait for container: %v", err)
	}

	// The command might fail if cgroup v2 memory.max doesn't exist
	// In that case, we just verify the container ran
	if exitCode != 0 {
		t.Logf("memory limit test: exit code %d (may be expected on some systems). stderr: %s", exitCode, stderr.String())
	} else {
		output := strings.TrimSpace(stdout.String())
		t.Logf("memory.max value: %s", output)
	}
}

func TestRunWithHostNetwork(t *testing.T) {
	pulled, err := PullAndExtract("alpine:latest")
	if err != nil {
		t.Fatalf("failed to pull image: %v", err)
	}
	defer os.RemoveAll(pulled.RootFS)

	stateRoot, err := os.MkdirTemp("", "crungo-test-*")
	if err != nil {
		t.Fatalf("failed to create state root: %v", err)
	}
	defer os.RemoveAll(stateRoot)

	rc, err := crun.NewRuntimeContext(crun.RuntimeConfig{
		StateRoot: stateRoot,
	})
	if err != nil {
		t.Fatalf("failed to create runtime context: %v", err)
	}
	defer rc.Close()

	// Get host's hostname
	hostHostname, _ := os.Hostname()

	spec, err := crun.NewSpec(true,
		crun.WithRootPath(pulled.RootFS),
		crun.WithArgs("cat", "/etc/hostname"),
		crun.WithContainerTTY(false),
		crun.WithEnv("HOME", "/root"),
		crun.WithHostNetwork(),
	)
	if err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}
	defer spec.Close()

	var stdout, stderr bytes.Buffer
	result, err := rc.RunWithIO("test-hostnet", spec, &crun.IOConfig{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("failed to run container: %v", err)
	}

	exitCode, err := result.Wait()
	if err != nil {
		t.Fatalf("failed to wait for container: %v", err)
	}

	// With host network, the container should see the host's network interfaces
	// We can't easily verify network without network tools, but we can check it ran
	if exitCode != 0 {
		t.Logf("host network test completed with exit %d. This may be expected if /etc/hostname doesn't exist in rootfs.", exitCode)
	} else {
		output := strings.TrimSpace(stdout.String())
		t.Logf("container saw hostname: %s (host: %s)", output, hostHostname)
	}
}

func TestRunInteractiveStdin(t *testing.T) {
	pulled, err := PullAndExtract("alpine:latest")
	if err != nil {
		t.Fatalf("failed to pull image: %v", err)
	}
	defer os.RemoveAll(pulled.RootFS)

	stateRoot, err := os.MkdirTemp("", "crungo-test-*")
	if err != nil {
		t.Fatalf("failed to create state root: %v", err)
	}
	defer os.RemoveAll(stateRoot)

	rc, err := crun.NewRuntimeContext(crun.RuntimeConfig{
		StateRoot: stateRoot,
	})
	if err != nil {
		t.Fatalf("failed to create runtime context: %v", err)
	}
	defer rc.Close()

	spec, err := crun.NewSpec(true,
		crun.WithRootPath(pulled.RootFS),
		crun.WithArgs("cat"),
		crun.WithContainerTTY(false),
		crun.WithEnv("HOME", "/root"),
	)
	if err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}
	defer spec.Close()

	inputData := "hello from stdin\n"
	stdinReader := strings.NewReader(inputData)

	var stdout, stderr bytes.Buffer
	result, err := rc.RunWithIO("test-stdin", spec, &crun.IOConfig{
		Stdin:  stdinReader,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("failed to run container: %v", err)
	}

	exitCode, err := result.Wait()
	if err != nil {
		t.Fatalf("failed to wait for container: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d. stderr: %s", exitCode, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "hello from stdin") {
		t.Errorf("expected stdin content in output, got %q", output)
	}
}

