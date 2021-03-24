package local_proxy_server

import (
	"fmt"
	"io"
	"net"
)

var (
	localUnix string = "/tmp/csi/csi-provisioner.sock"
	localAddr string = "192.168.56.101:8888"
)

func main() {
	fmt.Println("this is proxy server:")
	ln, err := net.Listen("tcp", localAddr)
	if err != nil {
		fmt.Println("tcp_listen:", err)
		return
	}
	defer ln.Close()
	for {
		tcpConn, err := ln.Accept() //接受tcp客户端连接，并返回新的套接字进行通信
		if err != nil {
			fmt.Println("Accept:", err)
			return
		}
		go serverHandle(tcpConn) //创建新的协程进行转发
	}
}

func serverHandle(tcpConn net.Conn) {
	remoteTcp, err := net.Dial("unix", localUnix) //连接目标服务器
	if err != nil {
		fmt.Println(err)
		return
	}
	go io.Copy(remoteTcp, tcpConn)
	go io.Copy(tcpConn, remoteTcp)
}
