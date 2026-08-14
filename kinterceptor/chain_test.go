package kinterceptor

import (
	"testing"

	"kinz/kiface"
)

// testRequest is a minimal IRequest double for chain tests.
type testRequest struct {
	kiface.BaseRequest
	msg         kiface.IMessage
	gotResponse kiface.IcResp
}

func (r *testRequest) GetMessage() kiface.IMessage { return r.msg }
func (r *testRequest) SetMessage(msg kiface.IMessage) {
	r.msg = msg
}
func (r *testRequest) GetResponse() kiface.IcResp { return r.gotResponse }
func (r *testRequest) SetResponse(resp kiface.IcResp) {
	r.gotResponse = resp
}

// recordInterceptor appends a marker and continues the chain.
func recordInterceptor(marks *[]string, name string) kiface.IInterceptor {
	return kiface.InterceptorFunc(func(chain kiface.IChain) kiface.IcResp {
		*marks = append(*marks, name)
		return chain.Proceed(chain.Request())
	})
}

// stopInterceptor appends a marker and does NOT continue the chain.
func stopInterceptor(marks *[]string, name string) kiface.IInterceptor {
	return kiface.InterceptorFunc(func(chain kiface.IChain) kiface.IcResp {
		*marks = append(*marks, name)
		return chain.Request()
	})
}

func TestChainProceedOrder(t *testing.T) {
	var marks []string
	interceptors := []kiface.IInterceptor{
		recordInterceptor(&marks, "a"),
		recordInterceptor(&marks, "b"),
	}
	req := &testRequest{}
	chain := NewChain(interceptors, 0, req)

	resp := chain.Proceed(req)
	if len(marks) != 2 || marks[0] != "a" || marks[1] != "b" {
		t.Fatalf("marks = %v, want [a b]", marks)
	}
	if resp != kiface.IcReq(req) {
		t.Fatalf("resp = %v, want the request", resp)
	}
}

func TestChainStopEarly(t *testing.T) {
	var marks []string
	interceptors := []kiface.IInterceptor{
		recordInterceptor(&marks, "a"),
		stopInterceptor(&marks, "b"),
		recordInterceptor(&marks, "c"),
	}
	req := &testRequest{}
	chain := NewChain(interceptors, 0, req)

	chain.Proceed(req)
	if len(marks) != 2 || marks[0] != "a" || marks[1] != "b" {
		t.Fatalf("marks = %v, want [a b] (c must not run)", marks)
	}
}

func TestChainGetIMessage(t *testing.T) {
	msg := &messageStub{id: 7}
	req := &testRequest{msg: msg}
	chain := NewChain(nil, 0, req)

	if got := chain.GetIMessage(); got != kiface.IMessage(msg) {
		t.Fatalf("GetIMessage = %v, want the stub message", got)
	}
}

func TestChainProceedWithIMessage(t *testing.T) {
	msg := &messageStub{id: 1}
	newMsg := &messageStub{id: 2}
	req := &testRequest{msg: msg}

	var sawSetResponse bool
	interceptor := kiface.InterceptorFunc(func(chain kiface.IChain) kiface.IcResp {
		return chain.ProceedWithIMessage(newMsg, "resp")
	})
	interceptor2 := kiface.InterceptorFunc(func(chain kiface.IChain) kiface.IcResp {
		if r, ok := chain.Request().(kiface.IRequest); ok {
			if r.GetMessage() != kiface.IMessage(newMsg) {
				t.Fatalf("request message = %v, want newMsg", r.GetMessage())
			}
			if got := r.GetResponse(); got != "resp" {
				t.Fatalf("response = %v, want resp", got)
			}
		}
		sawSetResponse = true
		return chain.Request()
	})

	chain := NewChain([]kiface.IInterceptor{interceptor, interceptor2}, 0, req)
	chain.Proceed(req)
	if !sawSetResponse {
		t.Fatal("ProceedWithIMessage did not propagate the message/response")
	}
}

type messageStub struct {
	id uint32
}

func (m *messageStub) GetMsgID() uint32    { return m.id }
func (m *messageStub) GetDataLen() uint32  { return 0 }
func (m *messageStub) GetData() []byte     { return nil }
func (m *messageStub) GetRawData() []byte  { return nil }
func (m *messageStub) SetMsgID(uint32)     {}
func (m *messageStub) SetData([]byte)      {}
func (m *messageStub) SetDataLen(uint32)   {}
