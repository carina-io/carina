
namespace=carina

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
  echo "$ kubectl get pvc,pods -n $namespace"
  kubectl get pvc,pods -n $namespace
}

function uninstall() {
  echo "uninstall..."
  kubectl delete -f deployment.yaml
  kubectl delete -f statefulset.yaml
  kubectl delete -f topostatefulset.yaml
  kubectl delete -f raw-block-pod.yaml
  kubectl delete -f raw-block-pvc.yaml
  kubectl delete -f pvc.yaml
  kubectl delete -f namespace.yaml
  kubectl delete -f storageclass.yaml
  echo "-------------------------------"
  echo "$ kubectl get pv"
  kubectl get pv
}

function expand() {
  kubectl get pvc -n $namespace
  for z in `kubectl get pvc -n $namespace | awk 'NR!=1 {print $1}'`; do
    s=`kubectl get -o template pvc/$z -n $namespace --template={{.spec.resources.requests.storage}}`
    es=`expr ${s%Gi} \* 2`
    echo " ---- patch $z new size ${es}Gi --- "
    kubectl patch pvc $z -n $namespace -p '{ "spec": { "resources": { "requests": { "storage": "'${es}'Gi" }}}}'
  done
  sleep 5s
  echo "-------------------------------"
  echo "$ kubectl get pvc -n $namespace -w"
  kubectl get pvc -n $namespace -w
}

function scale() {
  echo "$ kubectl scale statefulset xxx --replicas=3"
  kubectl scale statefulset carina-stateful -n $namespace --replicas=3
  kubectl scale statefulset carina-topo-stateful -n $namespace --replicas=5
  sleep 5s
  echo "-------------------------------"
  echo "$ kubectl get pods -n $namespace -w"
  kubectl get pods -n $namespace -w
}

function delete() {
  for z in `kubectl get pods -n carina | awk 'NR!=1 {print $1}'`; do
    echo "--- delete pod $z ---"
    kubectl delete pod $z -n $namespace
  done
  sleep 5s
  echo "-------------------------------"
  echo "$ kubectl get pods -n $namespace -w"
  kubectl get pods -n $namespace -w
}

function exec() {
  for z in `kubectl get pods -n carina | grep Running | awk '{print $1}'`; do
    if [ "carina-block-pod" == $z ]; then
      echo "$ $z exec ls /dev"
      kubectl exec -it $z -n $namespace -- sh -c "ls /dev"
      echo "-------------------------------"
      continue
    fi
    echo "$ $z exec df -h"
    kubectl exec -it $z -n $namespace -- sh -c "df -h" | grep carina
    echo "-------------------------------"
  done
}

function help() {
    echo "-------------------------------"
    echo "./run.sh           ===> install all test yaml"
    echo "./run.sh uninstall ===> uninstall all test yaml"
    echo "./run.sh expand    ===> expand all pvc"
    echo "./run.sh scale     ===> scale stateful replicas"
    echo "./run.sh delete    ===> delete all pod"
}

operator=${1:-'install'}

if [ "uninstall" == $operator ]; then
  uninstall
elif [ "expand" == $operator ]; then
  expand
elif [ "scale" == $operator ]; then
  scale
elif [ "delete" == $operator ]; then
  delete
elif [ "exec" == $operator ]; then
  exec
elif [ "help" == $operator ]; then
  help
else
  install
fi