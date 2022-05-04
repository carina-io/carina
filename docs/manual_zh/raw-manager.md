### 部署物配置

 当我们部署carina时需要一个配置文件，该配置文件以configmap形式创建到K8S集群中，内容如下：

```yaml

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
          "name": "carina-vg-ssd" ,
          "re": ["loop2+"],
          "policy": "LVM",
          "nodeLabel": "kubernetes.io/hostname"
        },
        {
          "name": "carina-raw-ssd",
          "re": ["loop3+"],
          "policy": "RAW",
          "nodeLabel": "kubernetes.io/hostname"
        },
         {
          "name": "carina-raw-loop", 
          "re": ["loop4","loop5"], # 磁盘匹配策略，支持正则表达式
          "policy": "RAW",         # 磁盘使用方式
          "nodeLabel": "kubernetes.io/hostname" # 节点标签
        }
      ],
      "diskScanInterval": "300", # 300s 磁盘扫描间隔，0表示关闭本地磁盘扫描
      "schedulerStrategy": "spreadout" # binpack，spreadout支持这两个参数
    }

```

### 磁盘管理

   carina-node启动时会扫描本地磁盘，当发现符合条件的裸盘时会将其加入vg卷组 或者raw裸盘组

```shell
$   kubectl get nodestorageresource
NAME             NODE             TIME
docker-desktop   docker-desktop   12m
```
```
    allocatable:
      carina.storage.io/carina-raw-loop/loop4: "99"
      carina.storage.io/carina-raw-ssd/loop3: "199"
    capacity:
      carina.storage.io/carina-raw-loop/loop4: "100"
      carina.storage.io/carina-raw-ssd/loop3: "200"
    disks:
    - name: loop3
      path: /dev/loop3
      sectorSize: 512
      size: 214748364800
      udevInfo:
        name: loop3
        properties:
          DEVNAME: /dev/loop3
          DEVPATH: /devices/virtual/block/loop3
          DEVTYPE: disk
          MAJOR: "7"
          MINOR: "3"
          SUBSYSTEM: block
        sysPath: /devices/virtual/block/loop3
    - name: loop4
      path: /dev/loop4
      sectorSize: 512
      size: 107374182400
      udevInfo:
        name: loop4
        properties:
          DEVNAME: /dev/loop4
          DEVPATH: /devices/virtual/block/loop4
          DEVTYPE: disk
          MAJOR: "7"
          MINOR: "4"
          SUBSYSTEM: block
        sysPath: /devices/virtual/block/loop4
```

  如上配置文件和磁盘管理有关的参数有三个：

- diskSelector：该参数为一个正则表达式，carina-node会根据该配置过滤本地磁盘
- diskScanInterval：磁盘扫描间隔，0表示关闭本地磁盘扫描
- policy：磁盘分组策略，只支持按照裸盘raw,lvm

### 测试实例演示
根据自己环境修改storageclass的配置参数选择磁盘组名称，测试当前我的测试环境选择的是磁盘carina-raw-ssd 匹配的是/dev/loop3;
独占磁盘carina-raw-loop匹配的是/dev/loop4，为了简单这里只配置了一个磁盘匹配。

```
kubectl apply -f ./examples/kubernetes/storageclass-raw-exclusivity.yaml
kubectl apply -f ./examples/kubernetes/storageclass-raw.yaml
```
这里可以看到已经有了两个storageclass
```
 kubectl get sc 
NAME                         PROVISIONER          RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
csi-carina-raw               carina.storage.io    Delete          WaitForFirstConsumer   true                   4h57m
csi-carina-raw-exclusivity   carina.storage.io    Delete          WaitForFirstConsumer   true                   30h
```

#### 1. 创建statefuleset 卷是block的实例
```
kubectl create ns carina
kubectl apply -f examples/kubernetes/raw-sts-block.yaml
```
```
$ kubectl get pods -n carina
NAME                   READY   STATUS    RESTARTS   AGE
nginx-device-block-0   1/1     Running   0          2m5s
```
实例运行正常，检查磁盘分区是否创建
```
lsblk
```
```
loop0       7:0    0 395.1M  1 loop /mnt/wsl/docker-desktop/cli-tools
loop1       7:1    0 349.4M  1 loop 
loop2       7:2    0   200G  0 loop 
loop3       7:3    0   200G  0 loop 
└─loop3p1 259:0    0     5G  0 part 
loop4       7:4    0   100G  0 loop 
```

#### 2. 创建statefuleset 卷是Filesystem的实例
```
kubectl apply -f examples/kubernetes/raw-sts-fs.yaml
```
```
$ kubectl get pods -n carina
NAME                   READY   STATUS    RESTARTS   AGE
nginx-device-block-0   1/1     Running   0          3m15s
nginx-device-fs-0      1/1     Running   0          45s
```
实例运行正常，检查磁盘分区是否创建
```
lsblk
```
```
loop0       7:0    0 395.1M  1 loop /mnt/wsl/docker-desktop/cli-tools
loop1       7:1    0 349.4M  1 loop 
loop2       7:2    0   200G  0 loop 
loop3       7:3    0   200G  0 loop 
├─loop3p1 259:0    0     5G  0 part 
└─loop3p2 259:1    0    10G  0 part 
loop4       7:4    0   100G  0 loop 
```


#### 3.扩容statefuleset 卷是Filesystem的实例
进行在线扩容

```shell
$ kubectl patch pvc/html-nginx-device-fs-0 \
  --namespace "carina" \
  --patch '{"spec": {"resources": {"requests": {"storage": "15Gi"}}}}'
  
```

进入容器查看容量

```shell
$ kubectl exec -it nginx-device-fs-0 -n carina bash
$ df -h
Filesystem      Size  Used Avail Use% Mounted on
overlay         251G   32G  207G  14% /
tmpfs            64M     0   64M   0% /dev
tmpfs           5.9G     0  5.9G   0% /sys/fs/cgroup
/dev/loop3p2     10G   33M   10G   1% /data
/dev/sdd        251G   32G  207G  14% /etc/hosts
shm              64M     0   64M   0% /dev/shm
tmpfs           5.9G   12K  5.9G   1% /run/secrets/kubernetes.io/serviceaccount
tmpfs           5.9G     0  5.9G   0% /proc/acpi
tmpfs           5.9G     0  5.9G   0% /sys/firmware
```
可以看到没有变化，我们再执行检查可以看到,即目前我们方案是不支持非独占磁盘扩容的
```
$ kubectl describe pvc html-nginx-device-fs-0 -n carina  |grep failed
  Warning  VolumeResizeFailed     93s (x10 over 3m)      external-resizer carina.storage.io                                                             resize volume "pvc-c1efe394-3e27-4767-895c-fd31f13d5f6f" by resizer "carina.storage.io" failed: rpc error: code = Internal desc = can not exclusivityDisk pods
```

#### 4. 清理statefuleset 卷是Filesystem的实例
```
$ kubectl delete -f examples/kubernetes/raw-sts-fs.yaml
$ kubectl delete pvc html-nginx-device-fs-0 -n carina
```
在节点主机上执行
```
$ lsblk
loop0       7:0    0 395.1M  1 loop /mnt/wsl/docker-desktop/cli-tools
loop1       7:1    0 349.4M  1 loop 
loop2       7:2    0   200G  0 loop 
loop3       7:3    0   200G  0 loop 
└─loop3p1 259:0    0     5G  0 part 
loop4       7:4    0   100G  0 loop 
```
可以看到磁盘pods占用的磁盘分区已经被清理了

#### 5. 创建独占磁盘实例
```
kubectl apply -f examples/kubernetes/raw-deploy-fs-expand-exclusivity.yaml
```
```
$ kubectl get pods -n carina 
NAME                                        READY   STATUS    RESTARTS   AGE
carina-deployment-expand-59874c95f6-fd6wc   1/1     Running   0          50s
nginx-device-block-0                        1/1     Running   0          123m

$ kubectl get lv 

NAME                                       SIZE   GROUP                   NODE             STATUS
pvc-49319663-13e7-453f-b412-4edc1a8a2777   20Gi   carina-raw-loop/loop4   docker-desktop   Success
pvc-d862f14a-2d4b-4128-aef4-8305e77f8b8f   5Gi    carina-raw-ssd/loop3    docker-desktop   Success
```

#### 6. 扩容独占磁盘实例

进行在线扩容

```shell
$ kubectl patch pvc/csi-carina-raw-expand \
  --namespace "carina" \
  --patch '{"spec": {"resources": {"requests": {"storage": "30Gi"}}}}'
  
```

```
$ kubectl get lv 

NAME                                       SIZE   GROUP                   NODE             STATUS
pvc-49319663-13e7-453f-b412-4edc1a8a2777   30Gi   carina-raw-loop/loop4   docker-desktop   Success
pvc-d862f14a-2d4b-4128-aef4-8305e77f8b8f   5Gi    carina-raw-ssd/loop3    docker-desktop   Success
```
kubectl get pv -n carina
```
AGECLASS                 REASON   AGE
pvc-49319663-13e7-453f-b412-4edc1a8a2777   30Gi       RWO            Delete           Bound    carina/csi-carina-raw-expand       csi-carina-raw-exclusivity            5m44s
pvc-d862f14a-2d4b-4128-aef4-8305e77f8b8f   5Gi        RWO            Delete           Bound    carina/html-nginx-device-block-0   csi-carina-raw                        65m
```

进入容器查看容量

```shell
$ kubectl exec -it $(kubectl get pods -l  app=web-server -n carina | awk   '{if (NR>1){ print $1}}') -n carina bash
$ df -h
Filesystem      Size  Used Avail Use% Mounted on
overlay         251G   32G  207G  14% /
tmpfs            64M     0   64M   0% /dev
tmpfs           5.9G     0  5.9G   0% /sys/fs/cgroup
/dev/sdd        251G   32G  207G  14% /etc/hosts
shm              64M     0   64M   0% /dev/shm
/dev/loop4p1     28G   33M   28G   1% /var/lib/www/html
tmpfs           5.9G   12K  5.9G   1% /run/secrets/kubernetes.io/serviceaccount
tmpfs           5.9G     0  5.9G   0% /proc/acpi
tmpfs           5.9G     0  5.9G   0% /sys/firmware
```

可以看到独占磁盘的实例已经扩容了



#### 7. 清理所有资源
```
kubectl delete -f examples/kubernetes/raw-deploy-fs-expand-exclusivity.yaml
kubectl delete -f examples/kubernetes/raw-sts-block.yaml
kubectl delete pvc/html-nginx-device-block-0 -n carina
kubectl delete -f ./examples/kubernetes/storageclass-raw-exclusivity.yaml
kubectl delete -f ./examples/kubernetes/storageclass-raw.yaml
```
检查节点已经没有分区了
```
$ lsblk
NAME  MAJ:MIN RM   SIZE RO TYPE MOUNTPOINT
loop0   7:0    0 395.1M  1 loop /mnt/wsl/docker-desktop/cli-tools
loop1   7:1    0 349.4M  1 loop 
loop2   7:2    0   200G  0 loop 
loop3   7:3    0   200G  0 loop 
loop4   7:4    0   100G  0 loop 
```

