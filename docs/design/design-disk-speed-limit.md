#### 磁盘限速

#### 介绍

- 磁盘限速是一种高级功能，这个特性并不常用，多见于特殊应用，如限制数据库读写速度

#### 局限与设计

- 磁盘限速只支持块设备限速
- 不支持文件系统限速，有些文件系统本身支持限速但是其限速条件比较苛刻
- 支持cgroup v1和cgroup v2，同时支持systemd和cgroupfs俩种cgroup driver

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
#### 测试

```
$ fio -direct=1 -rw=write -ioengine=libaio -bs=4k -size=100MB -numjobs=1 -name=/tmp/fio_test1.log
$ iostat 
```
#### 已知问题

- cgroup v1无法限制buffer Io，这就导致在写磁盘时需要显示的指定direct读写磁盘
- 在kernel 3.10版本直连读写磁盘限速良好，在4.18版本内核无法进行限速
- 使用cgroup v2必须同时开启memory和io这俩种控制器，否则无法实现buffer io限速

### cgroup v2支持
Linux kernel 3.10 开始提供v2版本cgroup（Linux Control Group v2）。开始是试验特性，隐藏在挂载参数-o __DEVEL__sane_behavior中，直到Linuxe Kernel 4.5.0的时候，cgroup v2才成为正式特性。

Cgroup V2 可限制 Buffered IO 的读写
>要想使用 Cgroup V2，需要在 Linux 系统里打开 Cgroup V2 的功能。因为目前即使最新版本的 Ubuntu Linux 或者 Centos Linux，仍然在使用 Cgroup v1 作为缺省的 Cgroup。
Cgroup V2 相比 Cgroup V1 做的最大的变动就是在 V2 中一个进程属于一个控制组，而每个控制组里可以定义自己需要的多个子系统。比如下面的 Cgroup V2 示意图里，进程 pid_y 属于控制组 group2，而在 group2 里同时打开了 io 和 memory 子系统 （Cgroup V2 里的 io 子系统就等同于 Cgroup v1 里的 blkio 子系统）。这样 就可以对 Buffered I/O 作磁盘读写的限速。为什么这样协作之后可以限制？可以这么理解，协作之后 io cgroup 知道了对应的 page cache 是属于这个进程的，那么内核同步这些 page cache 时所产生的 IO 会被计算到该进程的 IO 中。

- 开启
```shell
sudo yum update
sudo grubby --update-kernel=ALL --args="systemd.unified_cgroup_hierarchy=1"
sudo reboot
mount | grep cgroup
```
- 禁用v1

一个controller如果已经在cgroup v1中挂载了，那么在cgroup v2中就不可用。如果要在cgroup v2中使用，需要先将其从cgroup v1中卸载。
systemd用到了cgroup v1，会在系统启动时自动挂载controller，因此要在cgroup v2中使用的controller，最好通过内核启动参数cgroup_no_v1=list禁止cgroup v1使用：
``` shell
#/etc/default/grub文件中的GRUB_CMDLINE_LINUX添加
cgroup_no_v1=blkio    # list是用逗号间隔的多个controller
cgroup_no_v1=all      # all 将所有的controller设置为对cgroup v1不可用
grub2-mkconfig -o /boot/grub2/grub.cfg
```

```shell
mkdir -p /cgroup2
# 挂载cgroup
mount -t cgroup2 nodev  /cgroup2
#mount -t cgroup2 -o remount,nsdelegate none /sys/fs/cgroup/unified

echo "+io" > /cgroup2/cgroup.subtree_control
# 验证是否开启
cat /cgroup2/cg2/cgroup.controllers
# 查看 文件系统设备号
lsblk 
NAME   MAJ:MIN RM SIZE RO TYPE MOUNTPOINTsr0     11:0    1  41M  0 romvda    253:0    0  50G  0 disk└─vda1 253:1    0  50G  0 part /
# 限制设备最大可用写带宽为1MB/s
echo "253:0 wbps=1048576" > /cgroup2/cg2/io.max
# 限制设备最大可用读带宽为1MB/s
echo "253:0 rbps=1048576" > /cgroup2/cg2/io.max
# 限制设备最大可用带宽为1MB/s
echo "253:0 rwbps=1048576" > /cgroup2/cg2/io.max

# 限制设备最大可用写iops为1024/s
echo "253:0 wiops=1024" > /cgroup2/cg2/io.max
# 限制设备最大可用读iops为1024/s
echo "253:0 riops=1024" > /cgroup2/cg2/io.max
# 限制设备最大可用iops为1024/s
echo "253:0 rwiops=1024" > /cgroup2/cg2/io.max
#限制权重
echo 100 > /cgroup2/cg2/io.bfq.weight

# 测试 这里要设置一个比较大的文件，不然看不出效果
dd if=/dev/zero of=/tmp/file1 bs=512M count=1
# 或者使用命令
hdparm --direct -t /dev/sdb
iostat  1 -d  /dev/sdb
```

[参考] https://blog.dawnguo.cn/posts/%E5%AE%B9%E5%99%A8-cgroup-%E6%95%B4%E4%BD%93%E4%BB%8B%E7%BB%8D/