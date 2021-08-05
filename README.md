
#### Carina

- carina 是一个CSI插件，在Kubernetes集群中提供本地存储持久卷
- 项目状态：开发测试中
- CSI Version: 1.3.0

#### Carina architecture

![carina-arch](docs/img/carina.png)

#### 支持的环境

- Kubernetes：1.20 1.19 1.18
- Node OS：Linux
- Filesystems：ext4，xfs

#### 支持功能

| Carina功能 | 是否支持 |
| ---------- | -------- |
| 动态pv     | 支持     |
| 文件存储   | 支持     |
| 块存储     | 支持     |
| 容量限制   | 支持     |
| 自动扩容   | 支持     |
| 快照       | 不支持   |
| 拓扑       | 支持     |

#### 项目结构

- carina-controller：CSI controller service
- carina-scheduler：custom scheduler
- carina-node：CSI node service

#### 开始

- 目录`deploy/kubernetes` 执行 `./deploy.sh`进行部署，`./deploy.sh uninstall`进行卸载
- 详细部署及使用参考`docs/使用手册.md`

#### 文档

- docs目录包含设计及使用文档

#### 构建镜像

- 使用命令`make release`将构建镜像
