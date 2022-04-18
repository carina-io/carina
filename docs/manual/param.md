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
reclaimPolicy: Delete
allowVolumeExpansion: true
# WaitForFirstConsumer表示被容器绑定调度后再创建pv
volumeBindingMode: WaitForFirstConsumer
mountOptions:
```

- 参数`csi.storage.k8s.io/fstype`表示挂载后设备文件格式

- 参数`carina.storage.io/backend-disk-type`表示后端存储设备磁盘类型，填写慢盘类型比如Hdd
- 参数`carina.storage.io/cache-disk-type`表示缓存设备磁盘类型，填写快盘类型比如ssd
- 参数`carina.storage.io/cache-disk-ratio`表示缓存比例范围为1-100，该比率计算公式是 `cache-disk = backend * 100 / cache-disk-ratio`
- 参数`carina.storage.io/cache-policy`表示缓存策略共三种`writethrough|writeback|writearound`

### logicVolume对象参数整理
### pvc对象参数整理
### pod对象参数整理