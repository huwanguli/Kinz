package knet

import (
	"net"
	"sync"
	"testing"
	"time"

	"kinz/kconf"
	"kinz/kiface"
)

// These tests reproduce the runtime data race CI caught with -race:
// reconfiguring a running server/client (heartbeat template, hooks, codec,
// name) while connection goroutines read the same state through the connHost
// path (GetHeartBeat / codec.Clone / CallOnConnStart / CallOnConnStop).
//
// They pass without -race too (they assert no panics / working connections);
// the actual race detection happens in CI (`go test -race ./...`).

func TestServerRuntimeReconfigurationRace(t *testing.T) {
	cfg := testConfig(func(c *kconf.Config) {
		c.WorkerPoolSize = 1
		c.MaxConn = 64
	})
	srv, cancel := startTestServer(t, cfg)
	defer cancel()
	if _, err := srv.AddRouterSlices(1, func(req kiface.IRequest) {
		_ = req.GetConnection().SendMsg(2, req.GetData())
	}); err != nil {
		t.Fatalf("AddRouterSlices: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// reconfigurer: mutate runtime state while connections come and go
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			srv.StartHeartBeat(10 * time.Millisecond)
			srv.SetOnConnStart(func(kiface.IConnection) {})
			srv.SetOnConnStop(func(kiface.IConnection) {})
			srv.SetCodec(NewTLVPack())
			_ = srv.GetCodec()
			_ = srv.GetHeartBeat()
			_ = srv.GetOnConnStart()
		}
	}()

	// connection churn: dialing triggers GetHeartBeat/codec.Clone/
	// CallOnConnStart on connection goroutines
	go func() {
		defer wg.Done()
		for i := 0; i < 40; i++ {
			conn, err := net.Dial("tcp", srv.Address().String())
			if err != nil {
				t.Error(err)
				return
			}
			time.Sleep(time.Millisecond)
			_ = conn.Close()
		}
	}()
	wg.Wait()
}

// TestConnectionStartStopConcurrent reproduces the CI race in
// TestClientReconnect: an external Stop() (wg.Wait) racing with Start()
// (wg.Add) on the same connection. wg.Add now happens in NewConnection, and
// Stop only waits when Start actually launched the goroutines, so the two can
// interleave in any order without a race, deadlock, or a leaked heartbeat
// goroutine. Verified by CI with -race.
func TestConnectionStartStopConcurrent(t *testing.T) {
	srv := NewServer().(*Server)
	srv.StartHeartBeat(10 * time.Millisecond) // exercise the heartbeat path too

	for i := 0; i < 50; i++ {
		clientEnd, serverEnd := net.Pipe()
		c := NewConnection(srv, serverEnd, uint64(i+1), NewTLVPack(),
			NewMsgHandler(0, 0), kconf.Default(), nil)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			c.Start()
		}()
		go func() {
			defer wg.Done()
			c.Stop()
		}()
		wg.Wait()
		_ = clientEnd.Close()
	}
}

func TestClientRuntimeReconfigurationRace(t *testing.T) {
	srv, cancel := startTestServer(t, testConfig(func(c *kconf.Config) {
		c.WorkerPoolSize = 1
		c.MaxConn = 16
	}))
	defer cancel()
	if _, err := srv.AddRouterSlices(1, func(req kiface.IRequest) {
		_ = req.GetConnection().SendMsg(2, req.GetData())
	}); err != nil {
		t.Fatalf("AddRouterSlices: %v", err)
	}

	cl := NewClient("127.0.0.1", srv.Address().(*net.TCPAddr).Port,
		WithReconnect(10*time.Millisecond, 50*time.Millisecond, 2))
	cc, ok := cl.(*Client) // GetHeartBeat is the internal connHost method
	if !ok {
		t.Fatal("NewClient did not return *Client")
	}
	if err := cl.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer cl.Stop()

	var wg sync.WaitGroup
	wg.Add(2)

	// reconfigurer
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			cl.SetName("reconfig")
			cl.SetOnConnStart(func(kiface.IConnection) {})
			cl.SetOnConnStop(func(kiface.IConnection) {})
			cl.SetCodec(NewTLVPack())
			cl.StartHeartBeat(10 * time.Millisecond)
			_ = cl.GetName()
			_ = cl.GetCodec()
			_ = cc.GetHeartBeat()
		}
	}()

	// connection churn: closing the server side forces reconnect, and dial()
	// reads codec/clone + Connection.Start reads GetHeartBeat/CallOnConnStart
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			srv.GetConnMgr().Range(func(id uint64, conn kiface.IConnection) bool {
				conn.Stop()
				return false
			})
			time.Sleep(5 * time.Millisecond)
		}
	}()
	wg.Wait()
}
