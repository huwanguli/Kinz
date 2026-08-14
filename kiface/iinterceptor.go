package kiface

// IcReq is the interceptor-chain input (any value).
type IcReq interface{}

// IcResp is the interceptor-chain output (any value).
type IcResp interface{}

// IInterceptor is one step of the request pipeline (responsibility chain).
type IInterceptor interface {
	// Intercept processes the chain request; call chain.Proceed to continue.
	Intercept(chain IChain) IcResp
}

// IChain is the responsibility chain over interceptors.
type IChain interface {
	// Request returns the current request payload.
	Request() IcReq
	// GetIMessage returns the IMessage inside the current request, if any.
	GetIMessage() IMessage
	// Proceed advances to the next interceptor with req.
	Proceed(req IcReq) IcResp
	// ProceedWithIMessage replaces the message and advances.
	ProceedWithIMessage(iMessage IMessage, response IcReq) IcResp
}
