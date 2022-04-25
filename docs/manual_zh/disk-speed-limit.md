#### 磁盘限速

carina提供了磁盘限速的高级功能，该功能可以限制容器读写挂载磁盘的速度，使用方式也很简单只要在pod的annontation加入如下注解即可

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
        kubernetes.customized/blkio.throttle.read_bps_device: "10485760"
        kubernetes.customized/blkio.throttle.read_iops_device: "10000"
        kubernetes.customized/blkio.throttle.write_bps_device: "10485760"
        kubernetes.customized/blkio.throttle.write_iops_device: "100000"
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

- 可以看到在pod注解增加了

  ```shell
  kubernetes.customized/blkio.throttle.read_bps_device: "10485760"
  kubernetes.customized/blkio.throttle.read_iops_device: "10000"
  kubernetes.customized/blkio.throttle.write_bps_device: "10485760"
  kubernetes.customized/blkio.throttle.write_iops_device: "100000"
   ---
   # 该annotations会被设置到如下文件
   /sys/fs/cgroup/blkio/blkio.throttle.read_bps_device
   /sys/fs/cgroup/blkio/blkio.throttle.read_iops_device
   /sys/fs/cgroup/blkio/blkio.throttle.write_bps_device
   /sys/fs/cgroup/blkio/blkio.throttle.write_iops_device
  ```

- 备注1：支持设置一个或多个annontation，增加或移除annontation会在一分钟内同步到cgroup
- 备注2：只支持块设备直连读写磁盘限速，测试命令`dd if=/dev/zero of=out.file bs=1M count=512 oflag=dsync`
- 备注3：使用的cgroup v1，由于cgroup v1本身缺陷无法限速buffer io，目前很多组件依赖cgroup v1尚未切换到cgroup v2
- 备注4：已知在kernel 3.10下直连磁盘读写可以限速，在kernel 4.18版本无法限制buffer io 
- 备注5：如果将磁盘限速设置的太低，会导致设备格式化不成功，容器处于pending状态，此时在容器所在节点上执行 `pa aux |grep xfs`可以看到阻塞中的mkfs.xfs进程，此时需要在容器所在节点cgroup下执行`echo 250:2 0 >  /sys/fs/cgroup/blkio/blkio.throttle.write_bps_device`即取消cgroup限制即可成功格式化磁盘；其中`250:2`为创建的lvm卷设备号

