package trunk

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDropWritesReason(t *testing.T) {
	rr := httptest.NewRecorder()
	Drop(rr, nil, CircuitOpen)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rr.Code)
	}
	if got := rr.Header().Get("X-Switchboard-Reason"); got != "the line is busy, try again shortly" {
		t.Errorf("X-Switchboard-Reason = %q", got)
	}
	if !contains(rr.Body.String(), "switchboard") {
		t.Errorf("body = %q", rr.Body.String())
	}
}

func TestNumberChangedStatus410(t *testing.T) {
	rr := httptest.NewRecorder()
	NumberChanged(rr, nil, "555-1234")
	if rr.Code != http.StatusGone {
		t.Errorf("status = %d, want 410", rr.Code)
	}
	if !contains(rr.Body.String(), "555-1234") {
		t.Errorf("body should mention the number: %q", rr.Body.String())
	}
}

func TestAllCircuitsBusy(t *testing.T) {
	cases := []struct {
		healthy []bool
		want    bool
	}{
		{nil, true},
		{[]bool{false, false}, true},
		{[]bool{true, false}, false},
		{[]bool{false, true}, false},
		{[]bool{true, true}, false},
	}
	for _, tc := range cases {
		got := AllCircuitsBusy(tc.healthy)
		if got != tc.want {
			t.Errorf("AllCircuitsBusy(%v) = %v, want %v", tc.healthy, got, tc.want)
		}
	}
}

func TestReasonStatus(t *testing.T) {
	if CircuitOpen.HTTPStatus() != http.StatusServiceUnavailable {
		t.Error("CircuitOpen should be 503")
	}
	if NumberNotInService.HTTPStatus() != http.StatusGone {
		t.Error("NumberNotInService should be 410")
	}
	if Unauthorized.HTTPStatus() != http.StatusUnauthorized {
		t.Error("Unauthorized should be 401")
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
