#### 设备调度

carina-scheduler实现了基于磁盘capacity容量的调度

配置文件

```yaml
 config.json: |-
    {
      "diskScanInterval": "300", # 300s 磁盘扫描间隔，0表示关闭本地磁盘扫描
      "diskGroupPolicy": "type", # 磁盘分组策略，只支持按照磁盘类型分组，更改成其他值无效
      "schedulerStrategy": "spreadout" # binpack，spreadout支持这两个参数
    }
```

在配置文件中和调度相关的配置为`schedulerStrategy`它支持 `spreadout|binpack`值，它有如下两种生效场景

- `schedulerStrategy`在`storageclass volumeBindingMode:Immediate`模式中选择只受磁盘容量影响，即在`spreadout`策略下Pvc创建后会立即在剩余容量最大的节点创建volume
- `schedulerStrategy`在`storageclass volumeBindingMode:WaitForFirstConsumer`模式pvc受pod调度影响，它影响的只是调度策略评分，这个评分可以通过自定义调度器日志查看`kubectl logs -f carina-scheduler-6cc9cddb4b-jdt68 -n kube-system`
- 当多个节点磁盘容量大于请求容量10倍，则这些节点的调度评分是相同的

备注：carina存在`admissionregistration`，会将所有使用carina提供存储卷的POD的调度器更改为carina-scheduler