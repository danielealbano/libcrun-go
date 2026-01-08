# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.x.x   | :white_check_mark: |

All current releases receive security updates. As the project matures, older minor versions may be deprecated.

## Security Considerations

libcrun-go provides Go bindings for libcrun, a container runtime. By design, this library:

- Manages container isolation boundaries
- Handles privileged operations (namespaces, cgroups, capabilities)
- Interfaces directly with the Linux kernel via syscalls

Vulnerabilities in this library could lead to container escapes or privilege escalation. We take all security reports seriously.

## Reporting a Vulnerability

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, report vulnerabilities via:

1. **Email**: Send details to [d.albano@gmail.com](mailto:d.albano@gmail.com)
2. **GitHub Security Advisory**: Use [GitHub's private vulnerability reporting](https://github.com/danielealbano/libcrun-go/security/advisories/new)

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Potential impact
- Any suggested fixes (optional)

### Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial assessment**: Within 7 days
- **Resolution target**: Within 30 days for critical issues

### After Reporting

- You'll receive updates on the investigation progress
- Once fixed, we'll coordinate disclosure timing with you
- Security fixes are released as patch versions with CVE assignment when appropriate
- Contributors will be credited (unless anonymity is requested)

## Security Best Practices

When using libcrun-go:

- Run containers with minimal privileges (rootless when possible)
- Keep the library updated to the latest version
- Review container specs before execution
- Use resource limits (`WithMemoryLimit`, `WithPidsLimit`, etc.)
- Validate untrusted input before passing to container APIs
