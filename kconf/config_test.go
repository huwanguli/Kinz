package kconf

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Port != 8999 {
		t.Fatalf("Port = %d, want 8999", cfg.Port)
	}
	if cfg.MaxConn != 1024 {
		t.Fatalf("MaxConn = %d, want 1024", cfg.MaxConn)
	}
	if time.Duration(cfg.HeartbeatInterval) != 10*time.Second {
		t.Fatalf("HeartbeatInterval = %v, want 10s", cfg.HeartbeatInterval)
	}
}

func TestLoadMissingFileIsNotError(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 8999 {
		t.Fatalf("Port = %d, want default 8999", cfg.Port)
	}
}

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kinz.yaml")
	content := "Host: 127.0.0.1\nPort: 9001\nMaxConn: 64\nHeartbeatInterval: 5s\nWriteTimeout: 2500000000\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Host != "127.0.0.1" || cfg.Port != 9001 || cfg.MaxConn != 64 {
		t.Fatalf("yaml not applied: %+v", cfg)
	}
	if time.Duration(cfg.HeartbeatInterval) != 5*time.Second {
		t.Fatalf("HeartbeatInterval = %v, want 5s", cfg.HeartbeatInterval)
	}
	// numeric (nanosecond) duration form
	if time.Duration(cfg.WriteTimeout) != 2500*time.Millisecond {
		t.Fatalf("WriteTimeout = %v, want 2.5s", cfg.WriteTimeout)
	}
	// untouched fields keep defaults
	if cfg.MaxPacketSize != 4096 {
		t.Fatalf("MaxPacketSize = %d, want default 4096", cfg.MaxPacketSize)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kinz.yaml")
	if err := os.WriteFile(path, []byte("Host: [unclosed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadEnvOverride(t *testing.T) {
	t.Setenv("KINZ_HOST", "10.0.0.1")
	t.Setenv("KINZ_PORT", "7777")
	t.Setenv("KINZ_MAXCONN", "32")
	t.Setenv("KINZ_MAXPACKETSIZE", "8192")
	t.Setenv("KINZ_WORKERPOOLSIZE", "4")
	t.Setenv("KINZ_HEARTBEATINTERVAL", "2s")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Host != "10.0.0.1" || cfg.Port != 7777 || cfg.MaxConn != 32 {
		t.Fatalf("env not applied: %+v", cfg)
	}
	if cfg.MaxPacketSize != 8192 || cfg.WorkerPoolSize != 4 {
		t.Fatalf("env not applied: %+v", cfg)
	}
	if time.Duration(cfg.HeartbeatInterval) != 2*time.Second {
		t.Fatalf("HeartbeatInterval = %v, want 2s", cfg.HeartbeatInterval)
	}
}

func TestLoadInvalidEnv(t *testing.T) {
	for _, kv := range [][2]string{
		{"KINZ_PORT", "not-a-number"},
		{"KINZ_MAXCONN", "x"},
		{"KINZ_MAXPACKETSIZE", "x"},
		{"KINZ_WORKERPOOLSIZE", "x"},
		{"KINZ_HEARTBEATINTERVAL", "x"},
	} {
		t.Setenv(kv[0], kv[1])
		if _, err := Load(""); err == nil {
			t.Fatalf("expected error for invalid %s=%q", kv[0], kv[1])
		}
	}
}
