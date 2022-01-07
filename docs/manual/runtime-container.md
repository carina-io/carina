#### 运行时环境

carina-node运行在一个定制容器里，该容器预装了lvm2等服务，如果要详细了解该容器是如何构建的请参考`docs/runtime-container`目录

- 构建运行时镜像

```shell
$ cd docs/runtime-container
$ docker build -t runtime-container:latest .
```

- 构建多架构运行时镜像

```shell
$ cd docs/runtime-container
$ docker buildx build -t centos-mutilarch-lvm2:runtime --platform=linux/arm,linux/arm64,linux/amd64 . --push
```