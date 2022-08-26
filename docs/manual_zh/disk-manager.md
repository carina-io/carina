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
          "name": "carina-vg-ssd",
          "re": ["loop2+"],
          "policy": "LVM",
          "nodeLabel": "kubernetes.io/hostname"
        },
        {
          "name": "carina-raw-hdd",
          "re": ["vdb+", "sd+"],
          "policy": "RAW",
          "nodeLabel": "kubernetes.io/hostname"
        }
      ],
      "diskScanInterval": "300", # 300s 磁盘扫描间隔，0表示关闭本地磁盘扫描
      "diskGroupPolicy": "type", # 磁盘分组策略，只支持按照磁盘类型分组，更改成其他值无效
      "schedulerStrategy": "spreadout" # binpack，spreadout支持这两个参数
    }

```

#### 磁盘管理

   carina-node启动时会扫描本地磁盘，当发现符合条件的裸盘时会将其加入vg卷组，vg卷组名称分别为`carina-vg-hdd`、`carina-vg-ssd`

```shell
$  kubectl exec -it csi-carina-node-cmgmm -c csi-carina-node -n kube-system bash
$ pvs
  PV         VG            Fmt  Attr PSize   PFree  
  /dev/vdc   carina-vg-hdd lvm2 a--  <80.00g <79.95g
  /dev/vdd   carina-vg-hdd lvm2 a--  <80.00g  41.98g
$ vgs
  VG            #PV #LV #SN Attr   VSize   VFree   
  carina-vg-hdd   2  10   0 wz--n- 159.99g <121.93g
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

当变更为`"diskSelector": ["loop0", "vd+"]`时会自动移除对应的磁盘

```shell
$  kubectl exec -it csi-carina-node-cmgmm -c csi-carina-node -n kube-system bash
$ pvs
  PV         VG            Fmt  Attr PSize   PFree  
  /dev/loop0   carina-vg-hdd lvm2 a--  <80.00g <79.95g
$ vgs
  VG            #PV #LV #SN Attr   VSize   VFree   
  carina-vg-hdd   1  10   0 wz--n- 79.99g <79.93g
```

