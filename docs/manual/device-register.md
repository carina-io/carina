#### device registration[Deprecated] `This functionality has been replaced by NodeStorageresource`

> Device registration works for carina versions under v0.9.0.

When carina manages local disks, it treats them as devices and registed them to kubelet. Whenever there is an new local device or PV, the disk usage will be updated.

```shell
$ kubectl get node 10.20.9.154 -o template --template={{.status.capacity}}
map[
carina.storage.io/carina-vg-hdd:160 
carina.storage.io/carina-vg-ssd:0 
cpu:2
ephemeral-storage:208655340Ki 
hugepages-1Gi:0 
hugepages-2Mi:0 
memory:3880376Ki 
pods:110
]

$ kubectl get node 10.20.9.154 -o template --template={{.status.allocatable}} 
map[
carina.storage.io/carina-vg-hdd:150 
carina.storage.io/carina-vg-ssd:0 
cpu:2 
ephemeral-storage:192296761026 
hugepages-1Gi:0 
hugepages-2Mi:0 
memory:3777976Ki 
pods:110
]
```

- For each device, carina will records its capacity and allocatable. 10G of disk space is reserved for each device. 
- carina scheduler will do scheduling based on each node's disk usage.

Carina also tracks those informaction in an configmap.

```shell
$ kubectl get configmap carina-node-storage -n kube-system -o yaml
data:
  node: '[{
	"allocatable.carina.storage.io/carina-vg-hdd": "150",
	"allocatable.carina.storage.io/carina-vg-ssd": "0",
	"capacity.carina.storage.io/carina-vg-hdd": "160",
	"capacity.carina.storage.io/carina-vg-ssd": "0",
	"nodeName": "10.20.9.154"
}, {
	"allocatable.carina.storage.io/carina-vg-hdd": "146",
	"allocatable.carina.storage.io/carina-vg-ssd": "0",
	"capacity.carina.storage.io/carina-vg-hdd": "170",
	"capacity.carina.storage.io/carina-vg-ssd": "0",
	"nodeName": "10.20.9.153"
}]'
```