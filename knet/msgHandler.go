package knet

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"

	"kinz/kiface"
	"kinz/klog"
)

// groupRange is a middleware scope over a msgID range.
type groupRange struct {
	start, end uint32
	handlers   []kiface.RouterHandler
}

// MsgHandler implements kiface.IMsgHandle: function-style router slices with
// global/group middleware, a worker pool for ordered per-connection dispatch,
// and panic recovery.
type MsgHandler struct {
	mu             sync.RWMutex
	apis           map[uint32][]kiface.RouterHandler
	groupRanges    []groupRange
	globalHandlers []kiface.RouterHandler

	workerPoolSize   uint32
	maxWorkerTaskLen uint32
	taskQueue        []chan kiface.IRequest
	workerCtx        context.Context
	workerCancel     context.CancelFunc
	workerWG         sync.WaitGroup
}

// NewMsgHandler creates a MsgHandler. workerPoolSize == 0 disables the pool
// (one goroutine per message).
func NewMsgHandler(workerPoolSize, maxWorkerTaskLen uint32) *MsgHandler {
	return &MsgHandler{
		apis:             make(map[uint32][]kiface.RouterHandler),
		workerPoolSize:   workerPoolSize,
		maxWorkerTaskLen: maxWorkerTaskLen,
	}
}

// addSlices registers function-style handlers for msgID.
func (mh *MsgHandler) addSlices(msgID uint32, handlers ...kiface.RouterHandler) error {
	mh.mu.Lock()
	defer mh.mu.Unlock()
	if _, ok := mh.apis[msgID]; ok {
		return fmt.Errorf("%w: msgID %d", kiface.ErrMsgIDRegistered, msgID)
	}
	mh.apis[msgID] = append(mh.apis[msgID], handlers...)
	return nil
}

// AddRouterSlices implements kiface.IMsgHandle.
func (mh *MsgHandler) AddRouterSlices(msgID uint32, handlers ...kiface.RouterHandler) (kiface.IRouterSlices, error) {
	if err := mh.addSlices(msgID, handlers...); err != nil {
		return nil, err
	}
	return &routerSlices{mh: mh}, nil
}

// Group implements kiface.IMsgHandle.
func (mh *MsgHandler) Group(start, end uint32, handlers ...kiface.RouterHandler) (kiface.IGroupRouterSlices, error) {
	if start > end {
		return nil, fmt.Errorf("kinz: invalid group range [%d, %d]", start, end)
	}
	mh.mu.Lock()
	mh.groupRanges = append(mh.groupRanges, groupRange{start: start, end: end, handlers: handlers})
	mh.mu.Unlock()
	return &groupRouterSlices{mh: mh, start: start, end: end}, nil
}

// Use implements kiface.IMsgHandle: registers global middleware.
func (mh *MsgHandler) Use(handlers ...kiface.RouterHandler) (kiface.IRouterSlices, error) {
	if len(handlers) == 0 {
		return &routerSlices{mh: mh}, nil
	}
	mh.mu.Lock()
	mh.globalHandlers = append(mh.globalHandlers, handlers...)
	mh.mu.Unlock()
	return &routerSlices{mh: mh}, nil
}

// StartWorkerPool implements kiface.IMsgHandle (idempotent).
func (mh *MsgHandler) StartWorkerPool() {
	mh.mu.Lock()
	defer mh.mu.Unlock()
	if mh.workerCtx != nil || mh.workerPoolSize == 0 {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	mh.workerCtx = ctx
	mh.workerCancel = cancel
	mh.taskQueue = make([]chan kiface.IRequest, mh.workerPoolSize)
	for i := uint32(0); i < mh.workerPoolSize; i++ {
		mh.taskQueue[i] = make(chan kiface.IRequest, mh.maxWorkerTaskLen)
		mh.workerWG.Add(1)
		go mh.worker(i, mh.taskQueue[i])
	}
}

// StopWorkerPool implements kiface.IMsgHandle: cancels the workers, drains
// queued requests, and waits, bounded by ctx.
func (mh *MsgHandler) StopWorkerPool(ctx context.Context) {
	mh.mu.Lock()
	cancel := mh.workerCancel
	mh.workerCancel = nil
	mh.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	done := make(chan struct{})
	go func() {
		mh.workerWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (mh *MsgHandler) worker(id uint32, queue chan kiface.IRequest) {
	defer mh.workerWG.Done()
	for {
		select {
		case req := <-queue:
			mh.Execute(req)
		case <-mh.workerCtx.Done():
			// drain remaining queued requests, then exit
			for {
				select {
				case req := <-queue:
					mh.Execute(req)
				default:
					return
				}
			}
		}
	}
}

// SendMsgToTaskQueue implements kiface.IMsgHandle: routes a request to a worker
// by ConnID (preserving per-connection order). Without a pool, executes in a
// fresh goroutine. Blocking send applies backpressure when a queue is full.
func (mh *MsgHandler) SendMsgToTaskQueue(request kiface.IRequest) {
	mh.mu.RLock()
	poolSize := mh.workerPoolSize
	taskQueue := mh.taskQueue
	active := mh.workerCancel != nil
	mh.mu.RUnlock()

	if !active || poolSize == 0 || len(taskQueue) == 0 {
		go mh.Execute(request)
		return
	}
	workerID := request.GetConnection().GetConnID() % uint64(poolSize)
	taskQueue[workerID] <- request
}

// Execute implements kiface.IMsgHandle: builds the handler chain for the
// request's msgID (global middleware + range middleware + route handlers) and
// runs it. A panic in any handler is recovered here.
func (mh *MsgHandler) Execute(request kiface.IRequest) {
	defer func() {
		if r := recover(); r != nil {
			klog.L().Error("panic recovered in handler",
				"msgID", request.GetMsgID(),
				"panic", r,
				"stack", string(debug.Stack()))
		}
	}()

	msgID := request.GetMsgID()
	mh.mu.RLock()
	handlers, hasSlices := mh.apis[msgID]
	global := append([]kiface.RouterHandler(nil), mh.globalHandlers...)
	var groups []kiface.RouterHandler
	for _, g := range mh.groupRanges {
		if msgID >= g.start && msgID <= g.end {
			groups = append(groups, g.handlers...)
		}
	}
	mh.mu.RUnlock()

	if !hasSlices && len(global) == 0 && len(groups) == 0 {
		return // nothing registered for this msgID
	}

	all := append(global, groups...)
	all = append(all, handlers...)
	request.BindRouterSlices(all)
	request.RouterSlicesNext()
}
