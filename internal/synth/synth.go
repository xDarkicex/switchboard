// Package synth generates synthetic training data for a steady classifier from
// a declarative preset. Templates carry {slot} placeholders; the generator
// fills slots, then runs optional augmentations (synonym swap, word drop, word
// swap, paraphrase, capitalization swap, typo injection) before emitting the
// steady training format.
//
// Two output formats are supported:
//
//   - WriteTo(): single-label, one line per (label, text) pair. Used for the
//     legacy single-classifier pipeline.
//   - WriteMultiLabel(): multi-label, one line per (Example, dimension:value)
//     pair. Used for the pipeline classifier (one model per dimension).
package synth

import (
	"bufio"
	"fmt"
	"io"
	"math/rand/v2"
	"strings"
)

// Generator produces training data deterministically from a seed. Same seed
// + same preset = same output, every time.
type Generator struct {
	rng *rand.Rand
}

// New builds a Generator with the given seed. seed == 0 uses a fixed
// non-zero starting value so callers can pass 0 as "don't care".
func New(seed int64) *Generator {
	if seed == 0 {
		seed = 1
	}
	src := rand.NewPCG(uint64(seed), uint64(seed^0xDEADBEEF))
	return &Generator{rng: rand.New(src)}
}

// Generate produces perLabel examples for each label, deterministic from the
// generator's seed. Returns a map keyed by label name. Kept for the legacy
// single-label pipeline.
func (g *Generator) Generate(labels []Label, perLabel int) (map[string][]string, error) {
	out := make(map[string][]string, len(labels))
	for _, label := range labels {
		if len(label.Templates) == 0 {
			return nil, fmt.Errorf("synth: label %q has no templates", label.Name)
		}
		for range perLabel {
			text, err := g.synthesize(label)
			if err != nil {
				return nil, err
			}
			out[label.Name] = append(out[label.Name], text)
		}
	}
	return out, nil
}

// WriteTo generates and writes steady-format single-label training data to w.
// Each line is "__label__<name> <text>".
func (g *Generator) WriteTo(w io.Writer, labels []Label, perLabel int) error {
	examples, err := g.Generate(labels, perLabel)
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(w)
	for _, label := range labels {
		for _, text := range examples[label.Name] {
			if _, err := fmt.Fprintf(bw, "__label__%s %s\n", label.Name, text); err != nil {
				return err
			}
		}
	}
	return bw.Flush()
}

// GenerateIntents produces perIntent examples for each intent. Each generated
// example is tagged with the intent's Tags. Used for the multi-label pipeline.
func (g *Generator) GenerateIntents(intents []Intent, perIntent int) ([]Example, error) {
	var out []Example
	for _, intent := range intents {
		if len(intent.Templates) == 0 {
			return nil, fmt.Errorf("synth: intent %q has no templates", intent.Name)
		}
		for range perIntent {
			text, err := synthesizeIntent(intent, g.rng)
			if err != nil {
				return nil, err
			}
			out = append(out, Example{Text: text, Tags: intent.Tags})
		}
	}
	return out, nil
}

// WriteMultiLabel writes examples in steady multi-label format. For each
// Example, emits one line per non-empty dimension:value tag. The output is
// suitable for training per-dimension classifiers.
func (g *Generator) WriteMultiLabel(w io.Writer, examples []Example) error {
	bw := bufio.NewWriter(w)
	for _, ex := range examples {
		for _, label := range ex.Tags.Labels() {
			if _, err := fmt.Fprintf(bw, "__label__%s %s\n", label, ex.Text); err != nil {
				return err
			}
		}
	}
	return bw.Flush()
}

// synthesizeIntent produces one training example for an intent.
func synthesizeIntent(intent Intent, rng *rand.Rand) (string, error) {
	tmpl := intent.Templates[rng.IntN(len(intent.Templates))]
	text, err := fillSlots(tmpl, intent.Slots, rng)
	if err != nil {
		return "", fmt.Errorf("synth: intent %q: %w", intent.Name, err)
	}
	return text, nil
}

// synthesize produces one training example for a label (legacy API).
func (g *Generator) synthesize(label Label) (string, error) {
	tmpl := pickTemplate(label.Templates, g.rng)
	text, err := fillSlots(tmpl, label.Slots, g.rng)
	if err != nil {
		return "", err
	}
	text = applyAugment(text, label.Augment, g.rng)
	return text, nil
}

// pickTemplate selects a random template from the list.
func pickTemplate(templates []string, rng *rand.Rand) string {
	return templates[rng.IntN(len(templates))]
}

// fillSlots replaces every {slot} placeholder with a random value from the
// matching slot entry. Repeatedly runs until no more placeholders remain so
// nested slot values (e.g. {stages} containing {scene1}) are also resolved.
func fillSlots(pattern string, slots map[string][]string, rng *rand.Rand) (string, error) {
	text := pattern
	for iter := range 8 {
		_ = iter
		var progressed bool
		for slotName, values := range slots {
			placeholder := "{" + slotName + "}"
			if len(values) == 0 {
				return "", fmt.Errorf("synth: slot %q has no values", slotName)
			}
			if strings.Contains(text, placeholder) {
				text = replaceAll(text, placeholder, values[rng.IntN(len(values))])
				progressed = true
			}
		}
		if !progressed {
			break
		}
	}
	if strings.Contains(text, "{") && strings.Contains(text, "}") {
		return "", fmt.Errorf("synth: unresolved placeholder in %q", pattern)
	}
	return text, nil
}

// applyAugment runs each enabled augmentation in order. Paraphrase first
// (structural), then word-level transforms, then cosmetic.
func applyAugment(text string, augment AugmentConfig, rng *rand.Rand) string {
	if augment.Paraphrase {
		text = paraphrase(text, rng)
	}
	if augment.SynonymSwap {
		text = synonymSwap(text, rng)
	}
	if augment.WordDrop > 0 {
		text = wordDrop(text, capProb(augment.WordDrop), rng)
	}
	if augment.WordSwap > 0 {
		text = wordSwap(text, capProb(augment.WordSwap), rng)
	}
	if augment.CapitalizeSwap {
		text = capitalizeSwap(text, rng)
	}
	if augment.TypoInject > 0 {
		text = typoInject(text, capProb(augment.TypoInject), rng)
	}
	return text
}

// capProb clamps a probability to [0, 0.25] so augmentations don't shred
// the text.
func capProb(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 0.25 {
		return 0.25
	}
	return p
}

// replaceAll is a non-regex string replacement.
func replaceAll(s, old, newStr string) string {
	if old == "" {
		return s
	}
	var b strings.Builder
	for {
		idx := strings.Index(s, old)
		if idx < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:idx])
		b.WriteString(newStr)
		s = s[idx+len(old):]
	}
}
