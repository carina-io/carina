package main

import (
	"fmt"
	"io"
	"net"
	"os"
)

var (
	clientUnix string = "/var/lib/kubelet/plugins/csi.carina.com/csi.sock"
	ServerAddr string = "10.40.20.66:8888"
)

func main() {
	_ = os.Remove(clientUnix)
	ln, err := net.Listen("unix", clientUnix)
	if err != nil {
		fmt.Println("unix_listen:", err)
		return
	}
	defer ln.Close()
	for {
		tcpConn, err := ln.Accept() //接受tcp客户端连接，并返回新的套接字进行通信
		if err != nil {
			fmt.Println("Accept:", err)
			return
		}
		go clientHandle(tcpConn) //创建新的协程进行转发
	}
}

func clientHandle(tcpConn net.Conn) {
	remoteTcp, err := net.Dial("tcp", ServerAddr) //连接目标服务器
	if err != nil {
		fmt.Println(err)
		return
	}
	go io.Copy(remoteTcp, tcpConn)
	go io.Copy(tcpConn, remoteTcp)
}
