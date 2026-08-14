package knet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kinz/kconf"
	"kinz/kiface"
)

// ---------------------------------------------------------------------------
// TLVPack micro-benchmarks (codec only, no sockets).
// ---------------------------------------------------------------------------

// BenchmarkTLVPackPack measures serialization cost (header + payload → wire).
// Throughput is reported as wire bytes (8-byte header + payload).
func BenchmarkTLVPackPack(b *testing.B) {
	for _, size := range []int{16, 128, 1024, 4096} {
		b.Run(fmt.Sprintf("payload=%d", size), func(b *testing.B) {
			codec := NewTLVPack()
			msg := NewMessage(1, bytes.Repeat([]byte("x"), size))
			b.SetBytes(int64(8 + size))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := codec.Pack(msg); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkTLVPackDecode measures parsing cost for exactly one complete frame.
func BenchmarkTLVPackDecode(b *testing.B) {
	for _, size := range []int{16, 128, 1024, 4096} {
		b.Run(fmt.Sprintf("payload=%d", size), func(b *testing.B) {
			codec := NewTLVPack()
			wire, _ := codec.Pack(NewMessage(1, bytes.Repeat([]byte("x"), size)))
			b.SetBytes(int64(8 + size))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				msgs, err := codec.Decode(wire)
				if err != nil || len(msgs) != 1 {
					b.Fatalf("Decode = %d msgs, %v; want 1, nil", len(msgs), err)
				}
			}
		})
	}
}

// BenchmarkTLVPackDecodeSticky measures parsing of a 16-frame stream in one
// Decode call (the TCP sticky-packet path).
func BenchmarkTLVPackDecodeSticky(b *testing.B) {
	codec := NewTLVPack()
	wire, _ := codec.Pack(NewMessage(1, bytes.Repeat([]byte("x"), 64)))
	stream := make([]byte, 0, 16*len(wire))
	for i := 0; i < 16; i++ {
		stream = append(stream, wire...)
	}
	b.SetBytes(int64(len(stream)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msgs, err := codec.Decode(stream)
		if err != nil || len(msgs) != 16 {
			b.Fatalf("Decode = %d msgs, %v; want 16, nil", len(msgs), err)
		}
	}
}

// ---------------------------------------------------------------------------
// Message dispatch benchmarks (no sockets).
// ---------------------------------------------------------------------------

// BenchmarkMsgHandlerExecute measures the router-slice dispatch path directly:
// middleware/group lookup + handler chain, without the worker pool or sockets.
func BenchmarkMsgHandlerExecute(b *testing.B) {
	mh := NewMsgHandler(0, 0)
	var handled atomic.Int64
	if _, err := mh.AddRouterSlices(1, func(req kiface.IRequest) {
		handled.Add(1)
	}); err != nil {
		b.Fatal(err)
	}
	conn := &mockConn{}
	req := NewRequest(conn, NewMessage(1, []byte("hello")))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mh.Execute(req)
	}
	if handled.Load() != int64(b.N) {
		b.Fatalf("handled = %d, want %d", handled.Load(), b.N)
	}
}

// BenchmarkMsgHandlerDispatch measures the full queue path: SendMsgToTaskQueue
// enqueue → worker dequeue → Execute. One connection (one worker queue) is
// used; the blocking queue applies backpressure, so this approximates
// end-to-end handler throughput through the worker pool.
func BenchmarkMsgHandlerDispatch(b *testing.B) {
	mh := NewMsgHandler(4, 1024)
	var handled atomic.Int64
	if _, err := mh.AddRouterSlices(1, func(req kiface.IRequest) {
		handled.Add(1)
	}); err != nil {
		b.Fatal(err)
	}
	mh.StartWorkerPool()
	req := NewRequest(&mockConn{}, NewMessage(1, []byte("hello")))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mh.SendMsgToTaskQueue(req)
	}
	b.StopTimer()
	// StopWorkerPool drains the queued tail, so the handler must have run
	// exactly b.N times when it returns.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	mh.StopWorkerPool(ctx)
	if handled.Load() != int64(b.N) {
		b.Fatalf("handled = %d, want %d", handled.Load(), b.N)
	}
}

// BenchmarkConnectionSendMsg measures the server→client write path: codec.Pack
// + msgChan queue + writer goroutine + socket write, drained by the peer.
func BenchmarkConnectionSendMsg(b *testing.B) {
	srv := NewServer()
	client, sconn := tcpPair(b)
	defer client.Close()
	c := newTestConnection(srv, sconn)
	defer c.Stop()
	c.Start()

	const payloadSize = 128
	payload := bytes.Repeat([]byte("x"), payloadSize)
	head := make([]byte, 8)
	body := make([]byte, payloadSize)
	b.SetBytes(int64(8 + payloadSize))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := c.SendMsg(1, payload); err != nil {
			b.Fatal(err)
		}
		if _, err := io.ReadFull(client, head); err != nil {
			b.Fatal(err)
		}
		if _, err := io.ReadFull(client, body); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// End-to-end throughput benchmarks (real TCP loopback).
// ---------------------------------------------------------------------------

// BenchmarkRawEchoBaseline measures the same loopback echo round trip on a
// bare net.Conn with no framework (a goroutine per connection doing
// conn.Read → conn.Write). It is the floor to compare Kinz's end-to-end
// overhead against.
func BenchmarkRawEchoBaseline(b *testing.B) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				buf := make([]byte, 4096)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					if _, err := c.Write(buf[:n]); err != nil {
						return
					}
				}
			}()
		}
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()
	const payloadSize = 128
	payload := bytes.Repeat([]byte("x"), payloadSize)
	echo := make([]byte, payloadSize)
	b.SetBytes(int64(payloadSize))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := conn.Write(payload); err != nil {
			b.Fatal(err)
		}
		if _, err := io.ReadFull(conn, echo); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEchoThroughput measures the full round trip: client write → server
// read/decode → worker dispatch → echo SendMsg → client read. Reported MB/s
// counts application payload bytes (wire bytes are payload + 8 header).
func BenchmarkEchoThroughput(b *testing.B) {
	for _, workers := range []uint32{1, 4} {
		b.Run(fmt.Sprintf("workers=%d", workers), func(b *testing.B) {
			for _, size := range []int{16, 128, 1024} {
				b.Run(fmt.Sprintf("payload=%d", size), func(b *testing.B) {
					benchEcho(b, size, workers)
				})
			}
		})
	}
}

func benchEcho(b *testing.B, payloadSize int, workers uint32) {
	cfg := testConfig(func(c *kconf.Config) {
		c.WorkerPoolSize = workers
		c.MaxConn = 64
	})
	srv, cancel := startTestServer(b, cfg)
	defer cancel()
	if _, err := srv.AddRouterSlices(1, func(req kiface.IRequest) {
		_ = req.GetConnection().SendMsg(2, req.GetData())
	}); err != nil {
		b.Fatalf("AddRouterSlices: %v", err)
	}

	conn := dialTLV(b, srv.Address())
	defer conn.Close()
	codec := NewTLVPack()
	payload := bytes.Repeat([]byte("x"), payloadSize)
	wire, err := codec.Pack(NewMessage(1, payload))
	if err != nil {
		b.Fatal(err)
	}
	head := make([]byte, 8)
	body := make([]byte, payloadSize)
	b.SetBytes(int64(payloadSize))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := conn.Write(wire); err != nil {
			b.Fatal(err)
		}
		if _, err := io.ReadFull(conn, head); err != nil {
			b.Fatal(err)
		}
		if _, err := io.ReadFull(conn, body); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMultiConnEcho measures aggregate throughput with concurrent
// connections (1/8/32), each running an independent echo round trip.
func BenchmarkMultiConnEcho(b *testing.B) {
	for _, conns := range []int{1, 8, 32} {
		b.Run(fmt.Sprintf("conns=%d", conns), func(b *testing.B) {
			benchMultiConn(b, conns)
		})
	}
}

func benchMultiConn(b *testing.B, conns int) {
	cfg := testConfig(func(c *kconf.Config) {
		c.WorkerPoolSize = 4
		c.MaxConn = 128
	})
	srv, cancel := startTestServer(b, cfg)
	defer cancel()
	if _, err := srv.AddRouterSlices(1, func(req kiface.IRequest) {
		_ = req.GetConnection().SendMsg(2, req.GetData())
	}); err != nil {
		b.Fatalf("AddRouterSlices: %v", err)
	}

	const payloadSize = 128
	payload := bytes.Repeat([]byte("x"), payloadSize)

	type clientConn struct {
		conn net.Conn
		wire []byte
	}
	clients := make([]clientConn, conns)
	codec := NewTLVPack()
	for i := range clients {
		c := dialTLV(b, srv.Address())
		wire, err := codec.Pack(NewMessage(1, payload))
		if err != nil {
			b.Fatal(err)
		}
		clients[i] = clientConn{conn: c, wire: wire}
	}
	defer func() {
		for _, c := range clients {
			_ = c.conn.Close()
		}
	}()

	b.SetBytes(int64(payloadSize))
	b.ReportAllocs()
	b.ResetTimer()

	var sent atomic.Int64 // total messages issued across all connections
	var errOnce sync.Once
	var opErr error
	var wg sync.WaitGroup
	for i := range clients {
		cli := clients[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			head := make([]byte, 8)
			body := make([]byte, payloadSize)
			for sent.Add(1) <= int64(b.N) {
				if _, err := cli.conn.Write(cli.wire); err != nil {
					errOnce.Do(func() { opErr = err })
					return
				}
				if _, err := io.ReadFull(cli.conn, head); err != nil {
					errOnce.Do(func() { opErr = err })
					return
				}
				if _, err := io.ReadFull(cli.conn, body); err != nil {
					errOnce.Do(func() { opErr = err })
					return
				}
			}
		}()
	}
	wg.Wait()
	if opErr != nil {
		b.Fatal(opErr)
	}
}
