
function install() {
  echo "install..."
  kubectl apply -f namespace.yaml
  kubectl apply -f storageclass.yaml
  kubectl apply -f pvc.yaml
  kubectl apply -f deployment.yaml
  kubectl apply -f statefulset.yaml
  kubectl apply -f topostatefulset.yaml
  kubectl apply -f raw-block-pvc.yaml
  kubectl apply -f raw-block-pod.yaml
  sleep 10s
  echo "-------------------------------"
  echo "kubectl get pvc,pods -n carina"
  kubectl get pvc,pods -n carina
}

function uninstall() {
  echo "uninstall..."
  kubectl delete -f deployment.yaml
  kubectl delete -f statefulset.yaml
  kubectl delete -f topostatefulset.yaml
  kubectl delete -f raw-block-pod.yaml
  kubectl delete -f raw-block-pvc.yaml
  kubectl delete -f pvc.yaml
  kubectl delete -f storageclass.yaml
  kubectl delete -f namespace.yaml
  echo "-------------------------------"
  echo "kubectl get pv"
  kubectl get pv
}

operator=${1:-'install'}

if [ "uninstall" == $operator ]
then
  uninstall
else
  install
fi