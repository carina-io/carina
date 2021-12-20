# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](https://www.opencarina.io).

## [Unreleased]

## [0.9.1] - 2021-12-17

### Added

- add helm chart to deploy
- when node is notready, migrate pods to ready node if pod annotations contains "carina.io/rebuild-node-notready: true" (<https://github.com/carina-io/carina/issues/14>))
- Multiple VGS are supported for the same type of storage

### Changed

- csi configmap  change new version to support mutil vgroup (<https://github.com/carina-io/carina/issues/10>)

### Fixed

- Fixes configmap  spradout修改为spreadout(<https://github.com/carina-io/carina/issues/12>)
