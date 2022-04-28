#### 本地设备挂载块设备使用

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
  name: raw-block-pvc
  namespace: carina
spec:
  accessModes:
    - ReadWriteOnce
  volumeMode: Block
  resources:
    requests:
      storage: 13Gi
  storageClassName: csi-carina-sc
```

PVC创建完成后，carina-controller会创建LogicVolume，carina-node则负责监听LogicVolume的创建事件，并在本地创建lvm存储卷

```shell
$ kubectl get lv
NAME                                       SIZE   GROUP           NODE          STATUS
pvc-319c5deb-f637-423b-8b52-42ecfcf0d3b7   7Gi    carina-vg-hdd   10.20.9.154   Success
```

挂载到容器内使用`kubectl apply -f pod.yaml`

```yaml
---
apiVersion: v1
kind: Pod
metadata:
  name: carina-block-pod
  namespace: carina
spec:
  containers:
    - name: centos
      securityContext:
        capabilities:
          add: ["SYS_RAWIO"]
      image: centos:latest
      imagePullPolicy: "IfNotPresent"
      command: ["/bin/sleep", "infinity"]
      volumeDevices:
        - name: data
          devicePath: /dev/xvda
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: raw-block-pvc
```

