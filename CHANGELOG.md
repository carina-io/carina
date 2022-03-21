# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](https://www.opencarina.io).

## [Unreleased]

## [v1.0.0] - 2020-04-x

- Removed csu.proto upgrade CSI_VERSION=1.5
- Remove device registration and use the CRD resource NodeStorageResource instead
- Added controllers that maintain NodeStorageResource
- The scheduler supports fetching resources from NodeStorageResource
- Upgrade go.mod to rely on K8s1.23
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


