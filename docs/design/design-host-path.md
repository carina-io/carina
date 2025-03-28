

#### 支持主机目录作为存储卷

#### 前言

- 为了简单使用我们设计一种使用主机目录作为存储卷，提供给容器使用的方案，主要满足以下场景
- 主机上存在某个目录，发布的POD只想简单的使用主机的目录挂载到容器内，该设计便是支持该方案
- 主机已经使用共享存储挂载了一个共用目录，将该目录挂载到carina-node容器内，发布pvc即可管理目录中的文件

#### 功能设计

- 对于主机目录类型的存储卷，仅仅支持pvc被动选择节点，
- 你可以为pvc配置注解`volume.kubernetes.io/selected-node`,实现手动选择节点
- sc配置为`WaitForFirstConsumer`,在该pvc被pod使用的时候跟随pod的节点选择
- 默认配置文件中`/opt/carina-hostpath`是创建pv卷的主机目录
- 如果想添加自定义的目录作为pv卷的存储目录，需要修改kube-system/csi-carina-node的DS文件将目录目录挂载到容器内并在configmap中配置

#### 实现细节

- 配置文件，支持动态更新

  ```json
  config.json: |-
    {
      "diskSelector": [
        {
          "name": "carina-lvm",
          "re": ["loop2+"],
          "policy": "LVM",
          "nodeLabel": "kubernetes.io/hostname"
        },
        {
          "name": "carina-raw",
          "re": ["loop3+"],
          "policy": "RAW",
          "nodeLabel": "kubernetes.io/hostname"
        },
        {
          "name": "carina-host",
          "re": ["/opt/carina-hostpath"],
          "policy": "HOST",  
          "nodeLabel": "kubernetes.io/hostname"
        }
      ],
      "diskScanInterval": "300",
      "schedulerStrategy": "spreadout"
    }
  ```


- policy: HOST 表示主机目录
- re: ["/opt/carina-hostpath"] 仅仅支持绝对路径且只能写一个

#### 使用步骤

- 创建sc

```shell
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: carina-hostpath
provisioner: carina.storage.io
parameters:
  # disk group
  carina.storage.io/disk-group-name: "carina-host"
reclaimPolicy: Delete
allowVolumeExpansion: true
# WaitForFirstConsumer表示被容器绑定调度后再创建pv
volumeBindingMode: WaitForFirstConsumer
mountOptions:

```

- 创建pvc
```shell
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-carina-host
  namespace: carina
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: carina-hostpath
  volumeMode: Filesystem
```

- 创建pod

```shell
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
            claimName: csi-carina-host
            readOnly: false

```