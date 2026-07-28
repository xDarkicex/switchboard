// Package lines routes calls using boolean logic rules compiled from a YAML
// configuration. Each rule expresses constraints on the pipeline's dimension
// tags using AND/OR semantics, matched via the logic.Evaluator API.
//
// YAML rules look like:
//
//	rules:
//	  - name: sora_for_expensive_multimodal
//	    match:                                  # AND — all must hold
//	      style: [cinematic, photorealistic]
//	    match_any:                              # OR — any one holds
//	      - refs: [video, multi]
//	      - length: [multi_stage, long]
//	    backend: sora
//	    params: { duration: 60, quality: ultra }
//
// The Evaluator then computes: match AND match_any → boolean.
// Rules are evaluated in order; first match wins.
package lines

import (
	"github.com/xDarkicex/logic"
	"github.com/xDarkicex/switchboard/internal/dimensions"
)

// RoutingRule is a single routing decision compiled from YAML. The Match
// function is a pre-compiled evaluator chain from the logic package.
type RoutingRule struct {
	Name    string
	Match   func(dimensions.Tags) bool
	Provider string
	Model    string
}

// Engine is an ordered list of routing rules. First match wins.
type Engine struct {
	Rules []RoutingRule
}

// ruleSpec is the YAML-decoded form of a routing rule. The compiled Match
// function is built in NewEngine.
type ruleSpec struct {
	Name     string                `yaml:"name"`
	Match    map[string][]string   `yaml:"match"`
	MatchAny []map[string][]string `yaml:"match_any"`
	Provider string                `yaml:"provider"`
	Model    string                `yaml:"model"`
}

// EngineSpec is the YAML wrapper for an engine.
type EngineSpec struct {
	Rules []ruleSpec `yaml:"rules"`
}

// RoutingResult holds the routing decision.
type RoutingResult struct {
	Provider string
	Model    string
}

// NewEngine compiles ruleSpecs into a ready-to-use engine.
func NewEngine(specs []ruleSpec) *Engine {
	rules := make([]RoutingRule, 0, len(specs))
	for _, s := range specs {
		rules = append(rules, RoutingRule{
			Name:     s.Name,
			Match:    compile(s.Match, s.MatchAny),
			Provider: s.Provider,
			Model:    s.Model,
		})
	}
	return &Engine{Rules: rules}
}

// compile builds a single Match function from a rule spec. The match
// (AND) and match_any (OR) clauses are combined: if both are empty, always
// matches (default/fallback rule). If only one is set, that clause alone.
// If both are set, they are AND'd together.
func compile(match map[string][]string, matchAny []map[string][]string) func(dimensions.Tags) bool {
	all := buildAllOf(match)
	any := buildAnyOf(matchAny)
	switch {
	case all == nil && any == nil:
		return func(_ dimensions.Tags) bool { return true }
	case all == nil:
		return any
	case any == nil:
		return all
	default:
		return func(t dimensions.Tags) bool {
			return logic.Eval(all(t)).And(any(t)).Result()
		}
	}
}

// buildAllOf compiles the match field into a function. Each key is a
// dimension, each value is an allowed set. Tags must satisfy all
// criteria.
func buildAllOf(match map[string][]string) func(dimensions.Tags) bool {
	if len(match) == 0 {
		return nil
	}
	checks := make([]func(dimensions.Tags) bool, 0, len(match))
	for dim, values := range match {
		v := dim
		s := makeSet(values)
		checks = append(checks, func(t dimensions.Tags) bool {
			return s[t.Get(v)]
		})
	}
	return func(t dimensions.Tags) bool {
		for _, check := range checks {
			if !check(t) {
				return false
			}
		}
		return true
	}
}

// buildAnyOf compiles the match_any field into a function. Each element
// is a match-style map. Tags must satisfy at least one.
func buildAnyOf(matchAny []map[string][]string) func(dimensions.Tags) bool {
	if len(matchAny) == 0 {
		return nil
	}
	options := make([]func(dimensions.Tags) bool, len(matchAny))
	for i, m := range matchAny {
		options[i] = buildAllOf(m)
	}
	return func(t dimensions.Tags) bool {
		for _, opt := range options {
			if opt == nil || opt(t) {
				return true
			}
		}
		return false
	}
}

// Route returns the provider and model from the first matching rule.
// Returns empty strings if no rule matches.
func (e *Engine) Route(tags dimensions.Tags) (provider, model string) {
	for _, r := range e.Rules {
		if r.Match(tags) {
			return r.Provider, r.Model
		}
	}
	return "", ""
}

// makeSet creates a lookup set from a string slice.
func makeSet(values []string) map[string]bool {
	s := make(map[string]bool, len(values))
	for _, v := range values {
		s[v] = true
	}
	return s
}
