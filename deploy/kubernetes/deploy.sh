
function install() {
  echo "install..."
  kubectl label namespace kube-system carina.storage.io/webhook=ignore

  chmod +x gen_webhookca.sh && ./gen_webhookca.sh --service carina-controller --namespace kube-system --secret mutatingwebhook

  rm -rf mutatingwebhooks.yaml && cp mutatingwebhooks.yaml.tmpl mutatingwebhooks.yaml
  CA_BUNDLE=$(kubectl config view --raw --minify --flatten -o jsonpath='{.clusters[].cluster.certificate-authority-data}')
  sed -i "s#\${CA_BUNDLE}#${CA_BUNDLE}#g" mutatingwebhooks.yaml

  kubectl apply -f crd.yaml
  kubectl apply -f csi-config-map.yaml
  kubectl apply -f mutatingwebhooks.yaml
  kubectl apply -f csi-controller-psp.yaml
  kubectl apply -f csi-controller-rbac.yaml
  kubectl apply -f csi-carina-controller.yaml
  kubectl apply -f csi-node-psp.yaml
  kubectl apply -f csi-node-rbac.yaml
  kubectl apply -f csi-carina-node.yaml
  kubectl apply -f carina-scheduler.yaml
  sleep 3s
  echo "-------------------------------"
  echo "$ kubectl get pods -n kube-system |grep carina"
  kubectl get pods -n kube-system |grep carina
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
  kubectl delete -f crd.yaml

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