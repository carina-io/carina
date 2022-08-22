

docker run --name csi-node-register -d -e KUBE_NODE_NAME=10.20.9.61 -v /var/lib/kubelet/plugins/csi.carina.com:/csi \
-v /var/lib/kubelet/plugins_registry/:/registration carina/csi-node-driver-registrar:v2.1.0 \
--v=5 --csi-address=/csi/csi.sock --kubelet-registration-path=/var/lib/kubelet/plugins/csi.carina.com/csi.sock