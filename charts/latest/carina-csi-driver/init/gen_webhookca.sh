#!/bin/bash

set -e

usage() {
    cat <<EOF
Generate certificate suitable for use with an sidecar-injector webhook service.
This script uses k8s' CertificateSigningRequest API to a generate a
certificate signed by k8s CA suitable for use with sidecar-injector webhook
services. This requires permissions to create and approve CSR. See
https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster for
detailed explanation and additional instructions.
The server key/cert k8s CA cert are stored in a k8s secret.
usage: ${0} [OPTIONS]
The following flags are required.
       --service          Service name of webhook.
       --namespace        Namespace where webhook service and secret reside.
       --secret           Secret name for CA certificate and server certificate/key pair.
EOF
    exit 1
}

while [[ $# -gt 0 ]]; do
    case ${1} in
        --service)
            service="$2"
            shift
            ;;
        --secret)
            secret="$2"
            shift
            ;;
        --namespace)
            namespace="$2"
            shift
            ;;
        *)
            usage
            ;;
    esac
    shift
done

[ -z "${service}" ] && service=sidecar-injector-webhook-svc
[ -z "${secret}" ] && secret=sidecar-injector-webhook-certs
[ -z "${namespace}" ] && namespace=default

if [ ! -x "$(command -v openssl)" ]; then
    echo "openssl not found"
    exit 1
fi

csrName=${service}.${namespace}
<<<<<<< HEAD
tmpdir="/"
echo "creating certs in tmpdir ${tmpdir} "
kubectl get pods 
mkdir test 
touch abc.txt

=======
tmpdir="./"
echo "creating certs in tmpdir ${tmpdir} "
>>>>>>> 2759e6d... add disk support mutil vg group
cat <<EOF >> csr.conf
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${service}
DNS.2 = ${service}.${namespace}
DNS.3 = ${service}.${namespace}.svc
EOF

<<<<<<< HEAD
=======

>>>>>>> 2759e6d... add disk support mutil vg group
openssl genrsa -out server-key.pem 2048
openssl req -new -key server-key.pem -subj "/CN=${service}.${namespace}.svc" -days 36500 -out server.csr -config csr.conf

# clean-up any previously created CSR for our service. Ignore errors if not present.
kubectl delete csr ${csrName} 2>/dev/null || true
<<<<<<< HEAD

kubeVersion=$(kubectl version -oyaml   |grep serverVersion -A 10  |awk '/gitVersion:/{print$2}' | awk -F'v' '{print $2}')
if [ ${kubeVersion} -gt 1.20 ]; then
=======
kubeVersion=$(kubectl version --output=yaml  |grep serverVersion -A 10  |awk '/gitVersion:/{print$2}' | awk -F'v' '{print $2}')
# create  server cert/key CSR and  send to k8s API
if [[ ${kubeVersion} > 1.20.0 ]]; then
>>>>>>> 2759e6d... add disk support mutil vg group
cat <<EOF | kubectl create -f -
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: ${csrName}
spec:
  groups:
  - system:authenticated
  request: $(< server.csr base64 | tr -d '\n')
  signerName: kubernetes.io/kube-apiserver-client
  usages:
  - digital signature
  - key encipherment
  - client auth
EOF
else
cat <<EOF | kubectl create -f -
apiVersion: certificates.k8s.io/v1beta1
kind: CertificateSigningRequest
metadata:
  name: ${csrName}
spec:
  groups:
  - system:authenticated
  request: $(< server.csr base64 | tr -d '\n')
  usages:
  - digital signature
  - key encipherment
  - server auth
EOF
fi
<<<<<<< HEAD
# create  server cert/key CSR and  send to k8s API
=======

>>>>>>> 2759e6d... add disk support mutil vg group


# verify CSR has been created
while true; do
    kubectl get csr ${csrName}
    if [ "$?" -eq 0 ]; then
        break
    fi
done

# approve and fetch the signed certificate
kubectl certificate approve ${csrName}
# verify certificate has been signed
for _ in $(seq 10); do
    serverCert=$(kubectl get csr ${csrName} -o jsonpath='{.status.certificate}')
    if [[ ${serverCert} != '' ]]; then
        break
    fi
    sleep 1
done
if [[ ${serverCert} == '' ]]; then
    echo "ERROR: After approving csr ${csrName}, the signed certificate did not appear on the resource. Giving up after 10 attempts." >&2
    exit 1
fi
echo "${serverCert}" | openssl base64 -d -A -out "${tmpdir}"/server-cert.pem

<<<<<<< HEAD

# create the secret with CA cert and server cert/key
kubectl create secret generic ${secret} \
        --from-file=tls.key=server-key.pem \
        --from-file=tls.crt=server-cert.pem \
        --dry-run=client -o yaml |
    kubectl -n ${namespace} apply -f -


cat <<EOF | kubectl create -f -
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: carina-hook
webhooks:
  - name: pod-hook.carina.storage.io
    namespaceSelector:
      matchExpressions:
      - key: carina.storage.io/webhook
        operator: NotIn
        values: ["ignore"]
    clientConfig:
      caBundle:$(kubectl config view --raw --minify --flatten -o jsonpath='{.clusters[].cluster.certificate-authority-data}')
      service:
        name: carina-controller
        namespace: kube-system
        path: /pod/mutate
        port: 443
    failurePolicy: Ignore
    matchPolicy: Exact
    objectSelector: {}
    reinvocationPolicy: Never
    rules:
      - operations: ["CREATE"]
        apiGroups: [""]
        apiVersions: ["v1"]
        resources: ["pods"]
    admissionReviewVersions: ["v1beta1"]
    sideEffects: NoneOnDryRun
    timeoutSeconds: 30
EOF

# verify MutatingWebhookConfiguration has been created
while true; do
    kubectl get MutatingWebhookConfiguration carina-hook
    if [ "$?" -eq 0 ]; then
        break
    fi
done
=======
kubectl delete secret ${secret} 2>/dev/null || true
# run create the secret with CA cert and server cert/key
kubectl create secret generic ${secret} --from-file=tls.key=server-key.pem --from-file=tls.crt=server-cert.pem -n ${namespace}
>>>>>>> 2759e6d... add disk support mutil vg group
