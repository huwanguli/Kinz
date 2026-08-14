package kiface

// IcReq 拦截器输入数据
type IcReq interface{}

// IcResp 拦截器输出数据
type IcResp interface{}

type IInterceptor interface {
	Intercept(IChain) IcResp
	// 可自定义
}

// IChain 责任链
type IChain interface {
	Request() IcReq        // 获取当前责任链中的请求数据
	GetIMessage() IMessage // 获取 IMessage 数据
	Proceed(IcReq) IcResp  // 进入并执行下一个拦截器，且将数据传输给下一个拦截器
	ProceedWithIMessage(IMessage, IcReq) IcResp
}
