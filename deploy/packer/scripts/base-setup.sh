#!/bin/bash
# Base system setup for Vorn OS image.
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

echo "=== Vorn OS Base Setup ==="

# Update system
apt-get update
apt-get upgrade -y

# Install essential packages
apt-get install -y --no-install-recommends \
    apparmor apparmor-utils \
    nftables \
    tpm2-tools \
    logrotate \
    jq \
    sqlite3 \
    numactl \
    pciutils \
    ethtool \
    linux-tools-generic \
    curl \
    ca-certificates \
    gnupg

# Install Apptainer
add-apt-repository -y ppa:apptainer/ppa
apt-get update
apt-get install -y apptainer

echo "=== Base setup complete ==="
