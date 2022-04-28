#### volumeMode: filesystem

对于k8s存储卷来说其有一套标准的使用流程，如下我们将展示一下在使用carina存储驱动下这些文件如何配置及创建的

首选创建storageclass `kubectl apply -f storageclass.yaml`

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
  carina.storage.io/disk-group-name: hdd
reclaimPolicy: Delete
allowVolumeExpansion: true
# WaitForFirstConsumer表示被容器绑定调度后再创建pv
volumeBindingMode: WaitForFirstConsumer
mountOptions:
```

- 要标识创建设备的文件系统使用`csi.storage.k8s.io/fstype`参数
- 要标识设备使用的磁盘使用`carina.storage.io/disk-group-name` 支持 `hdd` `ssd`值

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

```shell
$ kubectl get lv
NAME                                       SIZE   GROUP           NODE          STATUS
pvc-319c5deb-f637-423b-8b52-42ecfcf0d3b7   7Gi    carina-vg-hdd   10.20.9.154   Success
```

挂载到容器内使用`kubectl apply -f deployment.yaml`

```yaml
---
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