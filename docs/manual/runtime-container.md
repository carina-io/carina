#### 运行时环境

carina-node运行在一个定制容器涅，该容器预装了lvm2等服务，如果要详细了解该容器是如何构建的请参考`docs/runtime-container`目录

镜像构建命令

```shell
$ cd docs/runtime-container
$ docker build -t runtime-container:latest .
```