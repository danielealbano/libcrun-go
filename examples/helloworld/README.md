# Hello World Example

A minimal example demonstrating libcrun-go by running a statically compiled binary in a container.

## How It Works

Instead of using a full rootfs (like busybox or alpine), this example uses a single statically compiled Go binary. The example:

1. Creates a temporary directory in `/tmp` for the rootfs
2. Copies the static binary into it
3. Creates a minimal `/etc/passwd` (required by libcrun to detect HOME)
4. Sets the `HOME` environment variable
5. Runs the container using that minimal rootfs
6. Cleans up the temporary directory on exit

The static binary has no external dependencies, so it can run in an empty filesystem with just itself.

## Prerequisites

- Linux (libcrun only works on Linux)
- Go 1.25+
- libcrun dependencies installed: `libsystemd-dev`, `libseccomp-dev`, `libcap-dev`
- Root privileges or properly configured rootless containers

## Build the Static Binary

First, compile the static hello world binary:

```bash
cd static-helloworld
make
cd ..
```

This creates a `helloworld` binary (~1.8MB) with no external dependencies.

## Run the Example

Run as root:
```bash
sudo go run main.go
```

Or build and run:
```bash
go build -o helloworld-example main.go
sudo ./helloworld-example
```

## Expected Output

```
Found static binary: /path/to/static-helloworld/helloworld
Using rootfs: /tmp/crun-rootfs-XXXXXX

Running container...

========== CONTAINER RESULTS ==========
Exit code: 0

--- STDOUT (30 bytes) ---
Hello, World from container!
--- STDERR (0 bytes) ---
(empty)
========================================
```

## Debug Mode

To enable verbose libcrun logging, set `DEBUG=1`:

```bash
sudo DEBUG=1 go run main.go
```

This enables debug verbosity and sets up a custom log handler that captures libcrun's internal logs. You'll see messages like:

```
[libcrun:DEBUG] Running linux container
[libcrun:DEBUG] Unsharing namespace: `pid`
[libcrun:DEBUG] Joining `network` namespace: `/proc/1/ns/net`
[libcrun:WARN] cannot detect HOME environment variable, setting default
```

This shows detailed information about namespace setup, cgroup configuration, and container execution.

## Rootless Mode

The example runs in rootless mode by default. For rootless containers to work, you may need:

1. User namespaces enabled (`/proc/sys/kernel/unprivileged_userns_clone` = 1)
2. Subuid/subgid mappings configured in `/etc/subuid` and `/etc/subgid`

If rootless doesn't work, running with `sudo` is the simplest alternative.

