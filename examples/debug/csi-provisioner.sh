

docker rm -f csi-provisioner

docker run --name csi-provisioner -d -e KUBECONFIG=/root/.kube/config -v /root/.kube:/root/.kube -v /tmp/csi:/csi:rw antmoveh/csi-provisioner:v2.1.0 \
--csi-address=unix:///csi/csi-provisioner.sock --v=5 --timeout=150s --leader-election=true --retry-interval-start=500ms \
--feature-gates=Topology=false --extra-create-metadata=true