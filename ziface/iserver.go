package ziface

import (
	"time"
)

// IServer 定义一个服务器接口
type IServer interface {
	// Start 启动服务器
	Start()
	// Stop 停止服务器
	Stop()
	// Serve 运行服务器
	Serve()

	// AddRouter 路由功能 给当前服务注册一个路由功能，供客户端使用
	AddRouter(msgID uint32, router IRouter)

	// AddRouterSlices REPRO: 后续新版路由方式: 路由集合
	AddRouterSlices(msgID uint32, router ...RouterHandler) IRouterSlices

	// Group REPRO:路由组管理
	Group(start, end uint32, Handlers ...RouterHandler) IGroupRouterSlices

	// Use REPRO:公共组件管理
	Use(Handlers ...RouterHandler) IRouterSlices

	// GetConnMgr 得到ConnMgr
	GetConnMgr() IConnManager

	// SetOnConnStart 注册OnConnStart 钩子函数的方法
	SetOnConnStart(func(IConnection))

	// SetOnConnStop 注册OnConnStop 钩子函数的方法
	SetOnConnStop(func(IConnection))

	// GetOnConnStart REPRO:得到创建钩子函数
	GetOnConnStart() func(IConnection)

	// GetOnConnStop REPRO:得到断开钩子函数
	GetOnConnStop() func(IConnection)

	// GetPacket REPRO:得到数据协议封包方式
	GetPacket() IDataPack

	// GetMsgHandler REPRO: 得到消息处理模块
	GetMsgHandler() IMsgHandle

	// SetPacket REPRO : 设置数据协议封包方式
	SetPacket(IDataPack)

	// StartHeartBeat REPRO : 启动心跳检测
	StartHeartBeat(time.Duration)

	// SetHeartBeatWithOption REPRO : 启动心跳检测（自定义回调）
	SetHeartBeatWithOption(time.Duration, *HeartBeatOption)

	// GetHeartBeat REPRO : 获取心跳检测器
	GetHeartBeat() IHeartbeatChecker

	// REPRO : 封包解包相关

	GetLengthField() *LengthField
	SetDecoder(IDecoder)
	AddInterceptor(IInterceptor)

	// SetWebsocketAuth REPRO：WebSocket相关
	// SetWebsocketAuth 添加websocket的认证方法
	// SetWebsocketAuth(func(r *http.Request) error)

	// ServerName REPRO: 得到服务器名称
	ServerName() string

	// CallOnConnStart TODEL:调用OnConnStart 钩子函数的方法
	CallOnConnStart(connection IConnection)

	// CallOnConnStop TODEL:调用OnConnStop 钩子函数的方法
	CallOnConnStop(connection IConnection)
}
