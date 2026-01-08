//go:build linux

package crun

import (
	"testing"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func TestSpecOptionWithRootPath(t *testing.T) {
	sp := &specs.Spec{}
	opt := WithRootPath("/test/rootfs")
	opt(sp)

	if sp.Root == nil || sp.Root.Path != "/test/rootfs" {
		t.Errorf("WithRootPath failed: got %v", sp.Root)
	}
}

func TestSpecOptionWithArgs(t *testing.T) {
	sp := &specs.Spec{}
	opt := WithArgs("/bin/sh", "-c", "echo hello")
	opt(sp)

	if sp.Process == nil {
		t.Fatal("Process is nil")
	}
	if len(sp.Process.Args) != 3 {
		t.Fatalf("Args length = %d, want 3", len(sp.Process.Args))
	}
	if sp.Process.Args[0] != "/bin/sh" {
		t.Errorf("Args[0] = %q, want /bin/sh", sp.Process.Args[0])
	}
}

func TestSpecOptionWithContainerTTY(t *testing.T) {
	// Test enabling TTY
	sp := &specs.Spec{}
	opt := WithContainerTTY(true)
	opt(sp)

	if sp.Process == nil {
		t.Fatal("Process is nil")
	}
	if !sp.Process.Terminal {
		t.Error("Terminal should be true")
	}

	// Test disabling TTY
	sp2 := &specs.Spec{Process: &specs.Process{Terminal: true}}
	opt2 := WithContainerTTY(false)
	opt2(sp2)

	if sp2.Process.Terminal {
		t.Error("Terminal should be false")
	}
}

func TestSpecOptionWithEnv(t *testing.T) {
	sp := &specs.Spec{Process: &specs.Process{}}
	opt1 := WithEnv("FOO", "bar")
	opt2 := WithEnv("BAZ", "qux")
	opt1(sp)
	opt2(sp)

	if len(sp.Process.Env) != 2 {
		t.Fatalf("Env length = %d, want 2", len(sp.Process.Env))
	}
	if sp.Process.Env[0] != "FOO=bar" {
		t.Errorf("Env[0] = %q, want FOO=bar", sp.Process.Env[0])
	}
	if sp.Process.Env[1] != "BAZ=qux" {
		t.Errorf("Env[1] = %q, want BAZ=qux", sp.Process.Env[1])
	}
}

func TestSpecOptionWithMemoryLimit(t *testing.T) {
	sp := &specs.Spec{}
	opt := WithMemoryLimit(512 * 1024 * 1024)
	opt(sp)

	if sp.Linux == nil || sp.Linux.Resources == nil || sp.Linux.Resources.Memory == nil {
		t.Fatal("Linux resources not initialized")
	}
	if *sp.Linux.Resources.Memory.Limit != 512*1024*1024 {
		t.Errorf("Memory limit = %d, want %d", *sp.Linux.Resources.Memory.Limit, 512*1024*1024)
	}
}

func TestSpecOptionWithCPUShares(t *testing.T) {
	sp := &specs.Spec{}
	opt := WithCPUShares(512)
	opt(sp)

	if sp.Linux == nil || sp.Linux.Resources == nil || sp.Linux.Resources.CPU == nil {
		t.Fatal("Linux resources not initialized")
	}
	if *sp.Linux.Resources.CPU.Shares != 512 {
		t.Errorf("CPU shares = %d, want %d", *sp.Linux.Resources.CPU.Shares, 512)
	}
}

func TestSpecOptionWithCPUQuota(t *testing.T) {
	sp := &specs.Spec{}
	opt := WithCPUQuota(50000)
	opt(sp)

	if sp.Linux == nil || sp.Linux.Resources == nil || sp.Linux.Resources.CPU == nil {
		t.Fatal("Linux resources not initialized")
	}
	if *sp.Linux.Resources.CPU.Quota != 50000 {
		t.Errorf("CPU quota = %d, want %d", *sp.Linux.Resources.CPU.Quota, 50000)
	}
}

func TestSpecOptionWithPidsLimit(t *testing.T) {
	sp := &specs.Spec{}
	opt := WithPidsLimit(100)
	opt(sp)

	if sp.Linux == nil || sp.Linux.Resources == nil || sp.Linux.Resources.Pids == nil {
		t.Fatal("Linux resources not initialized")
	}
	if sp.Linux.Resources.Pids.Limit != 100 {
		t.Errorf("Pids limit = %d, want %d", sp.Linux.Resources.Pids.Limit, 100)
	}
}

func TestSpecOptionWithHostname(t *testing.T) {
	sp := &specs.Spec{}
	opt := WithHostname("mycontainer")
	opt(sp)

	if sp.Hostname != "mycontainer" {
		t.Errorf("Hostname = %q, want mycontainer", sp.Hostname)
	}
}

func TestSpecOptionWithMount(t *testing.T) {
	sp := &specs.Spec{}
	opt := WithMount("/host/data", "/container/data", "none", []string{"bind", "ro"})
	opt(sp)

	if len(sp.Mounts) != 1 {
		t.Fatalf("Mounts length = %d, want 1", len(sp.Mounts))
	}
	mount := sp.Mounts[0]
	if mount.Source != "/host/data" {
		t.Errorf("Mount source = %q, want /host/data", mount.Source)
	}
	if mount.Destination != "/container/data" {
		t.Errorf("Mount destination = %q, want /container/data", mount.Destination)
	}
	if mount.Type != "none" {
		t.Errorf("Mount type = %q, want none", mount.Type)
	}
	if len(mount.Options) != 2 || mount.Options[0] != "bind" || mount.Options[1] != "ro" {
		t.Errorf("Mount options = %v, want [bind ro]", mount.Options)
	}
}

func TestSpecOptionWithAnnotation(t *testing.T) {
	sp := &specs.Spec{}
	opt := WithAnnotation("com.example/key", "value")
	opt(sp)

	if sp.Annotations == nil {
		t.Fatal("Annotations is nil")
	}
	if sp.Annotations["com.example/key"] != "value" {
		t.Errorf("Annotation = %q, want value", sp.Annotations["com.example/key"])
	}
}

func TestSpecOptionWithNetworkNamespace(t *testing.T) {
	sp := &specs.Spec{}
	opt := WithNetworkNamespace("/proc/1/ns/net")
	opt(sp)

	if sp.Linux == nil || len(sp.Linux.Namespaces) != 1 {
		t.Fatal("Namespace not added")
	}
	ns := sp.Linux.Namespaces[0]
	if ns.Type != specs.NetworkNamespace {
		t.Errorf("Namespace type = %q, want network", ns.Type)
	}
	if ns.Path != "/proc/1/ns/net" {
		t.Errorf("Namespace path = %q, want /proc/1/ns/net", ns.Path)
	}
}

func TestSetOrReplaceLinuxNamespace(t *testing.T) {
	sp := &specs.Spec{}

	// Add new namespace
	SetOrReplaceLinuxNamespace(sp, specs.NetworkNamespace, "/proc/1/ns/net")
	if len(sp.Linux.Namespaces) != 1 {
		t.Fatalf("Namespaces length = %d, want 1", len(sp.Linux.Namespaces))
	}
	if sp.Linux.Namespaces[0].Path != "/proc/1/ns/net" {
		t.Errorf("Path = %q, want /proc/1/ns/net", sp.Linux.Namespaces[0].Path)
	}

	// Replace existing namespace
	SetOrReplaceLinuxNamespace(sp, specs.NetworkNamespace, "/proc/2/ns/net")
	if len(sp.Linux.Namespaces) != 1 {
		t.Fatalf("Namespaces length = %d, want 1 after replace", len(sp.Linux.Namespaces))
	}
	if sp.Linux.Namespaces[0].Path != "/proc/2/ns/net" {
		t.Errorf("Path = %q, want /proc/2/ns/net after replace", sp.Linux.Namespaces[0].Path)
	}

	// Add different namespace type
	SetOrReplaceLinuxNamespace(sp, specs.MountNamespace, "")
	if len(sp.Linux.Namespaces) != 2 {
		t.Fatalf("Namespaces length = %d, want 2", len(sp.Linux.Namespaces))
	}
}

func TestRemoveLinuxNamespace(t *testing.T) {
	sp := &specs.Spec{
		Linux: &specs.Linux{
			Namespaces: []specs.LinuxNamespace{
				{Type: specs.NetworkNamespace, Path: "/proc/1/ns/net"},
				{Type: specs.MountNamespace},
				{Type: specs.PIDNamespace},
			},
		},
	}

	RemoveLinuxNamespace(sp, specs.MountNamespace)

	if len(sp.Linux.Namespaces) != 2 {
		t.Fatalf("Namespaces length = %d, want 2", len(sp.Linux.Namespaces))
	}

	for _, ns := range sp.Linux.Namespaces {
		if ns.Type == specs.MountNamespace {
			t.Error("MountNamespace should have been removed")
		}
	}
}

func TestRemoveLinuxNamespaceNil(t *testing.T) {
	// Should not panic on nil
	sp := &specs.Spec{}
	RemoveLinuxNamespace(sp, specs.NetworkNamespace)

	sp2 := &specs.Spec{Linux: &specs.Linux{}}
	RemoveLinuxNamespace(sp2, specs.NetworkNamespace)
}

func TestSpecOptionWithUser(t *testing.T) {
	sp := &specs.Spec{}
	opt := WithUser(1000, 2000)
	opt(sp)

	if sp.Process == nil {
		t.Fatal("Process is nil")
	}
	if sp.Process.User.UID != 1000 {
		t.Errorf("UID = %d, want 1000", sp.Process.User.UID)
	}
	if sp.Process.User.GID != 2000 {
		t.Errorf("GID = %d, want 2000", sp.Process.User.GID)
	}
}

func TestSpecOptionWithCwd(t *testing.T) {
	sp := &specs.Spec{}
	opt := WithCwd("/app")
	opt(sp)

	if sp.Process == nil {
		t.Fatal("Process is nil")
	}
	if sp.Process.Cwd != "/app" {
		t.Errorf("Cwd = %q, want /app", sp.Process.Cwd)
	}
}

func TestSpecOptionWithHostNetwork(t *testing.T) {
	sp := &specs.Spec{
		Linux: &specs.Linux{
			Namespaces: []specs.LinuxNamespace{
				{Type: specs.NetworkNamespace},
				{Type: specs.PIDNamespace},
				{Type: specs.MountNamespace},
			},
		},
	}

	opt := WithHostNetwork()
	opt(sp)

	// Network namespace should be removed
	for _, ns := range sp.Linux.Namespaces {
		if ns.Type == specs.NetworkNamespace {
			t.Error("NetworkNamespace should have been removed by WithHostNetwork")
		}
	}

	// Other namespaces should remain
	if len(sp.Linux.Namespaces) != 2 {
		t.Errorf("Namespaces length = %d, want 2", len(sp.Linux.Namespaces))
	}
}

func TestSpecOptionWithCapability(t *testing.T) {
	sp := &specs.Spec{}
	opt := WithCapability(CapNetRaw)
	opt(sp)

	if sp.Process == nil {
		t.Fatal("Process is nil")
	}
	if sp.Process.Capabilities == nil {
		t.Fatal("Capabilities is nil")
	}

	c := sp.Process.Capabilities
	capSets := [][]string{c.Bounding, c.Effective, c.Inheritable, c.Permitted, c.Ambient}
	names := []string{"Bounding", "Effective", "Inheritable", "Permitted", "Ambient"}

	for i, capSet := range capSets {
		found := false
		for _, cap := range capSet {
			if cap == string(CapNetRaw) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s does not contain %s", names[i], CapNetRaw)
		}
	}
}

func TestSpecOptionWithCapabilityNoDuplicates(t *testing.T) {
	sp := &specs.Spec{}

	// Apply same capability twice
	opt := WithCapability(CapNetRaw)
	opt(sp)
	opt(sp)

	// Should not have duplicates
	c := sp.Process.Capabilities
	capSets := [][]string{c.Bounding, c.Effective, c.Inheritable, c.Permitted, c.Ambient}
	names := []string{"Bounding", "Effective", "Inheritable", "Permitted", "Ambient"}

	for i, capSet := range capSets {
		count := 0
		for _, cap := range capSet {
			if cap == string(CapNetRaw) {
				count++
			}
		}
		if count != 1 {
			t.Errorf("%s has %d copies of %s, want 1", names[i], count, CapNetRaw)
		}
	}
}
