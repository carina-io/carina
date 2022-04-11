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
- diskGroupPolicy：磁盘分组策略，只支持按照磁盘类型分组，更改成其他值无效

### 测试实例演示
```
kubectl apply -f ./examples/kubernetes/storageclass-raw-exclusivity.yaml
kubectl apply -f ./examples/kubernetes/storageclass-raw.yaml
```
```
 kubectl get sc 
NAME                         PROVISIONER          RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
csi-carina-lvm               carina.storage.io    Delete          WaitForFirstConsumer   true                   4h57m
csi-carina-raw               carina.storage.io    Delete          WaitForFirstConsumer   true                   4h57m
csi-carina-raw-exclusivity   carina.storage.io    Delete          WaitForFirstConsumer   true                   30h
```

#### 1. 创建块实例
```
kubectl create ns carina
kubectl apply -f examples/kubernetes/raw-sts-block.yaml
```
```
NAME                   READY   STATUS    RESTARTS   AGE
nginx-device-block-0   1/1     Running   0          2m5s
```
实例运行正常
#### 2. 创建字符实例
```
kubectl apply -f examples/kubernetes/raw-sts-fs.yaml
kubectl apply -f examples/kubernetes/raw-deploy-fs-expand.yaml
```
```
nginx-device-block-0   1/1     Running   0          3m15s
nginx-device-fs-0      1/1     Running   0          45s
```
实例运行正常
#### 3. 创建独占磁盘实例
```
kubectl apply -f examples/kubernetes/raw-deploy-fs-expand-exclusivity.yaml
```
```
carina-deployment-expand-59874c95f6-p5vgt   1/1     Running   0          32s
nginx-device-block-0                        1/1     Running   0          4m51s
nginx-device-fs-0                           1/1     Running   0          2m21s
```
 kubectl get lv 
``` 
NAME                                       SIZE   GROUP                   NODE             STATUS
pvc-00776663-03ba-4e76-9c4a-9e710d60ea98   5Gi    carina-raw-ssd/loop3    docker-desktop   Success
pvc-68461f4b-d12a-4001-b93c-d3ab0bc3018c   10Gi   carina-raw-ssd/loop3    docker-desktop   Success
pvc-e810bd63-e917-4308-aa39-07652cd9eafc   65Gi   carina-raw-loop/loop4   docker-desktop   Success
```

#### 4. 扩容独占磁盘实例
扩容实例资源，edit raw-deploy-fs-expand-exclusivity.yaml ....
```
kubectl apply -f examples/kubernetes/raw-deploy-fs-expand-exclusivity.yaml
```

kubectl get lv 
```
pvc-e810bd63-e917-4308-aa39-07652cd9eafc   80Gi   carina-raw-loop/loop4   docker-desktop   Success
```
kubectl get pv -n carina
```
persistentvolume/pvc-e810bd63-e917-4308-aa39-07652cd9eafc   80Gi       RWO            Delete           Bound    carina/csi-carina-raw-expand       csi-carina-raw-exclusivity            17m
```
kubectl get nodestorageresource -oyaml
```
    allocatable:
      carina.storage.io/carina-raw-loop/loop4: "25"
      carina.storage.io/carina-raw-ssd/loop3: "184"
    capacity:
      carina.storage.io/carina-raw-loop/loop4: "100"
      carina.storage.io/carina-raw-ssd/loop3: "200"
    disks:
    - name: loop3
      partitions:
        "1":
          last: 5369757695
          name: carina.io/9e710d60ea98
          number: 1
          start: 1048576
        "2":
          last: 16107175935
          name: carina.io/d3ab0bc3018c
          number: 2
          start: 5369757696
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
      partitions:
        "1":
          last: 80000000511
          name: carina.io/07652cd9eafc
          number: 1
          start: 1048576
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
    syncTime: "2022-04-11T02:30:05Z"
```
lsblk
```
loop2       7:2    0   200G  0 loop 
loop3       7:3    0   200G  0 loop 
├─loop3p1 259:0    0     5G  0 part 
└─loop3p2 259:1    0    10G  0 part 
loop4       7:4    0   100G  0 loop 
└─loop4p1 259:2    0  74.5G  0 part 
loop5       7:5    0     5G  0 loop 
```
可以看到独占磁盘的实例支持扩容

#### 5. 清理实例
```
kubectl delete -f examples/kubernetes/raw-sts-fs.yaml
kubectl delete pvc html-nginx-device-fs-0 -n carina
```
lsblk 
```
lsblk
NAME      MAJ:MIN RM   SIZE RO TYPE MOUNTPOINT
loop0       7:0    0 395.1M  1 loop /mnt/wsl/docker-desktop/cli-tools
loop1       7:1    0 349.4M  1 loop 
loop2       7:2    0   200G  0 loop 
loop3       7:3    0   200G  0 loop 
└─loop3p1 259:0    0     5G  0 part 
loop4       7:4    0   100G  0 loop 
└─loop4p1 259:2    0  74.5G  0 part 
```
kubectl get nodestorageresource -oyaml |grep allocatable -A 5
```
   allocatable:
      carina.storage.io/carina-raw-loop/loop4: "25"
      carina.storage.io/carina-raw-ssd/loop3: "194"
    capacity:
      carina.storage.io/carina-raw-loop/loop4: "100"
      carina.storage.io/carina-raw-ssd/loop3: "200"
```
loop3p2 分区释放
```
kubectl delete -f examples/kubernetes/raw-sts-block.yaml
kubectl delete pvc html-nginx-device-block-0  -n carina
kubectl get nodestorageresource -oyaml |grep allocatable -A 5
```
 ```
 allocatable:
      carina.storage.io/carina-raw-loop/loop4: "25"
      carina.storage.io/carina-raw-ssd/loop3: "199"
    capacity:
      carina.storage.io/carina-raw-loop/loop4: "100"
      carina.storage.io/carina-raw-ssd/loop3: "200"
 ```
 实例释放后相对应的磁盘分区也会释放

#### 6. 再创建新实例
```
kubectl apply -f examples/kubernetes/raw-deploy-fs-expand.yaml
kubectl get pods -n carina 
```
```
NAME                                        READY   STATUS    RESTARTS   AGE
carina-deployment-expand-59874c95f6-p5vgt   1/1     Running   0          43m
carina-deployment-fs-6d96f88489-v524p       1/1     Running   0          38s
```
 kubectl get lv 
 ```
NAME                                       SIZE   GROUP                   NODE             STATUS
pvc-a162fc6d-0195-4921-ae93-89b8fd8e0f2f   5Gi    carina-raw-ssd/loop3    docker-desktop   Success
pvc-e810bd63-e917-4308-aa39-07652cd9eafc   80Gi   carina-raw-loop/loop4   docker-desktop   Success
```
新建的实例在非独占磁盘上，独占磁盘不接受新调度实例
#### 7. 扩容新建新实例
修改实例资源配额后执行
```
kubectl apply -f examples/kubernetes/raw-deploy-fs-expand.yaml
```
```
kubectl get pvc,pv -n carina 
NAME                                          STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS                 AGE
persistentvolumeclaim/csi-carina-raw-expand   Bound    pvc-e810bd63-e917-4308-aa39-07652cd9eafc   80Gi       RWO            csi-carina-raw-exclusivity   47m
persistentvolumeclaim/csi-carina-raw-fs       Bound    pvc-a162fc6d-0195-4921-ae93-89b8fd8e0f2f   5Gi        RWO            csi-carina-raw               4m36s

NAME                                                        CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM                          STORAGECLASS                 REASON   AGE
persistentvolume/pvc-a162fc6d-0195-4921-ae93-89b8fd8e0f2f   5Gi        RWO            Delete           Bound    carina/csi-carina-raw-fs       csi-carina-raw                        4m34s
persistentvolume/pvc-e810bd63-e917-4308-aa39-07652cd9eafc   80Gi       RWO            Delete           Bound    carina/csi-carina-raw-expand   csi-carina-raw-exclusivity            47m
```
可以看到没有任何变化,这里就验证了非独占磁盘实例不支持扩容的能力，可以保证分区数据完整性。

```
kubectl describe pvc csi-carina-raw-fs -n carina
```
```
Warning  ExternalExpanding      63s                    volume_expand                                                                                  Ignoring the PVC: didn't find a plugin capable of expanding the volume; waiting for an external controller to process this PVC.
  Normal   Resizing               40s (x7 over 63s)      external-resizer carina.storage.io                                                             External resizer is resizing volume pvc-a162fc6d-0195-4921-ae93-89b8fd8e0f2f
  Warning  VolumeResizeFailed     40s (x7 over 63s)      external-resizer carina.storage.io                                                             resize volume "pvc-a162fc6d-0195-4921-ae93-89b8fd8e0f2f" by resizer "carina.storage.io" failed: rpc error: code = Internal desc = can not exclusivityDisk pods
```
#### 8. 清理资源
```
kubectl delete -f examples/kubernetes/raw-deploy-fs-expand.yaml 
kubectl delete -f examples/kubernetes/raw-deploy-fs-expand-exclusivity.yaml
```
```
lsblk
NAME  MAJ:MIN RM   SIZE RO TYPE MOUNTPOINT
loop0   7:0    0 395.1M  1 loop /mnt/wsl/docker-desktop/cli-tools
loop1   7:1    0 349.4M  1 loop 
loop2   7:2    0   200G  0 loop 
loop3   7:3    0   200G  0 loop 
loop4   7:4    0   100G  0 loop 
```


