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
