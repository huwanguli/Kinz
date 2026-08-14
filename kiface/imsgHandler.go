package kiface

import "context"

// IMsgHandle dispatches requests to routers and runs the worker pool.
type IMsgHandle interface {
	// AddRouter registers a classic router for msgID.
	// Returns ErrMsgIDRegistered when msgID is already registered.
	AddRouter(msgID uint32, router IRouter) error
	// AddRouterSlices registers function-style handlers for msgID.
	AddRouterSlices(msgID uint32, handlers ...RouterHandler) (IRouterSlices, error)
	// Group scopes handlers to every msgID in [start, end].
	Group(start, end uint32, handlers ...RouterHandler) (IGroupRouterSlices, error)
	// Use registers global middleware applied to every message.
	Use(handlers ...RouterHandler) (IRouterSlices, error)

	// StartWorkerPool launches the worker goroutines (idempotent).
	StartWorkerPool()
	// StopWorkerPool drains and stops the workers, bounded by ctx.
	StopWorkerPool(ctx context.Context)
	// SendMsgToTaskQueue routes a request to a worker by ConnID.
	SendMsgToTaskQueue(request IRequest)
	// Execute runs the interceptor chain, then dispatches to the handler.
	Execute(request IRequest)

	// AddInterceptor appends an interceptor to the chain.
	AddInterceptor(interceptor IInterceptor)
	// SetHeadInterceptor prepends an interceptor to the chain.
	SetHeadInterceptor(interceptor IInterceptor)
}
