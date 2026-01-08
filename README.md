# libcrun-go

Go bindings for [libcrun](https://github.com/containers/crun), the fast and lightweight OCI container runtime written in C.

## Features

- Idiomatic Go API with functional options pattern
- Full container lifecycle: create, start, run, kill, delete, pause, exec
- Resource limits (memory, CPU, pids)
- Namespace management (network, mount, etc.)
- TTY support via console sockets
- Typed errors with `errors.Is()` support
- Both rootful and rootless container support

## Requirements

- **Linux only** (cgo required)
- Go 1.25+
- System libraries:
  - `libsystemd-dev`
  - `libseccomp-dev`
  - `libcap-dev`

On Debian/Ubuntu:

```bash
sudo apt install libsystemd-dev libseccomp-dev libcap-dev
```

## Bundled Dependencies

### libcrun binary
This package includes a **pre-built static libcrun library** because libcrun is not available as a standalone package - only the `crun` binary is typically distributed by package managers.

**Supported architectures:**
- x86_64 (amd64) ✓
- aarch64 (arm64) - not yet available

**Build configuration used:**

```bash
./configure \
    --with-spin \
    --with-python-bindings \
    --enable-libcrun \
    --enable-crun \
    --enable-embedded-yajl \
    --enable-dynload-libcrun
```

Additional build dependencies required: Python development headers, [libcriu](https://github.com/checkpoint-restore/criu) (optional).

### Bundled headers

| Component | Source | License |
|-----------|--------|---------|
| libcrun | [source](https://github.com/containers/crun/tree/main/src/libcrun) | [LGPL-2.1](https://github.com/containers/crun/blob/main/COPYING.libcrun) |
| libocispec | [source](https://github.com/containers/libocispec/tree/bf749566cda632fb2f5dcf9c4eb5bbec71ac7d5f) | [GPL-3.0 with special exception](https://github.com/containers/libocispec/blob/main/COPYING) |
| yajl | [source](https://github.com/lloyd/yajl) | [ISC License](https://github.com/containers/yajl/blob/main/COPYING) |

## Installation

```bash
go get github.com/danielealbano/libcrun-go
```

## Quick Start

```go
package main

import (
    "fmt"
    "os"

    crun "github.com/danielealbano/libcrun-go"
)

func main() {
    // Create runtime context
    rc, err := crun.NewRuntimeContext(crun.RuntimeConfig{
        StateRoot: "/run/crun",
    })
    if err != nil {
        panic(err)
    }
    defer rc.Close()

    // Create container spec with functional options
    spec, err := crun.NewSpec(true, // rootless
        crun.WithRootPath("/path/to/rootfs"),
        crun.WithArgs("/bin/echo", "hello from container"),
        crun.WithMemoryLimit(256 * 1024 * 1024), // 256MB
    )
    if err != nil {
        panic(err)
    }
    defer spec.Close()

    // Run container with I/O
    result, err := rc.RunWithIO("my-container", spec, &crun.IOConfig{
        Stdout: os.Stdout,
        Stderr: os.Stderr,
    })
    if err != nil {
        panic(err)
    }

    exitCode, _ := result.Wait()
    fmt.Printf("Container exited with code: %d\n", exitCode)
}
```

## Examples

### [Hello World](examples/helloworld/)

Minimal example running a statically compiled binary in a container. Demonstrates basic libcrun-go usage without requiring a full rootfs.

### [crungo](examples/crungo/)

A Docker/Podman-compatible CLI demonstrating advanced features:
- Pull and run OCI images from any registry
- Interactive mode with TTY support
- Volume mounts, environment variables
- Resource limits (CPU, memory)
- Host network mode

## API Overview

### Core Types

- **RuntimeContext** - execution environment for libcrun operations
- **ContainerSpec** - OCI spec holder (create via `NewSpec` or `LoadContainerSpecFromFile`)
- **Container** - live container handle with lifecycle methods

### Functional Options

Build specs ergonomically with `WithXxx` options:

```go
spec, _ := crun.NewSpec(true,
    crun.WithRootPath("/path/to/rootfs"),
    crun.WithArgs("/bin/sh", "-c", "echo hello"),
    crun.WithEnv("MY_VAR", "value"),
    crun.WithMemoryLimit(512 * 1024 * 1024),
    crun.WithCPUShares(1024),
    crun.WithPidsLimit(100),
    crun.WithHostname("my-container"),
    crun.WithMount("/host/data", "/data", "none", []string{"bind", "ro"}),
)
```

### Error Handling

Errors support `errors.Is()` for classification:

```go
if errors.Is(err, crun.ErrContainerNotFound) {
    // handle not found
}
if errors.Is(err, crun.ErrContainerExists) {
    // handle already exists
}
```

## Testing

### Unit Tests

Run unit tests (no root required):

```bash
make test-unit
```

### Integration Tests

Integration tests require **root privileges** and Docker (to create a test rootfs). The Makefile automatically uses `sudo`:

```bash
make test-integration
```

This will:
1. Create a temporary rootfs from `busybox:latest`
2. Run tests with `-tags=integration` as root
3. Clean up the rootfs

## Benchmarks

Run benchmarks (requires root, uses `sudo`):

```bash
make benchmark
```

Two benchmarks are available:
- `BenchmarkContainerThroughput` - libcrun-go performance
- `BenchmarkPodman` - podman baseline for comparison (same configurations)

### libcrun-go vs Podman Comparison

Measured on AMD Ryzen 9 5900X, running containers that execute `/bin/true`. Both use the same rootfs. Podman configured for minimal overhead (no networking, no logging, no SELinux, no seccomp):

| Workers | Duration | libcrun-go | podman | Speedup |
|---------|----------|------------|--------|---------|
| 1       | 1s       | 121/s      | 11/s   | **11x** |
| 4       | 1s       | 377/s      | 20/s   | **19x** |
| 8       | 1s       | 513/s      | 25/s   | **21x** |
| 16      | 1s       | 569/s      | 33/s   | **17x** |
| 1       | 5s       | 98/s       | 10/s   | **10x** |
| 4       | 5s       | 318/s      | 18/s   | **18x** |
| 8       | 5s       | 452/s      | 25/s   | **18x** |
| 16      | 5s       | 481/s      | 31/s   | **16x** |

**Average speedup: ~10-21x faster than podman**

**Why the difference?** Both libcrun-go and podman fork in these benchmarks - libcrun-go forks once per container to isolate stdout/stderr handling. However, podman has significantly more overhead on top of the fork.

**Podman benchmark configuration:** The benchmark disables networking, logging, SELinux, and seccomp to minimize overhead. However, podman still performs operations that cannot be disabled:
- Spawns `conmon` (container monitor daemon) for each container
- Fork/exec overhead for the podman process itself
- IPC between podman → conmon → container
- OCI spec generation and validation
- Container state management in `/var/lib/containers`

The benchmark measures end-to-end container lifecycle: create spec → run → wait → delete.

## License

This project is licensed under the [BSD 3-clause license](LICENSE).
