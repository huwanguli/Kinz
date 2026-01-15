package znet

import (
	"fmt"
	"time"
	"zinx/ziface"
	"zinx/zlog"
)

type HeartBeatChecker struct {
	interval         time.Duration //心跳检测时间间隔
	quitChan         chan bool     // 退出信号
	makeMsg          ziface.HeartBeatMsgFunc
	onRemoteNotAlive ziface.OnRemoteNotAlive

	msgID        uint32                 // 心跳的消息 ID
	router       ziface.IRouter         // 用户自定义的心跳检测消息业务处理路由
	routerSlices []ziface.RouterHandler // 新版路由
	conn         ziface.IConnection     // 绑定的路由

	beatFunc ziface.HeartBeatFunc //用户自定义心跳发送函数
}

func (h *HeartBeatChecker) SetHeartBeatMsgFunc(msgFunc ziface.HeartBeatMsgFunc) {
	if msgFunc == nil {
		h.makeMsg = msgFunc
	}
}

func (h *HeartBeatChecker) SetHeartbeatFunc(beatFunc ziface.HeartBeatFunc) {
	if beatFunc == nil {
		h.beatFunc = beatFunc
	}
}

func (h *HeartBeatChecker) BindRouter(msgID uint32, router ziface.IRouter) {
	if router != nil && msgID != ziface.HeartBeatDefaultMsgID {
		h.msgID = msgID
		h.router = router
	}
}

func (h *HeartBeatChecker) BindRouterSlices(msgID uint32, handler ...ziface.RouterHandler) {
	if len(handler) > 0 && msgID != ziface.HeartBeatDefaultMsgID {
		h.msgID = msgID
		h.routerSlices = append(h.routerSlices, handler...)
	}
}

func (h *HeartBeatChecker) start() {
	ticker := time.NewTicker(h.interval)
	for {
		select {
		case <-ticker.C:
			h.check()
		case <-h.quitChan:
			ticker.Stop()
			return
		}
	}
}

func (h *HeartBeatChecker) Start() {
	go h.start()
}

func (h *HeartBeatChecker) Stop() {
	zlog.L().InfoF("heartbeat checker stop, connID=%+v", h.conn.GetConnID())
}

func (h *HeartBeatChecker) SendHeartBeatMsg() error {
	msg := h.makeMsg(h.conn)

	err := h.conn.SendMsg(h.msgID, msg)
	if err != nil {
		zlog.L().ErrorF("send heartbeat msg error: %v, msgId=%+v msg=%+v", err, h.msgID, msg)
		return err
	}
	return nil
}
func (h *HeartBeatChecker) check() (err error) {
	if h.conn == nil {
		return nil
	}

	if !h.conn.IsAlive() {
		h.onRemoteNotAlive(h.conn)
	} else {
		if h.beatFunc != nil {
			err = h.beatFunc(h.conn)
		} else {
			err = h.SendHeartBeatMsg()
		}
	}
	return err
}

func (h *HeartBeatChecker) BindConn(conn ziface.IConnection) {
	h.conn = conn
	conn.SetHeartBeat(h)
}

func (h *HeartBeatChecker) Clone() ziface.IHeartbeatChecker {
	heartbeat := &HeartBeatChecker{
		interval:         h.interval,
		quitChan:         make(chan bool),
		makeMsg:          h.makeMsg,
		onRemoteNotAlive: h.onRemoteNotAlive,
		msgID:            h.msgID,
		beatFunc:         h.beatFunc,
		conn:             nil,
		router:           h.router,
	}
	heartbeat.routerSlices = append(heartbeat.routerSlices, h.routerSlices...)

	return heartbeat
}

func (h *HeartBeatChecker) MsgID() uint32 {
	return h.msgID
}

func (h *HeartBeatChecker) Router() ziface.IRouter {
	return h.router
}

func (h *HeartBeatChecker) RouterSlices() []ziface.RouterHandler {
	return h.routerSlices
}

// HeartBeatDefaultRouter 收到 remote 心跳消息的默认回调处理路由业务
type HeartBeatDefaultRouter struct {
	BaseRouter
}

func (r *HeartBeatDefaultRouter) Handle(req ziface.IRequest) {
	// TODO 补充req
	zlog.L().InfoF("Recv Heartbeat from %s, MsgID = %+v, Data = %s",
		req.GetConnection().RemoteAddr(), req.GetMsgID(), string(req.GetData()))
}

func HeartBeatDefaultHandle(req ziface.IRequest) {
	zlog.L().InfoF("Recv Heartbeat from %s, MsgID = %+v, Data = %s",
		req.GetConnection().RemoteAddr(), req.GetMsgID(), string(req.GetData()))
}

func makeDefaultMsg(conn ziface.IConnection) []byte {
	msg := fmt.Sprintf("heartbeat [%s->%s]", conn.LocalAddr(), conn.RemoteAddr())
	return []byte(msg)
}

func notAliveDefaultFunc(conn ziface.IConnection) {
	zlog.L().InfoF("Remote connection %s is not alive, stop it", conn.RemoteAddr())
	conn.Stop()
}

func NewHeartbeatChecker(interval time.Duration) ziface.IHeartbeatChecker {
	heartbeat := &HeartBeatChecker{
		interval: interval,
		quitChan: make(chan bool),

		// 使用默认的处理方法
		makeMsg:          makeDefaultMsg,
		onRemoteNotAlive: notAliveDefaultFunc,
		msgID:            ziface.HeartBeatDefaultMsgID,
		router:           &HeartBeatDefaultRouter{},
		routerSlices:     []ziface.RouterHandler{HeartBeatDefaultHandle},
		beatFunc:         nil,
	}
	return heartbeat
}

func (h *HeartBeatChecker) SetOnRemoteNotAlive(f ziface.OnRemoteNotAlive) {
	if f != nil {
		h.onRemoteNotAlive = f
	}
}
