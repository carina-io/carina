# Carina 
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/carina-io/carina/blob/main/LICENSE)
<img src="https://user-images.githubusercontent.com/88021699/130191822-50112078-fad9-4516-b76b-8b7566d1bfc3.jpg" width="50%">


> 中文 | [English](README.md)


Carina 是一款基于 Kubernetes CSI 标准实现的存储插件，用户可以使用标准的 storageClass/PVC/PV 原语申请 Carina 提供的存储介质；Carina包含三个主要组件：carina-scheduler、carina-controller以及carina-node，全部以容器化形式运行在Kubernetes中，并且占用极少的资源。Carina是为数据库而生的本地存储方案，编排管理本地磁盘并根据磁盘类型构建多种资源池，为数据库等应用提供极致性能的本地存储。

**Carina致力于为云原生数据库提供高性能、免运维的存储系统，并成为云原生时代数据库存储领域的DBA专家！**


# 支持环境

- Kubernetes：(CSI_VERSION=1.5.0)
- Node OS：Linux
- Filesystems：ext4，xfs

- 如果kubelet以容器化方式运行，需要挂载主机/dev:/dev目录
- 集群每个节点存在1..N块裸盘，支持SSD和HDD磁盘（可使用命令lsblk --output NAME,ROTA查看磁盘类型，ROTA=1为HDD磁盘 ROTA=0为SSD磁盘）
- 节点单块裸盘容量需要大于10G
- 如果服务器不支持bcache内核模块，参考[FAQ](docs/manual_zh/FAQ.md)，修改部署yaml

### carina版本支持范围
| kubernetes | v0.9   | v0.9.1 | v0.10  | v0.11.0  | v1.0   |
| ---------- | ------ | ------ | ------ | -------- | ------ |
| >=1.18     | 支持   | 支持   | 支持   | 支持     | 未发布 |
| >=1.25     | 不支持 | 不支持 | 不支持 | 实验性质 | 未发布 |

# 总体架构

![carina-arch](docs/img/architecture.png)

如上图架构所示，carina 能够自动发现本地裸盘，并根据其磁盘特性划分为 hdd 磁盘卷组及 ssd 磁盘卷组等，针对于本地数据高可用，carina 推出了基于 bcache 的磁盘缓存功能以及自动组建 RAID 功能.

- carina-node 是运行在每个节点上的 agent 服务，利用 lvm 技术管理本地磁盘，按照类别将本地磁盘划分到不同的 VG 中，并从中划分 LV 提供给 Pod 使用.
- carina-scheduler 是 Kubernetes 的调度插件，负责基于申请的 PV 大小、节点剩余磁盘空间大小，节点负载使用情况进行合理的调度。默认提供了 spreadout 及 binpack 两种调度策略.
- carina-controller 是 carina 的控制平面，监听 PVC 等资源，维护 PVC、LV 之间的关系


# 功能列表

- [磁盘管理](docs/manual_zh/disk-manager.md)
- [设备注册](docs/manual_zh/device-register.md)
- [基于文件系统使用](docs/manual_zh/pvc-xfs.md)
- [基于块设备使用](docs/manual_zh/pvc-device.md)
- [pvc扩容](docs/manual_zh/pvc-expand.md)
- [基于容量的调度](docs/manual_zh/capacity-scheduler.md)
- [卷拓扑](docs/manual_zh/topology.md)
- [磁盘缓存使用](docs/manual_zh/pvc-bcache.md)
- [raid管理](docs/manual_zh/raid-manager.md)
- [容灾转移](docs/manual_zh/failover.md)
- [磁盘限速](docs/manual_zh/disk-speed-limit.md)
- [指标监控](docs/manual_zh/metrics.md)
- [API](docs/manual_zh/api.md)


# 快速开始

- 快速部署

## 使用shell

- 该部署方式，部署的镜像TAG为latest，如果要部署指定版本carina需要更改镜像地址

```shell
$ cd deploy/kubernetes
# 安装，默认安装在kube-system
$ ./deploy.sh

# 卸载
$ ./deploy.sh uninstall
```

## 使用helm3

- 支持安装指定版本carina

```bash
helm repo add carina-csi-driver https://carina-io.github.io

helm search repo -l carina-csi-driver

helm install carina-csi-driver carina-csi-driver/carina-csi-driver --namespace kube-system --version v0.11.0
```

- [部署文档](docs/manual_zh/install.md)
- 详细部署及使用参考[使用手册](docs/user-guide.md)

## Carina 升级

- 先卸载老版本`./deploy.sh uninstall`,然后安装新版本`./deploy.sh`(卸载carina并不会影响存储卷的使用)


# 开发指南

- [开发文档](docs/manual_zh/development.md)
- [构建运行时容器](docs/manual_zh/runtime-container.md)


# 博客

* [blogs](http://www.opencarina.io/blog)

# 路线图

* [路线图](docs/roadmap/roadmap.md)


# 常见存储方案对比

|            | NFS/NAS  | SAN  | Ceph   | Carina    |
| ---------- | -------- | ---- | ------ | ----------|
| 设计场景   | 通用存储场景               | 高性能块设备                                | 追求扩展性的通用存储场景                   | 为云数据库而生的高性能块存储                                 |
| 文件存储   | 支持                       | 支持                                        | 支持                                       | 支持                                                         |
| 块存储     | 不支持                     | 视驱动程序而定                              | 支持                                       | 支持                                                         |
| 文件系统   | 不支持格式化               | 视驱动程序而定                              | 支持ext4/xfs等                             | 支持ext4/xfs等                                               |
| 宽带       | 差/中等                    | 中等                                        | 高                                         | 高                                                           |
| IOPS       | 差/中等                    | 高                                          | 中等                                       | 高                                                           |
| 延迟       | 差/中等                    | 低                                          | 差                                         | 低                                                         |
| CSI支持    | 支持                       | 支持                                        | 支持                                       | 支持                                                         |
| 快照       | 不支持                     | 视驱动程序而定                              | 支持                                       | 不支持                                                       |
| 克隆       | 不支持                     | 视驱动程序而定                              | 支持                                       | 不支持                                                       |
| 配额       | 不支持                     | 支持                                        | 支持                                       | 支持                                                         |
| 扩容       | 支持                       | 支持                                        | 支持                                       | 支持                                                         |
| 数据高可用 | 依赖RAID或NAS设备          | 支持                                        | 支持                                       | 依赖RAID                                                     |
| 可维护性   |                            | 不同的SAN设备需要不同的驱动程序，管理成本高 | 架构复杂，需要专人维护                     | 高                                                           |
| 成本       | NFS服务器或NAS设备，成本高 | SAN设备，客户端配置HBA卡，成本高            | 专用存储集群，客户端需配置存储网卡，成本高 | K8s集群中剩余的本地磁盘，成本低                              |
| 其他特性   | 容器迁移后数据跟随         | 容器迁移后数据跟随                          | 支持对象存储，容器迁移后数据跟随           | * 支持binpack/spreadout等调度策略<br>* 针对有状态容器，支持原地重启、重建<br>* 容器迁移后，数据不能跟随，需要应用层面实现数据恢复 |


# 同类型存储项目

- [topolvm](https://github.com/topolvm/topolvm)
- [官方csi-driver-host-path](https://github.com/kubernetes-csi/csi-driver-host-path)
- [local-path-provisioner](https://github.com/rancher/local-path-provisioner)
- [openebs](https://openebs.io/)

# FAQ
- [FAQ](docs/manual_zh/FAQ.md)

# 社区
- 微信用户扫码进入社区交流群

![carina-wx](docs/img/carina-wx.png)

# License
Carina is under the Apache 2.0 license. See the [LICENSE](https://github.com/FabEdge/fabedge/blob/main/LICENSE) file for details.
