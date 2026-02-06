#!/bin/bash
# Security hardening for Vorn OS image.
set -euo pipefail

echo "=== Applying security hardening ==="

# Kernel parameters for GPU workloads and security
cat > /etc/sysctl.d/99-vornos.conf << 'EOF'
# GPU workload tuning
vm.nr_hugepages = 8192
vm.swappiness = 1
vm.dirty_ratio = 40
vm.dirty_background_ratio = 10
vm.zone_reclaim_mode = 0

# Network tuning
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.core.netdev_max_backlog = 5000
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216
net.ipv4.tcp_tw_reuse = 1

# Security hardening
kernel.randomize_va_space = 2
kernel.dmesg_restrict = 1
kernel.kptr_restrict = 2
kernel.yama.ptrace_scope = 1
kernel.perf_event_paranoid = 2
net.ipv4.conf.all.rp_filter = 1
net.ipv4.conf.all.accept_redirects = 0
net.ipv6.conf.all.accept_redirects = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv4.tcp_syncookies = 1
fs.protected_hardlinks = 1
fs.protected_symlinks = 1

# File descriptor limits
fs.file-max = 2097152
fs.inotify.max_user_watches = 524288

# Shared memory for large model loading
kernel.shmmax = 68719476736
kernel.shmall = 4294967296
EOF

# GRUB kernel parameters
cat > /etc/default/grub.d/vornos.cfg << 'EOF'
GRUB_CMDLINE_LINUX="$GRUB_CMDLINE_LINUX iommu=pt intel_iommu=on nvidia.NVreg_RegistryDwords=RMNumGpuPerIoApic=0 hugepagesz=2M hugepages=4096"
EOF
update-grub

# Enable AppArmor
systemctl enable apparmor.service

# Disable unnecessary services
systemctl disable cups.service 2>/dev/null || true
systemctl disable avahi-daemon.service 2>/dev/null || true
systemctl disable ModemManager.service 2>/dev/null || true
systemctl mask unattended-upgrades.service 2>/dev/null || true

# Remove unnecessary packages
apt-get purge -y popularity-contest 2>/dev/null || true

# SSH hardening
sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config
sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin no/' /etc/ssh/sshd_config

echo "=== Hardening complete ==="
