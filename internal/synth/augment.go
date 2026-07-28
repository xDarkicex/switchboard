package synth

import (
	"math/rand/v2"
	"strings"
	"unicode"
)

// paraphrase applies a random structural substitution with 30% probability.
// Returns the text unchanged otherwise.
func paraphrase(text string, rng *rand.Rand) string {
	if rng.Float64() > 0.3 {
		return text
	}
	rules := []struct{ from, to string }{
		{"make a video of", "video of"},
		{"create a video of", "video of"},
		{"generate a video of", "video of"},
		{"show me", "let me see"},
		{"i want to see", "show me"},
		{"video of", "footage of"},
		{"a clip of", "a video of"},
		{"a clip showing", "a video of"},
	}
	idx := rng.IntN(len(rules))
	return strings.ReplaceAll(text, rules[idx].from, rules[idx].to)
}

// synonymSwap replaces words with random synonyms from the built-in map.
// The casing of the original word is preserved. Function words (a, the, of,
// me, ...) are skipped — swapping them produces ungrammatical noise.
func synonymSwap(text string, rng *rand.Rand) string {
	words := strings.Fields(text)
	for i, w := range words {
		lower := strings.ToLower(w)
		if functionWords[lower] {
			continue
		}
		alts, ok := synonyms[lower]
		if !ok {
			continue
		}
		pick := alts[rng.IntN(len(alts))]
		words[i] = matchCase(w, pick)
	}
	return strings.Join(words, " ")
}

// wordDrop drops each content word with the given probability. Skips very
// short texts to avoid producing empty or single-word outputs.
func wordDrop(text string, prob float64, rng *rand.Rand) string {
	words := strings.Fields(text)
	if len(words) <= 2 {
		return text
	}
	out := make([]string, 0, len(words))
	for _, w := range words {
		if rng.Float64() < prob {
			continue
		}
		out = append(out, w)
	}
	if len(out) == 0 {
		return text
	}
	return strings.Join(out, " ")
}

// wordSwap swaps adjacent words with the given probability.
func wordSwap(text string, prob float64, rng *rand.Rand) string {
	words := strings.Fields(text)
	if len(words) < 2 {
		return text
	}
	for i := 0; i < len(words)-1; i++ {
		if rng.Float64() < prob {
			words[i], words[i+1] = words[i+1], words[i]
		}
	}
	return strings.Join(words, " ")
}

// capitalizeSwap randomly uppercases, lowercases, or leaves the text alone.
func capitalizeSwap(text string, rng *rand.Rand) string {
	switch rng.IntN(3) {
	case 0:
		return strings.ToUpper(text)
	case 1:
		return strings.ToLower(text)
	default:
		return text
	}
}

// typoInject swaps adjacent letters with the given probability. Stays away
// from the first two characters to avoid mangling the first word.
func typoInject(text string, prob float64, rng *rand.Rand) string {
	runes := []rune(text)
	for i := 2; i < len(runes); i++ {
		if rng.Float64() < prob && unicode.IsLetter(runes[i]) {
			runes[i], runes[i-1] = runes[i-1], runes[i]
		}
	}
	return string(runes)
}

// matchCase preserves the casing pattern of the original when substituting.
// ALL CAPS stays ALL CAPS, Capitalized stays Capitalized, lower stays lower.
func matchCase(orig, replacement string) string {
	if orig == strings.ToUpper(orig) && len(orig) > 1 {
		return strings.ToUpper(replacement)
	}
	runes := []rune(orig)
	if len(runes) > 0 && unicode.IsUpper(runes[0]) {
		out := []rune(replacement)
		if len(out) > 0 {
			out[0] = unicode.ToUpper(out[0])
		}
		return string(out)
	}
	return replacement
}
