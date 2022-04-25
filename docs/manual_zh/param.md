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
  carina.storage.io/backend-disk-type: hdd
  carina.storage.io/cache-disk-type: ssd
  # 1-100 Cache Capacity Ratio
  carina.storage.io/cache-disk-ratio: "50"
  # writethrough/writeback/writearound
  carina.storage.io/cache-policy: writethrough
  carina.storage.io/disk-type: "hdd"
  carina.storage.io/exclusivity-disk: false
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
          - amd64
      - key: kubernetes.io/hostname
        values:
          - 10.20.9.153
          - 10.20.9.154
```

- 参数`csi.storage.k8s.io/fstype`表示挂载后设备文件格式
- 参数`carina.storage.io/backend-disk-type`表示后端存储设备磁盘类型，填写慢盘类型比如Hdd
- 参数`carina.storage.io/cache-disk-type`表示缓存设备磁盘类型，填写快盘类型比如ssd
- 参数`carina.storage.io/cache-disk-ratio`表示缓存比例范围为1-100，该比率计算公式是 `cache-disk = backend * 100 / cache-disk-ratio`
- 参数`carina.storage.io/cache-policy`表示缓存策略共三种`writethrough|writeback|writearound`
- 参数`carina.storage.io/disk-type`表示磁盘组类型
- 参数`carina.storage.io/exclusivity-disk`表示是否是当使用裸盘时独占磁盘
- 注意只有`volumeBindingMode: Immediate`类型的才支持根据`allowedTopologies`选择pv所在节点，当`volumeBindingMode: WaitForFirstConsumer`时，pv选择节点将根据pod的topology设置进行调度


### logicVolume对象参数整理
```yaml
- apiVersion: carina.storage.io/v1
  kind: LogicVolume
  metadata:
    annotations:
      carina.io/volume-manage-type: raw
      carina.storage.io/exclusive-disk: "false"
      #carina.storage.io/resize-requested-at: ""
    creationTimestamp: "2022-04-18T10:58:04Z"
    finalizers:
    - carina.storage.io/logicvolume
    name: pvc-5b484c13-4c87-414b-be5b-fe2addf046b9
    namespace: default
  spec:
    deviceGroup: carina-raw-ssd/vdc
    nameSpace: carina
    nodeName: 10.20.9.45
    pvc: html-nginx-device-block-0
    size: 5Gi
  status:
    currentSize: 5Gi
    deviceMajor: 253
    deviceMinor: 32
    status: Success
    volumeID: volume-pvc-5b484c13-4c87-414b-be5b-fe2addf046b9
```
- 注解`carina.io/volume-manage-type`表示卷管理类型，目前支持raw,lvm
- 注解`carina.storage.io/exclusive-disk`表示是否是独占磁盘


### pvc对象参数整理
```yaml
- apiVersion: v1
  kind: PersistentVolumeClaim
  metadata:
    annotations:
      pv.kubernetes.io/bind-completed: "yes"
      pv.kubernetes.io/bound-by-controller: "yes"
      volume.beta.kubernetes.io/storage-provisioner: carina.storage.io
      volume.kubernetes.io/selected-node: 10.20.9.45
    creationTimestamp: "2022-04-18T10:57:34Z"
    finalizers:
    - kubernetes.io/pvc-protection
    labels:
      app: nginx-device-block
    name: html-nginx-device-block-0
    namespace: carina
  spec:
    accessModes:
    - ReadWriteOnce
    resources:
      requests:
        storage: 5Gi
    storageClassName: csi-carina-raw
    volumeMode: Block
    volumeName: pvc-5b484c13-4c87-414b-be5b-fe2addf046b9
  status:
    accessModes:
    - ReadWriteOnce
    capacity:
      storage: 5Gi
    phase: Bound
```
- 注解`volume.beta.kubernetes.io/storage-provisioner`表示csi插件提供者
- 注解`volume.kubernetes.io/selected-node`表示已经选定节点


### pod对象参数整理
```yaml
 metadata:
      annotations:
        kubernetes.customized/blkio.throttle.read_bps_device: "10485760"
        kubernetes.customized/blkio.throttle.read_iops_device: "10000"
        kubernetes.customized/blkio.throttle.write_bps_device: "10485760"
        kubernetes.customized/blkio.throttle.write_iops_device: "100000"
        carina.io/rebuild-node-notready: true
```
- 注解`kubernetes.customized/blkio.throttle.read_bps_device`表示设置磁盘读bps值
- 注解`kubernetes.customized/blkio.throttle.write_bps_device`表示设置磁盘写bps值
- 注解`kubernetes.customized/blkio.throttle.read_iops_device`表示设置磁盘读iops值
- 注解`kubernetes.customized/blkio.throttle.write_iops_device`表示设置磁盘读iops值
- 注解`carina.io/rebuild-node-notready`并且其值为"true"则表示该容器希望carina对其进行迁移