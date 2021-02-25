
kubectl lable namespace kube-system carina.storage.io/webhook=ignore

./gen_webhookca.sh --service carina-controller --namespace kube-system --secret mutatingwebhook

rm -rf mutatingwebhooks.yaml && cp mutatingwebhooks.yaml.tmpl mutatingwebhooks.yaml
CA_BUNDLE=$(kubectl config view --raw --minify --flatten -o jsonpath='{.clusters[].cluster.certificate-authority-data}')
sed -i "s#\${CA_BUNDLE}#${CA_BUNDLE}#g" mutatingwebhooks.yaml

kubectl apply -f csi-config-map.yaml
kubectl apply -f mutatingwebhooks.yaml
kubectl apply -f csi-controller-psp.yaml
kubectl apply -f csi-controller-rbac.yaml
kubectl apply -f csi-carina-controller.yaml
kubectl apply -f csi-node-psp.yaml
kubectl apply -f csi-node-rbac.yaml
kubectl apply -f csi-carina.node.yaml