#### development guide

* pull code

```shell
$ git clone https://github.com/carina-io/carina.git
```
- golang 1.17

* compiling carina-controller / carina-node

```shell
# to build testing images
$ make docker-build
# to release a version
$ make release VERSION=v0.9
```

* compiling carina-scheduler.

Carina-scheduler is an independent project，which is just placed under carina for now.

```shell
$ cd scheduler
# to build testing images
$ make docker-build
# to release a version
$ make release VERSION=v0.9 
```

* how to run e2e test cases?

- For local volume projects, it's not ideal to run e2e tests via KIND clusters. It's recommended to test carina on physical or virtual nodes. 

```shell
$ cd test/e2e
$ make e2e
```