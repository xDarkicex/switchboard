package synth

// Intent is a coherent combination of dimensions with templates and slots.
// The generator produces perIntent examples from each intent, all tagged with
// the intent's Tags.
type Intent struct {
	Name      string              `yaml:"name"`
	Tags      Tags                `yaml:"tags"`
	Templates []string            `yaml:"templates"`
	Slots     map[string][]string `yaml:"slots"`
}

// Example is a generated training example: text + tags.
type Example struct {
	Text string
	Tags Tags
}

// FromRealPrompt converts an annotated real prompt into an Example.
func FromRealPrompt(id, text string, tags Tags) Example {
	return Example{Text: text, Tags: tags}
}
