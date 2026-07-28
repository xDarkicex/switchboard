// Package trunk refuses calls. When the operator is busy, when the called
// party is out-of-service, or when every upstream is on fire, trunk drops
// the call with a vintage intercept message.
package trunk

import (
	"fmt"
	"net/http"
)

// Reason explains why a call was dropped.
type Reason int

const (
	// CircuitOpen: the operator is on another call.
	CircuitOpen Reason = iota
	// NumberNotInService: the called party is no longer in service.
	NumberNotInService
	// AllCircuitsDown: every upstream is unreachable.
	AllCircuitsDown
	// Unauthorized: the caller didn't provide valid credentials.
	Unauthorized
)

// String returns the operator-voice explanation of the reason.
func (r Reason) String() string {
	switch r {
	case CircuitOpen:
		return "the line is busy, try again shortly"
	case NumberNotInService:
		return "the number you have dialed is no longer in service"
	case AllCircuitsDown:
		return "all circuits are busy, please try again later"
	case Unauthorized:
		return "your call is not going through, please check your credentials"
	default:
		return "your call cannot be completed as dialed"
	}
}

// HTTPStatus returns the HTTP status code to send for this reason.
func (r Reason) HTTPStatus() int {
	switch r {
	case NumberNotInService:
		return http.StatusGone
	case Unauthorized:
		return http.StatusUnauthorized
	default:
		return http.StatusServiceUnavailable
	}
}

// Drop refuses the call. Writes the standard switchboard intercept message
// to the client with the corresponding HTTP status.
func Drop(w http.ResponseWriter, _ *http.Request, reason Reason) {
	status := reason.HTTPStatus()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Switchboard-Reason", reason.String())
	w.WriteHeader(status)
	fmt.Fprintln(w, "switchboard: your call cannot be completed as dialed")
	fmt.Fprintf(w, "reason: %s\n", reason)
}

// NumberChanged writes the "number has been changed" intercept.
func NumberChanged(w http.ResponseWriter, _ *http.Request, calledParty string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Switchboard-Intercept", "number-changed")
	w.WriteHeader(http.StatusGone)
	fmt.Fprintf(w, "switchboard: the number %q is no longer in service\n", calledParty)
}

// AllCircuitsBusy returns true if every upstream is failing.
func AllCircuitsBusy(healthy []bool) bool {
	for _, ok := range healthy {
		if ok {
			return false
		}
	}
	return true
}
