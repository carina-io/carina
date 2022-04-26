## 背景

- 目前，在v0.9.0版本中，Carina将节点磁盘通过磁盘类型划分为不同的磁盘组，用户使用不同的磁盘组提供服务，在通常情况下这可以满足大部分需求，但是在某些情况下用户更喜欢灵活的使用。
- 比如，虽然所有的磁盘都是相同的类型，但是有的工作负载可能更重要，需要使用独立的磁盘，而不希望其他负载和它竞争磁盘IO
- 对于真正的高性能磁盘，如NVMe或Pmem，LVM和RAID两者都不重要。Carina需要提供最原始的磁盘给负载使用

## 设计

- 目前，用户通过`diskSelector`和`diskGroupPolicy`配置磁盘组，该配置在configmap中。为了更灵活的适应真实环境，我们提供更多灵活的配置方案
- 作为参考 0.9.0版本 configmap结构如下

```
data:
  config.json: |-
    {
      "diskSelector": ["loop*", "vd*"],
      "diskScanInterval": "300",
      "diskGroupPolicy": "type",
      "schedulerStrategy": "spreadout"
    }
```

### 变更设计

```yaml
diskSelectors:
  - name: group1
    re:
      - sd[b-f]
      - sd[m-o]
    policy: LVM
    nodeLabel: node-label
  - name: group2
    re:
      - sd[h-g]
    policy: RAW
    nodeLabel: node-label
```

#### diskSelector

- `diskSelector`是一个diskGroups的列表,每个diskGroup有四个主要字段，参数都是必须的，不是可选的

* name
  - vg卷组的名称，这里可以填写节点上不存在的vg卷组，也可以填写节点已存在的vg卷组
  - 注意这个名称在配置中是唯一的，不要重复
  - 命名规范`^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$`
* re
  - 这里是一个正则表达式列表，例如`["loop+", "sdb+"]`，carina-node在进行本地磁盘扫描时会根据这个正则表达式将匹配到的磁盘加入该vg卷组
  - 如果改动该re，会将vg卷组中不符合匹配条件的磁盘移除该卷组，当然如果磁盘使用中无法移除
* policy
  - 磁盘策略，目前支持 `LVM和RAW`两种策略
  - LVM策略表示该diskGroup将会被组建成VG卷组，并以lvm卷的方式提供给容器使用
  - RAW策略表示该diskGroup将会将节点上的裸盘直接提供给容器使用，在该策略下Carina将会根据request.storage大小，对磁盘分区，将该分区提供给容器使用
  - RAW策略下，用户可以在pod中增加`carina.storage.io/allow-pod-migration-if-node-notready: true` 表示该pvc独占磁盘，其他的pvc将不会在该磁盘上划分新分区
* nodeLabel
  - 该字段表示该磁盘组生效的节点范围，比如`diskAAA`，carina会获取当前节点label并进行对比，存在`diskAAA`的则该配置生效
  - 如果为空，表示该配置在所有节点生效

#### diskGroupPolicy

- 弃用，Carina不会根据磁盘类型自动分组。

### 对其他组件的副作用

#### carina-scheduler

- 需要支持自定义磁盘组的调度
- 在裸盘情况下，需要考虑pvc独占情况，如果多个PVC都需要独占磁盘，可能会出现PVC调度不合理磁盘剩余空间较大问题

#### 节点的容量和可分配

- 对于LVM diskgroup, carina仍然可以像以前一样工作。

- 对于原始diskGroups, carina应该添加额外的信息。例如,每个节点所能容纳的最大PV大小、每个节点所能容纳的总空闲磁盘数等等。

#### 默认设置

- 虽然carina不会使用具有任何文件系统和分区的磁盘，但为了尽量消除误用，用户应该明确的对磁盘进行分组，以避免出现意外情况

- 作为一个典型的环境，carina将使用下面的设置作为默认的configmap。

- 用户在将其放入生产环境前应再次检查。

```yaml
{
  "diskSelector": [
    {
      "name": "carina-vg-ssd",
      "re": ["loop2+"],
      "policy": "LVM",
      "nodeLabel": "kubernetes.io/hostname"
    },
    {
      "name": "carina-vg-hdd",
      "re": ["loop3+"],
      "policy": "LVM",
      "nodeLabel": "kubernetes.io/hostname"
    },
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

### 从v0.9.0或更低版本迁移

- 直接卸载v0.9.0版本carina，然后安装v0.9.1版本即可

- 由于配置的重大变更，v0.9.1版本需要进行历史存储卷的兼容，兼容方式便是在配置文件中增加了两个特殊配置，在v0.9.0版本中carina默认会创建`carina-vg-ssd和carian-vg-hdd` 在此明确标到配置上，假设你的节点原先没有ssd磁盘，尽可以删除`carina-vg-ssd`这项配置

- 副作用，为了兼容v0.9.0版本配置上的重大变更，`diskSelector`中`name`字段不能为`ssd和hdd`，如果配置了会优先于`carina-vg-ssd和carian-vg-hdd`生效

  ```json
      {
        "name": "carina-vg-ssd",
        "re": ["loop2+"],
        "policy": "LVM",
        "nodeLabel": "kubernetes.io/hostname"
      },
      {
        "name": "carina-vg-hdd",
        "re": ["loop3+"],
        "policy": "LVM",
        "nodeLabel": "kubernetes.io/hostname"
      }
  ```
  
#### 注意事项
 - 当多个vg组的re匹配到同一块盘时，最先匹配到这个盘的VG会优先使用，后边的在匹配到的直接跳过，建议慎重填写re内容，防止不期望的结果
