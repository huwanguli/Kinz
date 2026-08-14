package knet

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"kinz/kconf"
	"kinz/kiface"
)

// selfSignedTLSConfig returns a server tls.Config with a self-signed
// certificate for 127.0.0.1, and a client config that trusts it.
func selfSignedTLSConfig(t *testing.T) (server *tls.Config, client *tls.Config) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "kinz-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(certPEM) {
		t.Fatal("failed to append self-signed cert to pool")
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}},
		&tls.Config{RootCAs: pool, ServerName: "127.0.0.1"}
}

func TestServerTLS(t *testing.T) {
	serverTLS, clientTLS := selfSignedTLSConfig(t)
	cfg := testConfig(func(c *kconf.Config) {
		c.WorkerPoolSize = 1
		c.MaxConn = 16
	})
	srv := NewServer(WithConfig(cfg), WithTLS(serverTLS))
	if _, err := srv.AddRouterSlices(1, func(req kiface.IRequest) {
		_ = req.GetConnection().SendMsg(2, req.GetData())
	}); err != nil {
		t.Fatalf("AddRouterSlices: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Run(ctx) }()
	waitReady(t, srv)
	defer cancel()

	conn, err := tls.Dial("tcp", srv.Address().String(), clientTLS)
	if err != nil {
		t.Fatalf("tls dial: %v", err)
	}
	defer conn.Close()

	codec := NewTLVPack()
	wire, _ := codec.Pack(NewMessage(1, []byte("tls-hi")))
	if _, err := conn.Write(wire); err != nil {
		t.Fatalf("write: %v", err)
	}
	id, body := readMsg(t, conn, codec)
	if id != 2 || string(body) != "tls-hi" {
		t.Fatalf("echo = (%d, %q), want (2, tls-hi)", id, body)
	}
}

func TestServerTLSRejectsPlainClient(t *testing.T) {
	serverTLS, _ := selfSignedTLSConfig(t)
	cfg := testConfig(func(c *kconf.Config) {
		c.WorkerPoolSize = 1
		c.MaxConn = 16
	})
	srv := NewServer(WithConfig(cfg), WithTLS(serverTLS))
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Run(ctx) }()
	waitReady(t, srv)
	defer cancel()

	// A plain TCP client cannot complete the TLS handshake: its first read
	// must fail once the server aborts the handshake.
	conn, err := net.Dial("tcp", srv.Address().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("expected plain client read to fail against TLS server")
	}
}
