package knet

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"

	"kinz/kiface"
	"kinz/kinterceptor"
	"kinz/klog"
)

// groupRange is a middleware scope over a msgID range.
type groupRange struct {
	start, end uint32
	handlers   []kiface.RouterHandler
}

// MsgHandler implements kiface.IMsgHandle: classic routers and function-style
// router slices (with global/group middleware), a worker pool for ordered
// per-connection dispatch, and an interceptor chain.
type MsgHandler struct {
	mu             sync.RWMutex
	classicApis    map[uint32]kiface.IRouter
	apis           map[uint32][]kiface.RouterHandler
	groupRanges    []groupRange
	globalHandlers []kiface.RouterHandler

	workerPoolSize   uint32
	maxWorkerTaskLen uint32
	taskQueue        []chan kiface.IRequest
	workerCtx        context.Context
	workerCancel     context.CancelFunc
	workerWG         sync.WaitGroup

	interceptors    []kiface.IInterceptor
	headInterceptor kiface.IInterceptor
}

// NewMsgHandler creates a MsgHandler. workerPoolSize == 0 disables the pool
// (one goroutine per message).
func NewMsgHandler(workerPoolSize, maxWorkerTaskLen uint32) *MsgHandler {
	return &MsgHandler{
		classicApis:      make(map[uint32]kiface.IRouter),
		apis:             make(map[uint32][]kiface.RouterHandler),
		workerPoolSize:   workerPoolSize,
		maxWorkerTaskLen: maxWorkerTaskLen,
	}
}

// AddRouter implements kiface.IMsgHandle.
func (mh *MsgHandler) AddRouter(msgID uint32, router kiface.IRouter) error {
	mh.mu.Lock()
	defer mh.mu.Unlock()
	if _, ok := mh.classicApis[msgID]; ok {
		return fmt.Errorf("%w: msgID %d", kiface.ErrMsgIDRegistered, msgID)
	}
	if _, ok := mh.apis[msgID]; ok {
		return fmt.Errorf("%w: msgID %d", kiface.ErrMsgIDRegistered, msgID)
	}
	mh.classicApis[msgID] = router
	return nil
}

// addSlices registers function-style handlers for msgID.
func (mh *MsgHandler) addSlices(msgID uint32, handlers ...kiface.RouterHandler) error {
	mh.mu.Lock()
	defer mh.mu.Unlock()
	if _, ok := mh.classicApis[msgID]; ok {
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

// Execute implements kiface.IMsgHandle: runs the interceptor chain, then
// dispatches to the handler chain. A panic in any handler is recovered here.
func (mh *MsgHandler) Execute(request kiface.IRequest) {
	req := request
	if mh.headInterceptor != nil {
		chain := kinterceptor.NewChain(mh.interceptors, 0, req)
		if resp := mh.headInterceptor.Intercept(chain); resp != nil {
			if r, ok := resp.(kiface.IRequest); ok {
				req = r
			}
		}
	} else if len(mh.interceptors) > 0 {
		chain := kinterceptor.NewChain(mh.interceptors, 0, req)
		if resp := chain.Proceed(req); resp != nil {
			if r, ok := resp.(kiface.IRequest); ok {
				req = r
			}
		}
	}
	mh.dispatch(req)
}

// dispatch builds the handler chain for the request's msgID and runs it.
func (mh *MsgHandler) dispatch(request kiface.IRequest) {
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
	router, hasClassic := mh.classicApis[msgID]
	global := append([]kiface.RouterHandler(nil), mh.globalHandlers...)
	var groups []kiface.RouterHandler
	for _, g := range mh.groupRanges {
		if msgID >= g.start && msgID <= g.end {
			groups = append(groups, g.handlers...)
		}
	}
	mh.mu.RUnlock()

	if !hasSlices && !hasClassic && len(global) == 0 && len(groups) == 0 {
		return // nothing registered for this msgID
	}

	all := append(global, groups...)
	all = append(all, handlers...)
	if hasClassic && !hasSlices {
		request.BindRouter(router)
		all = append(all, func(req kiface.IRequest) { req.Call() })
	}
	request.BindRouterSlices(all)
	request.RouterSlicesNext()
}

// AddInterceptor implements kiface.IMsgHandle.
func (mh *MsgHandler) AddInterceptor(interceptor kiface.IInterceptor) {
	if interceptor == nil {
		return
	}
	mh.mu.Lock()
	mh.interceptors = append(mh.interceptors, interceptor)
	mh.mu.Unlock()
}

// SetHeadInterceptor implements kiface.IMsgHandle.
func (mh *MsgHandler) SetHeadInterceptor(interceptor kiface.IInterceptor) {
	if interceptor == nil {
		return
	}
	mh.mu.Lock()
	mh.headInterceptor = interceptor
	mh.mu.Unlock()
}
