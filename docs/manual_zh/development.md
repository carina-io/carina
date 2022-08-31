#### 开发

拉取代码

```shell
$ cd $GOPATH && mkdir carina-io
$ git clone https://github.com/carina-io/carina.git
```

- golang 1.17

编译 carina-controller / carina-node

```shell
# 生成测试镜像，镜像仓库可以改成自己的
$ make docker-build
# 发布版本，镜像仓库可以改成自己的
$ make release VERSION=v0.9
```

编译 carina-scheduler，carina-scheduler实质是一个完全独立的项目，它拥有独立的go.mod程序入口等，只是项目放置在carina下

```shell
$ cd scheduler
# 生成测试镜像，镜像仓库可以改成自己的 
$ make docker-build
# 发布版本，镜像仓库可以改成自己的
$ make release VERSION=v0.9 
```

如何运行e2e测试

- 对于本地存储项目来说，使用kind创建的集群运行测试效果并不理想，常用测试场景为使用vagrant创建虚拟机并创建集群，在虚拟机中创建模拟磁盘进行测试

```shell
$ cd test/e2e
$ make e2e
```

