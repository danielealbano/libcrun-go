# crungo

A minimal Docker/Podman-compatible CLI demonstrating libcrun-go capabilities.

## Features

- Pull and run OCI images from any registry (Docker Hub, GHCR, Quay, etc.)
- Docker/Podman-compatible flags
- **Real TTY support** with pseudo-terminal allocation (`-t`)
- Interactive mode with stdin support (`-i`)
- Environment variables
- Volume mounts
- Resource limits (CPU, memory)
- User specification
- Host network mode
- Terminal resize handling (SIGWINCH)

## Prerequisites

- Linux (libcrun only works on Linux)
- Go 1.25+
- libcrun dependencies: `libsystemd-dev`, `libseccomp-dev`, `libcap-dev`
- Root privileges (or properly configured rootless containers)

## Build

```bash
make build
```

Or directly:

```bash
go build -o crungo .
```

## Usage

```bash
crungo run [OPTIONS] IMAGE [COMMAND] [ARG...]
```

### Supported Flags

| Flag | Description |
|------|-------------|
| `-i, --interactive` | Keep stdin open |
| `-t, --tty` | Allocate a pseudo-TTY |
| `-e, --env KEY=VALUE` | Set environment variable (repeatable) |
| `-v, --volume host:container[:ro]` | Bind mount a volume (repeatable) |
| `-u, --user uid[:gid]` | Run as user |
| `--cpus` | CPU limit (e.g., "0.5", "2") |
| `-m, --memory` | Memory limit (e.g., "256m", "1g") |
| `--name` | Container name (default: random) |
| `-w, --workdir` | Working directory inside the container |
| `--entrypoint` | Override the image entrypoint |
| `--net` | Network mode: `none` (default, isolated) or `host` |
| `--crun-debug` | Enable libcrun debug logs |

**Note:** Containers are automatically removed when they exit (implicit `--rm`).

## Examples

### Simple Command

```bash
sudo ./crungo run alpine echo hello world
```

### Interactive Shell (with TTY)

```bash
sudo ./crungo run -it alpine /bin/sh
```

This gives you a fully functional shell with:
- Tab completion
- Arrow key navigation
- Ctrl+C / Ctrl+D handling
- Proper terminal resizing

### Interactive without TTY

```bash
sudo ./crungo run -i alpine /bin/sh
```

Use `-i` alone when you need to pipe input but don't need full terminal features.

### Run vim/top in Container

```bash
sudo ./crungo run -it alpine sh -c "apk add vim && vim"
sudo ./crungo run -it --net=host alpine sh -c "apk add htop && htop"
```

### With Host Network

```bash
sudo ./crungo run -it --net=host alpine /bin/sh
# Now you have full network access (shares host network namespace)
```

### Ping and DNS with Host Network

```bash
# Ping an external host
sudo ./crungo run --net=host alpine ping -c 3 8.8.8.8

# DNS lookup
sudo ./crungo run --net=host alpine nslookup google.com

# Curl a website (requires installing curl)
sudo ./crungo run --net=host alpine sh -c "apk add curl && curl -s https://httpbin.org/ip"
```

### With Environment Variables

```bash
sudo ./crungo run -e DB_HOST=localhost -e DB_PORT=5432 alpine env
```

### With Volume Mount

```bash
sudo ./crungo run -v ./data:/data:ro alpine ls -la /data
```

### With Resource Limits

```bash
sudo ./crungo run -m 256m --cpus 0.5 alpine cat /sys/fs/cgroup/memory.max
```

### Run as Different User

```bash
sudo ./crungo run -u 1000:1000 alpine id
```

### Override Entrypoint

```bash
sudo ./crungo run --entrypoint /bin/sh alpine -c "echo custom command"
```

### Debug libcrun

```bash
sudo ./crungo run --crun-debug alpine echo hello
```

## Network Modes

- **`--net=none` (default):** Container has its own isolated network namespace with only loopback interface. No external network access.
- **`--net=host`:** Container shares the host's network namespace. Full network access, DNS resolution via `/etc/resolv.conf`, and `CAP_NET_RAW` for tools like `ping`.

## Testing

### Unit Tests

```bash
make test-unit
# or
go test -v ./...
```

### Integration Tests

Integration tests require root privileges and network access to pull images:

```bash
make test-integration
# or
sudo go test -tags=integration -v ./...
```

## How It Works

1. **Image Pulling:** Uses [go-containerregistry](https://github.com/google/go-containerregistry) to pull OCI images from any registry. Supports Docker Hub, GHCR, Quay.io, and private registries (via `~/.docker/config.json`).

2. **Layer Extraction:** Extracts image layers to a temporary directory, handling whiteout files for layer deletions.

3. **Container Spec:** Builds an OCI runtime spec using libcrun-go's functional options pattern, merging image defaults with CLI overrides.

4. **Execution:** Runs the container using libcrun via the Go bindings. The execution mode depends on the flags:
   - **Non-interactive (default):** Uses `RunWithIO` with buffered stdout/stderr
   - **Interactive (`-i`):** Uses `RunWithIO` with stdin connected
   - **TTY (`-t`):** Uses `Create`/`Start` with console socket for real PTY

5. **Cleanup:** Automatically removes the rootfs and state directories when the container exits.

## TTY Implementation

When `-t` is specified, crungo uses a **real pseudo-terminal (PTY)** instead of pipes:

1. **Console Socket:** Creates a Unix domain socket and passes its path to `RuntimeConfig.ConsoleSocket`
2. **Container Creation:** Uses `rc.Create()` instead of `RunWithIO()` - this triggers libcrun to allocate a PTY pair
3. **PTY Master FD:** libcrun sends the PTY master file descriptor over the console socket using `SCM_RIGHTS`
4. **Raw Mode:** Puts the local terminal in raw mode using `golang.org/x/term`
5. **Bidirectional I/O:** Copies data between the local terminal and the PTY master
6. **Resize Handling:** Listens for `SIGWINCH` signals and syncs terminal size to the PTY

This allows full terminal functionality including:
- Programs like `vim`, `top`, `htop`, `less`
- Tab completion in shells
- Terminal escape sequences
- Proper line editing

**Note:** The `-t` flag requires stdin to be a real terminal. If stdin is not a TTY (e.g., in a script or CI), it will fail with an error message.

## Limitations

- No image caching (pulls fresh each time)
- No detached mode / background containers
- No exec into running containers
- No advanced networking (CNI, port mapping)

