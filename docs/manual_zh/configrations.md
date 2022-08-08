## configmap

#### Configurations
| 参数名                           |是否必填| 参数说明                                |可选值               |默认                  |
| ------------------------------  |-------|-----------------------------------------| --------------------|---------------------|
| `diskSelector.name`             |是     |磁盘分组名称                              |                     |                     |
| `diskSelector.re`               |是     |磁盘分组匹配策略，支持正则表达式            |                     |                     |
| `diskSelector.policy`           |是     |磁盘分组策略                              |                     |                     |
| `diskSelector.nodeLabel`        |是     |磁盘分组匹配节点标签                       |                     |                     |
| `diskScanInterval`              |是     |磁盘扫描间隔，0表示关闭本地磁盘扫描         |                     |                     |
| `schedulerStrategy`             |是     |磁盘分组调度策略:`binpack`为pv选择磁盘容量刚好满足`requests.storage`的节点 ，`spreadout`为pv选择磁盘剩余容量最多的节点  | `binpack`，`spreadout`  | `spreadout` |

#### example
```yaml
config.json: |-
    {
      "diskSelector": [
        {
          "name": "carina-vg-ssd",
          "re": ["loop2+"],
          "policy": "LVM",
          "nodeLabel": "kubernetes.io/hostname"
        },
        {
          "name": "carina-raw-hdd",
          "re": ["vdb+", "sd+"],
          "policy": "RAW",
          "nodeLabel": "kubernetes.io/hostname"
        }
      ],
      "diskScanInterval": "300",
      "schedulerStrategy": "spreadout"
    }
```


## storageClass

#### Configurations

| 参数名                                       |是否必填| 参数说明                                |可选值               |默认                                       |
| --------------------------------------------|-------|-----------------------------------------| --------------------|------------------------------------------|
| `csi.storage.k8s.io/fstype`                 |否     |挂载设备文件格式                         |`xfs`,`ext4`         |`ext4`                                    |
| `carina.storage.io/backend-disk-group-name` |否     |后端存储设备磁盘类型，填写慢盘磁盘分组名字   |用户配置的磁盘组名称   |                                          |
| `carina.storage.io/cache-disk-group-name`   |否     |缓存设备磁盘类型，填写快盘磁盘分组名字       |用户配置的磁盘组名称   |                                          |
| `carina.storage.io/cache-disk-ratio`        |否     |缓存比例范围为1-100，该比率计算公式是 `cache-disk-size = backend-disk-size * cache-disk-ratio / 100`  | 1-100 |   |
| `carina.storage.io/cache-policy`            |是     |缓存策略                                  |`writethrough`,`writeback`,`writearound` | |
| `carina.storage.io/disk-group-name`         |否     |磁盘组类型                                |用户配置的磁盘组名称    |                                         |
| `carina.storage.io/exclusively-raw-disk`    |否     |当使用裸盘时是否使用独占磁盘                |`true`,`false`        |`false`                                  |
| `reclaimPolicy`                             |否     |回收策略                                  |`Delete`,`Retain`     |`Delete`                                 |
| `allowVolumeExpansion`                      |是     |是否允许扩容                              |`true`,`false`         |`true`                                 |
| `volumeBindingMode`                         |是     |调度策略：WaitForFirstConsumer表示被容器绑定调度后再创建pv，Immediate表示一旦创建了pvc 也就完成了卷绑定和动态制备。|   `WaitForFirstConsumer`,`Immediate` | |
| `allowedTopologies`                         |否     |只有`volumeBindingMode: Immediate`类型的才支持根据`matchLabelExpressions`选择pv所在节点   |         | |


#### example
```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-carina-sc
provisioner: carina.storage.io
parameters:
  csi.storage.k8s.io/fstype: xfs
  carina.storage.io/backend-disk-group-name: hdd
  carina.storage.io/cache-disk-group-name: carina-vg-ssd
  carina.storage.io/cache-disk-ratio: "50"
  carina.storage.io/cache-policy: writethrough
  carina.storage.io/disk-group-name: "carina-vg-hdd"
  carina.storage.io/exclusively-raw-disk: false
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: Immediate 
mountOptions:
allowedTopologies:
  - matchLabelExpressions:
      - key: beta.kubernetes.io/os
        values:
          - arm64
          - amd64
```






## pod

#### Configurations

| 参数名                                                    |是否必填|参数说明         |    可选值     |默认    |
| -------------------------------------------------------- |------ |-----------------| ------------- |-------|
| `carina.storage.io/blkio.throttle.read_bps_device`       |是     |设置磁盘读bps值   |               |        |
| `carina.storage.io/blkio.throttle.write_bps_device`       |是     |设置磁盘写bps值   |               |        |
| `carina.storage.io/blkio.throttle.read_iops_device`       |是     |设置磁盘读iops值  |               |        |
| `carina.storage.io/blkio.throttle.write_iops_device`       |是     |设置磁盘读iops值  |               |        |
| `carina.storage.io/allow-pod-migration-if-node-notready` |否     |节点故障时是否迁移 |`true`,`false`|`false`   |

#### example
```yaml
 metadata:
      annotations:
        carina.storage.io/blkio.throttle.read_bps_device: "10485760"
        carina.storage.io/blkio.throttle.read_iops_device: "10000"
        carina.storage.io/blkio.throttle.write_bps_device: "10485760"
        carina.storage.io/blkio.throttle.write_iops_device: "100000"
        carina.storage.io/allow-pod-migration-if-node-notready: true
```
