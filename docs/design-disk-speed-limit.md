#### 磁盘限速

#### 介绍

- 磁盘限速是一种高级功能，这个特性并不常用，多见于特殊应用，如限制数据库读写速度

#### 局限与设计

- 磁盘限速只支持块设备限速
- 不支持文件系统限速，有些文件系统本身支持限速但是其限速条件比较苛刻
- 使用cgroup v1 实现磁盘限速

#### 具体实现

- 通过在pod添加annotations设置cgroup，示例如下

  ```yaml
  ---
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

  - 该注解会被写入cgroup

    ```shell
    /sys/fs/cgroup/blkio/blkio.throttle.read_bps_device
    /sys/fs/cgroup/blkio/blkio.throttle.read_iops_device
    /sys/fs/cgroup/blkio/blkio.throttle.write_bps_device
    /sys/fs/cgroup/blkio/blkio.throttle.write_iops_device
    ```

#### 已知问题

- cgroup v1无法限制buffer Io，这就导致在写磁盘时需要显示的指定direct读写磁盘
- 在kernel 3.10版本直连读写磁盘限速良好，在4.18版本内核无法进行限速