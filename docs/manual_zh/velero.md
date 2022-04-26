> 有没有更适合 k8s 的备份恢复方案？

传统的数据备份方案: 一种是利用存储数据的服务端实现定期快照的备份，另一种是在每台目标服务器上部署专有备份 agent 并指定备份数据目录，定期把数据远程复制到外部存储上。这两种方式均存在“备份机制固化”、“数据恢复慢”等问题，无法适应容器化后的弹性、池化部署场景。我们需要更贴合 k8s 容器场景的备份恢复能力，实现一键备份、快速恢复。



## kubernetes备份恢复之velero

- Velero地址：https://github.com/vmware-tanzu/velero
- Velero属于VMWare开源的Kubernetes集群备份、恢复、迁移工具.

> Velero 是一个云原生的灾难恢复和迁移工具，它本身也是开源的, 采用 Go 语言编写，可以安全的备份、恢复和迁移Kubernetes集群资源和持久卷。

>Velero 是西班牙语，意思是帆船，非常符合 Kubernetes 社区的命名风格。Velero 的开发公司 Heptio，之前已被 VMware 收购，其创始人2014就职于Google，当时被认为是 Kubernetes 核心成员。

> Velero 是一种云原生的Kubernetes优化方法，支持标准的K8S集群，既可以是私有云平台也可以是公有云。除了灾备之外它还能做资源移转，支持把容器应用从一个集群迁移到另一个集群。

> Heptio Velero ( 以前的名字为 ARK) 是一款用于 Kubernetes 集群资源和持久存储卷（PV）的备份、迁移以及灾难恢复等的开源工具。
 使用velero可以对集群进行备份和恢复，降低集群DR造成的影响。velero的基本原理就是将集群的数据备份到对象存储中，在恢复的时候将数据从对象存储中拉取下来。

### 1.  Velero特性
> Velero 目前包含以下特性：

- 支持 Kubernetes 集群数据备份和恢复
- 支持复制当前 Kubernetes 集群的资源到其它 Kubernetes 集群
- 支持复制生产环境到开发以及测试环境

### 2. Velero组件
> Velero 组件一共分两部分，分别是服务端和客户端。

- 服务端：运行在你 Kubernetes 的集群中
- 客户端：是一些运行在本地的命令行的工具，需要已配置好 kubectl 及集群 kubeconfig 的机器上

### 3. 支持备份存储
- AWS S3 以及兼容 S3 的存储，比如：Minio
- Azure BloB 存储
- Google Cloud 存储
- Aliyun OSS 存储(https://github.com/AliyunContainerService/velero-plugin)

### 4. 对比 etcd

- Etcd 是将集群的全部资源备份起来，是备份的快照数据。
- Velero 控制面更细化，可以对 Kubernetes 集群内对象级别进行备份，还可以通过对 Type、Namespace、Label 等对象进行分类备份或者恢复。

### 5. 使用场景
- 灾备场景: 提供备份恢复k8s集群的能力
- 迁移场景: 提供拷贝集群资源到其他集群的能力(复制同步开发、测试、生产环境的集群)

### 6. Velero工作流程
![velero](../img/velero.png)
运行时velero backup create my-backup：

- Velero客户端调用Kubernetes API服务器以创建Backup对象；
- 该BackupController将收到通知有新的Backup对象被创建并执行验证；
- BackupController开始备份过程，它通过查询API服务器以获取资源来收集数据以进行备份；
- BackupController将调用对象存储服务，例如，AWS S3 -上传备份文件。默认情况下，velero backup create支持任何持久卷的磁盘快照，您可以通过指定其他标志来调整快照，运行velero backup create --help可以查看可用的标志，可以使用--snapshot-volumes=false选项禁用快照。
 
### 7. 备份存储位置和卷快照位置
> Velero有两个自定义资源BackupStorageLocation和VolumeSnapshotLocation，用于配置Velero备份及其关联的持久卷快照的存储位置。

- BackupStorageLocation：定义为存储区，存储所有Velero数据的存储区中的前缀以及一组其他特定于提供程序的字段

- VolumeSnapshotLocation：完全由提供程序提供的特定的字段（例如AWS区域，Azure资源组，Portworx快照类型等）定义


### 7. velero 支持的后端存储,备份存储位置和卷快照位置

#### Velero 支持两种关于后端存储的 CRD，分别是

- BackupStorageLocation（对象数据）,主要支持的后端存储是 S3 兼容的存储，存储所有Velero数据的存储区中的前缀以及一组其他特定于提供程序的字段。比如：Mino 和阿里云 OSS 等 ;
- VolumeSnapshotLocation（pv 数据）,主要用来给 PV 做快照，需要云提供商提供插件,完全由提供程序提供的特定的字段（例如AWS区域，Azure资源组，Portworx快照类型等）定义。这个需要使用 CSI 等存储机制。你也可以使用专门的备份工具 Restic

> 注意：Restic 是一款 GO 语言开发的数据加密备份工具，顾名思义，可以将本地数据加密后传输到指定的仓库。支持的仓库有 Local、SFTP、Aws S3、Minio、OpenStack Swift、Backblaze B2、Azure BS、Google Cloud storage、Rest Server。
项目地址：https://github.com/restic/restic

### 8. 缺陷和注意事项
- Velero对每个提供商仅支持一组凭据，如果后端存储使用同一提供者，则不可能在不同的位置使用不同的凭据
- 卷快照仍然受提供商允许您创建快照的位置的限制，不支持跨公有云供应商备份带有卷的集群数据。例如，AWS和Azure不允许您在卷所在的区域中不同的可用区创建卷快照，如果您尝试使用卷快照位置（与集群卷所在的区域不同）来进行Velero备份，则备份将失败。

- 每个Velero备份都只能有一个BackupStorageLocation，VolumeSnapshotLocation，不可能（到目前为止）将单个Velero备份同时发送到多个备份存储位置，或者将单个卷快照同时发送到多个位置。但是，如果跨位置的备份冗余很重要，则始终可以设置多个计划的备份，这些备份仅在所使用的存储位置有所不同。
- 不支持跨提供商快照，如果您的集群具有多种类型的卷，例如EBS和Portworx，但VolumeSnapshotLocation仅配置了EBS，则Velero将仅对EBS卷进行快照。

- 恢复数据存储在主Velero存储桶的prefix/subdirectory下，并在备份创建时将BackupStorageLocationc存储到与用户选择的存储桶相对应的存储桶。

### 9. 使用用例

- 在单个Velero备份中创建不止一种持久卷的快照
```
velero snapshot-location create ebs-us-east-1 \
    --provider aws \
    --config region=us-east-1

velero snapshot-location create portworx-cloud \
    --provider portworx \
    --config type=cloud

velero backup create full-cluster-backup \
    --volume-snapshot-locations ebs-us-east-1,portworx-cloud    
```
- 在不同的地区将备份存储到不同的对象存储桶中
```
velero backup-location create default \
    --provider aws \
    --bucket velero-backups \
    --config region=us-east-1

velero backup-location create s3-alt-region \
    --provider aws \
    --bucket velero-backups-alt \
    --config region=us-west-1

velero backup create full-cluster-alternate-location-backup \
    --storage-location s3-alt-region
```
- 对于公有云提供的存储卷，将一部分快照存储在本地，一部分存储在公有云
```
velero snapshot-location create portworx-local \
    --provider portworx \
    --config type=local

velero snapshot-location create portworx-cloud \
    --provider portworx \
    --config type=cloud

velero backup create cloud-snapshot-backup \
    --volume-snapshot-locations portworx-cloud    
```
- 使用存储位置
```
velero backup-location create default \
    --provider aws \
    --bucket velero-backups \
    --config region=us-west-1

velero snapshot-location create ebs-us-west-1 \
    --provider aws \
    --config region=us-west-1

velero backup create full-cluster-backup
```
### 10. 安装和demo演示

- [安装和示例演示](velero-install.md)

### 使用velero
> 可通过定时和只读备份定期备份集群数据，在集群发生故障或升级失败时及时恢复。

- 定时备份：
```
velero schedule create <SCHEDULE NAME> --schedule "0 7 * * *"

kubectl patch backupstoragelocation <STORAGE LOCATION NAME> \
    --namespace velero \
    --type merge \
    --patch '{"spec":{"accessMode":"ReadOnly"}}'

velero restore create --from-backup <SCHEDULE NAME>-<TIMESTAMP>

kubectl patch backupstoragelocation <STORAGE LOCATION NAME> \
   --namespace velero \
   --type merge \
   --patch '{"spec":{"accessMode":"ReadWrite"}}'
```
- 备份

```
velero backup create <BACKUP-NAME>

velero backup describe <BACKUP-NAME>

velero restore create --from-backup <BACKUP-NAME>

velero restore get

velero restore describe <RESTORE-NAME-FROM-GET-COMMAND>

```
- 过滤备份对象
```
--include-namespaces:备份该命名空间下的所有资源，不包括集群资源

--include-resources:要备份的资源类型

--include-cluster-resources:是否备份集群资源
此选项可以具有三个可能的值：
	true：包括所有群集范围的资源；
	false：不包括群集范围内的资源；
	nil （“自动”或不提供）

--selector:通过标签选择匹配的资源备份

--exclude-namespaces:备份时该命名空间下的资源不进行备份

--exclude-resources:备份时该类型的资源不进行备份

--velero.io/exclude-from-backup=true:当标签选择器匹配到该资源时，若该资源带有此标签，也不进行备份

``` 



- 指定特定种类资源的备份顺序

> 可通过使用–ordered-resources参数，按特定顺序备份特定种类的资源，需要指定资源名称和该资源的对象名称列表，资源对象名称以逗号分隔，其名称格式为“命名空间/资源名称”，对于集群范围资源，只需使用资源名称。映射中的键值对以分号分隔，资源类型是复数形式。
```
velero backup create backupName --include-cluster-resources=true --ordered-resources 'pods=ns1/pod1,ns1/pod2;persistentvolumes=pv4,pv8' --include-namespaces=ns1

velero backup create backupName --ordered-resources 'statefulsets=ns1/sts1,ns1/sts0' --include-namespaces=n
```

### 11. 卸载velero
```
kubectl delete namespace/velero clusterrolebinding/velero
kubectl delete crds -l component=velero
```


### 12 备份hooks
> Velero支持在备份任务执行之前和执行后在容器中执行一些预先设定好的命令。有两种方法可以指定钩子：pod本身的注释声明和在定义Backup任务时的Spec中声明。
- Pre hooks
```
pre.hook.backup.velero.io/container:将要执行命令的容器，默认为pod中的第一个容器,可选的。

pre.hook.backup.velero.io/command:要执行的命令,如果需要多个参数，请将该命令指定为JSON数组。例如：["/usr/bin/uname", "-a"]

pre.hook.backup.velero.io/on-error:如果命令返回非零退出代码如何处理。默认为“Fail”，有效值为“Fail”和“Continue”，可选的。

pre.hook.backup.velero.io/timeout:等待命令执行的时间，如果命令超过超时，则认为该挂钩失败的。默认为30秒，可选的。

```
- Post hooks
```
post.hook.backup.velero.io/container:将要执行命令的容器，默认为pod中的第一个容器,可选的。

post.hook.backup.velero.io/command:要执行的命令,如果需要多个参数，请将该命令指定为JSON数组。例如：["/usr/bin/uname", "-a"]

post.hook.backup.velero.io/on-error:如果命令返回非零退出代码如何处理。默认为“Fail”，有效值为“Fail”和“Continue”，可选的。

post.hook.backup.velero.io/timeout:等待命令执行的时间，如果命令超过超时，则认为该挂钩失败的。默认为30秒，可选的
```
### 13. 还原hooks
> Velero支持还原hooks，可以在还原任务执行前或还原过程之后执行的自定义操作。有以下两种定义形式：

- InitContainer Restore Hooks：这些将在待还原的Pod的应用程序容器启动之前将init容器添加到还原的pod中，以执行任何必要的设置。

```
init.hook.restore.velero.io/container-image:要添加的init容器的容器镜像

init.hook.restore.velero.io/container-name:要添加的init容器的名称

init.hook.restore.velero.io/command:将要在初始化容器中执行的任务或命令
```
如进行备份之前，请使用以下命令将注释添加到Pod：
```
kubectl annotate pod -n <POD_NAMESPACE> <POD_NAME> \
    init.hook.restore.velero.io/container-name=restore-hook \
    init.hook.restore.velero.io/container-image=alpine:latest \
    init.hook.restore.velero.io/command='["/bin/ash", "-c", "date"]'

```
- Exec Restore Hooks：可用于在已还原的Kubernetes pod的容器中执行自定义命令或脚本。
```
post.hook.restore.velero.io/container:;执行hook的容器名称,默认为第一个容器,可选

post.hook.restore.velero.io/command:将在容器中执行的命令,必填

post.hook.restore.velero.io/on-error:如何处理执行失败,有效值为Fail和Continue,默认为Continue,使用Continue模式，仅记录执行失败;使用Fail模式时，将不会在自行其他的hook，还原的状态将为PartiallyFailed,可选

post.hook.restore.velero.io/exec-timeout:开始执行后要等待多长时间,默认为30秒,可选

post.hook.restore.velero.io/wait-timeout:等待容器准备就绪的时间,该时间应足够长，以使容器能够启动，并
```
如进行备份之前，请使用以下命令将注释添加到Pod
```
kubectl annotate pod -n <POD_NAMESPACE> <POD_NAME> \
    post.hook.restore.velero.io/container=postgres \
    post.hook.restore.velero.io/command='["/bin/bash", "-c", "psql < /backup/backup.sql"]' \
    post.hook.restore.velero.io/wait-timeout=5m \
    post.hook.restore.velero.io/exec-timeout=45s \
    post.hook.restore.velero.io/on-error=Continue
```

### 14.  问题集锦
#### 14.1 Velero可以将资源还原到与其备份来源不同的命名空间中。

答： 可以使用--namespace-mappings参数来指定：
```
velero restore create RESTORE_NAME \
  --from-backup BACKUP_NAME \
  --namespace-mappings old-ns-1:new-ns-1,old-ns-2:new-ns-2
```
#### 14.2  执行还原操作后，已有的NodePort类型的service如何处理

答： Velero有一个参数，可让用户决定保留原来的nodePorts。

velero restore create子命令具有 --preserve-nodeports标志保护服务nodePorts。此标志用于从备份中保留原始的nodePorts，可用作--preserve-nodeports或--preserve-nodeports=true
如果给定此标志，则Velero在还原Service时不会删除nodePorts，而是尝试使用备份时写入的nodePorts。

### 15 相关的CRD资源信息
- Backup
- Restore
- Schedule
- BackupStorageLocation
- VolumeSnapshotLocation