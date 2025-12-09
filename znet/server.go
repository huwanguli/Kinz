package znet

import (
	"errors"
	"fmt"
	"net"
	"zinx/ziface"
)

// Server IServer的接口实现，定义一个Server的服务器模块
type Server struct {
	// 服务器的名称
	Name string
	// 服务器绑定的ip版本
	IPVersion string
	// 服务器监听的ip
	IP string
	// 服务器监听的端口
	Port int
}

// CallBackToClient 定义当前客户端链接所绑定的handle API TODO(暂时写死，后续优化，由用户自定义)
func CallBackToClient(conn *net.TCPConn, data []byte, cnt int) error {
	// 回显业务
	fmt.Println("[Conn Handle] CallBackToClient ...")
	if _, err := conn.Write(data[:cnt]); err != nil {
		fmt.Println("[Conn Handle] CallBackToClient Write err:", err)
		return errors.New("CallBackToClient Write err:" + err.Error())
	}

	return nil
}

func (s *Server) Start() {
	// 1.获取一个TCP的Addr
	fmt.Printf("[Start] Server Listenner at IP :%s, Port:%d, is starting\n", s.IP, s.Port)

	go func() {
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

			// 将新连接的业务方法和Conn进行绑定，得到链接模块
			dealConn := NewConnection(conn, cid, CallBackToClient)
			cid++

			// 启动
			go dealConn.Start()
		}
	}()
}

func (s *Server) Stop() {
	// TODO 将服务器的资源、状态或者一些已经开辟的链接信息进行停止
}

func (s *Server) Serve() {
	// 启动服务器
	s.Start()

	// TODO 可以做一些启动服务器地额外的业务

	// 阻塞状态
	select {}
}

// NewServer 初始化Server模块
func NewServer(name string) ziface.IServer {
	s := &Server{
		Name:      name,
		IPVersion: "tcp4",
		IP:        "0.0.0.0",
		Port:      8999,
	}
	return s
}
