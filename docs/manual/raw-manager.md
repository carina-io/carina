#### 部署物配置

 当我们部署carina时需要一个配置文件，该配置文件以configmap形式创建到K8S集群中，内容如下：

```yaml

apiVersion: v1
kind: ConfigMap
metadata:
  name: carina-csi-config
  namespace: kube-system
  labels:
    class: carina
data:
  config.json: |-
     {
      "diskSelector": [
        {
          "name": "carina-vg-ssd" ,
          "re": ["loop2+"],
          "policy": "LVM",
          "nodeLabel": "kubernetes.io/hostname"
        },
        {
          "name": "carina-raw-ssd",
          "re": ["loop3+"],
          "policy": "RAW",
          "nodeLabel": "kubernetes.io/hostname"
        },
         {
          "name": "carina-raw-loop", 
          "re": ["loop4","loop5"], # 磁盘匹配策略，支持正则表达式
          "policy": "RAW",         # 磁盘使用方式
          "nodeLabel": "kubernetes.io/hostname" # 节点标签
        }
      ],
      "diskScanInterval": "300", # 300s 磁盘扫描间隔，0表示关闭本地磁盘扫描
      "schedulerStrategy": "spreadout" # binpack，spreadout支持这两个参数
    }

```

#### 磁盘管理

   carina-node启动时会扫描本地磁盘，当发现符合条件的裸盘时会将其加入vg卷组 或者raw裸盘组

```shell
$   kubectl get nodestorageresource
NAME             NODE             TIME
docker-desktop   docker-desktop   12m
```
```
spec:
    nodeName: docker-desktop
  status:
    allocatable:
      carina.storage.io/carina-raw-loop/loop4: 102398Mi
      carina.storage.io/carina-raw-ssd/loop3: 204798Mi
      carina.storage.io/carina-vg-ssd: "189"
    capacity:
      carina.storage.io/carina-raw-loop/loop4: 100Gi
      carina.storage.io/carina-raw-ssd/loop3: 200Gi
      carina.storage.io/carina-vg-ssd: "200"
    disks:
    - name: loop3
      partitions:
        "1":
          last: 5369757695
          name: carina.io/aea23900726f
          number: 1
          start: 1048576
        "2":
          last: 10738466815
          name: carina.io/80b9c67cecbb
          number: 2
          start: 5369757696
        "3":
          last: 13959692287
          name: carina.io/c77150a103f2
          number: 3
          start: 10738466816
        "4":
          last: 17180917759
          name: carina.io/e303db23d0f5
          number: 4
          start: 13959692288
      path: /dev/loop3
      sectorSize: 512
      size: 214748364800
      udevInfo:
        name: loop3
        properties:
          DEVNAME: /dev/loop3
          DEVPATH: /devices/virtual/block/loop3
          DEVTYPE: disk
          MAJOR: "7"
          MINOR: "3"
          SUBSYSTEM: block
        sysPath: /devices/virtual/block/loop3
    - name: loop4
      partitions:
        "1":
          last: 1074790399
          name: carina.io/aad968ee5f8a
          number: 1
          start: 1048576
      path: /dev/loop4
      sectorSize: 512
      size: 107374182400
      udevInfo:
        name: loop4
        properties:
          DEVNAME: /dev/loop4
          DEVPATH: /devices/virtual/block/loop4
          DEVTYPE: disk
          MAJOR: "7"
          MINOR: "4"
          SUBSYSTEM: block
        sysPath: /devices/virtual/block/loop4
    syncTime: "2022-04-09T02:40:40Z"
    vgGroups:
    - pvCount: 1
      pvName: /dev/loop2
      pvs:
      - pvAttr: a--
        pvFmt: lvm2
        pvFree: 214744170496
        pvName: /dev/loop2
        pvSize: 214744170496
        vgName: carina-vg-ssd
      vgAttr: wz--n-
      vgFree: 214744170496
      vgName: carina-vg-ssd
      vgSize: 214744170496
```

  如上配置文件和磁盘管理有关的参数有三个：

- diskSelector：该参数为一个正则表达式，carina-node会根据该配置过滤本地磁盘
- diskScanInterval：磁盘扫描间隔，0表示关闭本地磁盘扫描
- diskGroupPolicy：磁盘分组策略，只支持按照磁盘类型分组，更改成其他值无效

#### 配置变更场景

假设初始`"diskSelector": ["loop+", "vd+"]`则创建的VG卷组如下：

```shell
$  kubectl exec -it csi-carina-node-cmgmm -c csi-carina-node -n kube-system bash
$ pvs
  PV         VG            Fmt  Attr PSize   PFree  
  /dev/loop0   carina-vg-hdd lvm2 a--  <80.00g <79.95g
  /dev/loop1   carina-vg-hdd lvm2 a--  <80.00g  79.98g
$ vgs
  VG            #PV #LV #SN Attr   VSize   VFree   
  carina-vg-hdd   2  10   0 wz--n- 159.99g <121.93g
```


