#### Carina Installation

##### Prerequirement

- Kubernetes cluster with CSI_VERSION = 1.5.0
- If running kubelet in docker container，it should mount host 's `/dev:/dev` directory.
- Linux Kernel 3.10.0-1160.11.1.el7.x86_64
- Each node should have multiple raw disks. Carina will ignore nodes with no empty disks. 

#### Installation using helm

##### helm installation 

```
helm repo add carina-csi-driver https://carina-io.github.io

helm search repo -l carina-csi-driver

helm install carina-csi-driver carina-csi-driver/carina-csi-driver --namespace kube-system --version v0.9.0
```

##### upgrade

```
helm uninstall carina-csi-driver 
helm pull  carina-csi-driver/carina-csi-driver  --version v0.9.1 
tar -zxvf carina-csi-driver-v0.9.1.tgz   
# Edit carina-csi-driver/templates/csi-config-map.yaml to fill the current VG.
helm install carina-csi-driver carina-csi-driver/
```