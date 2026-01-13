package znet

import (
	"context"
	//"fmt"
	"log"
	//"net"
	"sync"
	"zinx/ziface"
)

type Client struct {
	sync.WaitGroup
	sync.Mutex
	started     bool
	ctx         context.Context
	cancel      context.CancelFunc
	Name        string
	Ip          string
	Port        int
	version     string
	conn        ziface.IConnection
	connMux     sync.Mutex
	onConnStart func(conn ziface.IConnection)
	onConnStop  func(conn ziface.IConnection)
	packet      ziface.IDataPack
	msgHandler  ziface.IMsgHandle
	errChan     chan error
}

func (c *Client) Restart() {
	c.Stop()

	c.Lock()

	if c.started {
		c.Unlock()
		return
	}

	c.started = true
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.Add(1)
	c.Unlock()

	go func() {
		defer c.Done()

		var connect ziface.IConnection

		switch c.version {
		default:
			//var conn net.Conn
			var err error
			// TODO 添加安全机制
			//d := &net.Dialer{}
			//conn, err = d.DialContext(c.ctx, "tcp", fmt.Sprintf("%s:%d", net.ParseIP(c.Ip), c.Port))
			if err != nil {
				// TODO 使用自定义全局日志
				log.Printf("connect err:%v", err)
				return
			}
			//connect = NewClientConn(c, conn.(*net.TCPConn))
		}
		c.SetConn(connect)
		log.Printf("connect success:%v", connect)

		go connect.Start()
		<-c.ctx.Done()
		log.Println("client exit.")
	}()
}

func (c *Client) Start() {
	// TODO 添加其他的机制
	c.Restart()
}

func (c *Client) Stop() {
	c.Lock()
	defer c.Unlock()
	if !c.started {
		return
	}
	c.started = false
	con := c.Conn()
	if con != nil {
		con.Stop()
	}

	if c.cancel != nil {
		c.cancel()
	}

	c.Wait()

}

func (c *Client) AddRouter(msgID uint32, router ziface.IRouter) {
	c.msgHandler.AddRouter(msgID, router)
}

func (c *Client) Conn() ziface.IConnection {
	c.connMux.Lock()
	defer c.connMux.Unlock()
	return c.conn
}

func (c *Client) SetConn(conn ziface.IConnection) {
	c.connMux.Lock()
	defer c.connMux.Unlock()
	c.conn = conn
}

func (c *Client) SetOnConnStart(f func(ziface.IConnection)) {
	c.onConnStart = f
}

func (c *Client) SetOnConnStop(f func(ziface.IConnection)) {
	c.onConnStop = f
}

func (c *Client) GetOnConnStart() func(ziface.IConnection) {
	return c.onConnStart
}

func (c *Client) GetOnConnStop() func(ziface.IConnection) {
	return c.onConnStop
}

func (c *Client) GetPacket() ziface.IDataPack {
	return c.packet
}

func (c *Client) SetPacket(pack ziface.IDataPack) {
	c.packet = pack
}

func (c *Client) GetMsgHandler() ziface.IMsgHandle {
	return c.msgHandler
}

func (c *Client) GetErrChan() <-chan error {
	return c.errChan
}

func (c *Client) SetName(s string) {
	c.Name = s
}

func (c *Client) GetName() string {
	return c.Name
}

func NewClient(ip string, port int, opts ...ClientOption) ziface.IClient {

	c := &Client{
		Name: "TcpClient",
		Ip:   ip,
		Port: port,

		msgHandler: NewMsgHandler(),
		packet:     NewDataPack(),
		version:    "tcp",
		errChan:    make(chan error, 1),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
