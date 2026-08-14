package knet

import (
	"context"

	"kinz/kiface"
)

// MsgHandler implements kiface.IMsgHandle. Behavioral implementation lands in Phase 2.
type MsgHandler struct{}

// NewMsgHandler creates a MsgHandler.
func NewMsgHandler() *MsgHandler { return &MsgHandler{} }

// AddRouter implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) AddRouter(msgID uint32, router kiface.IRouter) error {
	return kiface.ErrNotImplemented
}

// AddRouterSlices implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) AddRouterSlices(msgID uint32, handlers ...kiface.RouterHandler) (kiface.IRouterSlices, error) {
	return nil, kiface.ErrNotImplemented
}

// Group implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) Group(start, end uint32, handlers ...kiface.RouterHandler) (kiface.IGroupRouterSlices, error) {
	return nil, kiface.ErrNotImplemented
}

// Use implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) Use(handlers ...kiface.RouterHandler) (kiface.IRouterSlices, error) {
	return nil, kiface.ErrNotImplemented
}

// StartWorkerPool implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) StartWorkerPool() {}

// StopWorkerPool implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) StopWorkerPool(ctx context.Context) {}

// SendMsgToTaskQueue implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) SendMsgToTaskQueue(request kiface.IRequest) {}

// Execute implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) Execute(request kiface.IRequest) {}

// AddInterceptor implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) AddInterceptor(interceptor kiface.IInterceptor) {}

// SetHeadInterceptor implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) SetHeadInterceptor(interceptor kiface.IInterceptor) {}
