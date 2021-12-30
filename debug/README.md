
##### 本地项目开发
  - 为了方便的进行CSI Plugin本地开发调试，在此设计了几个简便方案
  
##### carina-controller
  - 首先CSI Provisioner Pod中多个容器使用共享sock文件进行通信
  - 这样我们在本地环境启动这几个容器，挂载同一个目录共享sock文件
  - ./csi-provisioner.sh 如此就可以方便的进行本地调试了
  
##### carina-node
  - CSI Node 这个麻烦点，因为它要暴露sock文件给kubelet， 真正调用Server的请求是kubelet发起的
  - 然而Kubelet监听的sock文件，只能用于同一个主机上进程间通信，无法跨主机通信
  - 所以编写了两个文件，进行socket代理
  - 本地运行local-proxy-server/server.go，在k8s节点运行node-proxy-client/client.go, 执行csi-node-register.sh脚本进行驱动注册
  - 本地IDE收到请求有助于观察kubelet Request参数及参数来源，真实执行mount等动作无法成功
  
##### 本地磁盘lvm卷
  - 原本创建volume需要client-go调谐程序，调用实现方法，才能进行lvm卷管理
  - 本地启动lvm-http-server服务，发起http请求即可进行lvm卷管理


  