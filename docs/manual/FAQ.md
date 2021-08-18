#### 答疑

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
  for i in $(seq 1 5); do
    truncate --size=200G /tmp/disk$i.device && \
    losetup -f /tmp/disk$i.device
  done
  ```

- ⑦如何模拟SSD磁盘

  ```shell
  $ echo 0 > /sys/block/loop0/queue/rotational
  $ lsblk -d -o name,rota
   NAME ROTA
   loop1     1
   loop0     0
  ```

  