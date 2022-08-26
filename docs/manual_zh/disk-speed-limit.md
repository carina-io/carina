#### 磁盘限速

carina提供了磁盘限速的高级功能，该功能可以限制容器读写挂载磁盘的速度，使用方式也很简单只要在pod的annotation加入如下注解即可

创建容器`kubectl apply -f deployment.yaml`

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

- 该注解值会被写入相应pod层次的cgroup配置文件中

  ```shell
  cgroup v1
  /sys/fs/cgroup/blkio/kubepods/burstable/pod0b0e005c-39ec-4719-bbfe-78aadbc3e4ad/blkio.throttle.read_bps_device
  /sys/fs/cgroup/blkio/kubepods/burstable/pod0b0e005c-39ec-4719-bbfe-78aadbc3e4ad/blkio.throttle.read_iops_device
  /sys/fs/cgroup/blkio/kubepods/burstable/pod0b0e005c-39ec-4719-bbfe-78aadbc3e4ad/blkio.throttle.write_bps_device
  /sys/fs/cgroup/blkio/kubepods/burstable/pod0b0e005c-39ec-4719-bbfe-78aadbc3e4ad/blkio.throttle.write_iops_device
  ```
  ```shell
  cgroup v2
  /sys/fs/cgroup/kubepods/burstable/pod0b0e005c-39ec-4719-bbfe-78aadbc3e4ad/io.max
  ```

- 备注1：支持设置一个或多个annotation，增加或移除annontation会在一分钟内同步到cgroup
- 备注2：只支持块设备磁盘限速，测试命令`dd if=/dev/zero of=out.file bs=1M count=512 oflag=dsync`
- 备注3：carina能够根据系统环境，自动决策使用cgroup v1还是cgroup v2
- 备注4：如果系统使用的是cgroup v2，那么支持buffer io限速(需要同时开启io和memory的controller)，否则只支持direct io限速
- 备注5：如果将磁盘限速设置的太低，会导致设备格式化不成功，容器处于pending状态，此时在容器所在节点上执行 `pa aux |grep xfs`可以看到阻塞中的mkfs.xfs进程，此时需要在容器所在节点cgroup下执行`echo 250:2 0 >  /sys/fs/cgroup/blkio/blkio.throttle.write_bps_device`即取消cgroup限制即可成功格式化磁盘；其中`250:2`为创建的lvm卷设备号