#### cache tiering


When carina-node starts up, it automatically loads kernel module `bcache` using `modprobe bcache`. If it's not supported by kernel, then cache tiering will not be enabled.

Creating storageclass using `kubectl apply -f storageclass.yaml`

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
volumeBindingMode: WaitForFirstConsumer
mountOptions:
```

- `csi.storage.k8s.io/fstype`: the filesystem formation
- `carina.storage.io/backend-disk-group-name`: the cold tier
- `carina.storage.io/cache-disk-group-name`: the hot tier
- `carina.storage.io/cache-disk-ratio`: percentage of hot/cold, ranging (0-100)
- `carina.storage.io/cache-policy`: `writethrough|writeback|writearound`

Creating PVC using `kubectl apply -f pvc.yaml`

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-carina-pvc
  namespace: carina
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 7Gi
  storageClassName: csi-carina-sc
  volumeMode: Filesystem
```

After PVC been created, carina-controller will create an LogicVolume internally and then carina-node will create local volume. For PVC using cache-tiering, carina will create two local volumes, one hot volume and one cold volume, and setup cache tiering using bcache.

```shell
$ kubectl get lv
NAME                                       SIZE   GROUP           NODE          STATUS
pvc-319c5deb-f637-423b-8b52-42ecfcf0d3b7   7Gi    carina-vg-hdd   10.20.9.154   Success
cache-9c5deb-f637-423b-8b52-42ecfcf0d3b7   3Gi    carina-vg-ssd   10.20.9.154   Success
```

Creating one deployment using that PVC `kubectl apply -f deployment.yaml`

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