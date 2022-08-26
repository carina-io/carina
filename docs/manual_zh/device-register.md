#### 设备注册[Deprecated] `该功能已经被nodestorageresource代替`


  当我们对本地磁盘进行管理时，将本地磁盘卷组当做设备注册到kubelet，每次新的磁盘加入或者pv被创建后会更新设备容量到kubelet，如所示

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

- 设备注册会注册到两个值`.status.capacity`为设备总容量，`.status.allocatable`为设备可用容量，我们预留了10G空间不可使用
- 当pv创建时会从该node信息中获取当前节点磁盘容量，然后根据pv调度策略进行调度

为了使用方便我们将各个节点的设备容量信息收集到了一个configmap里边

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

