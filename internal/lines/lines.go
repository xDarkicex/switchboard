// Package lines is the patch cord bundle — the rule chains that override
// per-label parameters. Each rule is a When/Patch pair; the bundle's Dial
// evaluates them in order against the call's parameters.
package lines

import "maps"

// Rule transforms a call's parameters when its When predicate matches.
// Keep When pure and fast — it's called on every classified call.
type Rule struct {
	Name  string
	When  func(label string, params map[string]any) bool
	Patch func(params map[string]any) map[string]any
}

// Bundle is an ordered list of rules. Dial applies them in order; the first
// matching rule's Patch runs first, then subsequent rules see the patched
// payload. If lazy is non-nil, rules are re-evaluated from it on each Dial.
type Bundle struct {
	rules []Rule
	lazy  func() []Rule
}

// New builds a new bundle from the given rules.
func New(rules ...Rule) *Bundle {
	return &Bundle{rules: rules}
}

// NewLazy builds a bundle that re-evaluates its rules on every Dial. Useful
// when the rules themselves depend on operator state that may change.
func NewLazy(rules func() []Rule) *Bundle {
	return &Bundle{lazy: rules}
}

// Dial evaluates the rules against a call and returns the patched params.
// The original params map is not modified.
func (b *Bundle) Dial(label string, params map[string]any) map[string]any {
	out := copyMap(params)
	if b.lazy != nil {
		b.rules = b.lazy()
	}
	for _, r := range b.rules {
		if r.When(label, out) {
			out = r.Patch(out)
		}
	}
	return out
}

// Directory returns the rules as a human-readable listing.
func (b *Bundle) Directory() []string {
	if b.lazy != nil {
		b.rules = b.lazy()
	}
	out := make([]string, 0, len(b.rules))
	for _, r := range b.rules {
		name := r.Name
		if name == "" {
			name = "rule"
		}
		out = append(out, name)
	}
	return out
}

// Set overrides a parameter value. Convenience for Patch functions.
func Set(key string, value any) func(map[string]any) map[string]any {
	return func(p map[string]any) map[string]any {
		p[key] = value
		return p
	}
}

// MatchLabel is a When predicate that matches a specific label.
func MatchLabel(label string) func(string, map[string]any) bool {
	return func(got string, _ map[string]any) bool {
		return got == label
	}
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	maps.Copy(out, m)
	return out
}
