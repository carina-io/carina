#### 1.  简介

- 该项目为云原生本地存储项目，支持在Kubernetes集群内容器挂载本地磁盘进行读写
- 该文档主要用于介绍项目部署及使用

#### 2. 项目部署

##### 2.1 部署要求

- Kubernetes 集群（CSI_VERSION=1.5.0）
- 如果kubelet以容器化方式运行，需要挂载主机`/dev:/dev`目录
- Linux Kernel 3.10.0-1160.11.1.el7.x86_64 (在此版本进行的测试，低于此版本也是可以的)
- 集群每个节点存在1..N块裸盘，支持SSD和HDD磁盘（可使用命令`lsblk --output NAME,ROTA`查看磁盘类型，ROTA=1为HDD磁盘 ROTA=0为SSD磁盘）

##### 2.2 执行部署

- 项目部署，可用`kubectl get pods -n kube-system | grep carina`命令查看部署进度

  ```shell
  $ cd deploy/kubernetes
  $ ./deploy.sh
  
  $ kubectl get pods -n kube-system |grep carina
  carina-scheduler-6cc9cddb4b-jdt68         0/1     ContainerCreating   0          3s
  csi-carina-node-6bzfn                     0/2     ContainerCreating   0          6s
  csi-carina-node-flqtk                     0/2     ContainerCreating   0          6s
  csi-carina-provisioner-7df5d47dff-7246v   0/4     ContainerCreating   0          12s
  ```

- 项目卸载

  ```shell
  $ cd deploy/kubernetes
  $ ./deploy.sh uninstall
  ```

- 注意事项

  - 安装卸载该服务，对已经挂载到容器内使用的volume卷无影响

#### 3. 项目介绍

##### 3.1  项目介绍

- 该项目主要是基于Kubernetes，提供给需要高性能本地磁盘的应用，例如数据库服务
- 节点服务启动时会自动将节点上磁盘按照SSD和HDD进行分组并组建成vg卷组
- 支持文件存储及块设备存储，其中文件存储支持xfs和ext4格式
- 文件存储和块设备存储均支持在线扩容

##### 3.2  使用详情

- 基于Kubernetes CSI进行设计开发，使用常规的`storageclass、pvc`即可创建volume卷

- 下边我们结合`storageclass`和`pvc`进行讲解使用细节

  ```yaml
  apiVersion: storage.k8s.io/v1
  kind: StorageClass
  metadata:
    name: csi-carina-sc
  provisioner: carina.storage.io # 这是该CSI驱动的名称，不允许更改
  parameters:
    # 这是kubernetes内置参数，我们支持xfs,ext4两种文件格式，如果不填则默认ext4
    csi.storage.k8s.io/fstype: xfs
    # 这是选择磁盘分组，该项目会自动将SSD及HDD磁盘分组
    # SSD：ssd HDD: hdd
    # 如果不填会随机选择磁盘类型
    carina.storage.io/disk-group-name: hdd
  reclaimPolicy: Delete
  allowVolumeExpansion: true # 支持扩容，定为true便可
  # WaitForFirstConsumer表示被容器绑定调度后再创建pv
  volumeBindingMode: WaitForFirstConsumer
  # 支持挂载参数设置，默认为空
  # 如果没有特殊的需求，为空便可满足大部分要求
  mountOptions:
    - rw
  ```

  - 备注1：`volumeBindingMode` 支持`Immediate`参数，但是对于本地存储通常设置为`WaitForFirstConsumer`
  - 备注2：`storageclass`支持`allowedTopologies`，只有在`volumeBindingMode:Immediate` 模式下生效

  ```yaml
  apiVersion: v1
  kind: PersistentVolumeClaim
  metadata:
    name: csi-carina-pvc
    namespace: carina
  spec:
    accessModes:
      - ReadWriteOnce # 本地存储只能被一个节点上Pod挂载
    resources:
      requests:
        storage: 7Gi
    storageClassName: csi-carina-sc
    volumeMode: Filesystem # block便会创建块设备
  ```

- 创建bcache类型磁盘并使用，如果要创建bcache类型磁盘需要在storageclass中设置特殊的参数

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-carina-sc
provisioner: carina.storage.io
parameters:
  # file system
  csi.storage.k8s.io/fstype: xfs
  # 数据存储磁盘类型
  carina.storage.io/backend-disk-group-name: hdd
  # 缓存磁盘类型
  carina.storage.io/cache-disk-group-name: ssd
  # 1-100 Cache Capacity Ratio
  # 假设pvc为10G，则会创建5G的ssd类型的缓存盘
  carina.storage.io/cache-disk-ratio: "50"
  # 三种缓存模式writethrough/writeback/writearound
  carina.storage.io/cache-policy: writethrough
reclaimPolicy: Delete
allowVolumeExpansion: true
# WaitForFirstConsumer表示被容器绑定调度后再创建pv
volumeBindingMode: WaitForFirstConsumer
mountOptions:
```

备注①：观察上述配置实际是hdd作为后端存储盘，ssd作为缓存盘，缓存比率为50%，也就是说如果`pvc.spec.resource.request.storage=10G`则会创建一个10G的hdd存储卷和一个5G的ssd存储卷，ssd存储卷作为hdd卷的缓存使用

备注②：缓存有三种模式分别为`writethrough/writeback/writearound`

备注③：由于bcache技术限制，当pvc扩容后需pod重启后才会生效，正常存储卷无需pod重启即刻生效

备注④：可以使用命令`echo 0 > /sys/block/loop0/queue/rotational`将loop0设置模拟成ssd磁盘


##### 3.3 项目测试

- 测试关注点：

  - ①每个节点服务启动时会自动将SSD及HDD裸盘添加到VG卷组

  - ②每隔五分钟扫描本地磁盘，如果有新发现的裸盘则会自动加入VG卷组，扫描时间可配置最少五分钟

  - ③节点服务启动后会将磁盘容量信息存储到`Custom Resource nsr` 可使用如下命令查看

    ```shell
    $ kubectl get nsr
    NAME          NODE           TIME
    k8s-mater     192.168.1.2     5m
    k8s-node      192.168.1.3     5s
    ```

  - ④项目启动时配置文件

    ```sh
    $ kubectl get configmap carina-csi-config -n kube-system
    NAME                DATA   AGE
    carina-csi-config   1      116m
    ```

    - kube-system/carina-csi-config

    ```yaml
    config.json: |-
      {
       "diskSelector": [
         {
           "name": "exist-vg-group",
           "re": ["loop4+"],
           "policy": "LVM",
           "nodeLabel": "kubernetes.io/hostname"
         },
         {
           "name": "new-vg-group",
           "re": ["loop5+"],
           "policy": "LVM",
           "nodeLabel": "kubernetes.io/hostname"
         },
         {
           "name": "raw",
           "re": ["vdb+", "sd+"],
           "policy": "RAW",
           "nodeLabel": "kubernetes.io/hostname"
         }
       ],
       "diskScanInterval": "300",
       "schedulerStrategy": "spreadout"
     }
    ```

    - 备注1：`diskSelector`若是A磁盘已经加入了VG卷组，修改为不在匹配A盘，如果该盘尚未使用则会在VG卷组中移除该磁盘
    - 备注2：`schedulerStrategy`中`binpack`为pv选择磁盘容量刚好满足`requests.storage`的节点 ，`spreadout`为pv选择磁盘剩余容量最多的节点
    - 备注3：`schedulerStrategy`在`storageclass volumeBindingMode:Immediate`模式中选择只受磁盘容量影响，即在`spreadout`策略下Pvc创建后会立即在剩余容量最大的节点创建volume
    - 备注4：`schedulerStrategy`在`storageclass volumeBindingMode:WaitForFirstConsumer`模式pvc受pod调度影响，它影响的只是调度策略评分，这个评分可以通过自定义调度器日志查看`kubectl logs -f carina-scheduler-6cc9cddb4b-jdt68 -n kube-system`
    - 备注5：当多个节点磁盘容量大于请求容量10倍，则这些节点的调度评分是相同的

  - ⑥关于TODO（`topologyKey: topology.carina.storage.io/node`）使用方法参考`examples/kubernetes/topostatefulset.yaml`

- 执行测试

  - 可执行测试脚本进行基本的功能验证

    ```shell
    $ cd examples/kubernetes
    $ ./test.sh help
    -------------------------------
    ./test.sh           ===> install all test yaml
    ./test.sh uninstall ===> uninstall all test yaml
    ./test.sh expand    ===> expand all pvc
    ./test.sh scale     ===> scale statefulset replicas
    ./test.sh delete    ===> delete all pod
    ./test.sh exec      ===> show pod filesystem
    ```

- 测试细节

  - 该项目有一个CRD资源，可使用命令`kubectl get lv`查看

    ```shell
    $ kubectl get lv
    NAME                                       SIZE   GROUP           NODE          STATUS
    pvc-319c5deb-f637-423b-8b52-42ecfcf0d3b7   7Gi    carina-vg-hdd   10.20.9.154   Success
    pvc-5b3703d8-f262-48e3-818f-dfbc35c67d90   3Gi    carina-vg-hdd   10.20.9.154   Success
    ```

    - 该资源和PVC / PV一一对应，可以快速查看volume是否创建成功，这里状态success便表示lvm卷已经创建成功

  - 查看创建的lvm卷

    ```shell
    $  kubectl exec -it csi-carina-node-cmgmm -c csi-carina-node -n kube-system bash
    $ pvs
      PV         VG            Fmt  Attr PSize   PFree  
      /dev/vdc   carina-vg-hdd lvm2 a--  <80.00g <79.95g
      /dev/vdd   carina-vg-hdd lvm2 a--  <80.00g  41.98g
    $ vgs
      VG            #PV #LV #SN Attr   VSize   VFree   
      carina-vg-hdd   2  10   0 wz--n- 159.99g <121.93g
    $ lvs
      LV                                              VG            Attr       LSize  Pool                                          Origin Data%  Meta%  Move Log Cpy%Sync Convert
      thin-pvc-319c5deb-f637-423b-8b52-42ecfcf0d3b7   carina-vg-hdd twi-aotz--  7.00g                                                      0.15   10.79                                                     
      volume-pvc-319c5deb-f637-423b-8b52-42ecfcf0d3b7 carina-vg-hdd Vwi-aotz--  7.00g thin-pvc-319c5deb-f637-423b-8b52-42ecfcf0d3b7        0.15                          
    ```

    - 特别说明：如果在集群节点上安装了lvm2服务，在节点上执行`lvs`命令看到的卷组可能与容器内不同，这是由于节点lvm缓存所致，执行`lvscan`刷新节点缓存便可以了
    - 每一个pv对应一个`thin pool`和lvm卷，可以观察到卷名称为`volume-`和`pv name`组成

##### 高级功能

- 磁盘限速

  - 支持设备限速，在pod annotations中添加如下注解（参考examples/kubernetes/deploymentspeedlimit.yaml）

    ```yaml
        metadata:
          annotations:
            carina.storage.io/blkio.throttle.read_bps_device: "10485760"
            carina.storage.io/blkio.throttle.read_iops_device: "10000"
            carina.storage.io/blkio.throttle.write_bps_device: "10485760"
            carina.storage.io/blkio.throttle.write_iops_device: "100000"
     ---
     # 该annotations会被设置到如下文件
     /sys/fs/cgroup/blkio/blkio.throttle.read_bps_device
     /sys/fs/cgroup/blkio/blkio.throttle.read_iops_device
     /sys/fs/cgroup/blkio/blkio.throttle.write_bps_device
     /sys/fs/cgroup/blkio/blkio.throttle.write_iops_device
    ```

    - 备注1：支持设置一个或多个annontation，增加或移除annontation会在一分钟内同步到cgroup
    - 备注2：只支持块设备直连读写磁盘限速，测试命令`dd if=/dev/zero of=out.file bs=1M count=512 oflag=dsync`
    - 备注3：使用的cgroup v1，由于cgroup v1本身缺陷无法限速buffer io，目前很多组件依赖cgroup v1尚未切换到cgroup v2
    - 备注4：已知在kernel 3.10下直连磁盘读写可以限速，在kernel 4.18版本无法限制buffer io 

- 清理孤儿卷

  - 每十分钟会遍历本地volume，然后检查k8s中是否有对应的logicvolume，若是没有则删除本地volume

  - 每十分钟会遍历k8s中logicvolume，然后检查logicvolume是否有对应的pv，若是没有则删除logicvolume

  - 当节点被删除时，在这个节点的上的所有volume将在其他节点重建

    ```shell
    # 示例
    $ kubectl get lv
    NAME                                       SIZE   GROUP           NODE          STATUS
    pvc-177854eb-f811-4612-92c5-b8bb98126b94   5Gi    carina-vg-hdd   10.20.9.154   Success
    pvc-1fed3234-ff89-4c58-8c65-e21ca338b099   5Gi    carina-vg-hdd   10.20.9.153   Success
    pvc-527b5989-3ac3-4d7a-a64d-24e0f665788b   10Gi   carina-vg-hdd   10.20.9.154   Success
    pvc-b987d27b-39f3-4e74-9465-91b3e6b13837   3Gi    carina-vg-hdd   10.20.9.154   Success
    
    $ kubectl delete node 10.20.9.154
    # volume进行重建，重建会丢失原先的volume数据
    $ kubectl get lv
    NAME                                       SIZE   GROUP           NODE          STATUS
    pvc-177854eb-f811-4612-92c5-b8bb98126b94   5Gi    carina-vg-hdd   10.20.9.153   Success
    pvc-1fed3234-ff89-4c58-8c65-e21ca338b099   5Gi    carina-vg-hdd   10.20.9.153   Success
    pvc-527b5989-3ac3-4d7a-a64d-24e0f665788b   10Gi   carina-vg-hdd   10.20.9.153   Success
    pvc-b987d27b-39f3-4e74-9465-91b3e6b13837   3Gi    carina-vg-hdd   10.20.9.153   Success
    ```

- 指标监控

  - carina-node 为host网络模式部署并监听 `8080`端口，其中8080为metrics，可通过如下配置进行修改

    ```shell
            - "--metrics-addr=:8080"
    ```

    备注：若是修改监听端口，务必同步修改`service：csi-carina-node`

  - carina-controller 监听`8080 8443`，其中8080为metrics、8443为webhook，可通过如下配置进行修改

    ```shell
            - "--metrics-addr=:8080"
            - "--webhook-addr=:8443"
    ```

  - carina 指标

  | 指标                                           | 描述                   |
  | ---------------------------------------------- | ---------------------- |
  | carina_scrape_collector_duration_seconds       | 收集器持续时间         |
  | carina_scrape_collector_success                | 收集器成功次数         |
  | carina_volume_group_stats_capacity_bytes_total | vg卷组容量             |
  | carina_volume_group_stats_capacity_bytes_used  | vg卷组使用量           |
  | carina_volume_group_stats_lv_total             | 节点lv数量             |
  | carina_volume_group_stats_pv_total             | 节点pv数量             |
  | carina_volume_stats_reads_completed_total      | 成功读取的总数         |
  | carina_volume_stats_reads_merged_total         | 合并的读的总数         |
  | carina_volume_stats_read_bytes_total           | 成功读取的字节总数     |
  | carina_volume_stats_read_time_seconds_total    | 所有读花费的总秒数     |
  | carina_volume_stats_writes_completed_total     | 成功完成写的总数       |
  | carina_volume_stats_writes_merged_total        | 合并写的数量           |
  | carina_volume_stats_write_bytes_total          | 成功写入的总字节数     |
  | carina_volume_stats_write_time_seconds_total   | 所有写操作花费的总秒数 |
  | carina_volume_stats_io_now                     | 当前正在处理的I/O秒数  |
  | carina_volume_stats_io_time_seconds_total      | I/O花费的总秒数        |

- carina 提供了丰富的存储卷指标，kubelet本身也暴露的 PVC 容量等指标，在 Grafana Kubernetes 内置视图，可以看到此模板。注意具体 PVC 存储容量指标只有当该 PVC 被使用并且挂载到该节点时才会显示
    

#### 4. 答疑

- ①该项目总共有哪些组件，各个组件功能职责是怎么样的

  - 共有三个组件，carina-scheduler、carina-controller、carina-node

  - carina-scheduler：自定义调度器，凡是Pod绑定了由该驱动提供服务的Pvc，则由该调度器进行调度
  - carina-controller：负责监听pvc创建等，当pv调度到指定节点，则创建CRD（logicvolume）
  - carina-node：负责管理本地磁盘节点，并监听CRD（logicvolume），管理本地lvm卷
  - 通过查看各个服务日志，可以获取详细的服务运行信息

- ②pv创建成功后，还能进行Pod迁移吗

  - pv一旦创建成功，Pod只能运行在该节点，无论重启还是删除重建
  - 如果要在节点损坏后，迁移pv请使用节点故障转移功能 

- ③如何让Pod和PVC在指定节点运行

  - 在pod `spec.nodeName`指定节点名称将跳过调度器
  - 对于`WaitForFirstConsumer`策略的StorageClass，在PVC Annotations增加 `volume.kubernetes.io/selected-node: nodeName`可指定pv调度节点
  - 除非明确知道该方式的应用场景，否则不建议直接修改Pvc

- ④k8s节点删除，应如何处理调度到节点上的pv

  - 如果确定volume卷不在使用，直接删除pvc在重建便可

- ⑤如何创建磁盘以方便测试

  - 可使用如下方法创建`loop device`

  ```shell
  for i in $(seq 1 10); do
    truncate --size=200G /tmp/disk$i.device && \
    losetup -f /tmp/disk$i.device
  done
  ```

  
