//go:build linux && cgo

package main

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	crun "github.com/danielealbano/libcrun-go"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	// Run command flags
	interactive   bool
	tty           bool
	envVars       []string
	volumes       []string
	user          string
	cpus          string
	memory        string
	containerName string
	workdir       string
	entrypoint    string
	netMode       string
	crunDebug     bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "crungo",
		Short: "A minimal Docker/Podman-compatible CLI using libcrun-go",
		Long: `crungo is a minimal container runtime CLI that demonstrates libcrun-go capabilities.
It supports pulling OCI images and running them with common container options.`,
	}

	runCmd := &cobra.Command{
		Use:   "run [OPTIONS] IMAGE [COMMAND] [ARG...]",
		Short: "Run a container from an image",
		Long: `Pull an image (if not cached) and run a container.
The container is automatically removed when it exits.`,
		Args: cobra.MinimumNArgs(1),
		RunE: runContainer,
	}

	// Define flags
	runCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Keep stdin open")
	runCmd.Flags().BoolVarP(&tty, "tty", "t", false, "Allocate a pseudo-TTY")
	runCmd.Flags().StringArrayVarP(&envVars, "env", "e", nil, "Set environment variables (KEY=VALUE)")
	runCmd.Flags().StringArrayVarP(&volumes, "volume", "v", nil, "Bind mount a volume (host:container[:ro])")
	runCmd.Flags().StringVarP(&user, "user", "u", "", "Run as user (uid[:gid])")
	runCmd.Flags().StringVar(&cpus, "cpus", "", "CPU limit (e.g., '0.5', '2')")
	runCmd.Flags().StringVarP(&memory, "memory", "m", "", "Memory limit (e.g., '256m', '1g')")
	runCmd.Flags().StringVar(&containerName, "name", "", "Container name (default: random)")
	runCmd.Flags().StringVarP(&workdir, "workdir", "w", "", "Working directory inside the container")
	runCmd.Flags().StringVar(&entrypoint, "entrypoint", "", "Override the image entrypoint")
	runCmd.Flags().StringVar(&netMode, "net", "none", "Network mode: 'none' (isolated) or 'host' (share host network)")
	runCmd.Flags().BoolVar(&crunDebug, "crun-debug", false, "Enable libcrun debug logs")

	rootCmd.AddCommand(runCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runContainer(cmd *cobra.Command, args []string) error {
	imageRef := args[0]
	var containerCmd []string
	if len(args) > 1 {
		containerCmd = args[1:]
	}

	// Enable debug logging if requested
	if crunDebug {
		crun.SetVerbosity(crun.VerbosityDebug)
	}

	// Generate container name if not provided
	ctrName := containerName
	if ctrName == "" {
		ctrName = generateName()
	}

	// Pull and extract image
	pulled, err := PullAndExtract(imageRef)
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer os.RemoveAll(pulled.RootFS)

	// Create state root
	stateRoot, err := os.MkdirTemp("", "crungo-state-*")
	if err != nil {
		return fmt.Errorf("failed to create state root: %w", err)
	}
	defer os.RemoveAll(stateRoot)

	// Build spec options
	specOpts, err := buildSpecOptions(pulled, containerCmd)
	if err != nil {
		return fmt.Errorf("failed to build spec options: %w", err)
	}

	// Handle network mode
	switch netMode {
	case "none":
		// Default: isolated network namespace (no changes needed)
	case "host":
		// Share host network namespace
		specOpts = append(specOpts, crun.WithHostNetwork())
		// Add CAP_NET_RAW for ping and raw sockets
		specOpts = append(specOpts, crun.WithCapability(crun.CapNetRaw))
		// Bind mount /etc/resolv.conf for DNS resolution
		specOpts = append(specOpts, crun.WithMount("/etc/resolv.conf", "/etc/resolv.conf", "none", []string{"bind", "ro"}))
	default:
		return fmt.Errorf("invalid network mode %q: use 'none' or 'host'", netMode)
	}

	// Choose execution mode based on flags
	if tty {
		// Real TTY mode: use console socket + Create/Start pattern
		return runWithTTY(stateRoot, ctrName, specOpts)
	} else if interactive {
		// Interactive without TTY: use RunWithIO with stdin
		return runInteractiveNonTTY(stateRoot, ctrName, specOpts)
	}
	// Non-interactive: use RunWithIO with buffered output
	return runNonInteractive(stateRoot, ctrName, specOpts)
}

func buildSpecOptions(pulled *PulledImage, containerCmd []string) ([]crun.SpecOption, error) {
	var opts []crun.SpecOption

	// Set rootfs
	opts = append(opts, crun.WithRootPath(pulled.RootFS))

	// Determine command to run
	finalCmd := determineCommand(pulled.Config, containerCmd)
	if len(finalCmd) == 0 {
		return nil, fmt.Errorf("no command specified and image has no default command")
	}
	opts = append(opts, crun.WithArgs(finalCmd...))

	// Set TTY mode - when true, we'll use console socket for real PTY
	opts = append(opts, crun.WithContainerTTY(tty))

	// Set working directory
	wd := workdir
	if wd == "" {
		wd = pulled.Config.WorkingDir
	}
	if wd != "" {
		opts = append(opts, crun.WithCwd(wd))
	}

	// Set environment variables from image
	for _, env := range pulled.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			opts = append(opts, crun.WithEnv(parts[0], parts[1]))
		}
	}

	// Set HOME if not already set
	hasHome := false
	for _, env := range pulled.Config.Env {
		if strings.HasPrefix(env, "HOME=") {
			hasHome = true
			break
		}
	}
	if !hasHome {
		opts = append(opts, crun.WithEnv("HOME", "/root"))
	}

	// Set environment variables from CLI (override image defaults)
	for _, env := range envVars {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			opts = append(opts, crun.WithEnv(parts[0], parts[1]))
		} else {
			// If no value, try to get from host environment
			if val, ok := os.LookupEnv(parts[0]); ok {
				opts = append(opts, crun.WithEnv(parts[0], val))
			}
		}
	}

	// Set user
	if user != "" {
		userSpec, err := parseUser(user)
		if err != nil {
			return nil, fmt.Errorf("invalid user: %w", err)
		}
		opts = append(opts, crun.WithUser(userSpec.UID, userSpec.GID))
	}

	// Set memory limit
	if memory != "" {
		memBytes, err := parseMemory(memory)
		if err != nil {
			return nil, fmt.Errorf("invalid memory: %w", err)
		}
		opts = append(opts, crun.WithMemoryLimit(memBytes))
	}

	// Set CPU limit
	if cpus != "" {
		cpuQuota, err := parseCPUs(cpus)
		if err != nil {
			return nil, fmt.Errorf("invalid cpus: %w", err)
		}
		opts = append(opts, crun.WithCPUQuota(cpuQuota))
	}

	// Set volume mounts
	for _, vol := range volumes {
		volSpec, err := parseVolume(vol)
		if err != nil {
			return nil, fmt.Errorf("invalid volume: %w", err)
		}

		// Make source path absolute
		source := volSpec.Source
		if !filepath.IsAbs(source) {
			absSource, err := filepath.Abs(source)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve volume path %q: %w", source, err)
			}
			source = absSource
		}

		// Check if source exists
		if _, err := os.Stat(source); err != nil {
			return nil, fmt.Errorf("volume source %q does not exist: %w", source, err)
		}

		mountOpts := []string{"bind"}
		if volSpec.ReadOnly {
			mountOpts = append(mountOpts, "ro")
		}
		opts = append(opts, crun.WithMount(source, volSpec.Dest, "none", mountOpts))
	}

	return opts, nil
}

func determineCommand(config ImageConfig, containerCmd []string) []string {
	// If entrypoint is overridden from CLI
	if entrypoint != "" {
		if len(containerCmd) > 0 {
			return append([]string{entrypoint}, containerCmd...)
		}
		return []string{entrypoint}
	}

	// If command is provided from CLI
	if len(containerCmd) > 0 {
		// If image has entrypoint, prepend it
		if len(config.Entrypoint) > 0 {
			return append(config.Entrypoint, containerCmd...)
		}
		return containerCmd
	}

	// Use image defaults
	if len(config.Entrypoint) > 0 {
		if len(config.Cmd) > 0 {
			return append(config.Entrypoint, config.Cmd...)
		}
		return config.Entrypoint
	}

	return config.Cmd
}

func runNonInteractive(stateRoot, ctrName string, specOpts []crun.SpecOption) error {
	// Create runtime context (no console socket needed)
	rc, err := crun.NewRuntimeContext(crun.RuntimeConfig{
		StateRoot: stateRoot,
	})
	if err != nil {
		return fmt.Errorf("failed to create runtime context: %w", err)
	}
	defer rc.Close()

	// Create spec
	spec, err := crun.NewSpec(true, specOpts...)
	if err != nil {
		return fmt.Errorf("failed to create container spec: %w", err)
	}
	defer spec.Close()

	var stdout, stderr bytes.Buffer

	result, err := rc.RunWithIO(ctrName, spec, &crun.IOConfig{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return fmt.Errorf("failed to run container: %w", err)
	}

	exitCode, err := result.Wait()
	if err != nil {
		return fmt.Errorf("failed to wait for container: %w", err)
	}

	// Print output
	if stdout.Len() > 0 {
		io.Copy(os.Stdout, &stdout)
	}
	if stderr.Len() > 0 {
		io.Copy(os.Stderr, &stderr)
	}

	// Show exit code
	fmt.Fprintf(os.Stderr, "Container exited with code %d\n", exitCode)

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}

func runInteractiveNonTTY(stateRoot, ctrName string, specOpts []crun.SpecOption) error {
	// Create runtime context (no console socket needed)
	rc, err := crun.NewRuntimeContext(crun.RuntimeConfig{
		StateRoot: stateRoot,
	})
	if err != nil {
		return fmt.Errorf("failed to create runtime context: %w", err)
	}
	defer rc.Close()

	// Create spec
	spec, err := crun.NewSpec(true, specOpts...)
	if err != nil {
		return fmt.Errorf("failed to create container spec: %w", err)
	}
	defer spec.Close()

	result, err := rc.RunWithIO(ctrName, spec, &crun.IOConfig{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		return fmt.Errorf("failed to run container: %w", err)
	}

	exitCode, err := result.Wait()
	if err != nil {
		return fmt.Errorf("failed to wait for container: %w", err)
	}

	// Show exit code
	fmt.Fprintf(os.Stderr, "Container exited with code %d\n", exitCode)

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}

// runWithTTY runs a container with a real PTY using console socket
func runWithTTY(stateRoot, ctrName string, specOpts []crun.SpecOption) error {
	// Create console socket for receiving PTY master fd
	socketDir, err := os.MkdirTemp("", "crungo-console-*")
	if err != nil {
		return fmt.Errorf("failed to create socket dir: %w", err)
	}
	defer os.RemoveAll(socketDir)

	socketPath := filepath.Join(socketDir, "console.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create console socket: %w", err)
	}
	defer listener.Close()

	// Create runtime context WITH console socket
	rc, err := crun.NewRuntimeContext(crun.RuntimeConfig{
		StateRoot:     stateRoot,
		ConsoleSocket: socketPath,
	})
	if err != nil {
		return fmt.Errorf("failed to create runtime context: %w", err)
	}
	defer rc.Close()

	// Create spec
	spec, err := crun.NewSpec(true, specOpts...)
	if err != nil {
		return fmt.Errorf("failed to create container spec: %w", err)
	}
	defer spec.Close()

	// Channel to receive PTY connection
	ptyConnChan := make(chan net.Conn, 1)
	ptyErrChan := make(chan error, 1)

	// Start goroutine to accept PTY master fd
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			ptyErrChan <- err
			return
		}
		ptyConnChan <- conn
	}()

	// Create container (this triggers libcrun to send PTY fd over socket)
	ctr, err := rc.Create(ctrName, spec, crun.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	defer ctr.Delete(true)

	// Wait for PTY connection
	var ptyConn net.Conn
	select {
	case ptyConn = <-ptyConnChan:
		defer ptyConn.Close()
	case err := <-ptyErrChan:
		return fmt.Errorf("failed to accept PTY connection: %w", err)
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for PTY master fd")
	}

	// Extract PTY master fd from socket
	ptyFd, err := receivePTYFd(ptyConn.(*net.UnixConn))
	if err != nil {
		return fmt.Errorf("failed to receive PTY fd: %w", err)
	}
	ptyFile := os.NewFile(uintptr(ptyFd), "pty-master")
	defer ptyFile.Close()

	// Put local terminal in raw mode
	stdinFd := int(os.Stdin.Fd())
	if !term.IsTerminal(stdinFd) {
		return fmt.Errorf("stdin is not a terminal; -t requires a terminal")
	}

	oldState, err := term.MakeRaw(stdinFd)
	if err != nil {
		return fmt.Errorf("failed to set terminal raw mode: %w", err)
	}
	defer term.Restore(stdinFd, oldState)

	// Handle SIGWINCH (terminal resize)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)
	defer signal.Stop(sigChan)

	// Set initial terminal size
	syncTerminalSize(stdinFd, ptyFd)

	// Handle resize signals in background
	go func() {
		for range sigChan {
			syncTerminalSize(stdinFd, ptyFd)
		}
	}()

	// Start container
	if err := ctr.Start(); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Bidirectional copy between PTY and stdin/stdout
	var wg sync.WaitGroup
	wg.Add(2)

	// stdin -> PTY
	go func() {
		defer wg.Done()
		io.Copy(ptyFile, os.Stdin)
	}()

	// PTY -> stdout
	go func() {
		defer wg.Done()
		io.Copy(os.Stdout, ptyFile)
	}()

	// Wait for container to exit
	exitCode := 0
	for {
		running, err := ctr.IsRunning()
		if err != nil {
			break
		}
		if !running {
			// Get exit status
			state, err := ctr.State()
			if err == nil && state.Status == "stopped" {
				// libcrun stores exit code in annotations or we can't get it easily
				// For now, assume 0 if stopped normally
			}
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Close PTY to unblock the copy goroutines
	ptyFile.Close()
	wg.Wait()

	// Restore terminal before printing
	term.Restore(stdinFd, oldState)

	// Show exit code
	fmt.Fprintf(os.Stderr, "\nContainer exited with code %d\n", exitCode)

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}

// receivePTYFd extracts the PTY master file descriptor from a Unix socket
// using SCM_RIGHTS (ancillary data).
func receivePTYFd(conn *net.UnixConn) (int, error) {
	// Buffer for regular data (libcrun sends a single byte)
	buf := make([]byte, 1)
	// Buffer for ancillary data (control message with fd)
	// Size: cmsg header (16 bytes on 64-bit) + space for file descriptors
	oob := make([]byte, 64)

	_, oobn, _, _, err := conn.ReadMsgUnix(buf, oob)
	if err != nil {
		return -1, fmt.Errorf("failed to read from console socket: %w", err)
	}

	if oobn == 0 {
		return -1, fmt.Errorf("no control message received from console socket")
	}

	// Parse control messages
	scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return -1, fmt.Errorf("failed to parse control message: %w", err)
	}

	if len(scms) == 0 {
		return -1, fmt.Errorf("no socket control messages found")
	}

	// Extract file descriptors
	fds, err := syscall.ParseUnixRights(&scms[0])
	if err != nil {
		return -1, fmt.Errorf("failed to parse unix rights: %w", err)
	}

	if len(fds) == 0 {
		return -1, fmt.Errorf("no file descriptors received")
	}

	return fds[0], nil
}

// syncTerminalSize copies the terminal size from src fd to dst fd
func syncTerminalSize(srcFd, dstFd int) {
	width, height, err := term.GetSize(srcFd)
	if err != nil {
		return
	}

	// Set the PTY size using TIOCSWINSZ
	ws := struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}{
		Row: uint16(height),
		Col: uint16(width),
	}
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(dstFd), syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(&ws)))
}

func generateName() string {
	adjectives := []string{"happy", "clever", "brave", "calm", "eager", "fancy", "gentle", "jolly", "kind", "lively"}
	nouns := []string{"panda", "tiger", "eagle", "dolphin", "falcon", "koala", "otter", "penguin", "rabbit", "wolf"}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	adj := adjectives[r.Intn(len(adjectives))]
	noun := nouns[r.Intn(len(nouns))]

	return fmt.Sprintf("%s_%s_%d", adj, noun, r.Intn(1000))
}

