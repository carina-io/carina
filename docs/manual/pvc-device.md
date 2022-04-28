#### volumeMode: block


Creating storageclass `kubectl apply -f storageclass.yaml`

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
volumeBindingMode: WaitForFirstConsumer
mountOptions:
```


Creating PVC `kubectl apply -f pvc.yaml`

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

Checking the LV object.

```shell
$ kubectl get lv
NAME                                       SIZE   GROUP           NODE          STATUS
pvc-319c5deb-f637-423b-8b52-42ecfcf0d3b7   7Gi    carina-vg-hdd   10.20.9.154   Success
```

mount volume as block in pod`kubectl apply -f pod.yaml`

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