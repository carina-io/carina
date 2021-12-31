#### 存储服务调度器设计

#### 介绍

- 默认的Kubernetes调度器做了很多有价值的调度策略，并且应用稳定，但是其未有依据本地磁盘的调度策略
- 因此我们基于`Schedule framework v2`调度框架实现自定义调度策略

#### 功能设计

- 设计两种调度策略，通过配置文件动态获取
- 我们要在两个服务实现调度策略，分别为控制服务和自定义调度器
- 控制服务实现的调度策略是硬性的直接将pv调度到node节点，自定义调度器的调度是软性的只能给符合条件的节点更高的评分
- 对于`sc volumeBindingMode: Immediate` 需要控制服务实现pv调度
- 对于`sc volumeBindingMode: WaitForFirstConsumer` 需要自定义调度器实现调度策略

#### 具体细节

- 配置文件，支持动态修改调度策略

  - binpack：选择恰好满足pvc容量的节点
  - spreadout：选择剩余容量最大的节点，这个是默认调度策略
  
```
   config.json: |-
      {
        "schedulerStrategy": "spreadout" # binpack，spreadout支持这两个参数
      }
  ```
  
- 控制器服务实现的调度策略

  - `sc volumeBindingMode: Immediate`

  ```
  +-------------+                 +--------------------------------+             +--------------+
  |             |---------------> |    Controller Server           |             | select node  |
  | Create PVC  |   csi-driver    |    get node capacity           |------------>| create volume|
  |             |---------------> | execution scheduling algorithm |             | create pv    |
  +-------------+                 +--------------------------------+             +--------------+  
  ```

- 自定义调度器

  ```
  +-------------+                  +-----------------------+              +---------------------+
  |             |----------------->|   Custom Scheduler    |              | set pvc annontation |
  | Create POD  | custom scheduler |   get node capacity   |------------->| create volume       |  
  |             |----------------->|   node score          |              | create pv           | 
  +-------------+                  +-----------------------+              +---------------------+
  ```