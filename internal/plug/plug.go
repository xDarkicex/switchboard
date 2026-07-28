// Package plug is the patch cord — the upstream transport. It establishes
// the socket to the backend and forwards the call, including SSE streams
// via nanite/sse for zero-alloc buffer management and proper flush semantics.
package plug

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	nanitesse "github.com/xDarkicex/nanite/sse"
)

// TieLine is a long-distance upstream connection. One per backend.
type TieLine struct {
	Name       string
	BaseURL    string
	APIKey     string
	Labels     []string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// Cord is a live patch cord between the operator and an upstream.
type Cord struct {
	tieLine *TieLine
}

// NewCord prepares a new cord for a tie line. Reuses the tie line's HTTP
// client if already set; otherwise builds a sensible default.
func NewCord(tieLine *TieLine) *Cord {
	if tieLine.HTTPClient == nil {
		timeout := tieLine.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		tieLine.HTTPClient = &http.Client{Timeout: timeout}
	}
	if tieLine.Timeout == 0 {
		tieLine.Timeout = 30 * time.Second
	}
	return &Cord{tieLine: tieLine}
}

// In is the one-shot convenience. Builds a Cord and forwards the call.
func In(tieLine *TieLine, w http.ResponseWriter, r *http.Request) error {
	return NewCord(tieLine).TalkPath(w, r)
}

// CordTest returns true if the tie line is alive. Issues a short GET to the
// base URL.
func (c *Cord) CordTest() bool {
	if c.tieLine.BaseURL == "" {
		return false
	}
	req, err := http.NewRequest(http.MethodGet, c.tieLine.BaseURL, nil)
	if err != nil {
		return false
	}
	resp, err := c.tieLine.HTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < http.StatusInternalServerError
}

// TalkPath forwards the caller's request to the upstream and copies the
// response back. Streams SSE responses chunk-by-chunk with Flush.
func (c *Cord) TalkPath(w http.ResponseWriter, r *http.Request) error {
	upstream, err := c.tieLine.HTTPClient.Do(c.buildRequest(r))
	if err != nil {
		return fmt.Errorf("plug: upstream %q failed: %w", c.tieLine.Name, err)
	}
	defer upstream.Body.Close()

	copyHeader(w.Header(), upstream.Header)
	w.WriteHeader(upstream.StatusCode)

	if isSSE(upstream.Header) {
		return streamSSE(w, upstream.Body)
	}
	_, err = io.Copy(w, upstream.Body)
	return err
}

// Cutoff is a no-op for v1. Future versions may drain idle connections.
func (c *Cord) Cutoff() error {
	return nil
}

// ErrNoTieLine is returned when a build request can't reach the upstream.
var ErrNoTieLine = errors.New("plug: tie line has no base URL")

// buildRequest constructs the upstream HTTP request from the caller's,
// forwarding headers, method, body, and adding auth.
func (c *Cord) buildRequest(r *http.Request) *http.Request {
	if c.tieLine.BaseURL == "" {
		return nil
	}
	target := strings.TrimRight(c.tieLine.BaseURL, "/") + r.URL.Path
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	var body io.Reader
	if r.Body != nil && r.Body != http.NoBody {
		body = r.Body
	}
	upstream, err := http.NewRequestWithContext(r.Context(), r.Method, target, body)
	if err != nil {
		return nil
	}
	copyHeader(upstream.Header, r.Header)
	upstream.Header.Set("Host", upstream.URL.Host)
	if c.tieLine.APIKey != "" {
		upstream.Header.Set("Authorization", "Bearer "+c.tieLine.APIKey)
	}
	return upstream
}

// isSSE reports whether the upstream response is an event stream.
func isSSE(h http.Header) bool {
	return strings.HasPrefix(h.Get("Content-Type"), "text/event-stream")
}

// streamSSE pipes the upstream body to the client using nanite/sse flush
// semantics. Uses http.ResponseController for proper write deadline management.
func streamSSE(w http.ResponseWriter, body io.Reader) error {
	rc := http.NewResponseController(w)
	for {
		buf := make([]byte, 4096)
		n, err := body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
			_ = rc.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if werr := rc.Flush(); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// StreamSSE proxies upstream SSE bytes to a nanite/sse Connection. Uses
// the connection's zero-alloc buffer pool and ResponseController for
// proper write deadline management.
func StreamSSE(tieLine *TieLine, r *http.Request, conn *nanitesse.Connection) error {
	cord := NewCord(tieLine)
	upstream, err := cord.tieLine.HTTPClient.Do(cord.buildRequest(r))
	if err != nil {
		return fmt.Errorf("plug: upstream SSE %q failed: %w", tieLine.Name, err)
	}
	defer upstream.Body.Close()
	if !isSSE(upstream.Header) {
		return fmt.Errorf("plug: upstream %q did not return SSE (Content-Type: %q)", tieLine.Name, upstream.Header.Get("Content-Type"))
	}
	buf := make([]byte, 4096)
	for {
		n, err := upstream.Body.Read(buf)
		if n > 0 {
			if werr := conn.SendBytes(buf[:n]); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// copyHeader duplicates headers from src to dst. Skips hop-by-hop headers.
func copyHeader(dst, src http.Header) {
	hop := map[string]bool{
		"Connection":          true,
		"Keep-Alive":          true,
		"Proxy-Authenticate":  true,
		"Proxy-Authorization": true,
		"Te":                  true,
		"Trailer":             true,
		"Transfer-Encoding":   true,
		"Upgrade":             true,
	}
	for k, vs := range src {
		if hop[k] {
			continue
		}
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}
