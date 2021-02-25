

kubectl delete secret mutatingwebhook
kubectl delete -f csi-config-map.yaml
kubectl delete -f mutatingwebhooks.yaml
kubectl delete -f csi-controller-psp.yaml
kubectl delete -f csi-controller-rbac.yaml
kubectl delete -f csi-carina-controller.yaml
kubectl delete -f csi-node-psp.yaml
kubectl delete -f csi-node-rbac.yaml
kubectl delete -f csi-carina.node.yaml

kubectl label namespace kube-system carina.storage.io/webhook-