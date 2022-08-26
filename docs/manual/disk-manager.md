#### carina-configmap


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
      "diskScanInterval": "300", # disk scan interval in seconds
      "diskGroupPolicy": "type", # the policy to group local disks
      "schedulerStrategy": "spreadout" # binpack or spreadout
    }

```

#### disk management

Each carina-node will scan local disks and group them into different groups.

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

#### 配置变更场景

With `"diskSelector": ["loop+", "vd+"]`and `diskGroupPolicy: LVM`, carina will create below VG: 

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

If changing diskSelector to `"diskSelector": ["loop0", "vd+"]`, then carina will automatically remove related disks.

```shell
$  kubectl exec -it csi-carina-node-cmgmm -c csi-carina-node -n kube-system bash
$ pvs
  PV         VG            Fmt  Attr PSize   PFree  
  /dev/loop0   carina-vg-hdd lvm2 a--  <80.00g <79.95g
$ vgs
  VG            #PV #LV #SN Attr   VSize   VFree   
  carina-vg-hdd   1  10   0 wz--n- 79.99g <79.93g
```