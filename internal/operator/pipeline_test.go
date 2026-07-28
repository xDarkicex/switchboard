package operator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPipelineRequiresAllModels(t *testing.T) {
	cfg := PipelineConfig{
		Length: PipelineSlot{ModelPath: "/nonexistent/path"},
	}
	_, err := NewPipeline(cfg)
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestPipelineStruct(t *testing.T) {
	// Sanity check the struct fields are exported correctly.
	cfg := PipelineConfig{}
	cfg.Length.ModelPath = "/tmp/length.bin"
	cfg.Style.ModelPath = "/tmp/style.bin"
	if cfg.Length.DefaultVal != "" {
		t.Error("DefaultVal should default to empty")
	}
}

func TestPipelineSlotFields(t *testing.T) {
	slot := PipelineSlot{
		ModelPath:  "/tmp/test.bin",
		DefaultVal: "fallback",
	}
	if slot.ModelPath != "/tmp/test.bin" {
		t.Error("ModelPath not set")
	}
	if slot.DefaultVal != "fallback" {
		t.Error("DefaultVal not set")
	}
}

func TestPipelineClassifyDebugMissing(t *testing.T) {
	// Create a minimal pipeline with no classifiers to test error path.
	p := &Pipeline{classifiers: map[string]*Classifier{}}
	_, err := p.ClassifyDebug("length", "test")
	if err == nil {
		t.Fatal("expected error for missing classifier")
	}
	// Cleanup if any temp files; here there are none.
	_ = filepath.Clean
	_ = os.Remove
}
