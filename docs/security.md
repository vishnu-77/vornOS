# Vorn OS Security Architecture

## Overview

Vorn OS implements defense-in-depth for AI workloads: model signing, encryption at rest, process isolation, mandatory access control, and network firewalling.

## Model Signing

### Key Hierarchy
- **Organization root key** (offline, Ed25519): signs annual signing keys
- **Signing keys** (annual rotation): stored in `/etc/vorn/trust.d/`
- **Verification**: `vornd` loads all public keys from `trust.d/` at startup

### Signing Flow
```
model_file → SHA-256(model_file) → Ed25519_sign(hash, privkey) → blob.sig
```

### Verification Flow
```
blob.sig + blob.sha256 → try all keys in trust.d/ → accept if any verify
```

### Key Rotation
1. Generate new Ed25519 keypair with `scripts/gen-keys.sh`
2. Place new `.pub` file in `/etc/vorn/trust.d/`
3. Restart `vornd` to load updated trust bundle
4. Old keys remain valid until manually removed

## Encrypted Model Store

### Architecture
- **DEK** (Data Encryption Key): 256-bit random, generated once per installation
- **KEK** (Key Encryption Key): derived from passphrase via Argon2id (or TPM 2.0)
- **Wrapped DEK**: `AES-256-GCM(KEK, nonce, DEK)` stored at `/var/lib/vorn/encrypt/model-store.key.enc`

### Boot Unsealing
1. `vorn-model-unlocker.service` runs at boot (before `vornd`)
2. Reads passphrase from secure input or TPM
3. Derives KEK via Argon2id (time=3, memory=256MB, threads=4)
4. Unwraps DEK
5. Stores DEK in Linux kernel keyring (`@u` scope, `vorn-model-dek` description)
6. `vornd` reads DEK from keyring at startup

### Per-Model Encryption
```
nonce(12B) || AES-256-GCM(DEK, nonce, plaintext, aad=model_id) || tag(16B)
```

Each model is encrypted independently with a unique random nonce.

## Process Isolation

### Research Mode (Apptainer)
- `--nv`: NVIDIA GPU pass-through via driver mount
- `--contain`: isolated PID/IPC/mount namespaces
- `--writable-tmpfs`: ephemeral filesystem
- `--bind`: workspace directories only
- AppArmor profile: `vorn-job-default` (GPU device access, deny network, deny sensitive paths)
- Seccomp filter: `default.json` (~250 allowed syscalls)

### Secure Mode (Kata Containers)
- Full VM isolation via `kata-runtime`
- VFIO GPU passthrough (no direct `/dev/nvidia*` in guest)
- OCI bundle with restricted mounts
- AppArmor profile: `vorn-job-secure` (read-only model, deny all GPU direct, deny network/mount)
- Seccomp filter: `gpu-only.json` (no network syscalls, AF_UNIX only)

### Appliance Mode
- Direct exec with `CUDA_VISIBLE_DEVICES` isolation
- Minimal overhead for trusted workloads
- cgroup v2 resource limits

## System Hardening

### Kernel Parameters (`/etc/sysctl.d/99-vornos.conf`)
| Parameter | Value | Purpose |
|-----------|-------|---------|
| `kernel.randomize_va_space` | 2 | Full ASLR |
| `kernel.dmesg_restrict` | 1 | Block unprivileged dmesg |
| `kernel.kptr_restrict` | 2 | Hide kernel pointers |
| `kernel.yama.ptrace_scope` | 1 | Restrict ptrace to parent |
| `net.ipv4.tcp_syncookies` | 1 | SYN flood protection |
| `fs.protected_hardlinks` | 1 | Hardlink protection |
| `fs.protected_symlinks` | 1 | Symlink protection |

### GRUB Parameters
```
iommu=pt intel_iommu=on hugepagesz=2M hugepages=4096
```

### Firewall (nftables)
- **Default policy**: drop all inbound
- **Allowed inbound**: SSH (22), API (9800), Metrics (9801) from RFC1918 only
- **Outbound**: unrestricted (for model downloads, package updates)

### SSH Hardening
- Password authentication disabled
- Root login disabled
- Key-based auth only

### Daemon Capabilities
- `vornd`: `CAP_NET_BIND_SERVICE` + `CAP_SYS_PTRACE` only
- Job containers: zero capabilities

## Audit Logging

All security-relevant events are logged to `/var/log/vorn/audit.log` (JSON format, 365-day retention):
- Model add/remove/verify (success and failure)
- Job submit/start/complete/fail/kill
- Runtime register/remove
- Config reload
- Authentication failures
- Policy violations

Audit events are dual-written: JSON file + SQLite `audit_log` table.

## TLS / mTLS

- gRPC API on TCP supports mutual TLS
- Certificates in `/etc/vorn/tls/` (server.crt, server.key, ca.crt)
- Unix socket (`/var/run/vorn/vornd.sock`) for local access (no TLS needed)
- Socket permissions: 0660, owned by `vorn:vorn`
