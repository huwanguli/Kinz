package kiface

// IRouter is the classic three-stage router. Embed BaseRouter in
// implementations and override only the methods you need.
type IRouter interface {
	// PreHandle runs before Handle.
	PreHandle(request IRequest)
	// Handle runs the main business logic.
	Handle(request IRequest)
	// PostHandle runs after Handle.
	PostHandle(request IRequest)
}

// RouterHandler is a function-style message handler.
type RouterHandler func(request IRequest)

// IRouterSlices is the function-style router with middleware support.
type IRouterSlices interface {
	// Use appends global middleware handlers and returns the router for chaining.
	Use(handlers ...RouterHandler) IRouterSlices
	// AddHandler registers handlers for msgID.
	// Returns ErrMsgIDRegistered when msgID is already registered.
	AddHandler(msgID uint32, handlers ...RouterHandler) error
	// Group scopes handlers to every msgID in [start, end].
	Group(start, end uint32, handlers ...RouterHandler) IGroupRouterSlices
	// GetHandlers returns the handlers registered for msgID.
	GetHandlers(msgID uint32) ([]RouterHandler, bool)
}

// IGroupRouterSlices is a router scoped to a msgID range.
type IGroupRouterSlices interface {
	// Use appends middleware scoped to the group.
	Use(handlers ...RouterHandler) IGroupRouterSlices
	// AddHandler registers handlers for a msgID inside the group range.
	AddHandler(msgID uint32, handlers ...RouterHandler) error
}
