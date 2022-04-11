
## 基于本地存储使用裸盘设计方案

### 介绍

- 在业务实际应用场景中，基于提高性能的考虑，磁盘管理不使用lvm,而是直接使用裸盘。

### 功能设计

- 控制节点创建CRD资源和负责维护crd节点状态，节点服务监听CRD资源创建，收到创建事件后维护本地磁盘状态
- 通过配置文件获取扫描时间间隔和磁盘的匹配条件,检查磁盘是否为裸盘，如果发现有匹配多块裸盘按磁盘顺序取第一块。
- 裸盘会作为设备注册到节点。
- 裸盘按storageclass 参数配置可以分为独占式和共享式。独占式支持扩容，共享式不支持扩容。

### 实现细节
- 控制器周期获取节点状态，来维护NodeDeive的状态
- 裸盘磁盘匹配规则
```
config.json: |-
    {
      "diskSelector": [
        {
          "name": "carina-raw-ssd" ,
          "re": ["loop3"], 
          "policy": "RAW",
          "nodeLabel": "kubernetes.io/hostname"
        },
        {
          "name": "carina-raw-hdd",
          "re": ["loop*+"],#可以匹配多块磁盘
          "policy": "RAW",
          "nodeLabel": "kubernetes.io/hostname"
        }
      ],
      "diskScanInterval": "300",
      "schedulerStrategy": "spreadout"
    }
```
- storageClass新增加参数配置"carina.io/exclusivly-disk-claim",true是指pod独占磁盘,false是多个pod共享磁盘；
```yaml
  apiVersion: storage.k8s.io/v1
  kind: StorageClass
  metadata:
    name: csi-carina-sc
  provisioner: carina.storage.io # 这是该CSI驱动的名称，不允许更改
  parameters:
    # 这是kubernetes内置参数，我们支持xfs,ext4两种文件格式，如果不填则默认ext4
    csi.storage.k8s.io/fstype: xfs
    carina.storage.io/disk-type: carina-raw-ssd 
    carina.storage.io/exclusivity-disk: false  # 新增加参数是否是独占式，默认fasle
  reclaimPolicy: Delete
  allowVolumeExpansion: true # 支持扩容，定为true便可
  # WaitForFirstConsumer表示被容器绑定调度后再创建pv
  volumeBindingMode: WaitForFirstConsumer
  # 支持挂载参数设置，默认为空
  # 如果没有特殊的需求，为空便可满足大部分要求
  mountOptions:
    - rw
  ```

- 调度策略不变。binpack：选择恰好满足pvc容量的节点；spreadout：选择剩余容量最大的节点，这个是默认调度策略

```
   config.json: |-
      {
        "schedulerStrategy": "spreadout" # binpack，spreadout支持这两个参数
      }
```
- 磁盘模型划分如下

  ```
  独占式磁盘
  +-------------------------+                
  | 主分区 ( pvc容量)        |
  | 主分区 (磁盘剩余容量)    |
  +-------------------------+  
  
  共享式磁盘
  +-------------------------+                
  |主分区 (pvc1容量)          |
  |主分区 (磁盘剩余容量)       |
  |主分区 (pvc2容量)          |
  |主分区 (pvcn容量)          |
  |主分区 (磁盘剩余容量)       |
  +-------------------------+     

  +-------------------------+                
  | 主分区 ( pvc容量)        |
  |                         |
  |                         |
  
  +-------------------------+  
  
  共享式磁盘
  +-------------------------------------+                
  |主分区 ( pvc容量)                     |
  |主分区 (磁盘剩余容量)                  |
  +--------------------------------------+  

  ```


### 实现逻辑
#### 1. 管理分区容量
每个节点定时磁盘检查新增裸盘注册设备和查询和记录裸盘空闲空间片段位置信息到nodedevice里
```
df -h  /dev/loop2p1
```
#### 2. 裸盘调度策略
  - 选着所有节点上裸盘剩余可用，选着裸盘上剩余可用分区位置；如果是独占磁盘，筛选没有分区的磁盘，满足不同的调度策略；如果不是独占磁盘，优先筛选有分区的磁盘去满足调度策略，如果没有匹配则筛选空白磁盘满足调度策略
  - binpack：选择恰好满足pvc容量的节点，和 剩余可用分区片段恰好满足
  - spreadout：选择剩余容量最大的节点和选择剩余容量最大的分区片段，这个是默认调度策略
#### 3. 创建分区
 >默认创建GPT分区 ，检测裸盘已有分区。查看可分配空间节点位置，采用最佳适应算法（Best Fit），分区删除和增加必然造成很多不连续的空余空间。这就要求将所有的空闲区按容量递增顺序排成一个空白链。这样每次找到的第一个满足要求的空闲区，必然是最优的

使用分区命令创建如下：
```
# parted /dev/loop2 mklable gpt #设置分区格式
# parted /dev/loop2 mkpart myloop1 0 10G
# parted /dev/loop2 mkpart myloop2 10G 20G 
# parted /dev/loop2 mkpart myloop3 20G -0G
# parted /dev/loop2 p
parted /dev/loop2 p free
# blkid |grep myloop3  # 查看分区num,label uuid 

partprobe    #同步磁盘分区表                
fdisk -lu /dev/loop2
```
#### 4. 扩容分区
- ① storage配置参数注解pod独占整块磁盘可以扩容
- ② 多个pods共享磁盘不支持扩容
- ③ 扩展已有GPT分区
```

dump -0uj -f /tmp/loop2p1bak.bz2 /dev/loop2p1
restore -r -f /tmp/loop2p1bak.bz2
export path=$(findmnt -S /dev/loop2p1 --output=target --noheading)
umount path
parted /dev/loop2 resizepart 1（分区号）  600（end位置）
parted /dev/loop2 p
mount  /dev/loop2p1  path
```

#### 删除分区
```
lv 和裸盘分区绑定，删除lv 就删除裸盘pod占用的分区
parted /dev/loop2 rm 
parted /dev/loop2 p1
```
### 流程细节
#### controller : nodeController,pvcController,webhook,csiControllerGrpc
- 监听 ConfigMap是否变化,lvm是一个vg对应注册一个设备(carina-vg-XXX.sock)，裸设备则是一个裸盘或者分区对应一个注册设备(carina-raw-XXX.sock)；通过切割注册设备，判断注册设备的健康状态来检测使用量。
- PVC创建完成后,根据存储类型(此处为rbd)找到存储类StorageClass
- external-provisioner，watch到指定StorageClass的 PersistentVolumeClaim资源状态变更，会自动地调用csiControllerGrpc这两个CreateVolume、DeleteVolume接口；等待返回成功则创建pv，卷控制器会将 PV 与 PVC 进行绑定。
- CreateVolume 接口还会创建LogicVolume，一个LogicVolume对应一个lv, 增加注解或者标签标识是使用裸盘还是lvm
- k8s组件AttachDetachController 控制器观察到使用 CSI 类型 PV 的 Pod 被调度到某一节点，此时调用内部 in-tree CSI 插件（csiAttacher）的 Attach 函数创建一个 VolumeAttachment 对象到集群中。
- external-attacher watch到VolumeAttachment资源状态变更，会自动地调用外部 CSI插件这两个ControllerPublish、ControllerUnpublish接口。外部 CSI 插件挂载成功后，External Attacher会更新相关 VolumeAttachment 对象的 .Status.Attached 为 true。
- external-resizer  watch到PersistentVolumeClaim资源的容量发生变更，会自动地调用这个ControllerExpandVolume接口。

#### node : logicVolumeController,podController,csiNodeGrpc

- carina-node则负责监听LogicVolume的创建事件，获取lv类型，给LogicVolume绑定裸盘分区驱动设备组和设备id,更新状态。
- 给pods 配置 cgroup  blkio,限制进程读写的 IOPS 和吞吐量
- node-driver-registra 调用接口获取CSI插件信息，并向kubelet进行注册
- Volume Manager（Kubelet 组件）观察到有新的使用 CSI 类型 PV 的 Pod 调度到本节点上，于是调用内部 in-tree CSI 插件函数调用外部插件接口NodePublishVolume，NodeUnpublishVolume
- 启动磁盘检查是否有新裸盘加入，注册裸盘设备
- 一致性检查，清理孤儿卷, 每十分钟会遍历本地volume，然后检查k8s中是否有对应的logicvolume，若是没有则删除本地volume（remove lv）并且删除对应设备分区;
- 每十分钟会遍历k8s中logicvolume，然后检查logicvolume是否有对应的pv，若是没有则删除logicvolume



