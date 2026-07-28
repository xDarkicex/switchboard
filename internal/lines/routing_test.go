package lines

import (
	"testing"

	"github.com/xDarkicex/switchboard/internal/dimensions"
)

func makeSpec(name, provider, model string, match map[string][]string, matchAny []map[string][]string) ruleSpec {
	return ruleSpec{Name: name, Provider: provider, Model: model, Match: match, MatchAny: matchAny}
}

func TestRoutingExactMatch(t *testing.T) {
	e := NewEngine([]ruleSpec{
		makeSpec("sora", "openai", "sora", map[string][]string{
			"length": {"multi_stage"}, "refs": {"multi"},
		}, nil),
		makeSpec("fallback", "runway", "gen-3", nil, nil),
	})
	p, m := e.Route(dimensions.Tags{Length: "multi_stage", Refs: "multi"})
	if p != "openai" || m != "sora" {
		t.Errorf("Route = (%q, %q), want (openai, sora)", p, m)
	}
}

func TestRoutingPartialMatchMisses(t *testing.T) {
	e := NewEngine([]ruleSpec{
		makeSpec("sora", "openai", "sora", map[string][]string{
			"length": {"multi_stage"},
		}, nil),
		makeSpec("fallback", "runway", "gen-3", nil, nil),
	})
	p, m := e.Route(dimensions.Tags{Length: "short"})
	if p != "runway" || m != "gen-3" {
		t.Errorf("Route = (%q, %q), want (runway, gen-3)", p, m)
	}
}

func TestRoutingMatchAny(t *testing.T) {
	e := NewEngine([]ruleSpec{
		makeSpec("sora", "openai", "sora", nil, []map[string][]string{
			{"length": {"multi_stage"}},
			{"refs": {"multi"}},
		}),
		makeSpec("fallback", "runway", "gen-3", nil, nil),
	})
	if p, _ := e.Route(dimensions.Tags{Length: "multi_stage"}); p != "openai" {
		t.Errorf("len match: %q", p)
	}
	if p, _ := e.Route(dimensions.Tags{Refs: "multi"}); p != "openai" {
		t.Errorf("refs match: %q", p)
	}
	if p, _ := e.Route(dimensions.Tags{Length: "short"}); p != "runway" {
		t.Errorf("no match: %q", p)
	}
}

func TestRoutingEmptyRuleIsFallback(t *testing.T) {
	e := NewEngine([]ruleSpec{
		makeSpec("specific", "openai", "sora", map[string][]string{
			"length": {"multi_stage"},
		}, nil),
		makeSpec("fallback", "runway", "gen-3", nil, nil),
	})
	if p, _ := e.Route(dimensions.Tags{}); p != "runway" {
		t.Errorf("empty tags = %q, want runway", p)
	}
}

func TestRoutingNoMatchReturnsEmpty(t *testing.T) {
	// Engine with only specific rules and no fallback (empty Match).
	e := NewEngine([]ruleSpec{
		{Name: "specific", Provider: "openai", Model: "sora",
			Match: map[string][]string{"length": {"multi_stage"}},
		},
	})
	p, m := e.Route(dimensions.Tags{Length: "short"})
	if p != "" || m != "" {
		t.Errorf("Route = (%q, %q), want empty", p, m)
	}
}

func TestRoutingAllAndAny(t *testing.T) {
	e := NewEngine([]ruleSpec{
		makeSpec("combo", "openai", "sora", map[string][]string{
			"cost": {"expensive"},
		}, []map[string][]string{
			{"length": {"multi_stage"}},
			{"refs": {"multi", "video"}},
		}),
		makeSpec("fallback", "runway", "gen-3", nil, nil),
	})
	// Both AND and any-of match.
	if p, _ := e.Route(dimensions.Tags{Cost: "expensive", Length: "multi_stage"}); p != "openai" {
		t.Errorf("both match: got %q", p)
	}
	// AND misses.
	if p, _ := e.Route(dimensions.Tags{Cost: "cheap", Refs: "multi"}); p != "runway" {
		t.Errorf("match miss: got %q, want runway", p)
	}
}
