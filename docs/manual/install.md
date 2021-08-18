#### Carina部署

##### 部署条件

- Kubernetes 集群（已验证版本1.18.2，1.19.4， 1.20.4）
- 如果kubelet以容器化方式运行，需要挂载主机`/dev`目录
- Linux Kernal >= 3.10.0-1160.11.1.el7.x86_64
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

