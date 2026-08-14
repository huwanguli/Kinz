package kiface

// IRequest binds a connection and a message, and carries per-request state
// through the router chain (middleware support: Call/Abort, context Set/Get).
type IRequest interface {
	// GetConnection returns the connection that produced this request.
	GetConnection() IConnection
	// GetData returns the message payload.
	GetData() []byte
	// GetMsgID returns the message id.
	GetMsgID() uint32
	// GetMessage returns the underlying message.
	GetMessage() IMessage
	// GetResponse returns the interceptor-chain response (nil when none).
	GetResponse() IcResp
	// SetResponse stores the interceptor-chain response.
	SetResponse(IcResp)

	// BindRouter binds the classic router that handles this request.
	BindRouter(IRouter)
	// Call invokes the bound classic router (PreHandle/Handle/PostHandle).
	Call()
	// Abort stops the remaining function-style handlers; the current one finishes.
	Abort()

	// BindRouterSlices binds the function-style handler chain.
	BindRouterSlices([]RouterHandler)
	// RouterSlicesNext advances to the next function-style handler.
	RouterSlicesNext()

	// Copy returns a shallow copy of the request (worker-pool reuse).
	Copy() IRequest
	// Set stores a value in the request context.
	Set(key string, value interface{})
	// Get reads a value from the request context.
	Get(key string) (interface{}, bool)
}

// BaseRequest is a no-op base for custom IRequest implementations.
type BaseRequest struct{}

func (b BaseRequest) GetConnection() IConnection        { return nil }
func (b BaseRequest) GetData() []byte                   { return nil }
func (b BaseRequest) GetMsgID() uint32                  { return 0 }
func (b BaseRequest) GetMessage() IMessage              { return nil }
func (b BaseRequest) GetResponse() IcResp               { return nil }
func (b BaseRequest) SetResponse(IcResp)                {}
func (b BaseRequest) BindRouter(IRouter)                {}
func (b BaseRequest) Call()                             {}
func (b BaseRequest) Abort()                            {}
func (b BaseRequest) BindRouterSlices([]RouterHandler)  {}
func (b BaseRequest) RouterSlicesNext()                 {}
func (b BaseRequest) Copy() IRequest                    { return nil }
func (b BaseRequest) Set(key string, value interface{}) {}
func (b BaseRequest) Get(key string) (interface{}, bool) {
	return nil, false
}
