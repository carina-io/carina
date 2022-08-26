#### Carina部署

##### 部署条件

- Kubernetes 集群（CSI_VERSION=1.5.0）
- 如果kubelet以容器化方式运行，需要挂载主机`/dev`目录
- Linux Kernel 3.10.0-1160.11.1.el7.x86_64，非硬性要求，基于此环境进行的测试较多
- 集群每个节点存在1..N块裸盘，支持SSD和HDD磁盘（可使用命令`lsblk --output NAME,ROTA`查看磁盘类型，ROTA=1为HDD磁盘 ROTA=0为SSD磁盘），集群某些节点没有裸盘也无影响，会在创建pv时自动过滤掉该节点

##### 执行部署

- 项目部署，可用`kubectl get pods -n kube-system | grep carina`命令查看部署进度

  ```shell
  $ cd deploy/kubernetes
  $ ./deploy.sh

  $ kubectl get pods -n kube-system |grep carina
  carina-scheduler-6cc9cddb4b-jdt68         0/1     ContainerCreating   0          3s
  csi-carina-node-6bzfn                     0/2     ContainerCreating   0          6s
  csi-carina-node-flqtk                     0/2     ContainerCreating   0          6s
  csi-carina-provisioner-7df5d47dff-7246v   0/4     ContainerCreating   0          12s
  ```

- 项目卸载

  ```shell
  $ cd deploy/kubernetes
  $ ./deploy.sh uninstall
  ```

- 注意事项

  - 安装卸载该服务，对已经挂载到容器内使用的volume卷无影响

#### Helm安装

- 通过helm chart支持多版本carina安装

##### 安装

```
helm repo add carina-csi-driver https://carina-io.github.io

helm search repo -l carina-csi-driver

helm install carina-csi-driver carina-csi-driver/carina-csi-driver --namespace kube-system --version v0.9.0
```

##### 版本升级

- 先卸载旧版本，然后安装新版本
helm uninstall carina-csi-driver 

```
helm pull  carina-csi-driver/carina-csi-driver  --version v0.9.1 
tar -zxvf carina-csi-driver-v0.9.1.tgz   
```
编辑 carina-csi-driver/templates/csi-config-map.yaml，把原来对应的节点上vg 组填在配置文件里
helm install carina-csi-driver carina-csi-driver/
##### 可配置参数说明

  {
      "diskSelector": [
        {
          "name": "carina-vg-ssd" ,  #原0.9.0版本默认vg组
          "re": ["loop2+"],          #这里需要修改匹配条件，可以查看节点磁盘名称是否满足此匹配。如果不满足，需要纳管哪些磁盘，相应这里填上满足匹配的条件即可
          "policy": "LVM",           # lvm管理
          "nodeLabel": "kubernetes.io/hostname" #节点匹配的标签,节点标签上有这个key，匹配条件才会生效
        },
        {
          "name": "carina-vg-hdd",   #原0.9.0版本默认vg组
          "re": ["loop3+"],
          "policy": "LVM",
          "nodeLabel": "kubernetes.io/hostname"
        },
        {
          "name": "exist-vg-group",  #已经存在的vg，可以别纳管
          "re": ["loop4+"],
          "policy": "LVM",
          "nodeLabel": "kubernetes.io/hostname"
        },
        {
          "name": "new-vg-group",    #空盘未匹配vg，这里可以创建新的vg纳管空盘
          "re": ["loop5+"],
          "policy": "LVM",
          "nodeLabel": "kubernetes.io/hostname"
        },
        {
          "name": "raw",
          "re": ["vdb+", "sd+"],
          "policy": "RAW",
          "nodeLabel": "kubernetes.io/hostname"
        }
      ],
      "diskScanInterval": "300",
      "schedulerStrategy": "spreadout"
    }
