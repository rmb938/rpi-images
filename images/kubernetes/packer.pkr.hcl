
variable "image_mount_path" {
  type    = string
  default = "/mnt/packer-chroot/kubernetes"
}

source "arm" "autogenerated_1" {
  file_checksum_type    = "none"
  file_target_extension = "tar.gz"
  file_unarchive_cmd    = ["tar", "-xpf", "$ARCHIVE_PATH", "-C", "$MOUNTPOINT"]
  file_urls             = ["file:///images/harden.tar.gz"]
  image_build_method    = "new"
  image_chroot_env      = ["PATH=/usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin"]
  image_mount_path      = "${var.image_mount_path}"
  image_partitions {
    filesystem              = "vfat"
    filesystem_make_options = ["-n", "system-boot"]
    mountpoint              = "/boot/firmware"
    name                    = "boot"
    size                    = "1G"
    start_sector            = "2048"
    type                    = "c"
  }
  image_partitions {
    filesystem              = "ext4"
    filesystem_make_options = ["-L", "writable"]
    mountpoint              = "/"
    name                    = "root"
    size                    = "5.2G"
    start_sector            = "2099200"
    type                    = "83"
  }
  image_path                   = "kubernetes.img"
  image_size                   = "5.32G"
  image_type                   = "dos"
  qemu_binary_destination_path = "/usr/bin/qemu-arm-static"
  qemu_binary_source_path      = "/usr/bin/qemu-arm-static"
}

build {
  sources = ["source.arm.autogenerated_1"]

  provisioner "ansible" {
    ansible_env_vars = ["ANSIBLE_RETRY_FILES_ENABLED=0"]
    extra_arguments  = ["-v", "--connection=chroot"]
    inventory_file   = "./kubernetes/ansible/hosts"
    playbook_file    = "./kubernetes/ansible/site.yml"
  }

}
