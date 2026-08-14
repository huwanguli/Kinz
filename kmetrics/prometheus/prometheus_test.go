package prometheus

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kinz/kmetrics"
)

func TestNewHandlerServesMetrics(t *testing.T) {
	reg := kmetrics.NewRegistry()
	reg.Counter("kinz_test_total", "test").Add(5)
	reg.Histogram("kinz_test_duration", "d", nil).Observe(0.25)

	h, err := NewHandler(reg)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	out := string(body)
	for _, want := range []string{"kinz_test_total 5", "kinz_test_duration_bucket"} {
		if !strings.Contains(out, want) {
			t.Fatalf("body missing %q:\n%s", want, out)
		}
	}
}

func TestNewHandlerNilRegistry(t *testing.T) {
	if _, err := NewHandler(nil); err == nil {
		t.Fatal("expected error for nil registry")
	}
}
