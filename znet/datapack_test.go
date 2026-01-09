package znet

import (
	"fmt"
	"io"
	"net"
	"testing"
)

// 测试 datapack 封包拆包的单元测试
func TestDataPack(t *testing.T) {
	// 模拟的服务器
	// 创建socketTCP
	listener, err := net.Listen("tcp", "127.0.0.1:7777")
	if err != nil {
		fmt.Println("server listen err:", err)
		return
	}

	// go 承载从客户端处理业务
	go func() {
		// 从客户端读取数据、拆包处理
		for {
			conn, err := listener.Accept()
			if err != nil {
				fmt.Println("server accept err:", err)
			}

			go func(conn net.Conn) {
				// 处理客户端请求
				// 拆包 1.读出head 2.根据head信息读出数据
				// 定义拆包对象
				dp := NewDataPack()
				for {
					headData := make([]byte, dp.GetHeadLen())
					_, err := io.ReadFull(conn, headData)
					if err != nil {
						fmt.Println("read head err:", err)
						break
					}

					// 第二次读取
					msgHead, err := dp.Unpack(headData)
					if err != nil {
						fmt.Println("unpack head err:", err)
						return
					}
					if msgHead.GetDataLen() > 0 {
						// msg 有数据
						// 第二次读取
						msg := msgHead.(*Message)
						msg.Data = make([]byte, msg.GetDataLen())

						// 根据 dataLen 再次读取
						_, err := io.ReadFull(conn, msg.Data)
						if err != nil {
							fmt.Println("read msg data err:", err)
							return
						}

						// 至此读取消息完成
						fmt.Println("msg Id: ", msg.Id, "msg dataLen: ", msg.DataLen, "msg data", string(msg.Data))
					}
				}
			}(conn)
		}
	}()

	// 模拟客户端
	conn, err := net.Dial("tcp", "127.0.0.1:7777")
	if err != nil {
		fmt.Println("client Dial err:", err)
		return
	}

	// 创建一个分封包对象
	dp := NewDataPack()

	// 模拟粘包过程，将俩个msg一起发送
	// 封第一个包
	msg1 := &Message{
		Id:      1,
		DataLen: 4,
		Data:    []byte{'z', 'i', 'n', 'x'},
	}
	sendData1, err := dp.Pack(msg1)
	if err != nil {
		fmt.Println("client PackData1 err:", err)
		return
	}
	// 封第二个包
	msg2 := &Message{
		Id:      2,
		DataLen: 7,
		Data:    []byte{'n', 'i', 'h', 'a', 'o', '!', '!'},
	}
	sendData2, err := dp.Pack(msg2)
	if err != nil {
		fmt.Println("client PackData2 err:", err)
		return
	}
	// 粘在一起
	sendData1 = append(sendData1, sendData2...)
	// 一起发送
	conn.Write(sendData1)

	// 客户端阻塞
	select {}
}
