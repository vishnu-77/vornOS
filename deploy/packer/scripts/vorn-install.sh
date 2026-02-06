#!/bin/bash
# Install Vorn OS components.
set -euo pipefail

VORN_VERSION="${VORN_VERSION:-0.1.0}"
echo "=== Installing Vorn OS ${VORN_VERSION} ==="

# Create vorn user
id -u vorn &>/dev/null || useradd --system --no-create-home --shell /usr/sbin/nologin vorn
usermod -aG video,render vorn

# Create directory hierarchy
mkdir -p /opt/vorn/bin /opt/vorn/lib/gpu-compat /opt/vorn/share/runtimes
mkdir -p /opt/vorn/share/policies /opt/vorn/share/prometheus /opt/vorn/libexec/hooks
mkdir -p /etc/vorn/trust.d /etc/vorn/policies /etc/vorn/apparmor.d /etc/vorn/seccomp /etc/vorn/tls
mkdir -p /var/lib/vorn/db /var/lib/vorn/runtimes/layers /var/lib/vorn/models/blobs
mkdir -p /var/lib/vorn/jobs /var/lib/vorn/scheduler /var/lib/vorn/encrypt /var/lib/vorn/cache
mkdir -p /var/log/vorn/jobs /var/run/vorn

# Install binaries
install -m 0755 /tmp/vorn-bin/vornd              /opt/vorn/bin/vornd
install -m 0755 /tmp/vorn-bin/vornctl            /opt/vorn/bin/vornctl
install -m 0755 /tmp/vorn-bin/vorn-gpu-healthd   /opt/vorn/bin/vorn-gpu-healthd 2>/dev/null || true
install -m 0755 /tmp/vorn-bin/vorn-model-unlocker /opt/vorn/bin/vorn-model-unlocker 2>/dev/null || true

# Create symlinks in PATH
ln -sf /opt/vorn/bin/vornd   /usr/local/bin/vornd
ln -sf /opt/vorn/bin/vornctl /usr/local/bin/vornctl

# Install config files
install -m 0644 /tmp/vorn-configs/vornd.yaml        /etc/vorn/vornd.yaml
install -m 0644 /tmp/vorn-configs/gpu-compat.yaml    /opt/vorn/lib/gpu-compat/matrix.yaml
install -m 0644 /tmp/vorn-configs/firewall.rules     /etc/vorn/firewall.rules
install -m 0644 /tmp/vorn-configs/logrotate.conf     /etc/vorn/logrotate.conf
cp /tmp/vorn-configs/policies/*.yaml /etc/vorn/policies/

# Install systemd units
install -m 0644 /tmp/vorn-systemd/vornd.service               /etc/systemd/system/
install -m 0644 /tmp/vorn-systemd/vorn-gpu-healthd.service     /etc/systemd/system/
install -m 0644 /tmp/vorn-systemd/vorn-model-unlocker.service  /etc/systemd/system/
install -m 0644 /tmp/vorn-systemd/vorn-firewall.service        /etc/systemd/system/
install -m 0644 /tmp/vorn-systemd/vorn-logrotate.timer         /etc/systemd/system/
install -m 0644 /tmp/vorn-systemd/vorn-logrotate.service       /etc/systemd/system/

# Install AppArmor profiles
cp /tmp/vorn-apparmor/* /etc/vorn/apparmor.d/

# Install seccomp filters
cp /tmp/vorn-seccomp/* /etc/vorn/seccomp/

# Set ownership
chown -R vorn:vorn /var/lib/vorn /var/log/vorn
chmod 700 /var/lib/vorn/encrypt

# Enable services
systemctl daemon-reload
systemctl enable vornd.service
systemctl enable vorn-gpu-healthd.service
systemctl enable vorn-firewall.service
systemctl enable vorn-logrotate.timer

echo "=== Vorn OS installation complete ==="
