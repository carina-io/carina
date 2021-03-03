
function install() {
  echo "test..."
  kubectl apply -f namespace.yaml
  kubectl apply -f storageclass.yaml
  kubectl apply -f pvc.yaml
  kubectl apply -f deployment.yaml
  kubectl apply -f statefulset.yaml
  kubectl apply -f topostatefulset.yaml
  kubectl apply -f raw-block-pvc.yaml
  kubectl apply -f raw-block-pod.yaml
  echo "wait..."
  kubectl get lv
  kubectl get pvc, pod -n carina
  sleep 30
  echo ""

}


function uninstall() {
  echo "uninstall..."
  kubectl delete secret mutatingwebhook -n kube-system
  kubectl delete -f csi-config-map.yaml
  kubectl delete -f mutatingwebhooks.yaml
  kubectl delete -f csi-controller-psp.yaml
  kubectl delete -f csi-controller-rbac.yaml
  kubectl delete -f csi-carina-controller.yaml
  kubectl delete -f csi-node-psp.yaml
  kubectl delete -f csi-node-rbac.yaml
  kubectl delete -f csi-carina-node.yaml
  kubectl delete -f carina-scheduler.yaml

  kubectl delete csr carina-controller.kube-system
  kubectl delete configmap carina-node-storage -n kube-system
  kubectl label namespace kube-system carina.storage.io/webhook-
}

operator=${1:-'install'}

if [ "uninstall" == $operator ]
then
  uninstall
else
  install
fi