package znet

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"zinx/utils"
	"zinx/ziface"
)

// Connection 链接模块
type Connection struct {
	// 当前Conn隶属于的Server
	TcpServer ziface.IServer
	// socket TCP套接字
	Conn *net.TCPConn
	// 链接的ID
	ConnID uint32
	// 链接的状态
	isClosed bool
	// 告知当前连接已经退出的channel (由Reader告知Writer)
	ExitChan chan bool
	// 无缓冲的管道，用于Reader 和 Writer之间的通信
	msgChan chan []byte
	// 消息的管理msgID与相对于的处理业务API MsgHandler
	MsgHandler   ziface.IMsgHandle
	property     map[string]interface{}
	propertyLock sync.RWMutex
}

func (c *Connection) SetProperty(key string, value interface{}) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()
	c.property[key] = value
}

func (c *Connection) GetProperty(key string) (interface{}, error) {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()
	if value, ok := c.property[key]; ok {
		return value, nil
	}

	return nil, errors.New("no property found")
}

func (c *Connection) RemoveProperty(key string) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()
	delete(c.property, key)
}

// NewConnection 初始化链接模块的方法
func NewConnection(server ziface.IServer, conn *net.TCPConn, connID uint32, msgHandler ziface.IMsgHandle) *Connection {
	c := &Connection{
		TcpServer:  server,
		Conn:       conn,
		ConnID:     connID,
		isClosed:   false,
		ExitChan:   make(chan bool, 1),
		msgChan:    make(chan []byte),
		MsgHandler: msgHandler,
		property:   make(map[string]interface{}),
	}

	// 将conn加入connMgr的Map中
	c.TcpServer.GetConnMgr().Add(c)

	return c
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

		// 判断是否已经开启工作池
		if utils.GlobalObject.WorkerPoolSize > 0 {
			// 将request发送给对应的TaskQueue
			c.MsgHandler.SendMsgToTaskQueue(req)
		} else {
			// 若Worker池没有启动，则仍然由一个单独的goroutine承载
			go c.MsgHandler.DoMsgHandler(req)
		}
	}
}

// StartWriter 写消息的goroutine 专门发送给客户端消息的模块
func (c *Connection) StartWriter() {
	fmt.Println("[Writer Goroutine is running]", c.Conn.RemoteAddr().String())
	defer fmt.Println("[Writer goroutine is stopped]", c.Conn.RemoteAddr().String())

	// 不断地阻塞等待channel的消息
	for {
		select {
		case data := <-c.msgChan:
			// 有数据要写给客户端
			if _, err := c.Conn.Write(data); err != nil {
				fmt.Println("write data error", err)
				return
			}
		case <-c.ExitChan:
			// 代表Reader已经退出，则Writer也要推出
			return
		}
	}
}

func (c *Connection) Start() {
	fmt.Println("Connection Start().. ConnID = ", c.ConnID)

	// 启动当前连接的读数据的业务
	go c.StartReader()

	// 启动从当前链接写数据的业务
	go c.StartWriter()

	// 按照开发者传递进来的创建链接需要调用的执行业务
	c.TcpServer.CallOnConnStart(c)
}

func (c *Connection) Stop() {
	fmt.Println("Connection Stop().. ConnID = ", c.ConnID)

	// 如果当前链接已经关闭
	if c.isClosed {
		return
	}
	c.isClosed = true

	// 按照开发者的需要调用Hook函数
	c.TcpServer.CallOnConnStop(c)

	// 关闭socket链接
	err := c.Conn.Close()
	if err != nil {
		fmt.Println("Connection Stop Error", err)
		panic(err)
	}
	// 告知Writer关闭
	c.ExitChan <- true

	// 将Conn从ConnMgr中删除
	c.TcpServer.GetConnMgr().Remove(c)

	// 回收资源
	close(c.ExitChan)
	close(c.msgChan)
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
	c.msgChan <- binaryMsg
	return nil
}
