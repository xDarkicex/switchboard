// Package dimensions defines the Tags struct shared across the pipeline.
// Each tag is a string in a fixed vocabulary; the pipeline classifier
// predicts one value per dimension using the corresponding steady model.
package dimensions

// Tags is the 8-dimension classification of a video generation prompt.
type Tags struct {
	Length     string `yaml:"length"`
	Complexity string `yaml:"complexity"`
	Style      string `yaml:"style"`
	Quality    string `yaml:"quality"`
	Camera     string `yaml:"camera"`
	Physics    string `yaml:"physics"`
	Refs       string `yaml:"refs"`
	Cost       string `yaml:"cost"`
}

// All is the canonical ordered list of dimension names.
var All = []string{
	"length", "complexity", "style", "quality",
	"camera", "physics", "refs", "cost",
}

// Set sets a single dimension's value.
func (t *Tags) Set(dim, value string) {
	switch dim {
	case "length":
		t.Length = value
	case "complexity":
		t.Complexity = value
	case "style":
		t.Style = value
	case "quality":
		t.Quality = value
	case "camera":
		t.Camera = value
	case "physics":
		t.Physics = value
	case "refs":
		t.Refs = value
	case "cost":
		t.Cost = value
	}
}

// Get returns the value for a dimension, or "" if unset.
func (t Tags) Get(dim string) string {
	switch dim {
	case "length":
		return t.Length
	case "complexity":
		return t.Complexity
	case "style":
		return t.Style
	case "quality":
		return t.Quality
	case "camera":
		return t.Camera
	case "physics":
		return t.Physics
	case "refs":
		return t.Refs
	case "cost":
		return t.Cost
	}
	return ""
}

// Match returns true if the tags satisfy the partial match map. Missing
// dimensions are considered a match.
func (t Tags) Match(criteria map[string]string) bool {
	for dim, want := range criteria {
		if got := t.Get(dim); got != "" && got != want {
			return false
		}
	}
	return true
}

// Equal returns true if two Tags have the same non-empty values.
func (t Tags) Equal(other Tags) bool {
	for _, dim := range All {
		if t.Get(dim) != other.Get(dim) {
			return false
		}
	}
	return true
}
