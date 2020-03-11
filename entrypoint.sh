#!/bin/bash -e
/usr/sbin/update-binfmts --enable qemu-arm

PACKER=/bin/packer

echo running $PACKER

exec $PACKER "${@}"
