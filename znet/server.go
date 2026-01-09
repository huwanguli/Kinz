package znet

import (
	"fmt"
	"net"
	"zinx/utils"
	"zinx/ziface"
)

// Server IServer的接口实现，定义一个Server的服务器模块
type Server struct {
	// 服务器的名称
	Name string
	// 服务器绑定的 ip版本
	IPVersion string
	// 服务器监听的 ip
	IP string
	// 服务器监听的端口
	Port int

	// 当前server的消息管理模块，用来绑定MsgID和对应的处理业务API的关系
	MsgHandler ziface.IMsgHandle

	// 链接管理器
	ConnMgr ziface.IConnManager

	// 该Server创建链接之后自动调用的 Hook函数
	OnConnStart func(conn ziface.IConnection)

	// 该Server销毁链接之后自动调用的 Hook函数
	OnConnStop func(conn ziface.IConnection)
}

func (s *Server) Start() {
	// 1.获取一个TCP的Addr
	fmt.Printf("[Start] Server Listenner at IP :%s, Port:%d, is starting\n", s.IP, s.Port)

	go func() {
		// 开启消息队列及 Worker工作池
		s.MsgHandler.StartWorkerPool()

		addr, err := net.ResolveTCPAddr(s.IPVersion, fmt.Sprintf("%s:%d", s.IP, s.Port))
		if err != nil {
			fmt.Printf("[Start] ResolveTCPAddr err:%s\n", err)
			return
		}

		// 2.尝试监听服务器的地址
		listenner, err := net.ListenTCP(s.IPVersion, addr)
		if err != nil {
			fmt.Printf("[Start] ListenTCP err:%s\n", err)
			return
		}

		fmt.Printf("[Start] Server Listenner at IP :%s\n", s.IP)
		var cid uint32
		cid = 0
		// 3.阻塞地等待客户端连接，处理客户端链接业务
		for {
			// 如果有客户端链接，阻塞会返回
			conn, err := listenner.AcceptTCP()
			if err != nil {
				fmt.Printf("[Start] AcceptTCP err:%s\n", err)
				continue
			}
			fmt.Printf("[Start] Accept TCP Conn:%s\n", conn.RemoteAddr().String())
			// 判断当前最大连接个数是否超出
			if s.ConnMgr.Len() >= utils.GlobalObject.MaxConn {
				fmt.Printf("[Start] MaxConn:%d\n, 已超出最大范围", s.ConnMgr.Len())
				// TODO 给用户响应一个错误信息（超出最大连接错误）
				conn.Close()
				continue
			}

			// 将新连接的业务方法和Conn进行绑定，得到链接模块
			dealConn := NewConnection(s, conn, cid, s.MsgHandler)
			cid++

			// 启动
			go dealConn.Start()
		}
	}()
}

func (s *Server) Stop() {
	// 将服务器的资源、状态或者一些已经开辟的链接信息进行停止
	fmt.Printf("[Stop] Server Listenner at IP :%s\n", s.IP)
	s.ConnMgr.ClearConn()
}

func (s *Server) Serve() {
	// 启动服务器
	s.Start()

	// TODO 可以做一些启动服务器地额外的业务

	// 阻塞状态
	select {}
}

// AddRouter 添加路由
func (s *Server) AddRouter(msgID uint32, r ziface.IRouter) {
	s.MsgHandler.AddRouter(msgID, r)
	//fmt.Printf("[AddRouter] Router is %#v\n", s.MsgHandler)
}

// NewServer 初始化Server模块
func NewServer() ziface.IServer {
	s := &Server{
		Name:       utils.GlobalObject.Name,
		IPVersion:  "tcp4",
		IP:         utils.GlobalObject.Host,
		Port:       utils.GlobalObject.TcpPort,
		MsgHandler: NewMsgHandler(),
		ConnMgr:    NewConnManager(),
	}
	return s
}

func (s *Server) GetConnMgr() ziface.IConnManager {
	return s.ConnMgr
}

// SetOnConnStart 注册OnConnStart 钩子函数的方法
func (s *Server) SetOnConnStart(hookFunc func(connection ziface.IConnection)) {
	s.OnConnStart = hookFunc
}

// SetOnConnStop 注册OnConnStop 钩子函数的方法
func (s *Server) SetOnConnStop(hookFunc func(connection ziface.IConnection)) {
	s.OnConnStop = hookFunc
}

// CallOnConnStart 调用OnConnStart 钩子函数的方法
func (s *Server) CallOnConnStart(conn ziface.IConnection) {
	if s.OnConnStart != nil {
		fmt.Println("[CallOnConnStart] OnConnStart is called")
		s.OnConnStart(conn)
	}
}

// CallOnConnStop 调用OnConnStop 钩子函数的方法
func (s *Server) CallOnConnStop(conn ziface.IConnection) {
	if s.OnConnStop != nil {
		fmt.Println("[CallOnConnStop] OnConnStop is called")
		s.OnConnStop(conn)
	}
}
