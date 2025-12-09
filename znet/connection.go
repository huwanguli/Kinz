package znet

import (
	"fmt"
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

	// 当前链接所绑定的处理业务API
	handleAPI ziface.HandleFunc

	// 告知当前连接一直推出的channel
	ExitChan chan bool
}

// NewConnection 初始化链接模块的方法
func NewConnection(conn *net.TCPConn, connID uint32, callbackApi ziface.HandleFunc) *Connection {
	return &Connection{
		Conn:      conn,
		ConnID:    connID,
		isClosed:  false,
		handleAPI: callbackApi,
		ExitChan:  make(chan bool, 1),
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
		// 读取数据到buf中, 暂时最大512字节
		buf := make([]byte, 512)
		cnt, err := c.Conn.Read(buf)
		if err != nil {
			// 读取失败则跳过这一次读取
			fmt.Println("Reader error:", err)
			continue
		}
		// 调用当前链接所绑定的 handle API
		if err := c.handleAPI(c.Conn, buf, cnt); err != nil {
			fmt.Println(c.ConnID, "Conn handle error:", err)
			break
		}
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
	c.Conn.Close()

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

func (c *Connection) Send(data []byte) error {
	return nil
}
