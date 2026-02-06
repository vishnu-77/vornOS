packer {
  required_plugins {
    qemu = {
      source  = "github.com/hashicorp/qemu"
      version = "~> 1.1"
    }
  }
}

variable "ubuntu_iso_url" {
  type    = string
  default = "https://releases.ubuntu.com/24.04/ubuntu-24.04-live-server-amd64.iso"
}

variable "ubuntu_iso_checksum" {
  type    = string
  default = "sha256:REPLACE_WITH_ACTUAL_CHECKSUM"
}

variable "nvidia_driver_version" {
  type    = string
  default = "545"
}

variable "vorn_version" {
  type    = string
  default = "0.1.0"
}

variable "disk_size" {
  type    = string
  default = "50G"
}

source "qemu" "vornos" {
  iso_url          = var.ubuntu_iso_url
  iso_checksum     = var.ubuntu_iso_checksum
  output_directory = "output-vornos"
  vm_name          = "vornos-${var.vorn_version}.qcow2"
  format           = "qcow2"

  cpus   = 4
  memory = 8192
  disk_size = var.disk_size

  headless = true

  http_directory = "../cloud-init"
  boot_command = [
    "<wait>e<down><down><down><end>",
    " autoinstall ds=nocloud-net;seedfrom=http://{{ .HTTPIP }}:{{ .HTTPPort }}/",
    "<F10>"
  ]

  ssh_username    = "vorn"
  ssh_password    = "vorn-build"
  ssh_timeout     = "30m"
  shutdown_command = "sudo shutdown -P now"
}

build {
  sources = ["source.qemu.vornos"]

  # Base system setup
  provisioner "shell" {
    script = "scripts/base-setup.sh"
  }

  # NVIDIA driver installation
  provisioner "shell" {
    script = "scripts/nvidia-driver.sh"
    environment_vars = [
      "NVIDIA_DRIVER_VERSION=${var.nvidia_driver_version}"
    ]
  }

  # Upload Vorn binaries
  provisioner "file" {
    source      = "../../bin/"
    destination = "/tmp/vorn-bin/"
  }

  # Upload configs
  provisioner "file" {
    source      = "../../configs/"
    destination = "/tmp/vorn-configs/"
  }

  # Upload deploy assets
  provisioner "file" {
    source      = "../systemd/"
    destination = "/tmp/vorn-systemd/"
  }

  provisioner "file" {
    source      = "../apparmor/"
    destination = "/tmp/vorn-apparmor/"
  }

  provisioner "file" {
    source      = "../seccomp/"
    destination = "/tmp/vorn-seccomp/"
  }

  # Vorn OS installation
  provisioner "shell" {
    script = "scripts/vorn-install.sh"
    environment_vars = [
      "VORN_VERSION=${var.vorn_version}"
    ]
  }

  # Security hardening
  provisioner "shell" {
    script = "scripts/hardening.sh"
  }

  # Cleanup
  provisioner "shell" {
    script = "scripts/cleanup.sh"
  }

  post-processor "checksum" {
    checksum_types = ["sha256"]
    output         = "output-vornos/vornos-${var.vorn_version}.sha256"
  }
}
