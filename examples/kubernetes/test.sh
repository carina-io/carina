
namespace=carina

function install() {
  echo "install..."
  kubectl apply -f namespace.yaml
  kubectl apply -f storageclass.yaml
  kubectl apply -f pvc.yaml
  kubectl apply -f deployment.yaml
  kubectl apply -f statefulset.yaml
  kubectl apply -f topostatefulset.yaml
  kubectl apply -f lvm-block-pvc.yaml
  kubectl apply -f lvm-block-pod.yaml
  kubectl apply -f sample.yaml
  kubectl apply -f raw-sts-block.yaml
  kubectl apply -f raw-sts-fs.yaml
  kubectl apply -f deploymentspeedlimit.yaml
  kubectl apply -f speedlimit.yaml
  sleep 10s
  echo "-------------------------------"
  echo "$ kubectl get pvc,pods -n $namespace"
  kubectl get pvc,pods -n $namespace
}

function uninstall() {
  echo "uninstall..."
  kubectl delete -f speedlimit.yaml
  kubectl delete -f deploymentspeedlimit.yaml
  kubectl delete -f sample.yaml
  kubectl delete -f deployment.yaml
  kubectl delete -f statefulset.yaml
  kubectl delete -f topostatefulset.yaml
  kubectl delete -f lvm-block-pod.yaml
  kubectl delete -f lvm-block-pvc.yaml
  kubectl delete -f raw-sts-block.yaml
  kubectl delete -f raw-sts-fs.yaml
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
      kubectl exec $z -n $namespace -- sh -c "ls /dev"
      echo "-------------------------------"
      continue
    fi
    echo "$ $z exec df -h -T"
    kubectl exec -it $z -n $namespace -- sh -c "df -h -T" | grep carina
    echo "-------------------------------"
  done
}

function help() {
    echo "-------------------------------"
    echo "./test.sh           ===> install all test yaml"
    echo "./test.sh uninstall ===> uninstall all test yaml"
    echo "./test.sh expand    ===> expand all pvc"
    echo "./test.sh scale     ===> scale statefulset replicas"
    echo "./test.sh delete    ===> delete all pod"
    echo "./test.sh exec      ===> show pod filesystem"
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