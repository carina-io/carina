

# E2E 测试

##### 前提条件
  - 确保`kubectl`命令能正常使用
  - `kubectl`版本和集群版本一致，避免出现API参数不对应问题
  - 确保ginkgo命令可用

##### kind集群测试

- 创建kind集群
```cassandraql
make kc
```

- 删除kind集群
```cassandraql
make kd
```

- 特别注意:
  - kind创建的集群每个节点是一个docker服务，其`/dev`目录是共享的，`lsblk`视图也是共享的，所以在每个节点创建`loop device`是互相可见的，
  这样会对测试效果产生一定影响
  - 由于kind创建的k8s集群运行在一台机器，在执行e2e测试时k8s集群性能对测试结果产生较大影响
  - 所以建议使用多主机节点集群执行测试，实际在使用`vagrant`创建的多个k8s集群中运行良好
  - 当服务器不支持bcache时，需要删除deploycarina/csi-carina-node.yaml关于bcache内容
  ```
  # init-container中删除bcache的内核加载
  # csi-carina-node中删除关于bcahce的目录挂载
  - name: host-bcache
    mountPath: /sys/fs/bcache
    
  - name: host-bcache
    hostPath:
      path: /sys/fs/bcache  


- 安装carina
```cassandraql
make install
```

- 执行e2e测试
```cassandraql
make e2e
```

- 卸载carina
```cassandraql
make uninstall