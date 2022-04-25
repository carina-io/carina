### storageClass对象参数整理
```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-carina-sc
provisioner: carina.storage.io
parameters:
  # file system
  csi.storage.k8s.io/fstype: xfs
  # disk group
  carina.storage.io/backend-disk-type: hdd  // backend-disk-group-name
  carina.storage.io/cache-disk-type: ssd // cache-disk-group-name
  # 1-100 Cache Capacity Ratio
  carina.storage.io/cache-disk-ratio: "50"
  # writethrough/writeback/writearound
  carina.storage.io/cache-policy: writethrough
  carina.storage.io/disk-type: "hdd" // disk-group-name
  carina.storage.io/exclusivity-disk: false //carina.storage.io/exclusively-raw-disk
reclaimPolicy: Delete
allowVolumeExpansion: true
# WaitForFirstConsumer表示被容器绑定调度后再创建pv
volumeBindingMode: WaitForFirstConsumer # Immediate
mountOptions:
allowedTopologies:
  - matchLabelExpressions:
      - key: beta.kubernetes.io/os
        values:
          - linux
          - amd64 // 去掉 arch in [amd64/arm64]
      - key: kubernetes.io/hostname
        values:
          - 10.20.9.153
          - 10.20.9.154
```

- 参数`csi.storage.k8s.io/fstype`表示挂载后设备文件格式
- 参数`carina.storage.io/backend-disk-type`表示后端存储设备磁盘类型，填写慢盘类型比如Hdd  // 类型改成磁盘分组名字
- 参数`carina.storage.io/cache-disk-type`表示缓存设备磁盘类型，填写快盘类型比如ssd
- 参数`carina.storage.io/cache-disk-ratio`表示缓存比例范围为1-100，该比率计算公式是 `cache-disk-size = backend-disk-size * cache-disk-ratio / 100`    
- 参数`carina.storage.io/cache-policy`表示缓存策略共三种`writethrough|writeback|writearound`
- 参数`carina.storage.io/disk-type`表示磁盘组类型 // 
- 参数`carina.storage.io/exclusivity-disk`表示是否是当使用裸盘时独占磁盘 //
- 注意只有`volumeBindingMode: Immediate`类型的才支持根据`allowedTopologies`选择pv所在节点，当`volumeBindingMode: WaitForFirstConsumer`时，pv选择节点将根据pod的topology设置进行调度


## storageclass

#### Configurations
|---|-----|-----|----|---|
|参数名|是否必填|参数说明|可选值|
|aaaa|yes|sssss|bool， true or false|

#### example
```
yaml
```






### pod对象参数整理
```yaml
 metadata:
      annotations:
        kubernetes.customized/blkio.throttle.read_bps_device: "10485760"  // carina.storage.io/blkio.throttle.read.....
        kubernetes.customized/blkio.throttle.read_iops_device: "10000"
        kubernetes.customized/blkio.throttle.write_bps_device: "10485760"
        kubernetes.customized/blkio.throttle.write_iops_device: "100000"
        carina.io/rebuild-node-notready: true //carina.stroage.io/allow-pod-migration-if-node-notready: true
```
- 注解`kubernetes.customized/blkio.throttle.read_bps_device`表示设置磁盘读bps值
- 注解`kubernetes.customized/blkio.throttle.write_bps_device`表示设置磁盘写bps值
- 注解`kubernetes.customized/blkio.throttle.read_iops_device`表示设置磁盘读iops值
- 注解`kubernetes.customized/blkio.throttle.write_iops_device`表示设置磁盘读iops值
- 注解`carina.io/rebuild-node-notready`并且其值为"true"则表示该容器希望carina对其进行迁移


### carina-config configmap

