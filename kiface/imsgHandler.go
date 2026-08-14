package kiface

import "context"

// IMsgHandle dispatches requests to routers and runs the worker pool.
type IMsgHandle interface {
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
	// Execute dispatches a request to its handler chain (global/group
	// middleware + route handlers), recovering panics.
	Execute(request IRequest)
}
