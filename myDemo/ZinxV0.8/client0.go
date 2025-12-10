package main

import (
	"fmt"
	"io"
	"net"
	"time"
	"zinx/znet"
)

// 模拟客户端
func main() {
	fmt.Println("client start...")
	time.Sleep(1 * time.Second)
	// 1.直接链接服务器，得到一个conn链接
	conn, err := net.Dial("tcp", "127.0.0.1:7777")
	if err != nil {
		fmt.Printf("client Connect err:%s, exit!\n", err)
		return
	}
	// 2.链接调用Write，写入数据
	for {
		// 发送封包的Msg消息 MsgId为0
		dp := znet.NewDataPack()
		binaryMsg, err := dp.Pack(znet.NewMessage(1, []byte("ZinxV0.8 client0 Test Message")))
		if err != nil {
			fmt.Printf("client Pack err:%s\n", err)
			return
		}
		_, err = conn.Write(binaryMsg)
		if err != nil {
			fmt.Printf("client Write err:%s\n", err)
			return
		}

		// 服务器应该向客户端回复一个Msg Id为1 Data为ping

		// 获取head部分 得到ID 和 Len
		// 再根据head部分获取data部分

		binaryHead := make([]byte, dp.GetHeadLen())
		if _, err := io.ReadFull(conn, binaryHead); err != nil {
			fmt.Printf("client Read head err:%s\n", err)
			return
		}
		// 将二进制的head拆包到msg结构体中
		msgHead, err := dp.Unpack(binaryHead)
		if err != nil {
			fmt.Printf("client Unpack err:%s\n", err)
			return
		}

		if msgHead.GetDataLen() > 0 {
			// msg有数据，根据Len再次读取
			msg := msgHead.(*znet.Message)
			msg.Data = make([]byte, msg.GetDataLen())
			_, err := io.ReadFull(conn, msg.Data)
			if err != nil {
				fmt.Printf("client Read data err:%s\n", err)
				return
			}

			fmt.Println("-->Rev msg:", string(msg.Data), ",Id:", msg.Id, ",Len:", msg.DataLen)
		}

		// cpu阻塞
		time.Sleep(1 * time.Second)
	}
}
