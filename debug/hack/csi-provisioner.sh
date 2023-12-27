#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

#cert
sudo mkdir -p /tmp/k8s-webhook-server/serving-certs
sudo cp hack/certs/* /tmp/k8s-webhook-server/serving-certs
sudo chmod -R 775 /tmp/k8s-webhook-server/serving-certs
ls /tmp/k8s-webhook-server/serving-certs
#csi
sudo mkdir -p /tmp/csi
sudo touch /tmp/csi/csi-provisioner.sock
sudo chmod -R 775 /tmp/csi

#config 
sudo mkdir -p  /etc/carina/
sudo cp hack/config.json /etc/carina/
sudo chmod -R 775 /etc/carina/

#node
sudo mkdir -p /dev/carina
sudo chmod 775 /dev/carina

#docker
docker rm -f csi-provisioner

docker run --name csi-provisioner -d -e KUBECONFIG=/root/.kube/config -v /root/.kube:/root/.kube -v /tmp/csi:/csi:rw carina/csi-provisioner:v3.4.1 \
--csi-address=unix:///csi/csi-provisioner.sock --v=5 --timeout=150s --leader-election=true --retry-interval-start=500ms \
--feature-gates=Topology=true --extra-create-metadata=true

docker rm -f csi-resizer

docker run --name csi-resizer -d -v /root/.kube:/root/.kube -v /tmp/csi:/csi:rw carina/csi-resizer:v1.7.0 \
--csi-address=unix:///csi/csi-provisioner.sock --v=5 --timeout=150s --leader-election=true --retry-interval-start=500ms \
--handle-volume-inuse-error=false --kubeconfig=/root/.kube/config