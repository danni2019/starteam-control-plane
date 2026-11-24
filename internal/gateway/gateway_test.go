package gateway

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFallbackAndRecovery(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("loopback listener unavailable: %v", err)
	}
	l.Close()

	ready := make(chan bool, 3)
	ready <- true
	ready <- false
	ready <- true
	start := func(label string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" && label == "primary" && !<-ready {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(label))
		}))
	}
	primary, standby := start("primary"), start("standby")
	defer primary.Close()
	defer standby.Close()
	gateway, err := New(primary.URL, standby.URL)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"primary", "standby", "primary"} {
		w := httptest.NewRecorder()
		gateway.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/demo", nil))
		if w.Code != http.StatusOK || w.Body.String() != expected || w.Header().Get("X-Starteam-Backend") != expected {
			t.Fatalf("want backend %s, got %d %q", expected, w.Code, w.Body.String())
		}
	}
}

func TestRejectsWriteWithoutProbing(t *testing.T) {
	gateway, _ := New("http://127.0.0.1:9001", "http://127.0.0.1:9002")
	w := httptest.NewRecorder()
	gateway.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/demo", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", w.Code)
	}
}
