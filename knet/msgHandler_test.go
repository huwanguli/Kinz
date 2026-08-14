package knet

import (
	"errors"
	"testing"

	"kinz/kiface"
)

func msgIDRequest(id uint32) *Request {
	return NewRequest(nil, NewMessage(id, nil))
}

func TestAddSlicesDuplicate(t *testing.T) {
	mh := NewMsgHandler(0, 0)
	if err := mh.addSlices(1, func(kiface.IRequest) {}); err != nil {
		t.Fatalf("addSlices: %v", err)
	}
	if err := mh.addSlices(1, func(kiface.IRequest) {}); !errors.Is(err, kiface.ErrMsgIDRegistered) {
		t.Fatalf("duplicate = %v, want ErrMsgIDRegistered", err)
	}
}

func TestRouterSlicesOrder(t *testing.T) {
	mh := NewMsgHandler(0, 0)
	var order []string
	next := func(req kiface.IRequest, name string) {
		order = append(order, name)
		req.RouterSlicesNext()
	}
	mh.globalHandlers = append(mh.globalHandlers,
		func(req kiface.IRequest) { next(req, "g1") },
		func(req kiface.IRequest) { next(req, "g2") },
	)
	if _, err := mh.AddRouterSlices(7,
		func(req kiface.IRequest) { next(req, "h1") },
		func(req kiface.IRequest) { next(req, "h2") },
	); err != nil {
		t.Fatalf("AddRouterSlices: %v", err)
	}

	mh.Execute(msgIDRequest(7))
	want := []string{"g1", "g2", "h1", "h2"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestAbort(t *testing.T) {
	mh := NewMsgHandler(0, 0)
	var order []string
	if _, err := mh.AddRouterSlices(9,
		func(req kiface.IRequest) { order = append(order, "a"); req.Abort() },
		func(kiface.IRequest) { order = append(order, "b") },
	); err != nil {
		t.Fatalf("AddRouterSlices: %v", err)
	}
	mh.Execute(msgIDRequest(9))
	if len(order) != 1 || order[0] != "a" {
		t.Fatalf("order = %v, want [a] (b must be aborted)", order)
	}
}

func TestGroupRange(t *testing.T) {
	mh := NewMsgHandler(0, 0)
	var ran []uint32
	if _, err := mh.Group(10, 20, func(req kiface.IRequest) {
		ran = append(ran, req.GetMsgID())
	}); err != nil {
		t.Fatalf("Group: %v", err)
	}
	mh.Execute(msgIDRequest(15))
	mh.Execute(msgIDRequest(25))
	if len(ran) != 1 || ran[0] != 15 {
		t.Fatalf("ran = %v, want [15] (25 outside group)", ran)
	}
}

func TestGroupOutOfRange(t *testing.T) {
	mh := NewMsgHandler(0, 0)
	g, err := mh.Group(10, 20)
	if err != nil {
		t.Fatalf("Group: %v", err)
	}
	if err := g.AddHandler(30, func(kiface.IRequest) {}); err == nil {
		t.Fatal("expected error for out-of-range AddHandler")
	}
	if err := g.AddHandler(15, func(kiface.IRequest) {}); err != nil {
		t.Fatalf("in-range AddHandler: %v", err)
	}
}

func TestMiddlewareBeforeAfter(t *testing.T) {
	mh := NewMsgHandler(0, 0)
	var order []string
	mh.globalHandlers = append(mh.globalHandlers, func(req kiface.IRequest) {
		order = append(order, "mw-before")
		req.RouterSlicesNext()            // descend into the chain
		order = append(order, "mw-after") // runs after the handler returns
	})
	if _, err := mh.AddRouterSlices(7, func(kiface.IRequest) {
		order = append(order, "handler")
	}); err != nil {
		t.Fatalf("AddRouterSlices: %v", err)
	}

	mh.Execute(msgIDRequest(7))
	want := []string{"mw-before", "handler", "mw-after"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestMiddlewareAfterRunsOnAbort(t *testing.T) {
	mh := NewMsgHandler(0, 0)
	var order []string
	mh.globalHandlers = append(mh.globalHandlers, func(req kiface.IRequest) {
		order = append(order, "mw-before")
		req.RouterSlicesNext()
		order = append(order, "mw-after") // must still run after an abort
	})
	if _, err := mh.AddRouterSlices(8, func(req kiface.IRequest) {
		order = append(order, "handler")
		req.Abort()
	}); err != nil {
		t.Fatalf("AddRouterSlices: %v", err)
	}

	mh.Execute(msgIDRequest(8))
	want := []string{"mw-before", "handler", "mw-after"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestExecutePanicRecovery(t *testing.T) {
	mh := NewMsgHandler(0, 0)
	if _, err := mh.AddRouterSlices(5, func(kiface.IRequest) { panic("boom") }); err != nil {
		t.Fatalf("AddRouterSlices: %v", err)
	}
	// must not panic
	mh.Execute(msgIDRequest(5))
}

func TestExecuteNoHandler(t *testing.T) {
	mh := NewMsgHandler(0, 0)
	mh.Execute(msgIDRequest(99)) // must be a no-op
}

func TestSendMsgToTaskQueueNoPool(t *testing.T) {
	mh := NewMsgHandler(0, 0)
	done := make(chan struct{})
	if _, err := mh.AddRouterSlices(11, func(kiface.IRequest) { close(done) }); err != nil {
		t.Fatalf("AddRouterSlices: %v", err)
	}
	mh.SendMsgToTaskQueue(msgIDRequest(11))
	<-done // executes on a fresh goroutine
}
