package kconf

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzLoadYAML feeds arbitrary YAML-ish bytes through the config loader. Load
// must never panic: malformed YAML, wrong types and garbage input all have a
// defined (possibly error) outcome.
//
// Run with: go test ./kconf/ -fuzz=FuzzLoadYAML -fuzztime=30s
func FuzzLoadYAML(f *testing.F) {
	f.Add([]byte("Host: 127.0.0.1\nPort: 9001\nMaxConn: 64\n"))
	f.Add([]byte(":: not yaml at all ::\n"))
	f.Add([]byte("Port: not-a-number\nWriteTimeout: NaN\n"))
	f.Add([]byte("Port: 99999999999999999999999\n")) // int overflow path

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		path := filepath.Join(dir, "kinz.yaml")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		_, _ = Load(path) // error is fine; panic is not
	})
}
