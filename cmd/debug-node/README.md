
##### 需求
  - 为了方便的对CSI Node Server进行开发调试，需要Kubelet直接调用本地IDE启动的服务
  
##### 需求分析  
  - Kubelet与CSI Node Server通过socket套接字通信
  - 因为CSI Node Server监听的是Unix套接字，只能进行单主机进程间通信，无法跨主机通信
  - 因此需要一个socket代理服务，分别在Kubelet节点和本地开发环境启动，进行流量转发
  
##### 使用方法
  - server.go 直接本地运行，其目标unix地址为CSI Node Server监听的sock文件
  - client.go 编译成二进制文集，在Kubelet节点运行
  
##### 注意
  - client.go转发地址与server.go监听地址不同，是因为我的开发环境为虚拟机，在宿主机做了端口映射
