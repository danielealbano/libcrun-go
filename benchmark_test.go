//go:build linux && cgo && integration

package crun

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
)

// BenchmarkContainerThroughput measures libcrun-go throughput across various configurations.
// Run with: make benchmark
func BenchmarkContainerThroughput(b *testing.B) {
	if os.Getuid() != 0 {
		b.Skip("Benchmark requires root privileges")
	}

	rootfs := os.Getenv("TEST_ROOTFS")
	if rootfs == "" {
		rootfs = "/tmp/test-rootfs"
	}
	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		b.Skip("No test rootfs found. Set TEST_ROOTFS env var or create /tmp/test-rootfs with busybox")
	}

	rc, err := NewRuntimeContext(RuntimeConfig{
		StateRoot: b.TempDir(),
	})
	if err != nil {
		b.Fatalf("Failed to create runtime context: %v", err)
	}
	defer rc.Close()

	durations := []time.Duration{1 * time.Second, 5 * time.Second}
	parallelisms := []int{1, 4, 8, 16}

	for _, duration := range durations {
		for _, parallelism := range parallelisms {
			name := fmt.Sprintf("P%d_T%ds", parallelism, int(duration.Seconds()))
			b.Run(name, func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					var (
						completed int64
						failed    int64
						mu        sync.Mutex
						wg        sync.WaitGroup
					)

					done := make(chan struct{})
					time.AfterFunc(duration, func() { close(done) })

					for w := 0; w < parallelism; w++ {
						wg.Add(1)
						go func(workerID int) {
							defer wg.Done()
							localCompleted := 0
							localFailed := 0

							for i := 0; ; i++ {
								select {
								case <-done:
									mu.Lock()
									completed += int64(localCompleted)
									failed += int64(localFailed)
									mu.Unlock()
									return
								default:
								}

								containerID := fmt.Sprintf("tp-%d-%d", workerID, i)
								spec, err := NewSpec(false,
									WithRootPath(rootfs),
									WithContainerTTY(false),
									WithArgs("/bin/true"),
								)
								if err != nil {
									localFailed++
									continue
								}

								result, err := rc.RunWithIO(containerID, spec, &IOConfig{})
								if err != nil {
									spec.Close()
									localFailed++
									continue
								}

								_, _ = result.Wait()
								localCompleted++
								_ = result.Container.Delete(true)
								spec.Close()
							}
						}(w)
					}

					wg.Wait()

					rate := float64(completed) / duration.Seconds()
					b.ReportMetric(rate, "containers/s")
					b.ReportMetric(float64(failed), "failed")
				}
			})
		}
	}
}

// BenchmarkPodman measures podman throughput for comparison with libcrun-go.
// Uses the same rootfs and configurations as BenchmarkContainerThroughput.
// Run with: make benchmark
func BenchmarkPodman(b *testing.B) {
	if os.Getuid() != 0 {
		b.Skip("Benchmark requires root privileges")
	}

	rootfs := os.Getenv("TEST_ROOTFS")
	if rootfs == "" {
		rootfs = "/tmp/test-rootfs"
	}
	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		b.Skip("No test rootfs found. Set TEST_ROOTFS env var or create /tmp/test-rootfs with busybox")
	}

	podmanPath, err := exec.LookPath("podman")
	if err != nil {
		b.Skip("podman not found in PATH - skipping benchmark")
	}

	durations := []time.Duration{1 * time.Second, 5 * time.Second}
	parallelisms := []int{1, 4, 8, 16}

	for _, duration := range durations {
		for _, parallelism := range parallelisms {
			name := fmt.Sprintf("P%d_T%ds", parallelism, int(duration.Seconds()))
			b.Run(name, func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					var (
						completed int64
						failed    int64
						mu        sync.Mutex
						wg        sync.WaitGroup
					)

					done := make(chan struct{})
					time.AfterFunc(duration, func() { close(done) })

					for w := 0; w < parallelism; w++ {
						wg.Add(1)
						go func() {
							defer wg.Done()
							localCompleted := 0
							localFailed := 0

							for {
								select {
								case <-done:
									mu.Lock()
									completed += int64(localCompleted)
									failed += int64(localFailed)
									mu.Unlock()
									return
								default:
								}

								// Run podman with minimal overhead:
								// --network=none: skip networking setup
								// --log-driver=none: skip logging
								// --security-opt label=disable: skip SELinux labeling
								// --security-opt seccomp=unconfined: skip seccomp setup
								// --rootfs: use same rootfs as libcrun-go
								cmd := exec.Command(podmanPath,
									"run", "--rm",
									"--network=none",
									"--log-driver=none",
									"--security-opt", "label=disable",
									"--security-opt", "seccomp=unconfined",
									"--rootfs", rootfs,
									"/bin/true",
								)
								if err := cmd.Run(); err != nil {
									localFailed++
								} else {
									localCompleted++
								}
							}
						}()
					}

					wg.Wait()

					rate := float64(completed) / duration.Seconds()
					b.ReportMetric(rate, "containers/s")
					b.ReportMetric(float64(failed), "failed")
				}
			})
		}
	}
}

// BenchmarkCrun measures crun CLI throughput for comparison with libcrun-go.
// Uses the same rootfs and configurations as BenchmarkContainerThroughput.
// Run with: make benchmark
func BenchmarkCrun(b *testing.B) {
	if os.Getuid() != 0 {
		b.Skip("Benchmark requires root privileges")
	}

	rootfs := os.Getenv("TEST_ROOTFS")
	if rootfs == "" {
		rootfs = "/tmp/test-rootfs"
	}
	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		b.Skip("No test rootfs found. Set TEST_ROOTFS env var or create /tmp/test-rootfs with busybox")
	}

	crunPath, err := exec.LookPath("crun")
	if err != nil {
		b.Skip("crun not found in PATH - skipping benchmark")
	}

	durations := []time.Duration{1 * time.Second, 5 * time.Second}
	parallelisms := []int{1, 4, 8, 16}

	for _, duration := range durations {
		for _, parallelism := range parallelisms {
			name := fmt.Sprintf("P%d_T%ds", parallelism, int(duration.Seconds()))
			b.Run(name, func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					var (
						completed int64
						failed    int64
						mu        sync.Mutex
						wg        sync.WaitGroup
					)

					done := make(chan struct{})
					time.AfterFunc(duration, func() { close(done) })

					for w := 0; w < parallelism; w++ {
						wg.Add(1)
						go func(workerID int) {
							defer wg.Done()
							localCompleted := 0
							localFailed := 0

							// Create a bundle directory for this worker
							bundleDir, err := os.MkdirTemp("", fmt.Sprintf("crun-bench-%d-", workerID))
							if err != nil {
								mu.Lock()
								failed++
								mu.Unlock()
								return
							}
							defer os.RemoveAll(bundleDir)

							// Create minimal OCI spec
							spec := createMinimalOCISpec(rootfs)
							specJSON, err := json.Marshal(spec)
							if err != nil {
								mu.Lock()
								failed++
								mu.Unlock()
								return
							}

							configPath := filepath.Join(bundleDir, "config.json")
							if err := os.WriteFile(configPath, specJSON, 0644); err != nil {
								mu.Lock()
								failed++
								mu.Unlock()
								return
							}

							for i := 0; ; i++ {
								select {
								case <-done:
									mu.Lock()
									completed += int64(localCompleted)
									failed += int64(localFailed)
									mu.Unlock()
									return
								default:
								}

								containerID := fmt.Sprintf("crun-%d-%d", workerID, i)

								// crun run --bundle <path> <container-id>
								// The container exits immediately after /bin/true
								cmd := exec.Command(crunPath,
									"run",
									"--bundle", bundleDir,
									containerID,
								)
								if err := cmd.Run(); err != nil {
									localFailed++
								} else {
									localCompleted++
								}
							}
						}(w)
					}

					wg.Wait()

					rate := float64(completed) / duration.Seconds()
					b.ReportMetric(rate, "containers/s")
					b.ReportMetric(float64(failed), "failed")
				}
			})
		}
	}
}

// createMinimalOCISpec creates a minimal OCI runtime spec for benchmarking.
func createMinimalOCISpec(rootfsPath string) *specs.Spec {
	return &specs.Spec{
		Version: "1.0.0",
		Root: &specs.Root{
			Path:     rootfsPath,
			Readonly: false,
		},
		Process: &specs.Process{
			Terminal: false,
			User: specs.User{
				UID: 0,
				GID: 0,
			},
			Args: []string{"/bin/true"},
			Env:  []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
			Cwd:  "/",
			Capabilities: &specs.LinuxCapabilities{
				Bounding:  []string{"CAP_AUDIT_WRITE", "CAP_KILL", "CAP_NET_BIND_SERVICE"},
				Effective: []string{"CAP_AUDIT_WRITE", "CAP_KILL", "CAP_NET_BIND_SERVICE"},
				Permitted: []string{"CAP_AUDIT_WRITE", "CAP_KILL", "CAP_NET_BIND_SERVICE"},
			},
			Rlimits: []specs.POSIXRlimit{
				{Type: "RLIMIT_NOFILE", Hard: 1024, Soft: 1024},
			},
		},
		Linux: &specs.Linux{
			Namespaces: []specs.LinuxNamespace{
				{Type: specs.PIDNamespace},
				{Type: specs.MountNamespace},
				{Type: specs.IPCNamespace},
				{Type: specs.UTSNamespace},
			},
			MaskedPaths: []string{
				"/proc/kcore",
				"/proc/keys",
				"/proc/timer_list",
			},
			ReadonlyPaths: []string{
				"/proc/asound",
				"/proc/bus",
				"/proc/fs",
			},
		},
		Mounts: []specs.Mount{
			{Destination: "/proc", Type: "proc", Source: "proc"},
			{Destination: "/dev", Type: "tmpfs", Source: "tmpfs", Options: []string{"nosuid", "strictatime", "mode=755", "size=65536k"}},
			{Destination: "/sys", Type: "sysfs", Source: "sysfs", Options: []string{"nosuid", "noexec", "nodev", "ro"}},
		},
	}
}
