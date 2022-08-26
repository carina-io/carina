#### Carina Installation

##### Prerequirement

- Kubernetes cluster with CSI_VERSION = 1.5.0
- If running kubelet in docker container，it should mount host 's `/dev:/dev` directory.
- Linux Kernel 3.10.0-1160.11.1.el7.x86_64
- Each node should have multiple raw disks. Carina will ignore nodes with no empty disks. 

##### Installation

- Using `kubectl get pods -n kube-system | grep carina` to check installation status.

  ```shell
  $ cd deploy/kubernetes
  $ ./deploy.sh

  $ kubectl get pods -n kube-system |grep carina
  carina-scheduler-6cc9cddb4b-jdt68         0/1     ContainerCreating   0          3s
  csi-carina-node-6bzfn                     0/2     ContainerCreating   0          6s
  csi-carina-node-flqtk                     0/2     ContainerCreating   0          6s
  csi-carina-provisioner-7df5d47dff-7246v   0/4     ContainerCreating   0          12s
  ```

- Uninstallation

  ```shell
  $ cd deploy/kubernetes
  $ ./deploy.sh uninstall
  ```
* The uninstallation will leave the pod still using carina PV untouched. 

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