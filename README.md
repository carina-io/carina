<img src="https://user-images.githubusercontent.com/88021699/130732359-4e7686a9-3010-4142-971d-b65498d9c911.jpg" width="50%">

# Carina

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/carina-io/carina/blob/main/LICENSE)

[![OpenSSF Best Practices](https://bestpractices.coreinfrastructure.org/projects/6908/badge)](https://bestpractices.coreinfrastructure.org/projects/6908)

> English | [中文](README_zh.md)

## Background

Storage systems are complex! There are more and more kubernetes native storage systems nowadays and stateful applications are shifting into cloud native world, for example, modern databases and middlewares. However, both modern databases and its storage providers try to solve some common problems in their own way. For example, they both deal with data replications and consistency. This introduces a giant waste of both capacity and performance and needs more mantainness effort. And besides that, stateful applications strive to be more peformant, eliminating every possible latency, which is unavoidable for modern distributed storage systems. Enters carina.

Carina is a standard kubernetes CSI plugin. Users can use standard kubernetes storage resources like storageclass/PVC/PV to request storage media. The key considerations of carina includes:

* Workloads need different storage systems. Carina will focus on cloudnative database scenario usage only.
* Completely kubernetes native and easy to install.
* Using local disks and group them as needed, user can provison different type of disks using different storage class.
* Scaning physical disks and building a RAID as required. If disk fails, just plugin a new one and it's done.
* Node capacity and performance aware, so scheduling pods more smartly.
* Extremly low overhead. Carina sit besides the core data path and provide raw disk performance to applications.
* Auto tiering. Admins can configure carina to combine the large-capacity-but-low-performant disk and small-capacity-but-high-performant disks as one storageclass, so user can benifit both from capacity and performance.
* If nodes fails, carina will automatically detach the local volume from pods thus pods can be rescheduled.
* Middleware runs on baremetals for decades. There are many valueable optimizations and enhancements which are definitely not outdated  even in cloudnative era. Let carina be an DBA expert of the storage domain for cloudnative databases!


**In short, Carina strives to provide extremely-low-latency and noOps storage system for cloudnative databases and be DBA expert of the storage domain in cloudnative era!**

# Running Environments

* Kubernetes：(CSI_VERSION=1.5.0)
* Node OS：Linux
* Filesystems：ext4，xfs

* If Kubelet is running in containerized mode, you need to mount the host /dev:/dev directory
* Each node in the cluster has 1..N Bare disks, supporting SSDS and HDDS. (You can run the LSBLK --output NAME,ROTA command to view the disk type. If ROTA=1 is HDD,ROTA =0 is SSD.)
* The capacity of a raw disk must be greater than 10 GB
* If the server does not support the bcache kernel module, see [FAQ](docs/manual/FAQ.md), Modify yamL deployment

### Kubernetes compatiblity
| kubernetes | v0.9       | v0.9.1     | v0.10      | v0.11.0      | v1.0        |
| ---------- | ---------- | ---------- | ---------- | ------------ | ----------- |
| >=1.18     | support    | support    | support    | support      | not released |
| >=1.25     | nonsupport | nonsupport | nonsupport | experimental | not released |

# Carina architecture

Carina is built for cloudnative stateful applications with raw disk performance and ops-free maintainess. Carina can scan local disks and classify them by disk types， for example, one node can have 10 HDDs and 2 SSDs. Carina then will group them into different disk pools and user can request different disk type by using different storage class. For data HA, carina now leverages STORCLI to build RAID groups.

![carina-arch](docs/img/architecture.png)

# Carina components

It has three componets: carina-scheduler, carina-controller and carina-node.

* carina-scheduler is an kubernetes scheduler plugin, sorting nodes based on the requested PV size、node's free disk space and node IO perf stats. By default, carina-scheduler supports binpack and spreadout policies.
* carina-controller is the controll plane of carina, which watches PVC resources and maintain the internal logivalVolume object.
* carina-node is an agent which runs on each node. It manage local disks using LVM.

# Features

* [disk management](docs/manual/disk-manager.md)
* [device registration](docs/manual/device-register.md)
* [volume mode: filesystem](docs/manual/pvc-xfs.md)
* [volume mode: block](docs/manual/pvc-device.md)
* [PVC resizing](docs/manual/pvc-expand.md)
* [scheduing based on capacity](docs/manual/capacity-scheduler.md)
* [volume tooplogy](docs/manual/topology.md)
* [PVC autotiering](docs/manual/pvc-bcache.md)
* [RAID management](docs/manual/raid-manager.md)
* [failover](docs/manual/failover.md)
* [io throttling](docs/manual/disk-speed-limit.md)
* [metrics](docs/manual/metrics.md)
* [API](docs/manual/api.md)

# Quickstart

## Install by shell

- In this deployment mode, the image TAG is Latest. If you want to deploy a specific version of Carina, you need to change the image address

```shell
$ cd deploy/kubernetes
# install， The default installation is kube-system.
$ ./deploy.sh

# uninstall
$ ./deploy.sh uninstall
```

## Install by helm3

- Support installation of specified versions of Carina

```bash
helm repo add carina-csi-driver https://carina-io.github.io

helm search repo -l carina-csi-driver

helm install carina-csi-driver carina-csi-driver/carina-csi-driver --namespace kube-system --version v0.11.0
```

* [deployment guide](docs/manual/install.md)
* [user guide](docs/user-guide.md)

## Upgrading

- Uninstall the old version `./deploy.sh uninstall` and then install the new version `./deploy.sh` (uninstalling carina will not affect volume usage)

# Contribution Guide

* [development guide](docs/manual/development.md)
* [build local runtime](docs/manual/runtime-container.md)

# Blogs

* [blogs](http://www.opencarina.io/blog)

# Roadmap

* [roadmap](docs/roadmap/roadmap.md)

# Typical storage providers

|            | NFS/NAS | SAN | Ceph | Carina |
| ---------- | --------| ----| -----| -------|
| typical usage | general storage   | high performance block device  |  extremly scalability  | high performance block device for cloudnative applications |
| filesystem | yes    | yes  | yes  | yes    |
| filesystem type | NFS | driver specific  | ext4/xfs | ext4/xfs |
| block | no | yes | yes | yes |
| bandwidth | standard | standard | high | high |
| IOPS | standard | high | standard | high |
| latency | standard | low | standard | low |
| CSI support| yes | yes | yes | yes |
| snapshot | no | driver specific| yes | no|
| clone | no | driver specific | yes | not yet, comming soon |
| quota| no | yes | yes | yes |
| resizing | yes | driver specific | yes | yes |
| data HA | RAID or NAS appliacne | yes | yes | RAID |
| ease of maintainess |   driver specific | multiple drivers for multiple SAN | high maintainess effort | ops-free |
| budget | high for NAS | high | high | low, using the extra disks in existing kubernetes cluster |
| others | data migrates with pods | data migrates with pods | data migrates with pods |*binpack or spreadout scheduling policy <br>* data doesn't migrate with pods  <br> * inplace rebulid if pod fails |

# FAQ

- [FAQ](docs/manual/FAQ.md)

# Similar projects

* [openebs](https://openebs.io/)
* [topolvm](https://github.com/topolvm/topolvm)
* [csi-driver-host-path](https://github.com/kubernetes-csi/csi-driver-host-path)
* [local-path-provisioner](https://github.com/rancher/local-path-provisioner)

# Known Users
Welcome to register the company name in [ADOPTERS.md](ADOPTERS.md)

![bocloud](static/bocloud.png)

# Community

- For wechat users

![carina-wx](docs/img/carina-wx.png)

# License

Carina is under the Apache 2.0 license. See the [LICENSE](https://github.com/FabEdge/fabedge/blob/main/LICENSE) file for details.

# Code of Conduct

Please refer to our [Carina Community Code of Conduct](https://github.com/carina-io/community/blob/main/code-of-conduct.md)
