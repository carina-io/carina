#### carina-node runtime image

carina-node runs in a cumtomized containers, embedding lvm2. Refer `docs/runtime-container` to learn abount how it works.

- building carina-node runtime image 

```shell
$ cd docs/runtime-container
$ docker build -t runtime-container:latest .
```

- multi-arch buildin

```shell
$ cd docs/runtime-container
$ docker buildx build -t centos-mutilarch-lvm2:runtime --platform=linux/arm,linux/arm64,linux/amd64 . --push
```