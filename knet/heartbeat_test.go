package knet

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"kinz/kiface"
)

// mockConn is a minimal IConnection double for heartbeat tests.
type mockConn struct {
	alive   bool
	stopped atomic.Bool
	sent    atomic.Int32
	lastID  atomic.Uint32
}

func (m *mockConn) Start()                  {}
func (m *mockConn) Stop()                   { m.stopped.Store(true) }
func (m *mockConn) GetConn() net.Conn        { return nil }
func (m *mockConn) GetConnID() uint64       { return 0 }
func (m *mockConn) GetRemoteAddr() net.Addr { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)} }
func (m *mockConn) LocalAddr() net.Addr     { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)} }
func (m *mockConn) SendMsg(id uint32, data []byte) error {
	m.sent.Add(1)
	m.lastID.Store(id)
	return nil
}
func (m *mockConn) IsAlive(timeout time.Duration) bool       { return m.alive }
func (m *mockConn) SetHeartBeat(hb kiface.IHeartbeatChecker) {}
func (m *mockConn) SetProperty(string, interface{})          {}
func (m *mockConn) GetProperty(string) (interface{}, error) {
	return nil, nil
}
func (m *mockConn) RemoveProperty(string) {}

func TestHeartbeatNotAliveStopsConn(t *testing.T) {
	conn := &mockConn{alive: false}
	hb := NewHeartbeatChecker(time.Second)
	hb.BindConn(conn)

	hb.check()
	if !conn.stopped.Load() {
		t.Fatal("dead connection was not stopped")
	}
}

func TestHeartbeatAliveSendsDefault(t *testing.T) {
	conn := &mockConn{alive: true}
	hb := NewHeartbeatChecker(time.Second)
	hb.BindConn(conn)

	hb.check()
	if conn.sent.Load() != 1 {
		t.Fatalf("sent = %d, want 1", conn.sent.Load())
	}
	if conn.lastID.Load() != kiface.HeartBeatDefaultMsgID {
		t.Fatalf("msgID = %d, want default %d", conn.lastID.Load(), kiface.HeartBeatDefaultMsgID)
	}
}

func TestHeartbeatCustomBeatFunc(t *testing.T) {
	conn := &mockConn{alive: true}
	hb := NewHeartbeatChecker(time.Second)
	var customCalled atomic.Bool
	hb.SetHeartbeatFunc(func(c kiface.IConnection) error {
		customCalled.Store(true)
		return nil
	})
	hb.BindConn(conn)

	hb.check()
	if !customCalled.Load() {
		t.Fatal("custom beatFunc not called")
	}
	if conn.sent.Load() != 0 {
		t.Fatalf("default send used: %d, want 0", conn.sent.Load())
	}
}

func TestHeartbeatSettersIgnoreNil(t *testing.T) {
	hb := NewHeartbeatChecker(time.Second)
	hb.SetHeartBeatMsgFunc(nil) // must keep default, not nil it
	hb.SetHeartbeatFunc(nil)
	hb.SetOnRemoteNotAlive(nil)
	if hb.makeMsg == nil {
		t.Fatal("makeMsg became nil after SetHeartBeatMsgFunc(nil)")
	}
	if hb.onRemoteNotAlive == nil {
		t.Fatal("onRemoteNotAlive became nil after SetOnRemoteNotAlive(nil)")
	}
}

func TestHeartbeatCloneKeepsConfig(t *testing.T) {
	hb := NewHeartbeatChecker(5 * time.Second)
	hb.SetTimeout(10 * time.Second)
	clone := hb.Clone()
	if clone.MsgID() != kiface.HeartBeatDefaultMsgID {
		t.Fatalf("clone msgID = %d, want default", clone.MsgID())
	}
	// clone must be independent: binding the clone must not affect the template
	conn := &mockConn{alive: true}
	clone.BindConn(conn)
	if hb.conn != nil {
		t.Fatal("template conn was mutated by clone.BindConn")
	}
}

func TestHeartbeatStartStopIdempotent(t *testing.T) {
	hb := NewHeartbeatChecker(time.Millisecond)
	conn := &mockConn{alive: true}
	hb.BindConn(conn)

	hb.Start()
	hb.Start() // second start must be a no-op
	hb.Stop()
	hb.Stop() // second stop must be a no-op (no panic)
}
