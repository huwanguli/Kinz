package main

import (
	"fmt"
	"net"
	"time"
)

// 模拟客户端
func main() {
	fmt.Println("client start...")
	time.Sleep(1 * time.Second)
	// 1.直接链接服务器，得到一个conn链接
	conn, err := net.Dial("tcp", "127.0.0.1:8999")
	if err != nil {
		fmt.Printf("client Connect err:%s, exit!\n", err)
		return
	}
	// 2.链接调用Write，写入数据
	for {
		_, err := conn.Write([]byte("hello ZinxV0.2!"))
		if err != nil {
			fmt.Printf("client Write err:%s\n", err)
			return
		}

		buf := make([]byte, 512)
		cnt, err := conn.Read(buf)
		if err != nil {
			fmt.Printf("client Read err:%s\n", err)
			return
		}

		fmt.Printf("server call back :%s\n", buf[:cnt])

		// cpu阻塞
		time.Sleep(1 * time.Second)
	}
}
