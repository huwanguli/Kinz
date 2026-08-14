package kiface

import (
	"net/url"
	"time"
)

type IClient interface {
	Restart() // 重启
	Start()   //启动
	Stop()    //停止
	// AddRouter 添加路由
	AddRouter(msgID uint32, router IRouter)
	Conn() IConnection

	// SetOnConnStart Client连接创建时的Hook函数
	SetOnConnStart(func(IConnection))

	// SetOnConnStop 连接断开时调用的Hook函数
	SetOnConnStop(func(IConnection))

	// GetOnConnStart 获取创建时的Hook函数
	GetOnConnStart() func(IConnection)

	GetOnConnStop() func(IConnection)

	// GetPacket 获取数据协议封包方式
	GetPacket() IDataPack

	// SetPacket 设置数据协议封包方式
	SetPacket(IDataPack)

	// GetMsgHandler 获取绑定的消息处理模块
	GetMsgHandler() IMsgHandle

	StartHeartBeat(time.Duration)

	StartHeartBeatWithOption(time.Duration, *HeartBeatOption)

	GetLengthField() *LengthField
	SetDecoder(IDecoder)
	AddInterceptor(IInterceptor) //添加拦截器
	GetErrChan() chan error

	// SetName 设置名称
	SetName(string)
	// GetName 返回名称
	GetName() string

	SetUrl(url *url.URL)
	GetUrl() *url.URL
}
