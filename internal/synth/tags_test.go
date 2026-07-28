package synth

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestTagsLabels(t *testing.T) {
	tags := Tags{
		Length:     "short",
		Complexity: "simple",
		Style:      "photorealistic",
		Quality:    "4k",
		Camera:     "static",
		Physics:    "none",
		Refs:       "none",
		Cost:       "cheap",
	}
	got := tags.Labels()
	want := []string{
		"length:short",
		"complexity:simple",
		"style:photorealistic",
		"quality:4k",
		"camera:static",
		"physics:none",
		"refs:none",
		"cost:cheap",
	}
	if !slices.Equal(got, want) {
		t.Errorf("Labels = %v, want %v", got, want)
	}
}

func TestTagsLabelsSkipsEmpty(t *testing.T) {
	tags := Tags{Length: "short", Style: "cinematic"}
	got := tags.Labels()
	want := []string{"length:short", "style:cinematic"}
	if !slices.Equal(got, want) {
		t.Errorf("Labels = %v, want %v", got, want)
	}
}

func TestTagsSet(t *testing.T) {
	tags := Tags{}
	tags.Set("length", "medium")
	tags.Set("style", "cinematic")
	if tags.Length != "medium" || tags.Style != "cinematic" {
		t.Errorf("Set failed: %+v", tags)
	}
	tags.Set("unknown", "ignored")
	if tags.Length != "medium" {
		t.Error("Set should ignore unknown dimension")
	}
}

func TestGenerateIntentsMultiLabel(t *testing.T) {
	g := New(42)
	intents := []Intent{
		{
			Name: "simple_photo",
			Tags: Tags{
				Length:     "short",
				Complexity: "simple",
				Style:      "photorealistic",
				Quality:    "4k",
				Camera:     "static",
				Physics:    "none",
				Refs:       "none",
				Cost:       "cheap",
			},
			Templates: []string{"Make a video of {subject}"},
			Slots: map[string][]string{
				"subject": {"sunset over the ocean", "bird flying"},
			},
		},
	}
	examples, err := g.GenerateIntents(intents, 5)
	if err != nil {
		t.Fatalf("GenerateIntents: %v", err)
	}
	if len(examples) != 5 {
		t.Fatalf("expected 5 examples, got %d", len(examples))
	}
	for _, ex := range examples {
		if len(ex.Tags.Labels()) != 8 {
			t.Errorf("expected 8 labels per example, got %d", len(ex.Tags.Labels()))
		}
	}
}

func TestWriteMultiLabel(t *testing.T) {
	g := New(42)
	examples := []Example{
		{
			Text: "Make a video of a sunset",
			Tags: Tags{
				Length: "short", Style: "photorealistic", Quality: "4k",
				Camera: "static", Physics: "none", Refs: "none", Cost: "cheap",
				Complexity: "simple",
			},
		},
	}
	var buf bytes.Buffer
	if err := g.WriteMultiLabel(&buf, examples); err != nil {
		t.Fatalf("WriteMultiLabel: %v", err)
	}
	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 8 {
		t.Errorf("expected 8 lines, got %d", len(lines))
	}
	for _, want := range []string{"length:short", "style:photorealistic", "complexity:simple"} {
		if !strings.Contains(out, "__label__"+want+" ") {
			t.Errorf("missing __label__%s", want)
		}
	}
}

func TestWriteMultiLabelFromRealPrompts(t *testing.T) {
	real := []RealPrompt{
		{
			ID: "test-1",
			Text: "A red fox stands on a mossy rock",
			Tags: Tags{
				Length: "short", Complexity: "simple", Style: "photorealistic",
				Quality: "4k", Camera: "dolly", Physics: "basic", Refs: "none", Cost: "medium",
			},
		},
	}
	var examples []Example
	for _, r := range real {
		examples = append(examples, r.ToExample())
	}
	g := New(1)
	var buf bytes.Buffer
	if err := g.WriteMultiLabel(&buf, examples); err != nil {
		t.Fatalf("WriteMultiLabel: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "__label__length:short A red fox") {
		t.Error("missing multi-label output")
	}
}

func TestLoadRealPrompts(t *testing.T) {
	yamlData := []byte(`
prompts:
  - id: test-1
    text: "A red fox"
    tags:
      length: short
      complexity: simple
      style: photorealistic
      quality: "4k"
      camera: static
      physics: none
      refs: none
      cost: cheap
`)
	got, err := LoadRealPrompts(yamlData)
	if err != nil {
		t.Fatalf("LoadRealPrompts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d prompts, want 1", len(got))
	}
	if got[0].Tags.Length != "short" {
		t.Errorf("Tags.Length = %q", got[0].Tags.Length)
	}
}
