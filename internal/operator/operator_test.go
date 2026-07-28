package operator

import (
	"os"
	"testing"
)

const modelPath = "../../models/length.bin"

func newTestOperator(t *testing.T) *Operator {
	t.Helper()
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("model not found at %s; run scripts/train-pipeline first: %v", modelPath, err)
	}
	op, err := New(modelPath, []string{"short", "medium", "long", "multi_stage"}, "medium")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = op.Close() })
	return op
}

func TestOperatorBusyLamp(t *testing.T) {
	op := newTestOperator(t)
	if op.IsBusy() {
		t.Fatal("busyLamp should be off at start")
	}
	if !op.IsRinging() {
		t.Fatal("operator should be ringing at start")
	}
	op.Switchhook(false)
	if !op.IsBusy() {
		t.Error("after Switchhook(false), should be busy")
	}
	op.Switchhook(true)
	if !op.IsRinging() {
		t.Error("after Switchhook(true), should be ringing")
	}
}

func TestOperatorErrBusy(t *testing.T) {
	op := newTestOperator(t)
	op.MarkBusy()
	_, err := op.Listen("test")
	if err != ErrBusy {
		t.Fatalf("Listen while busy: got %v, want ErrBusy", err)
	}
	op.ClearBusy()
	_, err = op.Listen("test")
	if err != nil {
		t.Fatalf("Listen after clear: %v", err)
	}
}

func TestOperatorNightService(t *testing.T) {
	op := newTestOperator(t)
	op.NightService("overridden")
	got, err := op.Listen("any text")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	if got != "overridden" {
		t.Errorf("nightService = %q, want overridden", got)
	}
	op.NightService("")
	got, err = op.Listen("any text")
	if err != nil {
		t.Fatalf("Listen after clear: %v", err)
	}
	if got == "overridden" {
		t.Error("nightService should be cleared")
	}
}

func TestOperatorClose(t *testing.T) {
	op := newTestOperator(t)
	if err := op.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Second close should not crash.
	_ = op.Close()
}
