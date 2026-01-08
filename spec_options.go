//go:build linux

package crun

import (
	"encoding/json"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// SpecOption is a functional option for configuring a spec via NewSpec.
type SpecOption func(*specs.Spec)

// Capability represents a Linux capability.
type Capability string

// Linux capabilities constants.
// See capabilities(7) for detailed descriptions.
const (
	CapChown             Capability = "CAP_CHOWN"
	CapDacOverride       Capability = "CAP_DAC_OVERRIDE"
	CapDacReadSearch     Capability = "CAP_DAC_READ_SEARCH"
	CapFowner            Capability = "CAP_FOWNER"
	CapFsetid            Capability = "CAP_FSETID"
	CapKill              Capability = "CAP_KILL"
	CapSetgid            Capability = "CAP_SETGID"
	CapSetuid            Capability = "CAP_SETUID"
	CapSetpcap           Capability = "CAP_SETPCAP"
	CapLinuxImmutable    Capability = "CAP_LINUX_IMMUTABLE"
	CapNetBindService    Capability = "CAP_NET_BIND_SERVICE"
	CapNetBroadcast      Capability = "CAP_NET_BROADCAST"
	CapNetAdmin          Capability = "CAP_NET_ADMIN"
	CapNetRaw            Capability = "CAP_NET_RAW"
	CapIpcLock           Capability = "CAP_IPC_LOCK"
	CapIpcOwner          Capability = "CAP_IPC_OWNER"
	CapSysModule         Capability = "CAP_SYS_MODULE"
	CapSysRawio          Capability = "CAP_SYS_RAWIO"
	CapSysChroot         Capability = "CAP_SYS_CHROOT"
	CapSysPtrace         Capability = "CAP_SYS_PTRACE"
	CapSysPacct          Capability = "CAP_SYS_PACCT"
	CapSysAdmin          Capability = "CAP_SYS_ADMIN"
	CapSysBoot           Capability = "CAP_SYS_BOOT"
	CapSysNice           Capability = "CAP_SYS_NICE"
	CapSysResource       Capability = "CAP_SYS_RESOURCE"
	CapSysTime           Capability = "CAP_SYS_TIME"
	CapSysTtyConfig      Capability = "CAP_SYS_TTY_CONFIG"
	CapMknod             Capability = "CAP_MKNOD"
	CapLease             Capability = "CAP_LEASE"
	CapAuditWrite        Capability = "CAP_AUDIT_WRITE"
	CapAuditControl      Capability = "CAP_AUDIT_CONTROL"
	CapSetfcap           Capability = "CAP_SETFCAP"
	CapMacOverride       Capability = "CAP_MAC_OVERRIDE"
	CapMacAdmin          Capability = "CAP_MAC_ADMIN"
	CapSyslog            Capability = "CAP_SYSLOG"
	CapWakeAlarm         Capability = "CAP_WAKE_ALARM"
	CapBlockSuspend      Capability = "CAP_BLOCK_SUSPEND"
	CapAuditRead         Capability = "CAP_AUDIT_READ"
	CapPerfmon           Capability = "CAP_PERFMON"
	CapBpf               Capability = "CAP_BPF"
	CapCheckpointRestore Capability = "CAP_CHECKPOINT_RESTORE"
)

// NewSpec creates a new ContainerSpec with the given options applied.
// Set rootless=true for an unprivileged container template.
func NewSpec(rootless bool, opts ...SpecOption) (*ContainerSpec, error) {
	sp, err := DefaultSpec(rootless)
	if err != nil {
		return nil, err
	}
	for _, opt := range opts {
		opt(sp)
	}
	return NewContainerSpec(sp)
}

// WithRootPath sets the root filesystem path.
func WithRootPath(path string) SpecOption {
	return func(sp *specs.Spec) {
		if sp.Root == nil {
			sp.Root = &specs.Root{}
		}
		sp.Root.Path = path
	}
}

// WithArgs sets the process arguments.
func WithArgs(args ...string) SpecOption {
	return func(sp *specs.Spec) {
		if sp.Process == nil {
			sp.Process = &specs.Process{}
		}
		sp.Process.Args = args
	}
}

// WithContainerTTY sets whether to allocate a TTY for the container's init process.
// Set to false for non-interactive processes (most common for tests/automation).
// Note: When true, you must also provide a console socket via RuntimeConfig.ConsoleSocket.
func WithContainerTTY(enabled bool) SpecOption {
	return func(sp *specs.Spec) {
		if sp.Process == nil {
			sp.Process = &specs.Process{}
		}
		sp.Process.Terminal = enabled
	}
}

// WithEnv adds an environment variable.
func WithEnv(key, value string) SpecOption {
	return func(sp *specs.Spec) {
		if sp.Process == nil {
			sp.Process = &specs.Process{}
		}
		sp.Process.Env = append(sp.Process.Env, key+"="+value)
	}
}

// WithMemoryLimit sets the memory limit in bytes.
func WithMemoryLimit(bytes int64) SpecOption {
	return func(sp *specs.Spec) {
		ensureLinuxResources(sp)
		if sp.Linux.Resources.Memory == nil {
			sp.Linux.Resources.Memory = &specs.LinuxMemory{}
		}
		sp.Linux.Resources.Memory.Limit = &bytes
	}
}

// WithCPUShares sets the CPU shares.
func WithCPUShares(shares uint64) SpecOption {
	return func(sp *specs.Spec) {
		ensureLinuxResources(sp)
		if sp.Linux.Resources.CPU == nil {
			sp.Linux.Resources.CPU = &specs.LinuxCPU{}
		}
		sp.Linux.Resources.CPU.Shares = &shares
	}
}

// WithCPUQuota sets the CPU quota.
func WithCPUQuota(quota int64) SpecOption {
	return func(sp *specs.Spec) {
		ensureLinuxResources(sp)
		if sp.Linux.Resources.CPU == nil {
			sp.Linux.Resources.CPU = &specs.LinuxCPU{}
		}
		sp.Linux.Resources.CPU.Quota = &quota
	}
}

// WithPidsLimit sets the pids limit.
func WithPidsLimit(limit int64) SpecOption {
	return func(sp *specs.Spec) {
		ensureLinuxResources(sp)
		if sp.Linux.Resources.Pids == nil {
			sp.Linux.Resources.Pids = &specs.LinuxPids{}
		}
		sp.Linux.Resources.Pids.Limit = limit
	}
}

// WithNetworkNamespace sets the network namespace path.
// If path is empty, a new network namespace is created.
func WithNetworkNamespace(path string) SpecOption {
	return func(sp *specs.Spec) {
		SetOrReplaceLinuxNamespace(sp, specs.NetworkNamespace, path)
	}
}

// WithMountNamespace sets the mount namespace path.
// If path is empty, a new mount namespace is created.
func WithMountNamespace(path string) SpecOption {
	return func(sp *specs.Spec) {
		SetOrReplaceLinuxNamespace(sp, specs.MountNamespace, path)
	}
}

// WithHostname sets the container hostname.
func WithHostname(name string) SpecOption {
	return func(sp *specs.Spec) {
		sp.Hostname = name
	}
}

// WithMount adds a mount to the spec.
func WithMount(source, dest, fstype string, options []string) SpecOption {
	return func(sp *specs.Spec) {
		sp.Mounts = append(sp.Mounts, specs.Mount{
			Source:      source,
			Destination: dest,
			Type:        fstype,
			Options:     options,
		})
	}
}

// WithAnnotation adds an annotation to the spec.
func WithAnnotation(key, value string) SpecOption {
	return func(sp *specs.Spec) {
		if sp.Annotations == nil {
			sp.Annotations = make(map[string]string)
		}
		sp.Annotations[key] = value
	}
}

// WithUser sets the user (UID and GID) for the container process.
func WithUser(uid, gid uint32) SpecOption {
	return func(sp *specs.Spec) {
		if sp.Process == nil {
			sp.Process = &specs.Process{}
		}
		sp.Process.User.UID = uid
		sp.Process.User.GID = gid
	}
}

// WithCwd sets the working directory for the container process.
func WithCwd(path string) SpecOption {
	return func(sp *specs.Spec) {
		if sp.Process == nil {
			sp.Process = &specs.Process{}
		}
		sp.Process.Cwd = path
	}
}

// WithHostNetwork configures the container to share the host's network namespace.
// This removes the network namespace from the spec, causing the container to use the host's network.
func WithHostNetwork() SpecOption {
	return func(sp *specs.Spec) {
		RemoveLinuxNamespace(sp, specs.NetworkNamespace)
	}
}

// WithCapability adds a Linux capability to the container process.
// The capability is added to all capability sets (Bounding, Effective, Inheritable, Permitted, Ambient).
// Example: WithCapability(CapNetRaw) to allow raw socket creation (needed for ping).
func WithCapability(cap Capability) SpecOption {
	return func(sp *specs.Spec) {
		if sp.Process == nil {
			sp.Process = &specs.Process{}
		}
		if sp.Process.Capabilities == nil {
			sp.Process.Capabilities = &specs.LinuxCapabilities{}
		}
		capStr := string(cap)
		c := sp.Process.Capabilities
		if !containsString(c.Bounding, capStr) {
			c.Bounding = append(c.Bounding, capStr)
		}
		if !containsString(c.Effective, capStr) {
			c.Effective = append(c.Effective, capStr)
		}
		if !containsString(c.Inheritable, capStr) {
			c.Inheritable = append(c.Inheritable, capStr)
		}
		if !containsString(c.Permitted, capStr) {
			c.Permitted = append(c.Permitted, capStr)
		}
		if !containsString(c.Ambient, capStr) {
			c.Ambient = append(c.Ambient, capStr)
		}
	}
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func ensureLinuxResources(sp *specs.Spec) {
	if sp.Linux == nil {
		sp.Linux = &specs.Linux{}
	}
	if sp.Linux.Resources == nil {
		sp.Linux.Resources = &specs.LinuxResources{}
	}
}

// DefaultSpec returns a typed OCI spec using libcrun's baseline template,
// unmarshalled into specs-go. Set rootless=true for an unprivileged template.
func DefaultSpec(rootless bool) (*specs.Spec, error) {
	js, err := Spec(rootless)
	if err != nil {
		return nil, err
	}
	var sp specs.Spec
	if err := json.Unmarshal([]byte(js), &sp); err != nil {
		return nil, err
	}
	return &sp, nil
}

// SetOrReplaceLinuxNamespace sets or replaces a Linux namespace entry on the Spec.
// If path != "", it attaches an existing namespace (e.g. "/proc/<pid>/ns/net").
// If path == "", it means "create a fresh namespace" of that type.
func SetOrReplaceLinuxNamespace(sp *specs.Spec, typ specs.LinuxNamespaceType, path string) {
	if sp.Linux == nil {
		sp.Linux = &specs.Linux{}
	}
	found := false
	for i := range sp.Linux.Namespaces {
		if sp.Linux.Namespaces[i].Type == typ {
			sp.Linux.Namespaces[i].Path = path
			found = true
			break
		}
	}
	if !found {
		sp.Linux.Namespaces = append(sp.Linux.Namespaces, specs.LinuxNamespace{
			Type: typ,
			Path: path,
		})
	}
}

// RemoveLinuxNamespace removes a namespace type from the Spec (if present).
func RemoveLinuxNamespace(sp *specs.Spec, typ specs.LinuxNamespaceType) {
	if sp.Linux == nil || len(sp.Linux.Namespaces) == 0 {
		return
	}
	ns := sp.Linux.Namespaces[:0]
	for _, n := range sp.Linux.Namespaces {
		if n.Type != typ {
			ns = append(ns, n)
		}
	}
	sp.Linux.Namespaces = ns
}
