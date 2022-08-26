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
kubectl get logicvolumes.carina.storage.io

sed -i "s#.*"default".*#""#g" /tmp/cluster-lv.json
sed -i "s#.*resourceVersion.*##g" /tmp/cluster-lv.json
sed -i "s#.*generation.*##g" /tmp/cluster-lv.json
kubectl apply -f /tmp/cluster-lv.json

echo "done"