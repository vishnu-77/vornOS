# Vorn OS Architecture

## Overview

Vorn OS is a GPU-first, secure runtime for AI workloads. It runs on hardened Ubuntu LTS
and provides a daemon (`vornd`) + CLI (`vornctl`) for managing GPU runtimes, signed models,
and isolated job execution.

## Components

```
┌─────────────────────────────────────────────────────────────┐
│                       vornctl (CLI)                          │
│                 (gRPC client → Unix/TCP)                     │
├─────────────────────────────────────────────────────────────┤
│                   gRPC API (mTLS / Unix)                     │
├─────────────────────────────────────────────────────────────┤
│                      vornd (daemon)                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────────────┐ │
│  │ Runtime   │ │ Model    │ │Scheduler │ │  Telemetry     │ │
│  │ Store     │ │ Store    │ │(VRAM     │ │  (Prometheus   │ │
│  │ (SQLite)  │ │ (Signed  │ │ bin-pack)│ │   40+ metrics) │ │
│  │           │ │  + AES)  │ │          │ │                │ │
│  └──────────┘ └──────────┘ └──────────┘ └────────────────┘ │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              Isolation Layer                          │   │
│  │  Apptainer (research) │ Kata (secure) │ Appliance    │   │
│  └──────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Policy Engine  │  Hooks  │  Audit Logger            │   │
│  └──────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────┤
│  GPU Health Monitor (vorn-gpu-healthd)                       │
├─────────────────────────────────────────────────────────────┤
│         NVIDIA GPU Driver + CUDA Runtime (in layers)         │
├─────────────────────────────────────────────────────────────┤
│    Hardened Ubuntu LTS (AppArmor, seccomp, nftables)         │
└─────────────────────────────────────────────────────────────┘
```

## Binaries

| Binary | Purpose |
|--------|---------|
| `vornd` | Main daemon: gRPC API, scheduler, job manager, metrics |
| `vornctl` | CLI client for operators |
| `vorn-gpu-healthd` | GPU health polling + fault detection |
| `vorn-model-unlocker` | DEK unsealing at boot |

## Data Flow

1. `vornctl run` submits a job referencing a model ID and runtime ID.
2. `vornd` verifies the model signature (Ed25519) and checks runtime compatibility.
3. The policy engine validates the request against the isolation mode's limits.
4. The scheduler allocates GPUs based on VRAM availability (first-fit bin-packing).
5. Pre-job hooks run (GPU reset, workspace preparation).
6. The isolation layer launches the job in Apptainer, Kata, or appliance mode.
7. The model is decrypted from the encrypted store into the job workspace.
8. Metrics are exported to Prometheus throughout execution.
9. Post-job hooks run (cleanup, artifact collection).
10. All operations are recorded in the audit log.

## Persistence

- **SQLite** (WAL mode) at `/var/lib/vorn/db/vorn.db`: runtimes, models, jobs, audit log
- **Filesystem**: model blobs, runtime SIF images, job workspaces
- **Scheduler state**: persisted to SQLite for crash recovery

## Security Layers

1. **Model signing** — Ed25519 with key rotation via trust bundle
2. **Model encryption** — AES-256-GCM with DEK/KEK architecture
3. **Process isolation** — Apptainer containers or Kata VMs
4. **Mandatory access control** — AppArmor profiles per isolation mode
5. **Syscall filtering** — seccomp BPF (default + gpu-only variants)
6. **Network firewalling** — nftables, API ports restricted to trusted CIDRs
7. **Transport security** — mTLS for gRPC, Unix socket for local access
8. **Audit trail** — dual-write to JSON file + SQLite (365-day retention)

## Key Design Decisions

- **Ed25519 model signing**: fast, small signatures, no CA infrastructure needed for MVP.
- **Apptainer over Docker**: better HPC/GPU support, rootless by default.
- **Kata for secure mode**: hardware-level isolation via lightweight VMs.
- **Drivers on host, CUDA in layers**: multiple CUDA versions coexist without conflicts.
- **SQLite over Postgres**: zero-dependency, single-node workload, WAL for concurrent reads.
- **Userland scheduler**: simple VRAM bin-packing; designed to be replaceable with k8s GPU operator.
- **systemd Type=notify**: daemon reports readiness after full initialization; watchdog detects hangs.
