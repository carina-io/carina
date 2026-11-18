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

- Carina writes those annotations into the `io.max` file of the pod's cgroup. The path depends on the kubelet cgroup driver:

  ```shell
  # systemd driver
  /sys/fs/cgroup/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod0b0e005c_39ec_4719_bbfe_78aadbc3e4ad.slice/io.max
  # cgroupfs driver
  /sys/fs/cgroup/kubepods/burstable/pod0b0e005c-39ec-4719-bbfe-78aadbc3e4ad/io.max
  ```

* Users can add one or more annotations. Adding or removing annotations will be synced to cgroupfs in about 60s. 
* Currently, only block device disk speed limit is supported. User can test io throttling with command `dd if=/dev/zero of=out.file bs=1M count=512 oflag=dsync`.
* cgroup v2 is required. On a cgroup v1 node, carina-node logs an error at startup and ignores these annotations.
* The `io` controller must be enabled in the parent cgroup's `cgroup.subtree_control`, otherwise `io.max` does not exist. Buffered io is limited too when the `memory` controller is also enabled.
* Removing an annotation resets the matching value to `max`. The annotation names keep their historical `blkio.throttle.*` form for compatibility.
* If user can set io throttling too low, it may cause the procedure of formating filesystem hangs there and then the pod will be in pending state forever.