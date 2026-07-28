// Package board is the switchboard itself — the directory of stations and
// the catalog of trunk lines. It owns the YAML config, the provider→model
// hierarchy, the env-var wiring for secrets, and the cost model.
package board

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the assembled switchboard configuration.
type Config struct {
	Server    Server     `yaml:"server"`
	Pipeline  Pipeline   `yaml:"pipeline"`
	Routing   string     `yaml:"routing"`
	Providers []Provider `yaml:"providers"`
	Defaults  Defaults   `yaml:"defaults"`
}

// Server holds the listening socket settings.
type Server struct {
	Listen       string        `yaml:"listen"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

// Pipeline is the 8-classifier pipeline config. Each slot is one dimension.
type Pipeline struct {
	Length     PipelineSlot `yaml:"length"`
	Complexity PipelineSlot `yaml:"complexity"`
	Style      PipelineSlot `yaml:"style"`
	Quality    PipelineSlot `yaml:"quality"`
	Camera     PipelineSlot `yaml:"camera"`
	Physics    PipelineSlot `yaml:"physics"`
	Refs       PipelineSlot `yaml:"refs"`
	Cost       PipelineSlot `yaml:"cost"`
}

// PipelineSlot pairs a model file with the default value when OOD.
type PipelineSlot struct {
	ModelPath  string   `yaml:"model_path"`
	DefaultVal string   `yaml:"default"`
	Labels     []string `yaml:"labels"`
}

// Defaults holds fallback values used when no provider matches.
type Defaults struct {
	Provider string        `yaml:"provider"`
	Model    string        `yaml:"model"`
	Timeout  time.Duration `yaml:"timeout"`
}

// Provider is an upstream video generation service.
type Provider struct {
	Name    string  `yaml:"name"`
	BaseURL string  `yaml:"base_url"`
	Auth    Auth    `yaml:"auth"`
	Models  []Model `yaml:"models"`
}

// Auth describes how to authenticate with a provider.
type Auth struct {
	Type string `yaml:"type"` // "bearer" | "api_key" | "none"
	Env  string `yaml:"env"`  // env var name for the key/token
}

// Model is a specific model offering from a provider with cost and
// capability information. The routing engine uses these to select
// the cheapest capable model.
type Model struct {
	Name         string   `yaml:"name"`
	CostPerSec   float64  `yaml:"cost_per_sec"`
	CostPerReq   float64  `yaml:"cost_per_req"`
	MaxDuration  int      `yaml:"max_duration"`
	Capabilities []string `yaml:"capabilities"`
}

// LoadFile reads a YAML config from disk.
func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("board: read %q: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("board: parse %q: %w", path, err)
	}
	if c.Defaults.Timeout == 0 {
		c.Defaults.Timeout = 30 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 60 * time.Second
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 5 * time.Second
	}
	if c.Server.IdleTimeout == 0 {
		c.Server.IdleTimeout = 60 * time.Second
	}
	if c.Server.Listen == "" {
		c.Server.Listen = ":8080"
	}
	return c, nil
}

// Directory returns a human-readable listing of providers and models.
func (c *Config) Directory() []string {
	var out []string
	for _, p := range c.Providers {
		for _, m := range p.Models {
			out = append(out, fmt.Sprintf("%s/%s @ $%.4f/sec (max %ds) caps: %v",
				p.Name, m.Name, m.CostPerSec, m.MaxDuration, m.Capabilities))
		}
	}
	return out
}

// ResolveSecrets loads auth keys from the named env vars. Missing keys are
// silent — the banner handles display during startup.
func (c *Config) ResolveSecrets() map[string]string {
	secrets := map[string]string{}
	for _, p := range c.Providers {
		if p.Auth.Env == "" || p.Auth.Type == "none" {
			continue
		}
		v, ok := os.LookupEnv(p.Auth.Env)
		if !ok {
			continue
		}
		secrets[p.Name] = v
	}
	return secrets
}

// ProviderByName returns the provider with the given name, or nil.
func (c *Config) ProviderByName(name string) *Provider {
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			return &c.Providers[i]
		}
	}
	return nil
}

// ModelByName returns the model with the given name for the provider, or nil.
func (p *Provider) ModelByName(name string) *Model {
	for i := range p.Models {
		if p.Models[i].Name == name {
			return &p.Models[i]
		}
	}
	return nil
}

// EstCost estimates the cost for a given duration (seconds) and an optional
// flat per-request cost. Returns total in dollars.
func (m *Model) EstCost(durationSecs float64) float64 {
	return m.CostPerSec*durationSecs + m.CostPerReq
}

// AllPipelineSlots returns the pipeline slots in canonical order.
func (c *Config) AllPipelineSlots() []struct {
	Dim  string
	Slot PipelineSlot
} {
	return []struct {
		Dim  string
		Slot PipelineSlot
	}{
		{"length", c.Pipeline.Length},
		{"complexity", c.Pipeline.Complexity},
		{"style", c.Pipeline.Style},
		{"quality", c.Pipeline.Quality},
		{"camera", c.Pipeline.Camera},
		{"physics", c.Pipeline.Physics},
		{"refs", c.Pipeline.Refs},
		{"cost", c.Pipeline.Cost},
	}
}
