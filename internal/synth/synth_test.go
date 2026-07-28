package synth

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewDeterministic(t *testing.T) {
	g1 := New(42)
	g2 := New(42)
	labels := []Label{{
		Name:      "alpha",
		Templates: []string{"hello {who}"},
		Slots:     map[string][]string{"who": {"world", "there"}},
		Augment:   AugmentConfig{SynonymSwap: true},
	}}
	out1, err := g1.Generate(labels, 50)
	if err != nil {
		t.Fatalf("g1: %v", err)
	}
	out2, err := g2.Generate(labels, 50)
	if err != nil {
		t.Fatalf("g2: %v", err)
	}
	if len(out1["alpha"]) != len(out2["alpha"]) {
		t.Fatalf("length mismatch: %d vs %d", len(out1["alpha"]), len(out2["alpha"]))
	}
	for i := range out1["alpha"] {
		if out1["alpha"][i] != out2["alpha"][i] {
			t.Fatalf("non-deterministic at %d: %q vs %q", i, out1["alpha"][i], out2["alpha"][i])
		}
	}
}

func TestGenerateProducesPerLabelCount(t *testing.T) {
	g := New(7)
	labels := []Label{
		{
			Name:      "a",
			Templates: []string{"alpha text"},
			Augment:   AugmentConfig{},
		},
		{
			Name:      "b",
			Templates: []string{"beta text"},
			Augment:   AugmentConfig{},
		},
	}
	out, err := g.Generate(labels, 100)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(out["a"]) != 100 {
		t.Fatalf("a: got %d, want 100", len(out["a"]))
	}
	if len(out["b"]) != 100 {
		t.Fatalf("b: got %d, want 100", len(out["b"]))
	}
}

func TestGenerateRejectsEmptyTemplates(t *testing.T) {
	g := New(1)
	_, err := g.Generate([]Label{{Name: "x", Templates: nil}}, 10)
	if err == nil {
		t.Fatal("expected error for empty templates")
	}
}

func TestFillSlotsMissingSlotFails(t *testing.T) {
	g := New(1)
	_, err := g.Generate([]Label{{
		Name:      "x",
		Templates: []string{"hello {missing}"},
		Slots:     map[string][]string{},
	}}, 1)
	if err == nil {
		t.Fatal("expected error for missing slot")
	}
}

func TestWriteToSteadyFormat(t *testing.T) {
	g := New(11)
	labels := []Label{{
		Name:      "video_simple",
		Templates: []string{"make a video of a sunset"},
		Augment:   AugmentConfig{},
	}}
	var buf bytes.Buffer
	if err := g.WriteTo(&buf, labels, 3); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, "__label__video_simple ") {
			t.Fatalf("line %d missing label prefix: %q", i, line)
		}
	}
}

func TestSynonymSwapReplacesKnownWords(t *testing.T) {
	g := New(3)
	labels := []Label{{
		Name:      "x",
		Templates: []string{"make a video of a sunset"},
		Augment:   AugmentConfig{SynonymSwap: true},
	}}
	out, err := g.Generate(labels, 200)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	varied := make(map[string]bool)
	for _, text := range out["x"] {
		varied[text] = true
	}
	if len(varied) < 5 {
		t.Fatalf("expected at least 5 distinct outputs, got %d", len(varied))
	}
}

func TestWordDropShortTextUnchanged(t *testing.T) {
	g := New(1)
	labels := []Label{{
		Name:      "x",
		Templates: []string{"hi there"},
		Augment:   AugmentConfig{WordDrop: 0.9},
	}}
	out, err := g.Generate(labels, 5)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, text := range out["x"] {
		if text != "hi there" {
			t.Fatalf("short text should be unchanged, got %q", text)
		}
	}
}

func TestCapitalizeSwapIdentifiable(t *testing.T) {
	g := New(1)
	labels := []Label{{
		Name:      "x",
		Templates: []string{"make a video of a sunset"},
		Augment:   AugmentConfig{CapitalizeSwap: true},
	}}
	out, err := g.Generate(labels, 200)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	seenUpper, seenLower := false, false
	for _, text := range out["x"] {
		if text == strings.ToUpper(text) {
			seenUpper = true
		}
		if text == strings.ToLower(text) {
			seenLower = true
		}
	}
	if !seenUpper || !seenLower {
		t.Fatalf("expected both upper and lower variants, got upper=%v lower=%v", seenUpper, seenLower)
	}
}

func TestLoadPresetInheritsAugment(t *testing.T) {
	yamlData := []byte(`
seed: 42
augment:
  synonym_swap: true
  word_drop: 0.05
labels:
  - name: a
    templates: ["hello"]
  - name: b
    templates: ["world"]
    augment:
      word_drop: 0.1
`)
	p, err := LoadPreset(yamlData)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if p.Labels[0].Augment.SynonymSwap != true {
		t.Fatalf("label a should inherit synonym_swap: %+v", p.Labels[0].Augment)
	}
	if p.Labels[1].Augment.WordDrop != 0.1 {
		t.Fatalf("label b should keep its own word_drop: %+v", p.Labels[1].Augment)
	}
	if p.Labels[1].Augment.SynonymSwap != true {
		t.Fatalf("label b should still inherit synonym_swap: %+v", p.Labels[1].Augment)
	}
}

func TestReplaceAllEmpty(t *testing.T) {
	if got := replaceAll("hello", "", "x"); got != "hello" {
		t.Fatalf("empty old: got %q", got)
	}
}
