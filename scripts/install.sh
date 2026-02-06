#!/usr/bin/env bash
# Full install script for Vorn OS components on Ubuntu LTS.
# Run as root or with sudo.
set -euo pipefail

VORN_VERSION="${VORN_VERSION:-0.1.0}"
echo "=== Vorn OS ${VORN_VERSION} Installer ==="

# ── Create system user ──
id -u vorn &>/dev/null || useradd --system --no-create-home --shell /usr/sbin/nologin vorn
usermod -aG video,render vorn 2>/dev/null || true

# ── Create directory hierarchy ──
mkdir -p /opt/vorn/bin /opt/vorn/lib/gpu-compat /opt/vorn/share/runtimes
mkdir -p /opt/vorn/share/policies /opt/vorn/share/prometheus /opt/vorn/libexec/hooks
mkdir -p /etc/vorn/trust.d /etc/vorn/policies /etc/vorn/apparmor.d /etc/vorn/seccomp /etc/vorn/tls
mkdir -p /var/lib/vorn/db /var/lib/vorn/runtimes/layers /var/lib/vorn/models/blobs
mkdir -p /var/lib/vorn/jobs /var/lib/vorn/scheduler /var/lib/vorn/encrypt /var/lib/vorn/cache
mkdir -p /var/log/vorn/jobs /var/run/vorn

# ── Install binaries ──
install -m 0755 bin/vornd              /opt/vorn/bin/vornd
install -m 0755 bin/vornctl            /opt/vorn/bin/vornctl
install -m 0755 bin/vorn-gpu-healthd   /opt/vorn/bin/vorn-gpu-healthd   2>/dev/null || true
install -m 0755 bin/vorn-model-unlocker /opt/vorn/bin/vorn-model-unlocker 2>/dev/null || true

# Symlinks into PATH
ln -sf /opt/vorn/bin/vornd              /usr/local/bin/vornd
ln -sf /opt/vorn/bin/vornctl            /usr/local/bin/vornctl
ln -sf /opt/vorn/bin/vorn-gpu-healthd   /usr/local/bin/vorn-gpu-healthd   2>/dev/null || true
ln -sf /opt/vorn/bin/vorn-model-unlocker /usr/local/bin/vorn-model-unlocker 2>/dev/null || true

# ── Install configuration files ──
if [ ! -f /etc/vorn/vornd.yaml ]; then
    install -m 0644 configs/vornd.yaml /etc/vorn/vornd.yaml
fi
install -m 0644 configs/gpu-compat.yaml /opt/vorn/lib/gpu-compat/matrix.yaml
install -m 0644 configs/firewall.rules  /etc/vorn/firewall.rules
install -m 0644 configs/logrotate.conf  /etc/vorn/logrotate.conf

# Install policies
cp configs/policies/*.yaml /etc/vorn/policies/ 2>/dev/null || true

# Install alerting rules
install -m 0644 deploy/prometheus/alerts.yml /opt/vorn/share/prometheus/alerts.yml

# ── Install systemd units ──
install -m 0644 deploy/systemd/vornd.service                /etc/systemd/system/
install -m 0644 deploy/systemd/vorn-gpu-healthd.service      /etc/systemd/system/
install -m 0644 deploy/systemd/vorn-model-unlocker.service   /etc/systemd/system/
install -m 0644 deploy/systemd/vorn-firewall.service         /etc/systemd/system/
install -m 0644 deploy/systemd/vorn-logrotate.timer          /etc/systemd/system/
install -m 0644 deploy/systemd/vorn-logrotate.service        /etc/systemd/system/

# ── Install AppArmor profiles ──
if [ -d deploy/apparmor ]; then
    cp deploy/apparmor/* /etc/vorn/apparmor.d/
fi

# ── Install seccomp filters ──
if [ -d deploy/seccomp ]; then
    cp deploy/seccomp/* /etc/vorn/seccomp/
fi

# ── Install hook scripts ──
if [ -d scripts/hooks ]; then
    install -m 0755 scripts/hooks/pre-job.sh  /opt/vorn/libexec/hooks/pre-job.sh
    install -m 0755 scripts/hooks/post-job.sh /opt/vorn/libexec/hooks/post-job.sh
fi

# ── Set ownership & permissions ──
chown -R vorn:vorn /var/lib/vorn /var/log/vorn /var/run/vorn
chmod 700 /var/lib/vorn/encrypt
chmod 700 /etc/vorn/tls

# ── Enable services ──
systemctl daemon-reload
systemctl enable vornd.service
systemctl enable vorn-gpu-healthd.service
systemctl enable vorn-firewall.service
systemctl enable vorn-logrotate.timer

echo ""
echo "=== Vorn OS ${VORN_VERSION} installed ==="
echo ""
echo "Next steps:"
echo "  1. Place Ed25519 trust keys in /etc/vorn/trust.d/"
echo "  2. Initialize model encryption: vorn-model-unlocker --init"
echo "  3. Configure TLS certs in /etc/vorn/tls/ (optional for mTLS)"
echo "  4. Edit /etc/vorn/vornd.yaml if needed"
echo "  5. systemctl start vornd"
echo "  6. vornctl status"
