

# E2E 测试

##### 前提条件
  - 确保`kubectl`命令能正常使用
  - `kubectl`版本和集群版本一致，避免出现API参数不对应问题

##### 部署存储驱动

```cassandraql
$ cd ../kubernetes/deploy
$ ./deploy.sh
```
##### 卸载存储驱动

```cassandraql
$ cd ../kubernetes/deploy
$ ./deploy.sh uninstall
```

##### 执行e2e测试

```cassandraql
$ make test
```

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