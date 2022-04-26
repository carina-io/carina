#### disk io throttling

Users can add annotation to pod to limit bandwidth or IOPS.

Example: `kubectl apply -f deployment.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: carina-deployment-speed-limit
  namespace: carina
  labels:
    app: web-server-speed-limit
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web-server-speed-limit
  template:
    metadata:
      annotations:
        carina.storage.io/blkio.throttle.read_bps_device: "10485760"
        carina.storage.io/blkio.throttle.read_iops_device: "10000"
        carina.storage.io/blkio.throttle.write_bps_device: "10485760"
        carina.storage.io/blkio.throttle.write_iops_device: "100000"
      labels:
        app: web-server-speed-limit
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
            claimName: csi-carina-pvc-speed-limit
            readOnly: false
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-carina-pvc-speed-limit
  namespace: carina
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 17Gi
  storageClassName: csi-carina-sc
  volumeMode: Filesystem
```

- Carina will convert those anntoations into carina PV's cgroupfs.

  ```shell
  carina.storage.io/blkio.throttle.read_bps_device: "10485760"
  carina.storage.io/blkio.throttle.read_iops_device: "10000"
  carina.storage.io/blkio.throttle.write_bps_device: "10485760"
  carina.storage.io/blkio.throttle.write_iops_device: "100000"
   ---
   /sys/fs/cgroup/blkio/blkio.throttle.read_bps_device
   /sys/fs/cgroup/blkio/blkio.throttle.read_iops_device
   /sys/fs/cgroup/blkio/blkio.throttle.write_bps_device
   /sys/fs/cgroup/blkio/blkio.throttle.write_iops_device
  ```

* Users can add one or more annotations. Adding or removing annotations will be synced to cgroupfs in about 60s. 
* Currently, buffered IO is still not supportted. User can test io throttling with command `dd if=/dev/zero of=out.file bs=1M count=512 oflag=dsync`. In future, Carina will support buffered io throttling using cgroup V2. 
* If user can set io throttling too low, it may cause the procedure of formating filesystem hangs there and then the pod will be in pending state forever.