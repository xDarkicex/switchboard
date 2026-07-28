package synth

// Tags is the 8-dimension classification of a video generation prompt.
// Every tag is a string in a fixed vocabulary; the pipeline classifier
// predicts one value per dimension using the corresponding steady model.
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

// Dimensions is the canonical ordered list of tag dimensions.
var Dimensions = []string{
	"length", "complexity", "style", "quality",
	"camera", "physics", "refs", "cost",
}

// Labels returns the non-empty tag values in dimension:value format
// used for multi-label steady training output.
func (t Tags) Labels() []string {
	out := make([]string, 0, 8)
	if v := t.Length; v != "" {
		out = append(out, "length:"+v)
	}
	if v := t.Complexity; v != "" {
		out = append(out, "complexity:"+v)
	}
	if v := t.Style; v != "" {
		out = append(out, "style:"+v)
	}
	if v := t.Quality; v != "" {
		out = append(out, "quality:"+v)
	}
	if v := t.Camera; v != "" {
		out = append(out, "camera:"+v)
	}
	if v := t.Physics; v != "" {
		out = append(out, "physics:"+v)
	}
	if v := t.Refs; v != "" {
		out = append(out, "refs:"+v)
	}
	if v := t.Cost; v != "" {
		out = append(out, "cost:"+v)
	}
	return out
}

// Set is a convenience for filling Tags from a map.
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
