# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](https://www.opencarina.io).

## [Unreleased]

## [v1.0.0] - 2020-04-x

- Removed csi.proto upgrade CSI_VERSION=1.5
- Remove device registration and use the CRD resource NodeStorageResource instead
- Added controllers that maintain NodeStorageResource
- The scheduler supports fetching resources from NodeStorageResource
- Upgrade go.mod to depend on K8s1.23
- Upgrade the Webhook certificate using job
- Raw disk support under development

## [0.9.1] - 2021-12-17

### Added

- add helm chart to deploy
- when node is notready, migrate pods to ready node if pod annotations contains "carina.io/rebuild-node-notready: true" (<https://github.com/carina-io/carina/issues/14>))
- multiple VGS are supported for the same type of storage


### Changed

- csi configmap change new version to support mutil vgroup (https://github.com/carina-io/carina/issues/10)

### Fixed

- Fixes configmap  spradout修改为spreadout(<https://github.com/carina-io/carina/issues/12>)



## [0.10.0] - 2022-04-25

### Added

- provisioning raw disk
- doc/manual_zh/velero.md


### Changed

#### storageclass config
- replace carina.storage.io/backend-disk-type: hdd   ==> carina.storage.io/backend-disk-group-name: hdd
- replace carina.storage.io/cache-disk-type: ssd     ==> carina.storage.io/cache-disk-group-name: ssd
- replace carina.storage.io/disk-type: "hdd"         ==> carina.storage.io/disk-group-name: "hdd"

#### pod config

- replace kubernetes.customized/blkio.throttle.read_bps_device: "10485760"  ==> carina.storage.io/blkio.throttle.read_bps_device: "10485760"
- replace kubernetes.customized/blkio.throttle.read_iops_device: "10000"    ==> carina.storage.io/blkio.throttle.read_iops_device: "10000"
- replace kubernetes.customized/blkio.throttle.write_bps_device: "10485760" ==> carina.storage.io/blkio.throttle.write_bps_device: "10485760"
- replace kubernetes.customized/blkio.throttle.write_iops_device: "100000"  ==> carina.storage.io/blkio.throttle.write_iops_device: "100000"
- replace carina.io/rebuild-node-notready: true                             ==>  carina.storage.io/allow-pod-migration-if-node-notready: true

## [0.11.0] - 2022-08-31

- Support the cgroup v1 and v2
- Adjustment of project structure
- The HTTP server is deleted
- Logicvolume changed from Namespace to Cluster, [upgrade](docs/manual_zh/install-v0.11.0.md)
- Fixed the problem that message notification is not timely
- Fix the metric server panic problem #91
- Mirrored warehouse has personal space migrated to Carina exclusive space
- To improve LVM volume performance, do not create a thin-pool when creating an LVM volume #96
- Add parameter `carina.storage.io/allow-pod-migration-if-notready` to storageclass. Webhook will automatically add 
this annotation for POD when SC has this parameter #95
- Nodestorageresource structuring and issue fixing #87
- Remove ConfigMap synchronization control #75
- The Carina E2E test is being refined
- Promote carina into cncf sandbox project and roadmap
- Update outdated documents
- Optimize the container scheduling algorithm to make it more concise and understandable

## [0.11.1] - 2022-09-19

- Repair The pv is lost due to node restart
- Added the upgrade upgrade script
- Helm chat deployment adds psp resources
- It is clear that the current version of carina supports 1.18-1.24
- Planning discussion carina supports the Kubernetes 1.25 solution
- Added e2e unit test scripts