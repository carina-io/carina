

#### 背景

- logicvolume是一个crd资源，该资源用于carina controller和node服务之间的数据交换，在v0.11之前的版本中由于注解标签错误，一直为Namespace级别资源，在v0.11版本之中我们修复了该问题，将该crd资源变更为Cluster级别资源，当然也引起了一些麻烦，如果你已经在使用carina<0.11版本，需要遵循如下的步骤进行carina版本升级。

#### 卸载carina

- 卸载集群内当前carina在部署carina v0.11，carina卸载后并不会影响集群内pvc的使用
- 这一步是非常必要的，因为我们要重建logicvolume，如果carina服务还存活的话，会导致节点上的lvm卷被删除

```shell
# 首先准备好安装文件 
$ cd deploy/kubernetes
$ ./deploy.sh uninstall
# 如果是helm部署，采用helm的卸载方式
$ helm uninstall carina-csi-driver
```

#### 升级logicvolume
- 执行如下脚本，便可完成logicvolume升级

```shell
$ cd deploy/kubernetes
$ ./lvupgrade.sh
```

- 注意①：由于kubernetes版本不同，可能存在升级失败的情况，切勿重复执行该脚本，lv文件存储位置为`/tmp/default-lv.yaml`；
- 注意②：倘若执行失败，请按照查看脚本并根据脚本内容分步执行；

#### 安装carina

```shell
# 注意这里安装的carina镜像版本为latest
$ cd deploy/kubernetes
$ ./deploy.sh
# 安装指定版本的镜像
$ helm install carina-csi-driver carina-csi-driver/carina-csi-driver --namespace kube-system --version v0.11.0
```