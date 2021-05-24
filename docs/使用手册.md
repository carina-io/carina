#### 1.  简介

- 该项目为云原生本地存储项目，支持在Kubernetes集群内容器挂载本地磁盘进行读写
- 该文档主要用于介绍项目部署及使用

#### 2. 项目部署

##### 2.1 部署要求

- Kubernetes 集群（已验证版本1.18.2，1.19.4， 1.20.4）
- 如果kubelet以容器化方式运行，需要挂载主机`/dev`目录
- Linux Kernal >= 3.10.0-1160.11.1.el7.x86_64
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
  ---
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
    carina.storage.io/disk-type: hdd
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
  ---
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

##### 3.3 项目测试

- 测试关注点：

  - ①每个节点服务启动时会自动将SSD及HDD裸盘添加到VG卷组

  - ②每隔五分钟扫描本地磁盘，如果有新发现的裸盘则会自动加入VG卷组，扫描时间可配置最少五分钟

  - ③节点服务启动后会将磁盘容量信息存储到`node.status.capacity` 可使用如下命令查看

    ```shell
    $ kubectl get node 10.20.9.154 -o template --template={{.status.capacity}}
    map[carina.storage.io/carina-vg-hdd:160 carina.storage.io/carina-vg-ssd:0 cpu:2 ephemeral-storage:208655340Ki hugepages-1Gi:0 hugepages-2Mi:0 memory:3880376Ki pods:110]
    
    $ kubectl get node 10.20.9.154 -o template --template={{.status.allocatable}} 
    map[carina.storage.io/carina-vg-hdd:150 carina.storage.io/carina-vg-ssd:0 cpu:2 ephemeral-storage:192296761026 hugepages-1Gi:0 hugepages-2Mi:0 memory:3777976Ki pods:110]
    ```

    - HDD磁盘：`carina.storage.io/carina-vg-hdd:160` ，SSD磁盘：`carina.storage.io/carina-vg-ssd:0` 单位为Gi
    - capacity为总容量，allocatable为可使用容量，调度器等组件使用的是allocatable显示的容量，`capacity-allocatable=10G`为系统预留
    - 当有新的pv创建成功后会变更`node.status.allocatable`，这变更会有点延迟

  - ④项目启动时配置文件

    ```sh
    $ kubectl get configmap carina-csi-config -n kube-system
    NAME                DATA   AGE
    carina-csi-config   1      116m
    ```

    - config.json

    ```yaml
      config.json: |-
        {
          "diskSelector": ["loop*", "vd*"], # 磁盘匹配策略，支持正则表达式
          "diskScanInterval": "300", # 300s 磁盘扫描间隔，0表示关闭本地磁盘扫描
          "diskGroupPolicy": "type", # 磁盘分组策略，只支持按照磁盘类型分组，更改成其他值无效
          "schedulerStrategy": "spradout" # binpack，spradout支持这两个参数
        }
    ```

    - 备注1：`diskSelector`若是A磁盘已经加入了VG卷组，修改为不在匹配A盘，如果该盘尚未使用则会在VG卷组中移除该磁盘
    - 备注2：`schedulerStrategy`中`binpack`为pv选择磁盘容量刚好满足`requests.storage`的节点 ，`spradout`为pv选择磁盘剩余容量最多的节点
    - 备注3：`schedulerStrategy`在`storageclass volumeBindingMode:Immediate`模式中选择只受磁盘容量影响，即在`spradout`策略下Pvc创建后会立即在剩余容量最大的节点创建volume
    - 备注4：`schedulerStrategy`在`storageclass volumeBindingMode:WaitForFirstConsumer`模式pvc受pod调度影响，它影响的只是调度策略评分，这个评分可以通过自定义调度器日志查看`kubectl logs -f carina-scheduler-6cc9cddb4b-jdt68 -n kube-system`
    - 备注5：当多个节点磁盘容量大于请求容量10倍，则这些节点的调度评分是相同的

  - ⑤服务器组件启动成功，会收集各个节点的存储使用情况更新到`configmap:carina-node-storag`

    ```shell
    $ kubectl get configmap carina-node-storage -n kube-system -o yaml
    data:
      node: '[{
    	"allocatable.carina.storage.io/carina-vg-hdd": "150",
    	"allocatable.carina.storage.io/carina-vg-ssd": "0",
    	"capacity.carina.storage.io/carina-vg-hdd": "160",
    	"capacity.carina.storage.io/carina-vg-ssd": "0",
    	"nodeName": "10.20.9.154"
    }, {
    	"allocatable.carina.storage.io/carina-vg-hdd": "146",
    	"allocatable.carina.storage.io/carina-vg-ssd": "0",
    	"capacity.carina.storage.io/carina-vg-hdd": "170",
    	"capacity.carina.storage.io/carina-vg-ssd": "0",
    	"nodeName": "10.20.9.153"
    }]'
    
    ```

    - 备注1：这个configmap是用于其他服务获取各个节点磁盘分组及容量使用，目前是收集的`node.status.capacity`及`node.status.allocatable`
    - 备注2：当一个pv创建后这个configmap将在30-60s后更新，主要是为了兼容pv大量创建避免configmap重复更新，以及node节点status状态更新不及时导致configmap无效更新
    - 备注3：该configmap由驱动程序自动维护更新，对于用户来说只需读取不要修改

  - ⑥关于topo（`topologyKey: topology.carina.storage.io/node`）使用方法参考`examples/kubernetes/topostatefulset.yaml`

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
            kubernetes.customized/blkio.throttle.read_bps_device: "10485760"
            kubernetes.customized/blkio.throttle.read_iops_device: "10000"
            kubernetes.customized/blkio.throttle.write_bps_device: "10485760"
            kubernetes.customized/blkio.throttle.write_iops_device: "100000"
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

  - carina-node 为host网络模式部署并监听 `8080 8089`端口，其中8080为metrics、8089为http，可通过如下配置进行修改

    ```shell
            - "--metrics-addr=:8080"
            - "--http-addr=:8089"
    ```

    备注：若是修改监听端口，务必同步修改`service：csi-carina-node`

  - carina-controller 监听`8080 8443 8089`，其中8080为metrics、8443为webhook、8089为http，可通过如下配置进行修改

    ```shell
            - "--metrics-addr=:8080"
            - "--webhook-addr=:8443"
            - "--http-addr=:8089"
    ```

  - carina-node和carina-controller，自定义指标

    ```shell
    	# vg剩余容量:  carina-devicegroup-vg_free_bytes
    	# vg总容量:  carina-devicegroup-vg_total_bytes
    	# volume容量:  carina-volume-volume_total_bytes
    	# volume使用量:  carina-volume-volume_used_bytes
    ```

    - 备注1：volume使用量lvm统计与`df -h`统计不同，误差在几十兆
    - 备注2：carina-controller实际是收集的所有carina-node的数据，实际只要通过carina-controller获取监控指标便可
    - 备注3：如果要使用prometheus收集监控指标，可部署servicemonitor(deployment/kubernetes/prometheus.yaml.tmpl)

  - carina-node和carina-controller均暴露了http接口，共提供两个方法

    ```shell
    # 获取所有vg信息：http://carina-controller:8089/devicegroup
    # 获取所有volume信息：http://carina-controller:8089/volume
    ```

    - 备注1：carina-node获取的是当前节点的所有vg及volume信息
    - 备注2：carina-controller接口是收集所有carina-node的vg及volume的汇总信息
    - 备注3：carina-controller服务的svc名称为carina-controller

#### 4. 答疑

- ①该项目总共有哪些组件，各个组件功能职责是怎么样的

  - 共有三个组件，carina-scheduler、carina-controller、carina-node

  - carina-scheduler：自定义调度器，凡是Pod绑定了由该驱动提供服务的Pvc，则由该调度器进行调度
  - carina-controller：负责监听pvc创建等，当pv调度到指定节点，则创建CRD（logicvolume）
  - carina-node：负责管理本地磁盘节点，并监听CRD（logicvolume），管理本地lvm卷
  - 通过查看各个服务日志，可以获取详细的服务运行信息

- ②已知问题，在集群性能极差或者磁盘性能极差情况下，会出现pv无法创建情况

  - 操作lvm卷请求会持续一分钟，每隔十秒重试一次，如果多次重试操作无法成功则会操作失败
  - 可以通过命令`kubectl get lv` 观察到错误响应

- ③pv创建成功后，还能进行Pod迁移吗

  - pv一旦创建成功，Pod只能运行在该节点，无论重启还是删除重建
  - 不支持pv迁移

- ④如何让Pod和PVC在指定节点运行

  - 在pod `spec.nodeName`指定节点名称将跳过调度器
  - 对于`WaitForFirstConsumer`策略的StorageClass，在PVC Annotations增加 `volume.kubernetes.io/selected-node: nodeName`可指定pv调度节点
  - 除非明确知道该方式的应用场景，否则不建议直接修改Pvc

- ⑤k8s节点删除，应如何处理调度到节点上的pv

  - 如果确定volume卷不在使用，直接删除pvc在重建便可

- ⑥如何创建磁盘以方便测试

  - 可使用如下方法创建`loop device`

  ```shell
  for i in $(seq 1 10); do
    truncate --size=200G /tmp/disk$i.device && \
    losetup -f /tmp/disk$i.device
  done
  ```

  