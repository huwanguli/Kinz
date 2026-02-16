package znet

import (
	"fmt"
	"zinx/utils"
	"zinx/ziface"
)

// MsgHandler 消息处理模块的实现
type MsgHandler struct {
	Apis map[uint32]ziface.IRouter // 存放每一个MsgID所对应的处理方法
	// 负责Worker读取任务的消息队列
	TaskQueue []chan ziface.IRequest
	// Worker工作池的数量
	WorkerPoolSize uint32
}

// NewMsgHandler 初始化MsgHandler
func NewMsgHandler() *MsgHandler {
	return &MsgHandler{
		Apis:           make(map[uint32]ziface.IRouter),
		WorkerPoolSize: utils.GlobalObject.WorkerPoolSize, // 从全局配置中获取
		TaskQueue:      make([]chan ziface.IRequest, utils.GlobalObject.WorkerPoolSize),
	}
}

// DoMsgHandler 调度/执行对应的Router消息处理方法
func (mh *MsgHandler) DoMsgHandler(request ziface.IRequest) {
	// 1.从request中读取msgId
	handler, ok := mh.Apis[request.GetMsgID()]
	if !ok {
		fmt.Println("[DoMsgHandler] No Handler, MsgID: ", fmt.Sprint(request.GetMsgID()))
		return
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

// StartWorkerPool 启动一个Worker工作池 (开启工作池的动作只能发生一次)
func (mh *MsgHandler) StartWorkerPool() {
	// 根据WorkerPoolSize分别开启Worker，每个Worker用一个go承载
	for i := 0; i < int(mh.WorkerPoolSize); i++ {
		// 一个worker被启动
		// 1.当前的Worker对应的Channel 开辟空间 以i为编号
		mh.TaskQueue[i] = make(chan ziface.IRequest, utils.GlobalObject.MaxWorkerTaskLen)
		// 2.启动当前的Worker，阻塞等待当前channel中消息的到来
		go mh.StartOneWorker(i, mh.TaskQueue[i])
	}
}

// StartOneWorker 启动一个Worker工作流程
func (mh *MsgHandler) StartOneWorker(workerID int, taskQueue chan ziface.IRequest) {
	fmt.Println("[StartOneWorker] workerID : " + fmt.Sprint(workerID))
	// 不断的阻塞等待对应的channel的消息
	for {
		select {
		// 如果有消息传来，出列的就是一个客户端的Request，执行当前Request的业务
		case request := <-taskQueue:
			mh.DoMsgHandler(request)
		}
	}
}

// SendMsgToTaskQueue 将消息交给TaskQueue,由Worker进行处理
func (mh *MsgHandler) SendMsgToTaskQueue(request ziface.IRequest) {
	// 1.将消息平均分配给不同的Worker
	//根据ConnID进行分配  TODO（后续可以通过更好的算法进行优化）
	workerID := request.GetConnection().GetConnID() % mh.WorkerPoolSize
	fmt.Println("[SendMsgToTaskQueue] ConnID:" + fmt.Sprint(request.GetConnection().GetConnID()) +
		"to workerID : " + fmt.Sprint(workerID))
	// 2.将消息发送给对应的Worker的TaskQueue即可
	mh.TaskQueue[workerID] <- request
}
