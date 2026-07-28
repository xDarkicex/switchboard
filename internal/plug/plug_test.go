package plug

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestTieLine(t *testing.T, h http.HandlerFunc) *TieLine {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &TieLine{
		Name:    "test",
		BaseURL: srv.URL,
		Timeout: 5 * time.Second,
	}
}

func TestTalkPathForwardsRequest(t *testing.T) {
	var seenMethod, seenPath, seenAuth string
	var seenBody []byte
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		seenAuth = r.Header.Get("Authorization")
		seenBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	tie := newTestTieLine(t, upstream)
	tie.APIKey = "secret-key"

	body := strings.NewReader(`{"hello":"world"}`)
	req := httptest.NewRequest("POST", "/v1/chat", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Caller", "alice")

	rr := httptest.NewRecorder()
	if err := NewCord(tie).TalkPath(rr, req); err != nil {
		t.Fatalf("TalkPath: %v", err)
	}
	if seenMethod != "POST" {
		t.Errorf("method = %q, want POST", seenMethod)
	}
	if seenPath != "/v1/chat" {
		t.Errorf("path = %q, want /v1/chat", seenPath)
	}
	if seenAuth != "Bearer secret-key" {
		t.Errorf("auth = %q, want Bearer secret-key", seenAuth)
	}
	if string(seenBody) != `{"hello":"world"}` {
		t.Errorf("body = %q", seenBody)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if rr.Body.String() != `{"ok":true}` {
		t.Errorf("response body = %q", rr.Body.String())
	}
}

func TestTalkPathForwardsQueryString(t *testing.T) {
	var seenQuery string
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	})
	tie := newTestTieLine(t, upstream)
	req := httptest.NewRequest("GET", "/v1/foo?limit=10&offset=20", nil)
	rr := httptest.NewRecorder()
	if err := NewCord(tie).TalkPath(rr, req); err != nil {
		t.Fatalf("TalkPath: %v", err)
	}
	if seenQuery != "limit=10&offset=20" {
		t.Errorf("query = %q, want limit=10&offset=20", seenQuery)
	}
}

func TestTalkPathStreamSSE(t *testing.T) {
	events := []string{"event: a\ndata: 1\n\n", "event: b\ndata: 2\n\n", "event: c\ndata: 3\n\n"}
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for _, e := range events {
			_, _ = w.Write([]byte(e))
			flusher.Flush()
		}
	})
	tie := newTestTieLine(t, upstream)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/stream", nil)
	if err := NewCord(tie).TalkPath(rr, req); err != nil {
		t.Fatalf("TalkPath: %v", err)
	}
	if !strings.HasPrefix(rr.Header().Get("Content-Type"), "text/event-stream") {
		t.Errorf("Content-Type = %q", rr.Header().Get("Content-Type"))
	}
	for _, e := range events {
		if !strings.Contains(rr.Body.String(), e) {
			t.Errorf("response missing event %q", e)
		}
	}
}

func TestTalkPathPropagatesStatus(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("i am a teapot"))
	})
	tie := newTestTieLine(t, upstream)
	req := httptest.NewRequest("GET", "/coffee", nil)
	rr := httptest.NewRecorder()
	if err := NewCord(tie).TalkPath(rr, req); err != nil {
		t.Fatalf("TalkPath: %v", err)
	}
	if rr.Code != http.StatusTeapot {
		t.Errorf("status = %d, want 418", rr.Code)
	}
	if rr.Body.String() != "i am a teapot" {
		t.Errorf("body = %q", rr.Body.String())
	}
}

func TestCordTestReportsHealth(t *testing.T) {
	healthy := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	tie := newTestTieLine(t, healthy)
	if !NewCord(tie).CordTest() {
		t.Error("CordTest should report healthy for 200")
	}
	broken := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	tie2 := newTestTieLine(t, broken)
	if NewCord(tie2).CordTest() {
		t.Error("CordTest should report down for 500")
	}
}

func TestTalkPathConcurrent(t *testing.T) {
	var count atomic.Int64
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		fmt.Fprintf(w, "ok %d", count.Load())
	})
	tie := newTestTieLine(t, upstream)
	cord := NewCord(tie)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			req := httptest.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()
			if err := cord.TalkPath(rr, req); err != nil {
				t.Errorf("TalkPath: %v", err)
			}
		})
	}
	wg.Wait()
	if count.Load() != 10 {
		t.Errorf("upstream hit count = %d, want 10", count.Load())
	}
}

func TestBuildRequestHasNoBaseURL(t *testing.T) {
	tie := &TieLine{Name: "broken"}
	cord := NewCord(tie)
	if cord.buildRequest(httptest.NewRequest("GET", "/foo", nil)) != nil {
		t.Error("buildRequest should return nil for empty base URL")
	}
}

// helper: empty import usage for bufio
var _ = bufio.NewScanner
