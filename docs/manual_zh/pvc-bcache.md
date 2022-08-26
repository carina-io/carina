#### 缓存盘使用

对于k8s存储卷来说其有一套标准的使用流程，如下我们将展示一下在使用carina存储驱动下这些文件如何配置及创建的

众所周知ssd磁盘价格昂贵且磁盘容量小于hdd磁盘，因此当我们需要ssd磁盘的读写速度，hdd磁盘的大存储量磁盘缓存成为我们一个选择，这一章节将介绍使用bcache创建设备缓存

在carina-node启动时，会自动为节点加载内核模块bcache `modprobe bcache`，可以使用命令`lsmod |grep bcache`查看加载是否成功，如果有些Linux kernel并未加载该内核模块，则磁盘缓存功能无法使用

创建storageclass `kubectl apply -f storageclass.yaml`

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-carina-sc
provisioner: carina.storage.io
parameters:
  # file system
  csi.storage.k8s.io/fstype: xfs
  # disk group
  carina.storage.io/backend-disk-group-name: hdd
  carina.storage.io/cache-disk-group-name: ssd
  # 1-100 Cache Capacity Ratio
  carina.storage.io/cache-disk-ratio: "50"
  # writethrough/writeback/writearound
  carina.storage.io/cache-policy: writethrough
reclaimPolicy: Delete
allowVolumeExpansion: true
# WaitForFirstConsumer表示被容器绑定调度后再创建pv
volumeBindingMode: WaitForFirstConsumer
mountOptions:
```

- 参数`csi.storage.k8s.io/fstype`表示挂载后设备文件格式

- 参数`carina.storage.io/backend-disk-group-name`表示后端存储设备磁盘类型，填写慢盘类型比如Hdd
- 参数`carina.storage.io/cache-disk-group-name`表示缓存设备磁盘类型，填写快盘类型比如ssd
- 参数`carina.storage.io/cache-disk-ratio`表示缓存比例范围为1-100，该比率计算公式是 `cache-disk = backend * 100 / cache-disk-ratio`
- 参数`carina.storage.io/cache-policy`表示缓存策略共三种`writethrough|writeback|writearound`

创建PVC `kubectl apply -f pvc.yaml`

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-carina-pvc
  namespace: carina
spec:
  accessModes:
    - ReadWriteOnce # 本地存储只能被一个节点上Pod挂载
  resources:
    requests:
      storage: 7Gi
  storageClassName: csi-carina-sc
  volumeMode: Filesystem # block便会创建块设备
```

PVC创建完成后，carina-controller会创建LogicVolume，carina-node则负责监听LogicVolume的创建事件，并在本地创建lvm存储卷

缓存盘会创建两个存储卷，这个是内部实现逻辑对于使用来说不用关心这些

```shell
$ kubectl get lv
NAME                                       SIZE   GROUP           NODE          STATUS
pvc-319c5deb-f637-423b-8b52-42ecfcf0d3b7   7Gi    carina-vg-hdd   10.20.9.154   Success
cache-9c5deb-f637-423b-8b52-42ecfcf0d3b7   3Gi    carina-vg-ssd   10.20.9.154   Success
```

挂载到容器内使用`kubectl apply -f deployment.yaml`

在使用上和普通的pvc并无区别

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: carina-deployment
  namespace: carina
  labels:
    app: web-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web-server
  template:
    metadata:
      labels:
        app: web-server
    spec:
      containers:
        - name: web-server
          image: nginx:latest
          imagePullPolicy: "IfNotPresent"
          volumeMounts:
            - name: mypvc
              mountPath: /var/lib/www/html
      volumes:
        - name: mypvc
          persistentVolumeClaim:
            claimName: csi-carina-pvc
            readOnly: false
```

