//go:build linux && cgo

// Package crun provides an idiomatic Go wrapper for libcrun without clashing
// with the standard library's context package. The central types are [RuntimeContext],
// [ContainerSpec], and [Container].
//
// # Bundled Library
//
// This package includes a pre-built static libcrun library because libcrun is not
// available as a standalone package - only the crun binary is typically packaged.
// The library is built with the following configuration:
//
//	./configure \
//		--with-spin \
//		--with-python-bindings \
//		--enable-libcrun \
//		--enable-crun \
//		--enable-embedded-yajl \
//		--enable-dynload-libcrun
//
// Currently only x86_64 (amd64) is provided. The aarch64 build is not yet available.
//
// The bundled files include headers from libcrun, libocispec, and yajl.
// See the README for license information on these bundled components.
//
// # Requirements
//
// This package requires Linux and cgo. The following system libraries must be installed:
//   - libsystemd-dev
//   - libseccomp-dev
//   - libcap-dev
//
// # Quick Start
//
// Example using the functional options pattern:
//
//	rc, _ := crun.NewRuntimeContext(crun.RuntimeConfig{
//	    StateRoot: "/run/crun",
//	})
//	defer rc.Close()
//
//	spec, _ := crun.NewSpec(true, // rootless
//	    crun.WithRootPath("/path/to/rootfs"),
//	    crun.WithArgs("/bin/echo", "hello"),
//	    crun.WithMemoryLimit(512 * 1024 * 1024),
//	)
//	defer spec.Close()
//
//	result, _ := rc.RunWithIO("my-container", spec, &crun.IOConfig{
//	    Stdout: os.Stdout,
//	    Stderr: os.Stderr,
//	})
//	exitCode, _ := result.Wait()
//
// # Functional Options
//
// Use WithXxx options to configure container specs ergonomically:
//   - [WithRootPath], [WithArgs], [WithEnv], [WithCwd] - basic process config
//   - [WithMemoryLimit], [WithCPUShares], [WithCPUQuota], [WithPidsLimit] - resource limits
//   - [WithMount], [WithHostname], [WithAnnotation] - container config
//   - [WithNetworkNamespace], [WithMountNamespace], [WithHostNetwork] - namespace control
//
// # Error Handling
//
// Errors support [errors.Is] for classification:
//
//	if errors.Is(err, crun.ErrContainerNotFound) {
//	    // handle not found
//	}
//
// Sentinel errors: [ErrContainerNotFound], [ErrContainerExists], [ErrInvalidContainerSpec]
//
// # Memory Management
//
// Both [RuntimeContext] and [ContainerSpec] hold C-side allocations.
// Always call Close() when done:
//
//	rc, _ := crun.NewRuntimeContext(cfg)
//	defer rc.Close()
//
//	spec, _ := crun.NewSpec(true, opts...)
//	defer spec.Close()
//
// Finalizers are installed as a safety net, but explicit Close() is preferred.
package crun

/*
// libcrun Go bindings.
//
// Headers and library are bundled under libcrun/ directory:
//   libcrun/include/     - headers (libcrun/, ocispec/, yajl/, config.h, go_crun.h)
//   libcrun/lib/x64/     - x86_64 static library
//   libcrun/lib/aarch64/ - aarch64 static library
//
// System dependencies required: libsystemd-dev, libseccomp-dev, libcap-dev

#cgo linux,amd64 CFLAGS: -I${SRCDIR}/libcrun/include
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/libcrun/lib/x64 -lcrun -lsystemd -lseccomp -lcap -lm
#cgo linux,arm64 CFLAGS: -I${SRCDIR}/libcrun/include
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/libcrun/lib/aarch64 -lcrun -lsystemd -lseccomp -lcap -lm

#include "go_crun.h"
*/
import "C"
import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"runtime"
	"runtime/cgo"
	"sync"
	"unsafe"
)

// Verbosity levels from libcrun.
const (
	VerbosityError   = int(C.LIBCRUN_VERBOSITY_ERROR)
	VerbosityWarning = int(C.LIBCRUN_VERBOSITY_WARNING)
	VerbosityDebug   = int(C.LIBCRUN_VERBOSITY_DEBUG)
)

func fromLibcrunErr(cerr *C.libcrun_error_t) error {
	if cerr == nil {
		return errors.New("libcrun: unknown error (nil)")
	}
	var status C.int
	msg := C.go_crun_err_to_cstr(cerr, &status)
	if msg == nil {
		return errors.New("libcrun: error without message")
	}
	defer C.free(unsafe.Pointer(msg))
	message := C.GoString(msg)
	return &Error{
		Code:    classifyError(message, int(status)),
		Message: message,
		Status:  int(status),
	}
}

// RuntimeConfig configures a RuntimeContext (maps to libcrun_context_t fields).
type RuntimeConfig struct {
	ID            string
	Bundle        string // default "."
	StateRoot     string
	ConsoleSocket string
	PIDFile       string
	NotifySocket  string
	Handler       string

	SystemdCgroup bool
	Detach        bool
	NoNewKeyring  bool
	ForceNoCgroup bool
	NoPivot       bool
}

// RuntimeContext is the per-operation environment used by libcrun.
type RuntimeContext struct {
	c  *C.libcrun_context_t
	mu sync.Mutex // protects c.id during concurrent operations
}

// NewRuntimeContext creates a new RuntimeContext. Call Close() when done.
func NewRuntimeContext(cfg RuntimeConfig) (*RuntimeContext, error) {
	c := C.go_crun_new_context()
	if c == nil {
		return nil, errors.New("libcrun: failed to allocate context")
	}
	setStr := func(dst **C.char, s, def string) {
		if s == "" {
			if def == "" {
				return
			}
			s = def
		}
		cs := C.CString(s)
		*dst = cs
	}
	setStr(&c.id, cfg.ID, "")
	setStr(&c.bundle, cfg.Bundle, ".")
	setStr(&c.state_root, cfg.StateRoot, "")
	setStr(&c.console_socket, cfg.ConsoleSocket, "")
	setStr(&c.pid_file, cfg.PIDFile, "")
	setStr(&c.notify_socket, cfg.NotifySocket, "")
	setStr(&c.handler, cfg.Handler, "")

	c.systemd_cgroup = C.bool(cfg.SystemdCgroup)
	c.detach = C.bool(cfg.Detach)
	c.no_new_keyring = C.bool(cfg.NoNewKeyring)
	c.force_no_cgroup = C.bool(cfg.ForceNoCgroup)
	c.no_pivot = C.bool(cfg.NoPivot)

	rc := &RuntimeContext{c: c}
	runtime.SetFinalizer(rc, func(x *RuntimeContext) { _ = x.Close() })
	return rc, nil
}

// Close releases C-side allocations associated with the RuntimeContext.
func (x *RuntimeContext) Close() error {
	if x == nil || x.c == nil {
		return nil
	}
	C.go_crun_free_context(x.c)
	x.c = nil
	return nil
}

// Get returns a Container handle for an existing container by ID.
// This does not verify the container exists - first operation will fail if it doesn't.
func (rc *RuntimeContext) Get(id string) *Container {
	return &Container{ID: id, runtime: rc}
}

// RunOptions controls container run behavior.
type RunOptions struct {
	Prefork bool
}

func runFlags(o RunOptions) C.uint {
	var f C.uint
	if o.Prefork {
		f |= C.LIBCRUN_RUN_OPTIONS_PREFORK
	}
	return f
}

// CreateOptions controls container creation (distinct flag set from Run).
type CreateOptions struct {
	Prefork bool
}

func createFlags(o CreateOptions) C.uint {
	var f C.uint
	if o.Prefork {
		f |= C.LIBCRUN_CREATE_OPTIONS_PREFORK
	}
	return f
}

// IOConfig configures container I/O streams for RunWithIO.
type IOConfig struct {
	Stdin  io.Reader // If nil, container stdin reads from /dev/null
	Stdout io.Writer // If nil, container stdout is discarded
	Stderr io.Writer // If nil, container stderr is discarded
}

// RunResult holds the result of a container run with I/O.
type RunResult struct {
	Container *Container
	Wait      func() (int, error) // blocks until container exits, returns exit code
}

// setContextID sets the container ID on the context for create/run operations.
func (x *RuntimeContext) setContextID(id string) {
	if x.c.id != nil {
		C.free(unsafe.Pointer(x.c.id))
	}
	x.c.id = C.CString(id)
}

// Run creates and starts the container in one operation.
// Returns a Container handle for further operations.
// WARNING: This method may hang if the container writes to stdout/stderr without
// proper I/O handling. Consider using RunWithIO for reliable operation.
func (x *RuntimeContext) Run(id string, spec *ContainerSpec, o RunOptions) (*Container, error) {
	if x == nil || x.c == nil || spec == nil || spec.c == nil {
		return nil, errors.New("libcrun: invalid runtime context or container spec")
	}
	x.setContextID(id)
	var err C.libcrun_error_t
	rc := C.libcrun_container_run(x.c, spec.c, runFlags(o), &err)
	if rc < 0 {
		return nil, fromLibcrunErr(&err)
	}
	return &Container{ID: id, runtime: x}, nil
}

// RunWithIO creates and starts the container with isolated I/O streams using pipes.
// This method forks before calling libcrun, allowing each container to have
// its own stdin/stdout/stderr. Multiple containers can run in parallel.
// Use Wait() on the returned RunResult to block until the container exits.
//
// NOTE: This method uses OS pipes for I/O, NOT a real pseudo-terminal (PTY).
// The container spec's Terminal field should be set to false when using this method.
// Programs that require a TTY (like vim, top, interactive shells with line editing)
// will not work correctly.
//
// For real PTY support, use the Create/Start pattern with a console socket:
//
//  1. Create a Unix socket listener and get its path
//  2. Pass the socket path to RuntimeConfig.ConsoleSocket when creating RuntimeContext
//  3. Set WithContainerTTY(true) in your spec options
//  4. Call rc.Create() to create the container - libcrun will send the PTY master
//     fd over the console socket via SCM_RIGHTS
//  5. Accept the connection and extract the fd using syscall.ParseUnixRights()
//  6. Put local terminal in raw mode (e.g., with golang.org/x/term)
//  7. Call ctr.Start() to start the container
//  8. Copy data bidirectionally between local stdin/stdout and the PTY fd
//
// See the crungo example for a complete implementation of TTY support.
func (x *RuntimeContext) RunWithIO(id string, spec *ContainerSpec, ioCfg *IOConfig) (*RunResult, error) {
	if x == nil || x.c == nil || spec == nil || spec.c == nil {
		return nil, errors.New("libcrun: invalid runtime context or container spec")
	}
	if ioCfg == nil {
		ioCfg = &IOConfig{}
	}

	// Create pipes for I/O (before locking to minimize lock time)
	var stdinR, stdinW, stdoutR, stdoutW, stderrR, stderrW *os.File
	var logR, logW *os.File
	var err error

	// Helper to close all opened pipes on error
	closePipes := func() {
		if stdinR != nil {
			stdinR.Close()
		}
		if stdinW != nil {
			stdinW.Close()
		}
		if stdoutR != nil {
			stdoutR.Close()
		}
		if stdoutW != nil {
			stdoutW.Close()
		}
		if stderrR != nil {
			stderrR.Close()
		}
		if stderrW != nil {
			stderrW.Close()
		}
		if logR != nil {
			logR.Close()
		}
		if logW != nil {
			logW.Close()
		}
	}

	// Stdin pipe (Go writes to stdinW, child reads from stdinR)
	stdinFd := C.int(-1)
	if ioCfg.Stdin != nil {
		stdinR, stdinW, err = os.Pipe()
		if err != nil {
			return nil, err
		}
		stdinFd = C.int(stdinR.Fd())
	}

	// Stdout pipe (child writes to stdoutW, Go reads from stdoutR)
	stdoutFd := C.int(-1)
	if ioCfg.Stdout != nil {
		stdoutR, stdoutW, err = os.Pipe()
		if err != nil {
			closePipes()
			return nil, err
		}
		stdoutFd = C.int(stdoutW.Fd())
	}

	// Stderr pipe (child writes to stderrW, Go reads from stderrR)
	stderrFd := C.int(-1)
	if ioCfg.Stderr != nil {
		stderrR, stderrW, err = os.Pipe()
		if err != nil {
			closePipes()
			return nil, err
		}
		stderrFd = C.int(stderrW.Fd())
	}

	// Log pipe (child writes structured logs, Go reads and forwards to handler)
	// Only create if a log handler is registered
	logFd := C.int(-1)
	handler := getLogHandler()
	if handler != nil {
		logR, logW, err = os.Pipe()
		if err != nil {
			closePipes()
			return nil, err
		}
		logFd = C.int(logW.Fd())
	}

	// Lock to protect context ID during fork (fork copies the context)
	x.mu.Lock()
	x.setContextID(id)

	// Call C function to fork and run
	var childPid C.pid_t
	var cerr C.libcrun_error_t
	rc := C.go_crun_run_with_pipes(x.c, spec.c, runFlags(RunOptions{}),
		stdinFd, stdoutFd, stderrFd, logFd, &childPid, &cerr)
	x.mu.Unlock()

	// Close child-side fds in Go (Go owns all fds, C doesn't close them)
	if stdinR != nil {
		stdinR.Close()
	}
	if stdoutW != nil {
		stdoutW.Close()
	}
	if stderrW != nil {
		stderrW.Close()
	}
	if logW != nil {
		logW.Close()
	}

	if rc < 0 {
		// Cleanup remaining pipes on error
		if stdinW != nil {
			stdinW.Close()
		}
		if stdoutR != nil {
			stdoutR.Close()
		}
		if stderrR != nil {
			stderrR.Close()
		}
		if logR != nil {
			logR.Close()
		}
		return nil, fromLibcrunErr(&cerr)
	}

	// Start I/O goroutines
	var wg sync.WaitGroup

	if ioCfg.Stdin != nil && stdinW != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer stdinW.Close()
			_, _ = io.Copy(stdinW, ioCfg.Stdin)
		}()
	}

	if ioCfg.Stdout != nil && stdoutR != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer stdoutR.Close()
			_, _ = io.Copy(ioCfg.Stdout, stdoutR)
		}()
	}

	if ioCfg.Stderr != nil && stderrR != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer stderrR.Close()
			_, _ = io.Copy(ioCfg.Stderr, stderrR)
		}()
	}

	// Start log reader goroutine if handler is set
	if handler != nil && logR != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer logR.Close()
			readLogPipe(logR, handler)
		}()
	}

	// Create Wait function
	waitFn := func() (int, error) {
		var exitCode C.int
		var werr C.libcrun_error_t
		wrc := C.go_crun_wait(childPid, &exitCode, &werr)
		if wrc < 0 {
			return -1, fromLibcrunErr(&werr)
		}
		// Wait for I/O goroutines to finish
		wg.Wait()
		return int(exitCode), nil
	}

	return &RunResult{
		Container: &Container{ID: id, runtime: x},
		Wait:      waitFn,
	}, nil
}

// Create creates the container (does not start).
// Returns a Container handle for further operations.
func (x *RuntimeContext) Create(id string, spec *ContainerSpec, o CreateOptions) (*Container, error) {
	if x == nil || x.c == nil || spec == nil || spec.c == nil {
		return nil, errors.New("libcrun: invalid runtime context or container spec")
	}
	x.setContextID(id)
	var err C.libcrun_error_t
	rc := C.libcrun_container_create(x.c, spec.c, createFlags(o), &err)
	if rc < 0 {
		return nil, fromLibcrunErr(&err)
	}
	return &Container{ID: id, runtime: x}, nil
}

// List returns Container handles for all containers under the configured state root.
func (x *RuntimeContext) List() ([]*Container, error) {
	if x == nil || x.c == nil {
		return nil, errors.New("libcrun: invalid runtime context")
	}
	var arr **C.char
	var n C.int
	var err C.libcrun_error_t
	rc := C.go_crun_list(x.c.state_root, &arr, &n, &err)
	if rc < 0 {
		return nil, fromLibcrunErr(&err)
	}
	defer C.go_crun_free_strv(arr, n)

	out := make([]*Container, int(n))
	elems := unsafe.Slice((**C.char)(unsafe.Pointer(arr)), int(n))
	for i := 0; i < int(n); i++ {
		out[i] = &Container{ID: C.GoString(elems[i]), runtime: x}
	}
	return out, nil
}

// ListIDs returns container IDs under the configured state root.
func (x *RuntimeContext) ListIDs() ([]string, error) {
	if x == nil || x.c == nil {
		return nil, errors.New("libcrun: invalid runtime context")
	}
	var arr **C.char
	var n C.int
	var err C.libcrun_error_t
	rc := C.go_crun_list(x.c.state_root, &arr, &n, &err)
	if rc < 0 {
		return nil, fromLibcrunErr(&err)
	}
	defer C.go_crun_free_strv(arr, n)

	out := make([]string, int(n))
	elems := unsafe.Slice((**C.char)(unsafe.Pointer(arr)), int(n))
	for i := 0; i < int(n); i++ {
		out[i] = C.GoString(elems[i])
	}
	return out, nil
}

// internal methods for Container to use

func (x *RuntimeContext) deleteContainer(id string, force bool) error {
	if x == nil || x.c == nil {
		return errors.New("libcrun: invalid runtime context")
	}
	cid := C.CString(id)
	defer C.free(unsafe.Pointer(cid))
	var err C.libcrun_error_t
	rc := C.libcrun_container_delete(x.c, nil, cid, C.bool(force), &err)
	if rc < 0 {
		return fromLibcrunErr(&err)
	}
	return nil
}

func (x *RuntimeContext) killContainer(id string, signal Signal) error {
	if x == nil || x.c == nil {
		return errors.New("libcrun: invalid runtime context")
	}
	cid := C.CString(id)
	csig := C.CString(string(signal))
	defer C.free(unsafe.Pointer(cid))
	defer C.free(unsafe.Pointer(csig))
	var err C.libcrun_error_t
	rc := C.libcrun_container_kill(x.c, cid, csig, &err)
	if rc < 0 {
		return fromLibcrunErr(&err)
	}
	return nil
}

func (x *RuntimeContext) startContainer(id string) error {
	if x == nil || x.c == nil {
		return errors.New("libcrun: invalid runtime context")
	}
	cid := C.CString(id)
	defer C.free(unsafe.Pointer(cid))
	var err C.libcrun_error_t
	rc := C.libcrun_container_start(x.c, cid, &err)
	if rc < 0 {
		return fromLibcrunErr(&err)
	}
	return nil
}

func (x *RuntimeContext) containerStateJSON(id string) (string, error) {
	if x == nil || x.c == nil {
		return "", errors.New("libcrun: invalid runtime context")
	}
	cid := C.CString(id)
	defer C.free(unsafe.Pointer(cid))
	var err C.libcrun_error_t
	var ln C.int
	buf := C.go_crun_state_json(x.c, cid, &ln, &err)
	if buf == nil {
		return "", fromLibcrunErr(&err)
	}
	defer C.free(unsafe.Pointer(buf))
	return C.GoStringN(buf, ln), nil
}

func (x *RuntimeContext) execJSON(id string, processJSON string) error {
	if x == nil || x.c == nil {
		return errors.New("libcrun: invalid runtime context")
	}
	cid := C.CString(id)
	cjson := C.CString(processJSON)
	defer C.free(unsafe.Pointer(cid))
	defer C.free(unsafe.Pointer(cjson))
	var err C.libcrun_error_t
	rc := C.go_crun_exec_json(x.c, cid, cjson, &err)
	if rc < 0 {
		return fromLibcrunErr(&err)
	}
	return nil
}

func (x *RuntimeContext) pauseContainer(id string) error {
	if x == nil || x.c == nil {
		return errors.New("libcrun: invalid runtime context")
	}
	cid := C.CString(id)
	defer C.free(unsafe.Pointer(cid))
	var err C.libcrun_error_t
	rc := C.go_crun_pause(x.c, cid, &err)
	if rc < 0 {
		return fromLibcrunErr(&err)
	}
	return nil
}

func (x *RuntimeContext) unpauseContainer(id string) error {
	if x == nil || x.c == nil {
		return errors.New("libcrun: invalid runtime context")
	}
	cid := C.CString(id)
	defer C.free(unsafe.Pointer(cid))
	var err C.libcrun_error_t
	rc := C.go_crun_unpause(x.c, cid, &err)
	if rc < 0 {
		return fromLibcrunErr(&err)
	}
	return nil
}

func (x *RuntimeContext) killAllContainer(id string, signal Signal) error {
	if x == nil || x.c == nil {
		return errors.New("libcrun: invalid runtime context")
	}
	cid := C.CString(id)
	csig := C.CString(string(signal))
	defer C.free(unsafe.Pointer(cid))
	defer C.free(unsafe.Pointer(csig))
	var err C.libcrun_error_t
	rc := C.go_crun_killall(x.c, cid, csig, &err)
	if rc < 0 {
		return fromLibcrunErr(&err)
	}
	return nil
}

func (x *RuntimeContext) updateContainer(id string, content string) error {
	if x == nil || x.c == nil {
		return errors.New("libcrun: invalid runtime context")
	}
	cid := C.CString(id)
	ccontent := C.CString(content)
	defer C.free(unsafe.Pointer(cid))
	defer C.free(unsafe.Pointer(ccontent))
	var err C.libcrun_error_t
	rc := C.go_crun_update(x.c, cid, ccontent, C.size_t(len(content)), &err)
	if rc < 0 {
		return fromLibcrunErr(&err)
	}
	return nil
}

func (x *RuntimeContext) isContainerRunning(id string) (bool, error) {
	if x == nil || x.c == nil {
		return false, errors.New("libcrun: invalid runtime context")
	}
	cid := C.CString(id)
	defer C.free(unsafe.Pointer(cid))
	var err C.libcrun_error_t
	rc := C.go_crun_is_running(x.c.state_root, cid, &err)
	if rc < 0 {
		return false, fromLibcrunErr(&err)
	}
	return rc > 0, nil
}

func (x *RuntimeContext) containerPIDs(id string, recurse bool) ([]int, error) {
	if x == nil || x.c == nil {
		return nil, errors.New("libcrun: invalid runtime context")
	}
	cid := C.CString(id)
	defer C.free(unsafe.Pointer(cid))
	var pids *C.pid_t
	var n C.int
	var err C.libcrun_error_t
	recurseInt := 0
	if recurse {
		recurseInt = 1
	}
	rc := C.go_crun_read_pids(x.c, cid, C.int(recurseInt), &pids, &n, &err)
	if rc < 0 {
		return nil, fromLibcrunErr(&err)
	}
	defer C.go_crun_free_pids(pids)

	out := make([]int, int(n))
	if n > 0 {
		pidSlice := unsafe.Slice((*C.pid_t)(pids), int(n))
		for i := 0; i < int(n); i++ {
			out[i] = int(pidSlice[i])
		}
	}
	return out, nil
}

// SetVerbosity sets the libcrun logging verbosity level.
func SetVerbosity(v int) { C.libcrun_set_verbosity(C.int(v)) }

// GetVerbosity returns the current libcrun logging verbosity level.
func GetVerbosity() int { return int(C.libcrun_get_verbosity()) }

// LogEntry represents a log message from libcrun.
type LogEntry struct {
	Errno     int    // System errno if applicable, 0 otherwise
	Message   string // Log message
	Verbosity int    // VerbosityError, VerbosityWarning, or VerbosityDebug
}

// LogHandler is the callback type for receiving libcrun logs.
type LogHandler func(entry LogEntry)

var (
	logHandleMu sync.Mutex
	logHandler  LogHandler // current handler (nil = no handler)
	logHandle   cgo.Handle // handle for C callback (0 when no handler)
)

//export goLogCallback
func goLogCallback(handle C.uintptr_t, errno C.int, msg *C.char, verbosity C.int) {
	h := cgo.Handle(handle)
	handler := h.Value().(LogHandler)
	if handler != nil {
		handler(LogEntry{
			Errno:     int(errno),
			Message:   C.GoString(msg),
			Verbosity: int(verbosity),
		})
	}
}

// getLogHandler returns the current log handler (thread-safe).
func getLogHandler() LogHandler {
	logHandleMu.Lock()
	defer logHandleMu.Unlock()
	return logHandler
}

// readLogPipe reads structured log entries from a pipe and calls the handler.
// Wire format: [errno:4][verbosity:4][msg_len:4][message:msg_len]
func readLogPipe(r io.Reader, handler LogHandler) {
	for {
		var errno, verbosity int32
		var msgLen uint32

		// Read header
		if err := binary.Read(r, binary.LittleEndian, &errno); err != nil {
			return // pipe closed or error
		}
		if err := binary.Read(r, binary.LittleEndian, &verbosity); err != nil {
			return
		}
		if err := binary.Read(r, binary.LittleEndian, &msgLen); err != nil {
			return
		}

		// Read message
		msg := make([]byte, msgLen)
		if _, err := io.ReadFull(r, msg); err != nil {
			return
		}

		// Call handler
		handler(LogEntry{
			Errno:     int(errno),
			Message:   string(msg),
			Verbosity: int(verbosity),
		})
	}
}

// SetLogHandler sets a Go function to receive all libcrun log messages.
// Pass nil to disable custom logging (reverts to stderr output).
//
// The handler receives logs from both:
//   - Direct libcrun calls (Run, Create, etc.)
//   - Forked child processes (RunWithIO) via a log pipe
//
// Note: The handler is called synchronously, so it should be fast and
// non-blocking. For expensive operations, consider using a buffered channel.
//
// Example:
//
//	crun.SetVerbosity(crun.VerbosityDebug)
//	crun.SetLogHandler(func(entry crun.LogEntry) {
//	    log.Printf("[libcrun] %s", entry.Message)
//	})
func SetLogHandler(handler LogHandler) {
	logHandleMu.Lock()
	defer logHandleMu.Unlock()

	// Release previous handle if set
	if logHandle != 0 {
		logHandle.Delete()
		logHandle = 0
	}
	logHandler = nil

	if handler == nil {
		C.go_crun_reset_log_handler()
		return
	}

	// Store handler and create cgo handle for C callback
	logHandler = handler
	logHandle = cgo.NewHandle(handler)
	C.go_crun_set_log_handler(C.uintptr_t(logHandle))
}
