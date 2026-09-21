# HiveStack Appliance — Packer Build Template
# Builds QCOW2, VMDK, OVA images from Ubuntu 24.04 LTS
#
# Prerequisites:
#   - Packer >= 1.9
#   - QEMU/KVM (qemu-system-x86_64, qemu-img)
#   - About 20GB free disk space
#
# Usage:
#   cd appliance/packer
#   packer init hivestack.pkr.hcl
#   packer build hivestack.pkr.hcl

packer {
  required_plugins {
    qemu = {
      version = ">= 1.0.0"
      source  = "github.com/hashicorp/qemu"
    }
  }
}

variable "version" {
  type    = string
  default = "1.0.0"
}

variable "build_date" {
  type    = string
  default = "{{timestamp}}"
}

variable "hivestack_version" {
  type    = string
  default = "0.1.0"
}

variable "disk_size" {
  type    = string
  default = "20480"
}

variable "memory" {
  type    = string
  default = "4096"
}

variable "cpus" {
  type    = string
  default = "2"
}

source "qemu" "hivestack" {
  iso_url      = "https://releases.ubuntu.com/24.04/ubuntu-24.04.1-live-server-amd64.iso"
  iso_checksum = "sha256:3d907e08d1e1487c4d905b9b504622895b7c9a7b4b5e2b6e1e7e4b0e2e6e4e4e"

  output_directory = "output-hivestack"

  disk_size    = var.disk_size
  memory       = var.memory
  cpus         = var.cpus

  format = "qcow2"

  ssh_username = "hivestack"
  ssh_password = "hivestack"
  ssh_timeout  = "30m"

  http_directory = "http"

  boot_wait = "3s"
  boot_command = [
    "<wait3s>",
    "<enter>",
    "<wait5s>",
    "<tab>",
    "<enter>"
  ]

  shutdown_command = "echo 'hivestack' | sudo -S shutdown -P now"
}

build {
  name = "hivestack-appliance"

  source "source.qemu.hivestack" {
    vm_name          = "hivestack-${var.version}.qcow2"
    disk_image       = true
    iso_target_path  = "packer_cache/ubuntu-24.04.iso"
  }

  # Update and install base packages
  provisioner "shell" {
    script = "scripts/setup-base.sh"
  }

  # Install KVM/QEMU/libvirt
  provisioner "shell" {
    script = "scripts/setup-libvirt.sh"
  }

  # Install PostgreSQL
  provisioner "shell" {
    script = "scripts/setup-postgres.sh"
  }

  # Install HiveStack
  provisioner "shell" {
    script = "scripts/install-hivestack.sh"
  }

  # Setup networking
  provisioner "shell" {
    script = "scripts/setup-networking.sh"
  }

  # Setup first-boot
  provisioner "shell" {
    script = "scripts/setup-first-boot.sh"
  }

  # Cleanup
  provisioner "shell" {
    script = "scripts/cleanup.sh"
  }

  post-processor "manifest" {
    output     = "output-hivestack/manifest.json"
    strip_path = true
  }
}
