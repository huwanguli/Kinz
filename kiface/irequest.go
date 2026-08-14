package kiface

type HandleStep int

// IFuncRequest 函数消息接口
type IFuncRequest interface {
	CallFunc()
}

// IRequest 将客户端请求的链接信息和请求数据绑定在一起
type IRequest interface {
	// GetConnection 得到当前链接数据
	GetConnection() IConnection

	// GetData 得到当前数据
	GetData() []byte

	// GetMsgID 得到当前请求消息的ID
	GetMsgID() uint32

	GetMessage() IMessage //获取请求消息的原始数据

	GetResponse() IcResp // 获取解析完后序列化数据

	SetResponse(IcResp) // 设置解析完后序列化数据

	BindRouter(router IRouter) // 绑定这次请求由哪个路由处理

	Call() // 转进到下一个处理器执行

	Abort() // 终止处理函数的运行，但调用该方法的函数会处理完毕

	// Goto TODO : 暂时不实现 指定接下来的Handle去执行哪个函数
	Goto()

	BindRouterSlices([]RouterHandler)

	RouterSlicesNext() // 执行下一个函数

	Copy() IRequest
	// Set 在Request中存放一个上下文
	Set(key string, value interface{})
	Get(key string) (interface{}, bool)
}

type BaseRequest struct{}

func (b BaseRequest) GetConnection() IConnection {
	return nil
}
func (b BaseRequest) GetData() []byte {
	return nil
}
func (b BaseRequest) GetMsgID() uint32 {
	return 0
}
func (b BaseRequest) GetMessage() IMessage {
	return nil
}
func (b BaseRequest) GetResponse() IcResp {
	return nil
}
func (b BaseRequest) SetResponse(resp IcResp) {

}
func (b BaseRequest) BindRouter(router IRouter) {

}
func (b BaseRequest) Call() {

}
func (b BaseRequest) Abort() {

}
func (b BaseRequest) Goto() {

}
func (b BaseRequest) BindRouterSlices(handlers []RouterHandler) {

}
func (b BaseRequest) RouterSlicesNext() {

}
func (b BaseRequest) Copy() IRequest {
	return nil
}
func (b BaseRequest) Set(key string, value interface{}) {

}
func (b BaseRequest) Get(key string) (interface{}, bool) {
	return nil, false
}
