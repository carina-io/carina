## 准备环境
- Kubernetes：(CSI_VERSION=1.5.0)
- Node OS：Linux
- Filesystems：ext4，xfs

- 如果kubelet以容器化方式运行，需要挂载主机/dev:/dev目录
- 集群每个节点存在1..N块裸盘，支持SSD和HDD磁盘（可使用命令lsblk --output NAME,ROTA查看磁盘类型，ROTA=1为HDD磁盘 ROTA=0为SSD磁盘）
- 节点单块裸盘容量需要大于10G
- 确保Webhook启用了 MutatingAdmissionWebhook 

## 准备模拟测试磁盘，如果已经有了磁盘和环境忽略此步骤

- 在节点dev1-node-2.novalocal，dev1-node-3.novalocal执行如下方法创建`loop device` 

```shell
for i in $(seq 1 5); do
truncate --size=200G /tmp/disk$i.device && \
losetup -f /tmp/disk$i.device
done
  ```
## 查看集群是否开启webhook MutatingAdmissionWebhook 

```
kubectl -n kube-system get pods $(kubectl get pods -n kube-system |grep api | awk '{print$1}') -oyaml |grep enable-admission-plugins
```
```
- --enable-admission-plugins=NodeRestriction,MutatingAdmissionWebhook,ValidatingAdmissionWebhook
```  

## 安装，如果已经安装并正常使用忽略此步骤

```
helm repo add carina-csi-driver https://carina-io.github.io
helm repo update
helm search repo -l carina-csi-driver
helm install carina-csi-driver carina-csi-driver/carina-csi-driver --namespace kube-system --version v0.9.1
```

- 设置storageclass  查看默认存储磁盘组是否满足匹配磁盘

```
allowVolumeExpansion: true
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  annotations:
    meta.helm.sh/release-name: carina-csi-driver
    meta.helm.sh/release-namespace: kube-system
  creationTimestamp: "2022-01-24T02:26:12Z"
  labels:
    app.kubernetes.io/managed-by: Helm
  name: csi-carina-sc
  resourceVersion: "50310602"
  uid: 442eeaf0-4d97-44d1-a526-1ddb80a01c27
parameters:
  carina.storage.io/disk-group-name: hdd
  csi.storage.k8s.io/fstype: xfs
provisioner: carina.storage.io
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
```
-  默认磁盘组匹配条件如下，可根据实际节点磁盘自行修改
```
 {
      "diskSelector": [
        {
          "name": "carina-vg-ssd" ,
          "re": ["loop2+"],
          "policy": "LVM",
          "nodeLabel": "kubernetes.io/hostname"
        },
        {
          "name": "carina-vg-hdd",
          "re": ["loop3+"],
          "policy": "LVM",
          "nodeLabel": "kubernetes.io/hostname"
        }
      ],
      "diskScanInterval": "300",
      "schedulerStrategy": "spreadout"
    }

```
## 本次实验环境如下,节点2,3 上挂有模拟磁盘
- kubectl get nodes 
```
NAME                    STATUS     ROLES                  AGE    VERSION
dev1-master.novalocal   Ready      control-plane,master   191d   v1.21.8
dev1-node-1.novalocal   Ready      node                   191d   v1.21.8
dev1-node-2.novalocal   Ready      node                   191d   v1.21.8
dev1-node-3.novalocal   Ready      node                   191d   v1.21.8
dev1-node-4.novalocal   NotReady   <none>                 19d    v1.21.8
dev1-node-5.novalocal   Ready      <none>                 19d    v1.21.8

```

- kubectl get  nodes dev1-node-1.novalocal -oyaml |grep allocatable -A 5

```
allocatable:
    carina.storage.io/carina-vg-hdd: "0"
    carina.storage.io/carina-vg-ssd: "0"
    carina.storage.io/carina-vg-test-ssd: "0"
    carina.storage.io/carina-vg-test-vdd: "0"
    cpu: "8"

```
- kubectl get  nodes dev1-node-2.novalocal -oyaml |grep allocatable -A 5

```
allocatable:
    carina.storage.io/carina-vg-hdd: "188"
    carina.storage.io/carina-vg-ssd: "189"
    carina.storage.io/carina-vg-test-ssd: "0"
    carina.storage.io/carina-vg-test-vdd: "0"
    cpu: "8"

```
- kubectl get  nodes dev1-node-3.novalocal -oyaml |grep allocatable -A 5

```
allocatable:
    carina.storage.io/carina-vg-hdd: "188"
    carina.storage.io/carina-vg-ssd: "189"
    carina.storage.io/carina-vg-test-ssd: "0"
    carina.storage.io/carina-vg-test-vdd: "0"
    cpu: "8"

```
- kubectl get  nodes dev1-node-4.novalocal -oyaml |grep allocatable -A 5

```
allocatable:
    carina.storage.io/carina-vg-hdd: "0"
    carina.storage.io/carina-vg-ssd: "0"
    carina.storage.io/carina-vg-test-ssd: "0"
    carina.storage.io/carina-vg-test-vdd: "0"
    cpu: "8"
```
- kubectl get  nodes dev1-node-5.novalocal -oyaml |grep allocatable -A 5

```
allocatable:
    carina.storage.io/carina-vg-hdd: "0"
    carina.storage.io/carina-vg-ssd: "0"
    carina.storage.io/carina-vg-test-ssd: "0"
    carina.storage.io/carina-vg-test-vdd: "0"
    cpu: "8"

```
## 测试存储是否运行正常，如果已经安装并正常使用忽略此步骤
-  下载示例文件
```
wget   https://mirror.ghproxy.com/https://github.com/carina-io/carina/blob/main/examples/kubernetes/namespace.yaml
wget   https://mirror.ghproxy.com/https://github.com/carina-io/carina/blob/main/examples/kubernetes/pvc.yaml
wget   https://mirror.ghproxy.com/https://github.com/carina-io/carina/blob/main/examples/kubernetes/deployment.yaml
```
-  运行示例demo
```
kubectl apply -f namespace.yaml -f pvc.yaml -f deployment.yaml
kubectl get pods -n carina -w 
```
```
ubuntu@LAPTOP-4FT6HT3J:~/www/src/github.com/carina-io/carina$ kubectl get pods -n carina 
NAME                                 READY   STATUS    RESTARTS   AGE
carina-deployment-7cd5cd85c9-mstwj   1/1     Running   0          14m
```
```
kubectl  delete -f pvc.yaml -f deployment.yaml
```

## 到此实验环境检测正常，安装mysql中间件示例 参考这个项目示例[https://github.com/bitpoke/mysql-operator]

-  安装operator

```
helm repo add bitpoke https://helm-charts.bitpoke.io
helm search repo -l mysql
```
- 设置参数persistence.enabled=fasle，关闭operator使用存储
```
helm install mysql-operator bitpoke/mysql-operator --namespace carina --version 0.6.2 --set orchestrator.persistence.enabled=false
kubectl get pods -n carina -w 

NAME                   READY   STATUS    RESTARTS   AGE     IP             NODE                    NOMINATED NODE   READINESS GATES
minio-dd87d97d-n4r96   1/1     Running   9          19d     10.245.1.136   dev1-node-1.novalocal   <none>           <none>
mysql-operator-0       2/2     Running   0          2m32s   10.245.5.130   dev1-node-5.novalocal   <none>           <none>

```

-  下载示例文件
```
wget https://mirror.ghproxy.com/https://github.com/bitpoke/mysql-operator/blob/master/examples/example-cluster.yaml
wget https://mirror.ghproxy.com/https://github.com/bitpoke/mysql-operator/blob/master/examples/example-cluster-secret.yaml
```

-  获取storageclass，设置数据存储
kubectl get sc 

```
NAME                 PROVISIONER             RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
csi-carina-sc        carina.storage.io       Delete          WaitForFirstConsumer   true                   65m
```
- 为了方便演示这里修改副本为 1
- 编辑example-cluster.yaml，挂载存储
```
  volumeSpec:
    persistentVolumeClaim:
      accessModes: [ "ReadWriteOnce" ]
      storageClassName: "csi-carina-sc"
      resources:
        requests:
          storage: 1Gi
```
- 部署集群
kubectl apply -f example-cluster-secret.yaml -f example-cluster.yaml -n carina 
```
secret/my-secret created
mysqlcluster.mysql.presslabs.org/my-cluster created
```
- kubectl get pods -n carina  -o wide 
```
NAME                 READY   STATUS    RESTARTS   AGE   IP            NODE                    NOMINATED NODE   READINESS GATES
my-cluster-mysql-0   4/4     Running   0          10m   10.245.2.76   dev1-node-2.novalocal   <none>           <none>

```
-  kubectl get nodes 
```
dev1-master.novalocal   Ready      control-plane,master   190d   v1.21.8
dev1-node-1.novalocal   Ready      node                   190d   v1.21.8
dev1-node-2.novalocal   Ready      node                   190d   v1.21.8
dev1-node-3.novalocal   Ready      node                   190d   v1.21.8
dev1-node-4.novalocal   NotReady   <none>                 19d    v1.21.8
dev1-node-5.novalocal   Ready      <none>                 19d    v1.21.8

```

- 模拟pod所在节点故障，pods 迁移过程, 关闭节点dev1-node-2.novalocal 或者执行 systemctl stop kubelet 
```
[root@dev1-master zhangkai]# kubectl get nodes -w 
NAME                    STATUS     ROLES                  AGE    VERSION
dev1-master.novalocal   Ready      control-plane,master   190d   v1.21.8
dev1-node-1.novalocal   Ready      node                   190d   v1.21.8
dev1-node-2.novalocal   Ready      node                   190d   v1.21.8
dev1-node-3.novalocal   Ready      node                   190d   v1.21.8
dev1-node-4.novalocal   NotReady   <none>                 19d    v1.21.8
dev1-node-5.novalocal   Ready      <none>                 19d    v1.21.8
dev1-node-1.novalocal   Ready      node                   190d   v1.21.8
dev1-master.novalocal   Ready      control-plane,master   190d   v1.21.8
dev1-node-2.novalocal   NotReady   node                   190d   v1.21.8
dev1-node-2.novalocal   NotReady   node                   190d   v1.21.8


```
- 节点故障后，故障节点pods 继续运行，kube-controller-manager的节点控制器部分等待pod-eviction-timeout**（默认设置为5分钟），以确保在计划将pod删除之前完全无法访问该节点。在pod逐出超时时间间隔（在本例中为5分钟）之后，节点控制器将在分区节点上运行的pod调度为**Termination**状态。kube-controller-manager的Deployment Controller部分开始在其他的节点上创建新的副本replicas和调度schedules；但是如果是Statefulset 则会出现令我们惊讶的是，没有为Statefulsets创建新的pod。
- kubectl get pods -o wide  -n carina -w
```

NAME                 READY   STATUS    RESTARTS   AGE   IP            NODE                    NOMINATED NODE   READINESS GATES
my-cluster-mysql-0   4/4     Running   0          35m   10.245.2.76   dev1-node-2.novalocal   <none>           <none>
my-cluster-mysql-0   4/4     Running   0          36m   10.245.2.76   dev1-node-2.novalocal   <none>           <none>
my-cluster-mysql-0   4/4     Terminating   0          41m   10.245.2.76   dev1-node-2.novalocal   <none>           <none>

```
- 这是因为在节点故障的情况下，主节点没有足够的信息来确定该节点实际上是故障还是故障是由于网络分区引起的。因此，主机拒绝采取任何措施，从而导致更多问题。主机采用一种实际的方法，减少了一个实例，但以一种可靠的方式工作。如果您确定节点确实发生故障或被删除，则可以采用一种自动的方法来检测节点故障并强行删除这些节点。这将确保在可用节点上重新启动有状态集的容器。

- 这里我们使用carina的存储就会避免这种情况，帮我们去处理了，首选恢复环境
```
dev1-master.novalocal   Ready      control-plane,master   191d   v1.21.8
dev1-node-1.novalocal   Ready      node                   191d   v1.21.8
dev1-node-2.novalocal   Ready      node                   191d   v1.21.8
dev1-node-3.novalocal   Ready      node                   191d   v1.21.8
dev1-node-4.novalocal   NotReady   <none>                 19d    v1.21.8
dev1-node-5.novalocal   Ready      <none>                 19d    v1.21.8
```
- kubectl get pods -n carina -o wide 
```
NAME                 READY   STATUS    RESTARTS   AGE   IP            NODE                    NOMINATED NODE   READINESS GATES
my-cluster-mysql-0   2/4     Running   0          43s   10.245.2.28   dev1-node-2.novalocal   <none>           <none>

```
- 给pods 添加注解: true
- 编辑example-cluster.yaml，设置注解
```
podSpec:
     annotations:
     carina.storage.io/allow-pod-migration-if-node-notready : "true"
```
-  kubectl apply -f example-cluster.yaml  -f example-cluster-secret.yaml -n carina 
```
mysqlcluster.mysql.presslabs.org/my-cluster created
secret/my-secret created
```y-cluster-mysql-0 annotated
```
- kubectl get pods my-cluster-mysql-0 -n carina -oyaml |grep allow-pod-migration-if-node-notready
```

carina.storage.io/allow-pod-migration-if-node-notready: "true"
```
-  关闭节点dev1-node-2.novalocal 或者执行 systemctl stop kubelet 
```
dev1-master.novalocal   Ready      control-plane,master   191d   v1.21.8
dev1-node-1.novalocal   Ready      node                   191d   v1.21.8
dev1-node-2.novalocal   Ready      node                   191d   v1.21.8
dev1-node-3.novalocal   Ready      node                   191d   v1.21.8
dev1-node-4.novalocal   NotReady   <none>                 19d    v1.21.8
dev1-node-5.novalocal   Ready      <none>                 19d    v1.21.8
dev1-node-2.novalocal   NotReady   node                   191d   v1.21.8
dev1-node-2.novalocal   NotReady   node                   191d   v1.21.8
dev1-node-2.novalocal   NotReady   node                   191d   v1.21.8

```
```
NAME                 READY   STATUS    RESTARTS   AGE   IP            NODE                    NOMINATED NODE   READINESS GATES
my-cluster-mysql-0   4/4     Running   0          23m   10.245.2.65   dev1-node-2.novalocal   <none>           <none>
my-cluster-mysql-0   4/4     Running   0          23m   10.245.2.65   dev1-node-2.novalocal   <none>           <none>
my-cluster-mysql-0   4/4     Terminating   0          29m   10.245.2.65   dev1-node-2.novalocal   <none>           <none>

```
- 如果这里在其他节点正常创建了，那么就不用处理以下情况了
- 问题：怎么没有创建呢，这是因为csi-carina-controller 正好也是在故障节点上，这个时候等待k8s去驱逐，在新的节点上重建
```
root@dev1-master zhangkai]# kubectl get pods -n kube-system -o wide  |grep carina  
carina-csi-driver-carina-scheduler-c9f5df55b-6bhx4   1/1     Running            0          85s     10.245.5.195   dev1-node-5.novalocal   <none>           <none>
carina-csi-driver-carina-scheduler-c9f5df55b-lj6km   1/1     Terminating        0          8m54s   10.245.2.152   dev1-node-2.novalocal   <none>           <none>
csi-carina-controller-597b8c546-l7bzm                4/4     Running            0          85s     10.245.0.45    dev1-node-3.novalocal   <none>           <none>
csi-carina-controller-597b8c546-plcrh                4/4     Terminating        0          8m54s   10.245.2.117   dev1-node-2.novalocal   <none>           <none>
csi-carina-node-2hw9q                                2/2     Terminating        0          5h2m    10.245.4.9     dev1-node-4.novalocal   <none>           <none>
csi-carina-node-5b82h                                2/2     Running            0          5h2m    10.245.5.60    dev1-node-5.novalocal   <none>           <none>
csi-carina-node-9kl9z                                2/2     Running         0          8m39s   <none>         dev1-master.novalocal   <none>           <none>
csi-carina-node-dh6l2                                2/2     Running            0          5h2m    10.245.0.234   dev1-node-3.novalocal   <none>           <none>
csi-carina-node-ww24j                                2/2     Running            0          8m14s   10.245.2.128   dev1-node-2.novalocal   <none>           <none>
csi-carina-node-zwlcq                                2/2     Running            6          8m48s   10.245.1.215   dev1-node-1.novalocal   <none>           <none>

```

- 如何处理呢
- 一种是 创建csi-carina-controller 多副本可以避免单节点故障
- 或者比较被动的方式等待控制器启动后5分钟自行处理
- 还可以需要手动删除 kubectl delete pods my-cluster-mysql-0 -n carina --force 


```
my-cluster-mysql-0   4/4     Running   0          23m   10.245.2.65   dev1-node-2.novalocal   <none>           <none>
my-cluster-mysql-0   4/4     Running   0          23m   10.245.2.65   dev1-node-2.novalocal   <none>           <none>
my-cluster-mysql-0   4/4     Terminating   0          29m   10.245.2.65   dev1-node-2.novalocal   <none>           <none>
my-cluster-mysql-0   4/4     Terminating   0          39m   10.245.2.65   dev1-node-2.novalocal   <none>           <none>
my-cluster-mysql-0   4/4     Terminating   0          39m   10.245.2.65   dev1-node-2.novalocal   <none>           <none>
my-cluster-mysql-0   0/4     Pending       0          0s    <none>        <none>                  <none>           <none>
my-cluster-mysql-0   0/4     Pending       0          1s    <none>        <none>                  <none>           <none>
my-cluster-mysql-0   0/4     Pending       0          13m   <none>        dev1-node-3.novalocal   <none>           <none>
my-cluster-mysql-0   0/4     Init:0/2      0          13m   <none>        dev1-node-3.novalocal   <none>           <none>
my-cluster-mysql-0   0/4     Init:0/2      0          13m   <none>        dev1-node-3.novalocal   <none>           <none>
my-cluster-mysql-0   0/4     Init:0/2      0          13m   10.245.0.48   dev1-node-3.novalocal   <none>           <none>
my-cluster-mysql-0   0/4     Init:1/2      0          13m   10.245.0.48   dev1-node-3.novalocal   <none>           <none>
my-cluster-mysql-0   0/4     Init:1/2      0          13m   10.245.0.48   dev1-node-3.novalocal   <none>           <none>
my-cluster-mysql-0   2/4     Running           0          14m   10.245.0.48   dev1-node-3.novalocal   <none>           <none>
my-cluster-mysql-0   2/4     Running           0          14m   10.245.0.48   dev1-node-3.novalocal   <none>           <none>
my-cluster-mysql-0   3/4     Running           0          14m   10.245.0.48   dev1-node-3.novalocal   <none>           <none>
my-cluster-mysql-0   4/4     Running           0          14m   10.245.0.48   dev1-node-3.novalocal   <none>           <none>

```
- 到这里测试完毕了
