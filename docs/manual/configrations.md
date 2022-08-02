## configmap

#### Configurations
| Parameter                           | Required| Description                                |Option Values               |Default                  |
| ------------------------------  |-------|-----------------------------------------| --------------------|---------------------|
| `diskSelector.name`             |Yes     |Disk group name                              |                     |                     |
| `diskSelector.re`               |Yes     |Matches the disk group policy supports regular expressions           |                     |                     |
| `diskSelector.policy`           |Yes     |Disk group name matching policy                             |                     |                     |
| `diskSelector.nodeLabel`        |Yes     |Disk group name matching node label                     |                     |                     |
| `diskScanInterval`              |Yes     |Disk scan interval, 0 to close the local disk scanning         |                     |                     |
| `schedulerStrategy`             |Yes     |Disk group name scheduling policies : binpack select the disk capacity for PV just met requests. storage node, spreadout of the most select the remaining disk capacity for PV nodes  | `binpack`，`spreadout`  | `spreadout` |

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

| Parameter                                       |Required| Description                                |Option Values               |Default                            |
| --------------------------------------------|-------|-----------------------------------------| --------------------|------------------------------------------|
| `csi.storage.k8s.io/fstype`                 |No     |Mount the device file format                         |`xfs`,`ext4`         |`ext4`                                    |
| `carina.storage.io/backend-disk-group-name` |No     |Back - end storage devices, disk type, fill out the slow disk group name   |User - configured disk group name   |                                          |
| `carina.storage.io/cache-disk-group-name`   |No     |Cache device type of disk, fill out the quick disk group name       |User - configured disk group name   |                                          |
| `carina.storage.io/cache-disk-ratio`        |No     |Cache range from 1-100 per cent, the rate equation is `cache-disk-size = backend-disk-size * cache-disk-ratio / 100`  | 1-100 |   |
| `carina.storage.io/cache-policy`            |Yes     |Cache policy                                  |`writethrough`,`writeback`,`writearound` | |
| `carina.storage.io/disk-group-name`         |No     |disk group name                                |User - configured disk group name   |                                         |
| `carina.storage.io/exclusively-raw-disk`    |No     |When using a raw disk whether to use exclusive disk             |`true`,`false`        |`false`                                  |
| `reclaimPolicy`                             |No     |GC policy                                  |`Delete`,`Retain`     |`Delete`                                 |
| `allowVolumeExpansion`                      |Yes     |Whether to allow expansion                              |`true`,`false`         |`true`                                 |
| `volumeBindingMode`                         |Yes     |Scheduling policy : waitforfirstconsumer means binding schedule after creating the container Once you create a PVC pv,immediate also completes the preparation of volumes bound and dynamic.|   `WaitForFirstConsumer`,`Immediate` | |
| `allowedTopologies`                         |No     |Only volumebindingmode : immediate that contains the type of support based on matchlabelexpressions select PV nodes   |         | |


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

| Parameter                                                    |Required|Description         |    Option Values     |Default    |
| -------------------------------------------------------- |------ |-----------------| ------------- |-------|
| `carina.storage.io/blkio.throttle.read_bps_device`       |Yes     |Set disk read BPS value   |               |        |
| `carina.storage.io/blkio.throttle.write_bps_device`       |Yes     |Set the disk is write  BPS value   |               |        |
| `carina.storage.io/blkio.throttle.read_iops_device`       |Yes     |Set disk read IOPS value  |               |        |
| `carina.storage.io/blkio.throttle.write_iops_device`       |Yes     |Set the disk is write  IOPS value  |               |        |
| `carina.storage.io/allow-pod-migration-if-node-notready` |No     |Whether to migrate when node is down |`true`,`false`|`false`   |

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
