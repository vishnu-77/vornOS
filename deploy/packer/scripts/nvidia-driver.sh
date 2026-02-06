#!/bin/bash
# Install NVIDIA drivers for Vorn OS.
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive
DRIVER_VERSION="${NVIDIA_DRIVER_VERSION:-545}"

echo "=== Installing NVIDIA Driver ${DRIVER_VERSION} ==="

# Add NVIDIA package repository
curl -fsSL https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2404/x86_64/3bf863cc.pub | \
    gpg --dearmor -o /usr/share/keyrings/cuda-archive-keyring.gpg

echo "deb [signed-by=/usr/share/keyrings/cuda-archive-keyring.gpg] https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2404/x86_64/ /" \
    > /etc/apt/sources.list.d/cuda.list

apt-get update

# Install driver and utilities
apt-get install -y --no-install-recommends \
    "nvidia-driver-${DRIVER_VERSION}" \
    "nvidia-utils-${DRIVER_VERSION}" \
    "libnvidia-compute-${DRIVER_VERSION}" \
    nvidia-container-toolkit

# Enable nvidia-persistenced
systemctl enable nvidia-persistenced

# Verify installation
nvidia-smi || echo "WARNING: nvidia-smi not available (expected in build environment without GPU)"

echo "=== NVIDIA driver installation complete ==="
