# Quickstart

## Prerequisites

- Ubuntu 22.04+ (server)
- NVIDIA GPU with driver >= 535
- Go 1.22+ (for building from source)
- Apptainer 1.3+ (for research mode)

## Build

```bash
make build
```

Binaries are placed in `bin/vornd` and `bin/vornctl`.

## Install

```bash
sudo ./scripts/install.sh
```

## Generate signing keys

```bash
./scripts/gen-keys.sh ./keys
sudo cp keys/vorn-sign.pub /etc/vorn/trust.pub
```

## Start the daemon

```bash
sudo systemctl start vornd
vornctl status
```

## Register a runtime

```bash
vornctl runtime register \
  --image ./deploy/apptainer/pytorch-runtime.sif \
  --cuda-version 12.4 \
  --arch sm_80
```

## Add a model

```bash
vornctl model add \
  --file ./my-model.onnx \
  --signing-key ./keys/vorn-sign.key
```

## Run a job

```bash
vornctl run \
  --model <model-id> \
  --runtime <runtime-id> \
  --gpu-limit 1 \
  --vram-limit 6000 \
  --mode research
```
