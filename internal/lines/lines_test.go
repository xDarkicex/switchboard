package lines

import "testing"

func TestDialAppliesMatchingRule(t *testing.T) {
	b := New(
		Rule{
			Name: "long-form",
			When: MatchLabel("high_quality_long"),
			Patch: func(p map[string]any) map[string]any {
				p["duration"] = 60
				p["quality"] = "ultra"
				return p
			},
		},
		Rule{
			Name: "short-form",
			When: MatchLabel("simple_short"),
			Patch: func(p map[string]any) map[string]any {
				p["duration"] = 5
				return p
			},
		},
	)
	cases := []struct {
		label string
		want  any
	}{
		{"high_quality_long", 60},
		{"simple_short", 5},
		{"fallback", 0},
	}
	for _, tc := range cases {
		got := b.Dial(tc.label, map[string]any{"duration": 0})
		if got["duration"] != tc.want {
			t.Errorf("Dial(%q).duration = %v, want %q", tc.label, got["duration"], tc.want)
		}
	}
}

func TestDialDoesNotMutateInput(t *testing.T) {
	b := New(Rule{
		When: MatchLabel("simple_short"),
		Patch: func(p map[string]any) map[string]any {
			p["duration"] = 5
			return p
		},
	})
	in := map[string]any{"duration": 0}
	_ = b.Dial("simple_short", in)
	if in["duration"] != 0 {
		t.Errorf("input mutated: %v", in)
	}
}

func TestDialChainsRules(t *testing.T) {
	b := New(
		Rule{
			When: MatchLabel("l1"),
			Patch: func(p map[string]any) map[string]any {
				p["a"] = 1
				return p
			},
		},
		Rule{
			When: func(_ string, p map[string]any) bool {
				return p["a"] == 1
			},
			Patch: func(p map[string]any) map[string]any {
				p["b"] = 2
				return p
			},
		},
	)
	out := b.Dial("l1", map[string]any{})
	if out["a"] != 1 || out["b"] != 2 {
		t.Errorf("chain failed: %v", out)
	}
}

func TestNewLazyEvaluatesPerCall(t *testing.T) {
	calls := 0
	b := NewLazy(func() []Rule {
		calls++
		return []Rule{
			{
				When: MatchLabel("x"),
				Patch: func(p map[string]any) map[string]any {
					p["k"] = calls
					return p
				},
			},
		}
	})
	out1 := b.Dial("x", map[string]any{})
	out2 := b.Dial("x", map[string]any{})
	if out1["k"] == out2["k"] {
		t.Errorf("lazy should evaluate per call, got %v both", out1["k"])
	}
}

func TestMatchLabel(t *testing.T) {
	if !MatchLabel("a")("a", nil) {
		t.Error("MatchLabel should match same label")
	}
	if MatchLabel("a")("b", nil) {
		t.Error("MatchLabel should not match different label")
	}
}

func TestDirectory(t *testing.T) {
	b := New(
		Rule{Name: "first", When: MatchLabel("a"), Patch: func(p map[string]any) map[string]any { return p }},
		Rule{Name: "second", When: MatchLabel("b"), Patch: func(p map[string]any) map[string]any { return p }},
	)
	d := b.Directory()
	if len(d) != 2 || d[0] != "first" || d[1] != "second" {
		t.Errorf("Directory = %v", d)
	}
}
