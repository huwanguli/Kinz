package znet

import (
	"fmt"
	"zinx/ziface"
)

// MsgHandler 消息处理模块的实现
type MsgHandler struct {
	Apis map[uint32]ziface.IRouter // 存放每一个MsgID所对应的处理方法
}

// NewMsgHandler 初始化MsgHandler
func NewMsgHandler() *MsgHandler {
	return &MsgHandler{
		Apis: make(map[uint32]ziface.IRouter),
	}
}

// DoMsgHandler 调度/执行对应的Router消息处理方法
func (mh *MsgHandler) DoMsgHandler(request ziface.IRequest) {
	// 1.从request中读取msgId
	handler, ok := mh.Apis[request.GetMsgID()]
	if !ok {
		fmt.Println("[DoMsgHandler] No Handler, MsgID: ", fmt.Sprint(request.GetMsgID()))
	}
	// 2.根据ID调度对应的router业务
	handler.PreHandle(request)
	handler.Handle(request)
	handler.PostHandle(request)
}

// AddRouter 添加路由器
func (mh *MsgHandler) AddRouter(msgId uint32, router ziface.IRouter) {
	// 判断当前msg绑定的api方法是否已经存在
	if _, ok := mh.Apis[msgId]; ok {
		// id 已注册
		panic("repeat msgId : " + fmt.Sprint(msgId))
	}
	// 若不存在则添加
	mh.Apis[msgId] = router
	fmt.Println("add msgId : " + fmt.Sprint(msgId))
}
