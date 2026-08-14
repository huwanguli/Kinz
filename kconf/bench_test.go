package kconf

import (
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkDefault measures building the defaults struct.
func BenchmarkDefault(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Default()
	}
}

// BenchmarkLoadYAML measures the full load chain: defaults → YAML file → env
// override, for a small typical config file.
func BenchmarkLoadYAML(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "kinz.yaml")
	content := "Host: 127.0.0.1\nPort: 9001\nMaxConn: 64\nWriteTimeout: 2500000000\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Load(path); err != nil {
			b.Fatal(err)
		}
	}
}
