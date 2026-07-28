// examples/basic/main.go is the smallest viable gateway. Loads the
// switchboard.yaml config, builds the pipeline + routing engine, classifies
// sample requests, and prints the routing decision without binding a port.
//
// Run from the project root:
//
//	go run ./examples/basic
//
// Expects models/<dimension>.bin trained by scripts/train-pipeline.
package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/xDarkicex/switchboard/internal/board"
	"github.com/xDarkicex/switchboard/internal/lines"
	"github.com/xDarkicex/switchboard/internal/operator"
)

func loadRoutingEngine(path string) (*lines.Engine, error) {
	if path == "" {
		return lines.NewEngine(nil), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var spec lines.EngineSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return lines.NewEngine(spec.Rules), nil
}

func main() {
	cfg, err := board.LoadFile("switchboard.yaml")
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}

	pipeCfg := operator.PipelineConfig{
		Length:     operator.PipelineSlot{ModelPath: cfg.Pipeline.Length.ModelPath, DefaultVal: cfg.Pipeline.Length.DefaultVal, Labels: cfg.Pipeline.Length.Labels},
		Complexity: operator.PipelineSlot{ModelPath: cfg.Pipeline.Complexity.ModelPath, DefaultVal: cfg.Pipeline.Complexity.DefaultVal, Labels: cfg.Pipeline.Complexity.Labels},
		Style:      operator.PipelineSlot{ModelPath: cfg.Pipeline.Style.ModelPath, DefaultVal: cfg.Pipeline.Style.DefaultVal, Labels: cfg.Pipeline.Style.Labels},
		Quality:    operator.PipelineSlot{ModelPath: cfg.Pipeline.Quality.ModelPath, DefaultVal: cfg.Pipeline.Quality.DefaultVal, Labels: cfg.Pipeline.Quality.Labels},
		Camera:     operator.PipelineSlot{ModelPath: cfg.Pipeline.Camera.ModelPath, DefaultVal: cfg.Pipeline.Camera.DefaultVal, Labels: cfg.Pipeline.Camera.Labels},
		Physics:    operator.PipelineSlot{ModelPath: cfg.Pipeline.Physics.ModelPath, DefaultVal: cfg.Pipeline.Physics.DefaultVal, Labels: cfg.Pipeline.Physics.Labels},
		Refs:       operator.PipelineSlot{ModelPath: cfg.Pipeline.Refs.ModelPath, DefaultVal: cfg.Pipeline.Refs.DefaultVal, Labels: cfg.Pipeline.Refs.Labels},
		Cost:       operator.PipelineSlot{ModelPath: cfg.Pipeline.Cost.ModelPath, DefaultVal: cfg.Pipeline.Cost.DefaultVal, Labels: cfg.Pipeline.Cost.Labels},
	}

	pipeline, err := operator.NewPipeline(pipeCfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pipeline:", err)
		os.Exit(1)
	}
	defer pipeline.Close()

	engine, err := loadRoutingEngine(cfg.Routing)
	if err != nil {
		fmt.Fprintln(os.Stderr, "routing:", err)
		os.Exit(1)
	}

	queries := []string{
		"Make a video of sunset over the ocean",
		"Create a video of a robot cooking pasta in a kitchen",
		"Generate a Pixar-style animation of a curious cat",
		"Write a poem about gravity",
		"Make a 5-minute cinematic video of an epic battle scene",
		"Continuous 10-second shot. [0:00-0:03] Wide shot of a cyberpunk alley. [0:03-0:06] A figure sprints across rooftops. [0:06-0:10] The figure dives through a window.",
	}

	fmt.Println("switchboard: pipeline + routing log")
	fmt.Println("==================================")
	for _, q := range queries {
		tags, err := pipeline.Classify(q)
		if err != nil {
			fmt.Printf("  %q -> error: %v\n", q, err)
			continue
		}
		provider, model := engine.Route(tags)
		fmt.Printf("  %q\n", q)
		fmt.Printf("    tags: length=%s style=%s quality=%s cost=%s camera=%s\n",
			tags.Length, tags.Style, tags.Quality, tags.Cost, tags.Camera)
		fmt.Printf("    -> %s/%s\n", provider, model)
	}

	fmt.Println()
	for _, line := range cfg.Directory() {
		fmt.Printf("  %s\n", line)
	}
}
