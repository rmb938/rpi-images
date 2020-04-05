#!/bin/bash
set -euxo pipefail

if [ -e /var/lib/kubelet/config.yaml ]; then
  echo "Kubernetes Node already has kubelet configuration"
  exit 1
fi

kubeadm_flags=''

if [ "${KUBEADM_COMMAND}" == "init" ]; then
  echo "Checking for Backup"
  gcloud auth activate-service-account --key-file=/usr/local/share/kubernetes/gcp-sa.json

  LAST_BACKUP=$(gsutil ls -l gs://${KUBEADM_CLUSTER_NAME}.backups.buckets.rmb938.me/ | head -n -1 | sort -k 2 | tail -n 1 | awk '{$1=$1};1' | cut -d ' ' -f3)

  if [ ! -z "${LAST_BACKUP}" ]; then
    echo "Found Backup"
    tmp_dir=$(mktemp -d -t etcd-backup-XXXXXXXXXX)
    echo "Downloading Backup"
    gsutil cp ${LAST_BACKUP} ${tmp_dir}/backup.tar.gz

    echo "Extracting Backup"
    tar -zxvf ${tmp_dir}/backup.tar.gz -C ${tmp_dir}

    echo "Restoring Kubernetes Certs"
    cp -r ${tmp_dir}/pki/ /etc/kubernetes/pki/

    echo "Restoring ETCD Backup"
    ETCDCTL_API=3
    HOSTNAME=$(hostname -f)
    etcdctl snapshot restore ${tmp_dir}/snapshot.db \
      --data-dir /var/lib/etcd \
      --name=${HOSTNAME} \
      --initial-cluster ${HOSTNAME}=https://${HOSTNAME}:2380 \
      --initial-advertise-peer-urls https://${HOSTNAME}:2380

    rm -rf ${tmp_dir}

    kubeadm_flags='--ignore-preflight-errors=DirAvailable--var-lib-etcd'
  else
    echo "Did not find"
  fi

  echo "Initing Kubernetes Cluster"
elif [ "${KUBEADM_COMMAND}" == "join" ]; then
  echo "Joining Kubernetes Cluster"
else
  echo "Unknown kubeadm command ${KUBEADM_COMMAND}"
  exit 1
fi

echo "Running kubeadm"
kubeadm ${KUBEADM_COMMAND} ${kubeadm_flags} --config /etc/kubernetes/kubeadm.yaml

echo "kubeadm has completed"
