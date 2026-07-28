package synth

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Preset is the top-level YAML config for a generator preset. Supports two
// formats:
//
//  1. Legacy single-label: Labels []Label — emits one steady label per prompt.
//  2. Multi-label: Intents []Intent with Tags — emits one steady label per
//     (dimension:value, prompt) pair, suitable for the per-dimension pipeline.
//
// When both fields are present, the legacy labels are ignored and only
// intents are processed.
type Preset struct {
	Seed       int64       `yaml:"seed"`
	Augment    AugmentConfig `yaml:"augment"`
	Labels     []Label     `yaml:"labels"`
	Vocabulary Vocabulary `yaml:"vocabulary"`
	Intents    []Intent    `yaml:"intents"`
}

// Vocabulary is the cinematography vocabulary an intent preset can pull from.
// Templates reference these via slot names: slots: { camera: [vocabulary.cameras] }
// or directly fill them with hand-picked phrases.
type Vocabulary struct {
	Cameras    []string               `yaml:"cameras"`
	Lighting   []string               `yaml:"lighting"`
	Audio      AudioVocabulary        `yaml:"audio"`
	Mood       []string               `yaml:"mood"`
	Quality    []string               `yaml:"quality"`
	FilmStocks map[string][]string    `yaml:"film_stocks"`
}

// AudioVocabulary groups audio cues into ambient, music, and sfx.
type AudioVocabulary struct {
	Ambient []string `yaml:"ambient"`
	Music   []string `yaml:"music"`
	SFX     []string `yaml:"sfx"`
}

// LoadPresetFile reads a preset YAML from disk and returns the parsed
// Preset. Per-label augment fields that are zero inherit from the
// preset-wide augment.
func LoadPresetFile(path string) (Preset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Preset{}, fmt.Errorf("synth: read preset %q: %w", path, err)
	}
	return LoadPreset(data)
}

// LoadPreset parses a preset YAML byte slice.
func LoadPreset(data []byte) (Preset, error) {
	var p Preset
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Preset{}, fmt.Errorf("synth: parse preset: %w", err)
	}
	for i := range p.Labels {
		p.Labels[i].Augment = p.Labels[i].Augment.Inherit(p.Augment)
		if p.Labels[i].Weight == 0 {
			p.Labels[i].Weight = 1
		}
	}
	return p, nil
}

// RealPrompt is one annotated entry in the real-prompt corpus.
type RealPrompt struct {
	ID      string `yaml:"id"`
	Source  string `yaml:"source"`
	Text    string `yaml:"text"`
	Tags    Tags   `yaml:"tags"`
}

// RealPromptsFile is the YAML wrapper for a real-prompt corpus.
type RealPromptsFile struct {
	Prompts []RealPrompt `yaml:"prompts"`
}

// LoadRealPromptsFile reads a real_prompts.yaml corpus from disk.
func LoadRealPromptsFile(path string) ([]RealPrompt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("synth: read %q: %w", path, err)
	}
	return LoadRealPrompts(data)
}

// LoadRealPrompts parses a real_prompts.yaml byte slice.
func LoadRealPrompts(data []byte) ([]RealPrompt, error) {
	var f RealPromptsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("synth: parse real_prompts: %w", err)
	}
	return f.Prompts, nil
}

// ToExamples converts real prompts to the Example type for the multi-label pipeline.
func (r RealPrompt) ToExample() Example {
	return Example{Text: r.Text, Tags: r.Tags}
}
