package znet

import (
	"errors"
	"fmt"
	"io"
	"net"
	"zinx/ziface"
)

// Connection 链接模块
type Connection struct {
	// socket TCP套接字
	Conn *net.TCPConn
	// 链接的ID
	ConnID uint32
	// 链接的状态
	isClosed bool
	// 告知当前连接已经退出的channel
	ExitChan chan bool
	// 消息的管理msgID与相对于的处理业务API MsgHandler
	MsgHandler ziface.IMsgHandle
}

// NewConnection 初始化链接模块的方法
func NewConnection(conn *net.TCPConn, connID uint32, msgHandler ziface.IMsgHandle) *Connection {
	return &Connection{
		Conn:       conn,
		ConnID:     connID,
		isClosed:   false,
		ExitChan:   make(chan bool, 1),
		MsgHandler: msgHandler,
	}
}

// StartReader 读数据的业务
func (c *Connection) StartReader() {
	fmt.Println("Reader Goroutine is running...", c.Conn.RemoteAddr().String())
	// 退出时直接将该链接关闭
	defer fmt.Println("Reader goroutine is stopped")
	defer c.Stop()
	// 处理业务
	for {
		// 创建一个拆包解包的对象
		dp := NewDataPack()
		// 读取客户端的MSG HEAD 8个字节
		headData := make([]byte, dp.GetHeadLen())
		if _, err := io.ReadFull(c.GetTCPConnection(), headData); err != nil {
			fmt.Println("read head data error", err)
			break
		}

		// 拆包得到msgID和msgDataLen 放进一个msg消息对象中
		msg, err := dp.Unpack(headData)
		if err != nil {
			fmt.Println("unpack data error", err)
			break
		}
		// 根据msgDataLen再次读取，并放进msg.Data中
		var data []byte
		if msg.GetDataLen() > 0 {
			data = make([]byte, msg.GetDataLen())
			if _, err := io.ReadFull(c.GetTCPConnection(), data); err != nil {
				fmt.Println("read data error", err)
				break
			}
		}
		msg.SetData(data)

		// 得到当前链接的数据的Request对象
		req := &Request{
			Conn: c,
			msg:  msg,
		}

		// 调用路由，从路由中找到注册的Conn对应的Router路由并执行
		go c.MsgHandler.DoMsgHandler(req)

	}
}

func (c *Connection) Start() {
	fmt.Println("Connection Start().. ConnID = ", c.ConnID)

	// 启动当前连接的读数据的业务
	go c.StartReader()

	// TODO 启动从当前链接写数据的业务
}

func (c *Connection) Stop() {
	fmt.Println("Connection Stop().. ConnID = ", c.ConnID)

	// 如果当前链接已经关闭
	if c.isClosed {
		return
	}
	c.isClosed = true

	// 关闭socket链接
	err := c.Conn.Close()
	if err != nil {
		fmt.Println("Connection Stop Error", err)
		panic(err)
	}

	// 回收资源
	close(c.ExitChan)
}

func (c *Connection) GetTCPConnection() *net.TCPConn {
	return c.Conn
}

func (c *Connection) GetConnID() uint32 {
	return c.ConnID
}

func (c *Connection) GetRemoteAddr() net.Addr {
	return c.Conn.RemoteAddr()
}

// SendMsg 提供一个封包的方法
func (c *Connection) SendMsg(msgId uint32, data []byte) error {
	if c.isClosed {
		return errors.New("connection is closed when send msg")
	}
	// 将data进行分包 MsgDataLen MsgId Data
	dp := NewDataPack()
	binaryMsg, err := dp.Pack(NewMessage(msgId, data))
	if err != nil {
		fmt.Println("Pack error msgId :=", msgId, err)
		return err
	}
	// 将数据发送给客户端
	_, err = c.Conn.Write(binaryMsg)
	if err != nil {
		return err
	}

	return nil
}
