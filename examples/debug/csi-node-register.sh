

docker run --name csi-node-register -e KUBE_NODE_NAME=10.20.9.154 -v /var/lib/kubelet/plugins/csi.carina.com:/csi \
-v /var/lib/kubelet/plugins_registry/:/registration antmoveh/csi-node-driver-registrar:v2.1.0 \
--v=5 --csi-address=/csi/csi.sock --kubelet-registration-path=/var/lib/kubelet/plugins/csi.carina.com/csi.sock