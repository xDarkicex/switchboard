// Package operator wraps a steady classifier behind the human-attendant
// metaphor. The Operator listens on the line, classifies the call, and
// returns the destination trunk. It also owns the busy lamp — the circuit
// breaker that refuses new calls when something upstream is on fire.
package operator

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/xDarkicex/steady"
)

// ErrBusy is returned when the operator is on another call. The caller should
// use trunk.Drop to refuse the incoming call instead of retrying.
var ErrBusy = errors.New("operator: the line is busy")

// Operator is the human attendant at the cord board. One Operator per
// switchboard — it's not safe to mutate the underlying steady model
// concurrently, but the Listen path is read-only and concurrency-safe.
type Operator struct {
	model           *steady.Model
	busyLamp        atomic.Bool
	defaultTrunk    string
	nightForwarding atomic.Pointer[string]
}

// New loads the trained model at modelPath and prepares the Operator. The
// caller must call Close when finished.
func New(modelPath string, labelNames []string, defaultTrunk string) (*Operator, error) {
	model, err := steady.Load(modelPath)
	if err != nil {
		return nil, fmt.Errorf("operator: %w", err)
	}
	if len(labelNames) > 0 {
		model.SetLabelNames(labelNames)
	}
	return &Operator{
		model:        model,
		defaultTrunk: defaultTrunk,
	}, nil
}

// Close releases the mmap'd model and its pools.
func (o *Operator) Close() error {
	return o.model.Close()
}

// Listen classifies the incoming call and returns the destination trunk.
// Empty (out-of-distribution) calls return the defaultTrunk. The call is
// refused with ErrBusy if the busyLamp is on. If nightService is set, it
// routes all calls to the night-forwarding destination (after-hours mode).
func (o *Operator) Listen(call string) (string, error) {
	if o.busyLamp.Load() {
		return "", ErrBusy
	}
	if n := o.nightForwarding.Load(); n != nil {
		return *n, nil
	}
	ps := o.model.Classify(call)
	if ps.IsEmpty() {
		return o.defaultTrunk, nil
	}
	best := bestLabel(ps.Confidences)
	if best >= len(ps.Kinds) {
		return o.defaultTrunk, nil
	}
	return ps.Kinds[best], nil
}

// ListenDebug returns the full prediction set for debugging. Same input
// rules as Listen, but nightService does not override here — you see what
// the operator would have classified without after-hours mode.
func (o *Operator) ListenDebug(call string) (steady.PredictionSet, error) {
	if o.busyLamp.Load() {
		return steady.PredictionSet{}, ErrBusy
	}
	return o.model.Classify(call), nil
}

// IsBusy returns true when the operator is on another call.
func (o *Operator) IsBusy() bool {
	return o.busyLamp.Load()
}

// IsRinging returns true when the operator is on the line (not busy).
func (o *Operator) IsRinging() bool {
	return !o.busyLamp.Load()
}

// Pickup accepts a call. Alias for Listen — the operator picks up the
// receiver and routes the call.
func (o *Operator) Pickup(call string) (string, error) {
	return o.Listen(call)
}

// Switchhook toggles the on-hook / off-hook state. duty == true puts the
// operator on the line; duty == false hangs up.
func (o *Operator) Switchhook(duty bool) {
	o.busyLamp.Store(!duty)
}

// NightService sets the after-hours forwarding trunk. Empty string clears
// it. When set, this overrides the defaultTrunk for both classifications and
// out-of-distribution calls.
func (o *Operator) NightService(forwarding string) {
	if forwarding == "" {
		o.nightForwarding.Store(nil)
		return
	}
	o.nightForwarding.Store(&forwarding)
}

// MarkBusy trips the busyLamp. Other callers will see ErrBusy until
// ClearBusy is called.
func (o *Operator) MarkBusy() {
	o.busyLamp.Store(true)
}

// ClearBusy turns the busyLamp off.
func (o *Operator) ClearBusy() {
	o.busyLamp.Store(false)
}

// bestLabel returns the index of the highest-confidence label.
func bestLabel(confidences []float32) int {
	best := 0
	for i, c := range confidences {
		if c > confidences[best] {
			best = i
		}
	}
	return best
}
