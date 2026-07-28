// Package synth generates synthetic training data for a steady classifier from
// a declarative preset. Templates carry {slot} placeholders; the generator
// fills slots, then runs optional augmentations (synonym swap, word drop, word
// swap, paraphrase, capitalization swap, typo injection) before emitting the
// steady training format: __label__<name>\t<text>.
package synth

// Label describes one class the classifier should learn. The Name is the
// steady label; Templates are pattern strings with {slot} placeholders that
// are filled from Slots. Augment overrides the preset-wide defaults for this
// label.
type Label struct {
	Name        string              `yaml:"name"`
	Description string              `yaml:"description"`
	Templates   []string            `yaml:"templates"`
	Slots       map[string][]string `yaml:"slots"`
	Augment     AugmentConfig       `yaml:"augment"`
	Weight      int                 `yaml:"weight"`
}

// AugmentConfig controls the augmentation pipeline. Zero values mean "off".
// A label can override the preset-wide defaults; any field left at zero here
// falls back to the preset-wide value via Inherit.
type AugmentConfig struct {
	// SynonymSwap replaces words with synonyms from the built-in vocab map.
	SynonymSwap bool `yaml:"synonym_swap"`
	// WordDrop drops a content word with this probability. Capped at 0.25.
	WordDrop float64 `yaml:"word_drop"`
	// WordSwap swaps two adjacent words with this probability.
	WordSwap float64 `yaml:"word_swap"`
	// Paraphrase applies structural substitutions (e.g. "make a video of X" -> "video of X").
	Paraphrase bool `yaml:"paraphrase"`
	// CapitalizeSwap randomly uppercases, lowercases, or leaves the text.
	CapitalizeSwap bool `yaml:"capitalize_swap"`
	// TypoInject injects realistic typos (adjacent-letter swap) with this probability.
	TypoInject float64 `yaml:"typo_inject"`
}

// Inherit returns a copy of a with any zero-valued field filled from parent.
// Lets a label override only the augment knobs it cares about while inheriting
// the rest from the preset-wide defaults.
func (a AugmentConfig) Inherit(parent AugmentConfig) AugmentConfig {
	if !a.SynonymSwap {
		a.SynonymSwap = parent.SynonymSwap
	}
	if a.WordDrop == 0 {
		a.WordDrop = parent.WordDrop
	}
	if a.WordSwap == 0 {
		a.WordSwap = parent.WordSwap
	}
	if !a.Paraphrase {
		a.Paraphrase = parent.Paraphrase
	}
	if !a.CapitalizeSwap {
		a.CapitalizeSwap = parent.CapitalizeSwap
	}
	if a.TypoInject == 0 {
		a.TypoInject = parent.TypoInject
	}
	return a
}
