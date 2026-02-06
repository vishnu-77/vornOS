# Vorn OS Operations Guide

## Installation

### From Source
```bash
make build
sudo ./scripts/install.sh
```

### From Packer Image
```bash
make image   # Produces QCOW2 at deploy/packer/output-vornos/vornos.qcow2
```

## Boot Sequence

```
BIOS/UEFI → GRUB → Kernel + nvidia.ko
  ├── nvidia-persistenced.service    # Keep GPUs initialized
  ├── vorn-firewall.service          # Load nftables rules
  ├── vorn-model-unlocker.service    # Unseal model DEK
  ├── vorn-gpu-healthd.service       # GPU health monitoring
  └── vornd.service                  # Main daemon (Type=notify)
```

## First-Time Setup

1. **Generate signing keys**:
   ```bash
   ./scripts/gen-keys.sh
   cp vorn-sign.pub /etc/vorn/trust.d/
   ```

2. **Initialize model encryption**:
   ```bash
   vorn-model-unlocker --init
   # Enter and confirm passphrase (stored as wrapped DEK)
   ```

3. **Configure** `/etc/vorn/vornd.yaml` (API address, log level, GPU settings)

4. **Start services**:
   ```bash
   systemctl start vornd
   vornctl status
   ```

## Service Management

| Service | Purpose | Type |
|---------|---------|------|
| `vornd.service` | Main daemon (gRPC API, scheduler, job manager) | notify |
| `vorn-gpu-healthd.service` | GPU health polling and fault detection | simple |
| `vorn-model-unlocker.service` | DEK unsealing at boot | oneshot |
| `vorn-firewall.service` | Load nftables rules | oneshot |
| `vorn-logrotate.timer` | Daily log rotation trigger | timer |

```bash
systemctl status vornd
journalctl -u vornd -f          # Follow daemon logs
journalctl -u vorn-gpu-healthd  # GPU health events
```

## CLI Operations

### Status
```bash
vornctl status                  # Daemon + GPU inventory
```

### Runtimes
```bash
vornctl runtime register --name pytorch-2.2 --image /path/to/pytorch.sif
vornctl runtime list
vornctl runtime inspect pytorch-2.2
```

### Models
```bash
vornctl model add --name llama-7b --file /path/to/model.bin --key /path/to/privkey.pem
vornctl model list
vornctl model verify llama-7b
```

### Jobs
```bash
vornctl run --model llama-7b --runtime pytorch-2.2 --gpus 2 --mode research
vornctl jobs list
vornctl jobs logs <job-id>
vornctl jobs kill <job-id>
```

## GPU Health States

| State | Meaning | Triggers |
|-------|---------|----------|
| `healthy` | Normal operation | Default state |
| `degraded` | Performance may be impacted | Temp >83°C, correctable ECC >100 |
| `failed` | GPU should not accept new jobs | Temp >90°C, uncorrectable ECC, Xid error |

GPU health is polled every 5 seconds by `vorn-gpu-healthd`.

## Log Files

| File | Retention | Content |
|------|-----------|---------|
| `/var/log/vorn/vornd.log` | 30 days | Daemon JSON logs |
| `/var/log/vorn/audit.log` | 365 days | Security audit events |
| `/var/log/vorn/gpu-health.log` | 90 days | GPU fault events |
| `/var/log/vorn/jobs/<job-id>.log` | Per job | Job stdout/stderr |

Log rotation is handled by `vorn-logrotate.timer` (daily).

## Monitoring

### Prometheus Endpoints
- **Metrics**: `http://<host>:9801/metrics`
- **Health**: `http://<host>:9801/healthz` (liveness)
- **Ready**: `http://<host>:9801/readyz` (readiness)

### Key Metrics
| Metric | Type | Description |
|--------|------|-------------|
| `vorn_gpu_temperature_celsius` | gauge | Per-GPU temperature |
| `vorn_gpu_memory_used_mb` | gauge | Per-GPU VRAM usage |
| `vorn_gpu_utilization_percent` | gauge | Per-GPU compute utilization |
| `vorn_gpu_health_state` | gauge | Per-GPU health (0/1/2) |
| `vorn_jobs_active` | gauge | Running job count |
| `vorn_jobs_total` | counter | Jobs by status |
| `vorn_jobs_duration_seconds` | histogram | Job execution time |
| `vorn_model_failures_total` | counter | Model verification failures |
| `vorn_scheduler_vram_allocated_mb` | gauge | VRAM allocated per GPU |
| `vorn_api_request_duration_seconds` | histogram | API latency |

### Alerting Rules
Alerting rules are installed at `/opt/vorn/share/prometheus/alerts.yml`. Import into your Prometheus configuration:
```yaml
rule_files:
  - /opt/vorn/share/prometheus/alerts.yml
```

## Execution Policies

Policies in `/etc/vorn/policies/` control resource limits per isolation mode:

| Policy | Max GPUs | Max VRAM | Max CPU | Max Walltime | Network |
|--------|----------|----------|---------|--------------|---------|
| `default` | 4 | 48 GB | 32 cores | 24h | denied |
| `research` | 8 | 80 GB | 64 cores | 72h | allowed |
| `secure` | 2 | 24 GB | 16 cores | 12h | denied |

## Firewall

Rules at `/etc/vorn/firewall.rules` are loaded by `vorn-firewall.service`:
```bash
# View active rules
nft list ruleset

# Reload rules
systemctl restart vorn-firewall
```

## Troubleshooting

### vornd won't start
```bash
journalctl -u vornd --no-pager -n 50
# Check config syntax
vornd --config /etc/vorn/vornd.yaml --validate
```

### GPU not detected
```bash
nvidia-smi                      # Driver loaded?
ls /dev/nvidia*                 # Device nodes exist?
journalctl -u nvidia-persistenced
```

### Model verification failures
```bash
ls /etc/vorn/trust.d/           # Trust keys present?
vornctl model verify <model-id> # Detailed error
```

### Jobs failing to launch
```bash
vornctl jobs logs <job-id>      # Check job stderr
apparmor_status                 # AppArmor profiles loaded?
cat /var/log/vorn/audit.log | jq '. | select(.job_id=="<job-id>")'
```
