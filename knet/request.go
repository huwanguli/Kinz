package knet

import "kinz/kiface"

// Request implements kiface.IRequest, binding a connection and a message with
// per-request routing state (handler chain index, abort flag, context).
type Request struct {
	conn kiface.IConnection
	msg  kiface.IMessage

	handlers []kiface.RouterHandler
	index    int
	aborted  bool

	ctx map[string]any
}

// NewRequest creates a Request for conn/msg.
func NewRequest(conn kiface.IConnection, msg kiface.IMessage) *Request {
	return &Request{conn: conn, msg: msg}
}

// GetConnection implements kiface.IRequest.
func (r *Request) GetConnection() kiface.IConnection { return r.conn }

// GetData implements kiface.IRequest.
func (r *Request) GetData() []byte { return r.msg.GetData() }

// GetMsgID implements kiface.IRequest.
func (r *Request) GetMsgID() uint32 { return r.msg.GetMsgID() }

// GetMessage implements kiface.IRequest.
func (r *Request) GetMessage() kiface.IMessage { return r.msg }

// SetMessage implements kiface.IRequest.
func (r *Request) SetMessage(m kiface.IMessage) { r.msg = m }

// BindRouterSlices implements kiface.IRequest.
func (r *Request) BindRouterSlices(handlers []kiface.RouterHandler) {
	r.handlers = handlers
	r.index = 0
	r.aborted = false
}

// RouterSlicesNext implements kiface.IRequest: advances to the next handler.
func (r *Request) RouterSlicesNext() {
	if r.aborted {
		return
	}
	if r.index >= len(r.handlers) {
		return
	}
	h := r.handlers[r.index]
	r.index++
	h(r)
}

// Abort implements kiface.IRequest: stops the remaining function-style
// handlers; the currently running handler finishes.
func (r *Request) Abort() { r.aborted = true }

// Copy implements kiface.IRequest: shallow copy for worker-pool reuse.
func (r *Request) Copy() kiface.IRequest {
	return &Request{
		conn:     r.conn,
		msg:      r.msg,
		handlers: r.handlers,
		index:    r.index,
	}
}

// Set implements kiface.IRequest.
func (r *Request) Set(key string, value interface{}) {
	if r.ctx == nil {
		r.ctx = make(map[string]any)
	}
	r.ctx[key] = value
}

// Get implements kiface.IRequest.
func (r *Request) Get(key string) (interface{}, bool) {
	if r.ctx == nil {
		return nil, false
	}
	v, ok := r.ctx[key]
	return v, ok
}
