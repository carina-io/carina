#### bcache功能设计实现

#### 前言

- 当本地磁盘存在ssd和hdd两种类型磁盘时，我们希望为POD提供的挂载卷既有ssd的高速读写能力，又具有hdd的大容量特性，因此我们引入了bcache来实现这一功能
- bcache是广泛采用的多盘缓存技术，在kernel 3.10正式加入linux内核，只需`modprobe bcache`即可开启
- lvm卷本身是有缓存卷功能的，它要求缓存卷与存储卷必须在一个vg下，这不符合我们目前的存储卷实现，我们目前将ssd/hdd磁盘分别创建了不同的vg

#### 功能设计

- 如果你要使用bcache功能，需要确保内核模块已经启动了bcache，在有些内核版本中bcache并未被默认加载即使执行`modprobe bcache`也不会启用
- 在`storageclass`进行如下设置将创建bcache

```yaml
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-carina-sc
provisioner: carina.storage.io
parameters:
  # file system
  csi.storage.k8s.io/fstype: xfs
  # 数据存储磁盘类型
  carina.storage.io/backend-disk-group-name: hdd
  # 缓存磁盘类型
  carina.storage.io/cache-disk-group-name: ssd
  # 1-100 Cache Capacity Ratio
  # 假设pvc为10G，则会创建5G的ssd类型的缓存盘
  carina.storage.io/cache-disk-ratio: "50"
  # 三种缓存模式writethrough/writeback/writearound
  carina.storage.io/cache-policy: writethrough
reclaimPolicy: Delete
allowVolumeExpansion: true
# WaitForFirstConsumer表示被容器绑定调度后再创建pv
volumeBindingMode: WaitForFirstConsumer
mountOptions:
```



#### 具体实现

- 目前一直当我们创建一个pvc时会创建logicvolume资源，carina-node会监听该资源的创建进而创建对应的lvm卷
- 在bcache情况下需要一个缓存卷，也就是说每一个pvc将有两个对应的lvm卷与之对应，因此我们实现为创建一个pvc时创建两个logicvolume

```
+-------------+                     +----------------------+                +-----------------+ 
|             |-------------------->|   pvc-logicvolume    |--------------->|  pvc-lvm        |
| create pvc  | create logicvolume  |                      | carian-node    |                 | 
|             |-------------------->|   cache-logicvolume  |--------------->|  cache-lvm      | 
+-------------+                     +----------------------+                +-----------------+  
```

- 在实现中将cache-logicvolume的owner设置为pvc-logicvolume，如此不用编写额外的删除逻辑，当删除pvc时自动会清理掉pvc-logicvolume和cache-logicvolume

#### 已知局限

- 由于bcache限制，当对lvm卷进行扩容时，bcache逻辑盘并不会随之扩容，因此需要容器删除重建才能使用扩容后的磁盘空间
