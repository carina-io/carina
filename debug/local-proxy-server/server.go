/*
   Copyright @ 2021 fushaosong <fushaosong@beyondlet.com>.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/
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
