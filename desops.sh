#!/usr/bin/env bash

for host_directory in hosts/*/; do
  sops -d ${host_directory}/user_data.encrypted >${host_directory}/user_data
done
