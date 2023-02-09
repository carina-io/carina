carina=`kubectl get pods -A |grep carina-node | wc -l`
if [ $carina != "0" ]; then
  echo "Please uninstall Carina first."
  kubectl get pods -A |grep carina
  exit -1
fi

echo "upgrade logicvolumes crd..."
scope=`kubectl get crd logicvolumes.carina.storage.io -o template --template={{.spec.scope}}`
if [ "$scope" != "Namespaced" ]; then
  echo "done"
  exit 0
fi

if [ `kubectl get logicvolumes.carina.storage.io | wc -l` == "1" ]; then
  kubectl delete crd logicvolumes.carina.storage.io
  echo "done"
  exit 0
fi

kubectl get logicvolumes.carina.storage.io -o json > /tmp/default-lv.json
cp /tmp/default-lv.json /tmp/cluster-lv.json

kubectl get logicvolumes.carina.storage.io | awk 'NR==1{next}{print $1}' | xargs kubectl patch logicvolumes.carina.storage.io -p '{"metadata":{"finalizers":null}}' --type=merge
kubectl get logicvolumes.carina.storage.io | awk 'NR==1{next}{print $1}' | xargs kubectl delete logicvolumes.carina.storage.io

kubectl delete crd logicvolumes.carina.storage.io
sleep 5
kubectl apply -f crd-logicvolume.yaml
kubectl get crd logicvolumes.carina.storage.io

#sed -i "s#.*"default".*#""#g" /tmp/cluster-lv.json
#sed -i "s#.*resourceVersion.*##g" /tmp/cluster-lv.json
#sed -i "s#.*generation.*##g" /tmp/cluster-lv.json
#kubectl apply -f /tmp/cluster-lv.json

kubectl get sc -o json > /tmp/default-sc.json
cp /tmp/default-sc.json /tmp/cluster-sc.json
sed -i 's#"carina.storage.io/disk-type": "hdd"#"carina.storage.io/disk-group-name": "carina-vg-hdd"#g' /tmp/cluster-sc.json
sed -i 's#"carina.storage.io/disk-type": "ssd"#"carina.storage.io/disk-group-name": "carina-vg-ssd"#g' /tmp/cluster-sc.json
kubectl get sc | grep carina.storage.io | awk '{print $1}' | xargs kubectl delete sc
kubectl apply -f /tmp/cluster-sc.json

kubectl get configmap carina-csi-config -n kube-system -o yaml > /tmp/default-config.yaml
disk=`cat /tmp/default-config.yaml | grep -o -m 1 '\[.*\]'`
cat > /tmp/cluster-config.yaml <<- EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: carina-csi-config
  namespace: kube-system
  labels:
    class: carina
data:
  config.json: |-
    {
      "diskSelector": [
        {
          "name": "carina-vg-hdd",
          "re": $disk,
          "policy": "LVM",
          "nodeLabel": "kubernetes.io/hostname"
        },
        {
          "name": "carina-vg-ssd",
          "re": $disk,
          "policy": "LVM",
          "nodeLabel": "kubernetes.io/hostname"
        }
      ],
      "diskScanInterval": "300",
      "schedulerStrategy": "spreadout"
    }
EOF

echo "WARNING !!! Modify the configuration file manually and execute 'kubectl apply -f /tmp/cluster-config.yaml'"
echo "----------------------------"
echo "cat /tmp/cluster-config.yaml"
echo "----------------------------"
cat /tmp/cluster-config.yaml

echo "done"
