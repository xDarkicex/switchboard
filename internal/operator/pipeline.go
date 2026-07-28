package operator

import (
	"fmt"

	"github.com/xDarkicex/steady"

	"github.com/xDarkicex/switchboard/internal/dimensions"
)

// Classifier wraps a single steady model trained for one dimension.
type Classifier struct {
	Dimension   string
	Model       *steady.Model
	DefaultVal  string
}

// Pipeline is the 8-classifier multi-dimension classifier. Each Classifier
// has its own mmap'd steady model. Classify returns a dimensions.Tags with
// one value per dimension.
type Pipeline struct {
	classifiers map[string]*Classifier
}

// PipelineConfig specifies the model paths and defaults for each dimension.
type PipelineConfig struct {
	Length      PipelineSlot `yaml:"length"`
	Complexity  PipelineSlot `yaml:"complexity"`
	Style       PipelineSlot `yaml:"style"`
	Quality     PipelineSlot `yaml:"quality"`
	Camera      PipelineSlot `yaml:"camera"`
	Physics     PipelineSlot `yaml:"physics"`
	Refs        PipelineSlot `yaml:"refs"`
	Cost        PipelineSlot `yaml:"cost"`
}

// PipelineSlot pairs a model file with the default value when OOD.
type PipelineSlot struct {
	ModelPath  string   `yaml:"model_path"`
	DefaultVal string   `yaml:"default"`
	Labels     []string `yaml:"labels"`
}

// NewPipeline loads all 8 dimension models and returns a ready-to-use
// Pipeline. Caller must call Close to release mmaps.
func NewPipeline(cfg PipelineConfig) (*Pipeline, error) {
	p := &Pipeline{classifiers: make(map[string]*Classifier, 8)}
	slots := []struct {
		dim string
		slot PipelineSlot
	}{
		{"length", cfg.Length},
		{"complexity", cfg.Complexity},
		{"style", cfg.Style},
		{"quality", cfg.Quality},
		{"camera", cfg.Camera},
		{"physics", cfg.Physics},
		{"refs", cfg.Refs},
		{"cost", cfg.Cost},
	}
	for _, s := range slots {
		if s.slot.ModelPath == "" {
			return nil, fmt.Errorf("pipeline: missing model_path for %s", s.dim)
		}
		model, err := steady.Load(s.slot.ModelPath)
		if err != nil {
			return nil, fmt.Errorf("pipeline: load %s: %w", s.dim, err)
		}
		if len(s.slot.Labels) > 0 {
			model.SetLabelNames(s.slot.Labels)
		}
		p.classifiers[s.dim] = &Classifier{
			Dimension:  s.dim,
			Model:      model,
			DefaultVal: s.slot.DefaultVal,
		}
	}
	return p, nil
}

// Close releases all classifier mmap'd models.
func (p *Pipeline) Close() error {
	var firstErr error
	for _, c := range p.classifiers {
		if err := c.Model.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Classify runs all 8 classifiers on the text and returns a Tags.
func (p *Pipeline) Classify(text string) (dimensions.Tags, error) {
	tags := dimensions.Tags{}
	for dim, c := range p.classifiers {
		ps := c.Model.Classify(text)
		if ps.IsEmpty() {
			tags.Set(dim, c.DefaultVal)
			continue
		}
		best := bestLabel(ps.Confidences)
		if best >= len(ps.Kinds) {
			tags.Set(dim, c.DefaultVal)
			continue
		}
		tags.Set(dim, ps.Kinds[best])
	}
	return tags, nil
}

// ClassifyDebug returns the full prediction set for a single dimension.
// Used for debugging routing decisions.
func (p *Pipeline) ClassifyDebug(dim, text string) (steady.PredictionSet, error) {
	c, ok := p.classifiers[dim]
	if !ok {
		return steady.PredictionSet{}, fmt.Errorf("pipeline: unknown dimension %q", dim)
	}
	return c.Model.Classify(text), nil
}
