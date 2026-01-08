//go:build linux && cgo

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	crun "github.com/danielealbano/libcrun-go"
)

// logBuffer collects libcrun log entries for later display
var (
	logBuffer   bytes.Buffer
	logBufferMu sync.Mutex
)

func main() {
	// Enable debug verbosity and log handler for libcrun if DEBUG=1 is set
	debugMode := os.Getenv("DEBUG") == "1"
	if debugMode {
		fmt.Println("Debug mode enabled (LIBCRUN_VERBOSITY_DEBUG)")
		crun.SetVerbosity(crun.VerbosityDebug)

		// Set up a custom log handler to collect libcrun's internal logs
		crun.SetLogHandler(func(entry crun.LogEntry) {
			level := "ERROR"
			switch entry.Verbosity {
			case crun.VerbosityWarning:
				level = "WARN"
			case crun.VerbosityDebug:
				level = "DEBUG"
			}
			logBufferMu.Lock()
			if entry.Errno != 0 {
				fmt.Fprintf(&logBuffer, "[libcrun:%s] %s (errno=%d)\n", level, entry.Message, entry.Errno)
			} else {
				fmt.Fprintf(&logBuffer, "[libcrun:%s] %s\n", level, entry.Message)
			}
			logBufferMu.Unlock()
		})
	}

	// Find the static binary - it should be in static-helloworld/helloworld relative to this example
	exePath, err := os.Executable()
	if err != nil {
		// Fallback to working directory if running via go run
		exePath, _ = os.Getwd()
	} else {
		exePath = filepath.Dir(exePath)
	}

	// Try multiple paths to find the static binary
	staticBinaryPath := ""
	candidates := []string{
		filepath.Join(exePath, "static-helloworld", "helloworld"),
		filepath.Join(".", "static-helloworld", "helloworld"),
		filepath.Join(filepath.Dir(exePath), "static-helloworld", "helloworld"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			staticBinaryPath, _ = filepath.Abs(candidate)
			break
		}
	}

	if staticBinaryPath == "" {
		fmt.Fprintln(os.Stderr, "Error: static binary not found. Run 'make' in static-helloworld/ first.")
		os.Exit(1)
	}

	fmt.Printf("Found static binary: %s\n", staticBinaryPath)

	// Create a temporary rootfs directory
	rootfs, err := os.MkdirTemp("", "crun-rootfs-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating rootfs: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(rootfs)

	// Copy the static binary to the rootfs
	destBinary := filepath.Join(rootfs, "helloworld")
	if err := copyFile(staticBinaryPath, destBinary); err != nil {
		fmt.Fprintf(os.Stderr, "Error copying binary to rootfs: %v\n", err)
		os.Exit(1)
	}

	// Create minimal /etc/passwd for crun (needed to detect HOME)
	etcDir := filepath.Join(rootfs, "etc")
	if err := os.MkdirAll(etcDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating /etc in rootfs: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(etcDir, "passwd"), []byte("root:x:0:0:root:/root:/bin/sh\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating /etc/passwd in rootfs: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Using rootfs: %s\n", rootfs)

	// Create a temporary state root for this example
	stateRoot, err := os.MkdirTemp("", "crun-state-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating state root: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(stateRoot)

	// Create runtime context
	rc, err := crun.NewRuntimeContext(crun.RuntimeConfig{
		StateRoot: stateRoot,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating runtime context: %v\n", err)
		os.Exit(1)
	}
	defer rc.Close()

	// Create container spec using the temporary rootfs
	spec, err := crun.NewSpec(
		true, // rootless mode
		crun.WithRootPath(rootfs),
		crun.WithArgs("/helloworld"),
		crun.WithContainerTTY(false), // Disable terminal - we use pipes
		crun.WithEnv("HOME", "/root"),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating container spec: %v\n", err)
		os.Exit(1)
	}
	defer spec.Close()

	// Capture container output
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	fmt.Println()
	fmt.Println("Running container...")

	// Run the container with I/O capture
	result, err := rc.RunWithIO("helloworld-example", spec, &crun.IOConfig{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running container: %v\n", err)
		os.Exit(1)
	}

	// Wait for container to finish
	exitCode, err := result.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error waiting for container: %v\n", err)
		os.Exit(1)
	}

	// Print the output from the container with clear delimiters
	fmt.Println()
	fmt.Println("========== CONTAINER RESULTS ==========")
	fmt.Printf("Exit code: %d\n", exitCode)
	fmt.Println()

	fmt.Printf("--- STDOUT (%d bytes) ---\n", stdout.Len())
	if stdout.Len() > 0 {
		fmt.Print(stdout.String())
		if !bytes.HasSuffix(stdout.Bytes(), []byte("\n")) {
			fmt.Println()
		}
	} else {
		fmt.Println("(empty)")
	}

	fmt.Printf("--- STDERR (%d bytes) ---\n", stderr.Len())
	if stderr.Len() > 0 {
		fmt.Print(stderr.String())
		if !bytes.HasSuffix(stderr.Bytes(), []byte("\n")) {
			fmt.Println()
		}
	} else {
		fmt.Println("(empty)")
	}
	fmt.Println("========================================")

	// Print libcrun logs if debug mode was enabled
	if debugMode {
		fmt.Println()
		fmt.Println("=========== LIBCRUN LOGS ===========")
		logBufferMu.Lock()
		if logBuffer.Len() > 0 {
			fmt.Print(logBuffer.String())
		} else {
			fmt.Println("(no logs captured)")
		}
		logBufferMu.Unlock()
		fmt.Println("====================================")
	}
}

// copyFile copies a file from src to dst, preserving the executable permission.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
