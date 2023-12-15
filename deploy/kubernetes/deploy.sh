#!/bin/bash

function install() {
  echo "install..."
  kubectl label namespace kube-system carina.storage.io/webhook=ignore

  if [ `kubectl get configmap carina-csi-config -n kube-system 2>/dev/null | wc -l` == "0" ]; then
    kubectl apply -f csi-config-map.yaml
  fi

  kubectl apply -f crd-logicvolume.yaml
  kubectl apply -f crd-nodestoreresource.yaml

  kubectl apply -f csi-controller-rbac.yaml
  kubectl apply -f csi-carina-controller.yaml
  kubectl apply -f csi-node-rbac.yaml
  kubectl apply -f csi-carina-node.yaml
  kubectl apply -f carina-scheduler.yaml
  kubectl apply -f storageclass-lvm.yaml
  kubectl apply -f storageclass-raw.yaml
  sleep 3s
  kubectl apply -f prometheus-service-monitor.yaml
  echo "-------------------------------"
  echo "$ kubectl get pods -n kube-system |grep carina"
  kubectl get pods -n kube-system |grep carina
}


function uninstall() {
  echo "uninstall..."
  kubectl delete secret mutatingwebhook -n kube-system
#  kubectl delete -f csi-config-map.yaml
  kubectl delete -f csi-controller-rbac.yaml
  kubectl delete -f csi-carina-controller.yaml
  kubectl delete -f csi-node-rbac.yaml
  kubectl delete -f csi-carina-node.yaml
  kubectl delete -f carina-scheduler.yaml
 
  kubectl delete csr carina-controller.kube-system
  kubectl delete configmap carina-node-storage -n kube-system
  kubectl label namespace kube-system carina.storage.io/webhook-

  for z in `kubectl get nodes | awk 'NR!=1 {print $1}'`; do
    kubectl label node $z topology.carina.storage.io/node-
  done

  if [ `kubectl get lv | wc -l` == 0 ]; then
    kubectl delete -f crd-logicvolume.yaml
  fi
  kubectl delete -f crd-nodestoreresource.yaml
  kubectl delete -f storageclass-lvm.yaml
  kubectl delete -f storageclass-raw.yaml
  kubectl delete -f prometheus-service-monitor.yaml
}

# Does not apply to 0.10.0->0.11.0 updates
function upgrade() {
  echo "upgrade..."
  kubectl delete secret mutatingwebhook -n kube-system
  kubectl delete -f csi-carina-controller.yaml
  kubectl delete -f csi-carina-node.yaml
  kubectl delete -f carina-scheduler.yaml
  kubectl apply -f csi-carina-controller.yaml
  kubectl apply -f csi-carina-node.yaml
  kubectl apply -f carina-scheduler.yaml
}

operator=${1:-'install'}

if [ "uninstall" == $operator ]
then
  uninstall
elif [ "upgrade" == $operator ]
then
  upgrade
else
  install
fi