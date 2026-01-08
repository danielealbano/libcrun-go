//go:build linux && cgo && integration

package crun

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
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
