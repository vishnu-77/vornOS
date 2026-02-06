#!/bin/bash
# Final cleanup for minimal image size.
set -euo pipefail

echo "=== Cleanup ==="

apt-get autoremove -y
apt-get clean
rm -rf /var/lib/apt/lists/*
rm -rf /tmp/vorn-*
rm -rf /var/tmp/*

# Remove build user password (if present)
passwd -l vorn 2>/dev/null || true

# Zero free space for image compression
dd if=/dev/zero of=/EMPTY bs=1M 2>/dev/null || true
rm -f /EMPTY

echo "=== Image ready ==="
