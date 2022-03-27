/*
   Copyright @ 2021 bocloud <fushaosong@beyondcent.com>.

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

package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
)

var (
	clientUnix string
	ServerAddr string
)

func init() {
	flag.StringVar(&clientUnix, "csi-address", "/var/lib/kubelet/plugins/csi.carina.com/csi.sock", "csi.sock path")
	flag.StringVar(&ServerAddr, "server-address", "0.0.0.0:8888", "server 地址")
}

func main() {
	flag.Parse()
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
