package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/xDarkicex/switchboard/internal/synth"
)

var synthCmd = &cobra.Command{
	Use:   "synth",
	Short: "Generate synthetic training data for the steady pipeline.",
	Long: `Loads a preset (intents + vocabulary) and one or more real-prompt
corpora, generates examples, and emits multi-label steady training data.

Output format: one line per (text, dimension:value) pair. Each text appears
under 8 labels (one per dimension), so the pipeline can train 8 models
from a single file.`,
	RunE: runSynth,
}

var (
	flagSynthPreset    string
	flagSynthRealFiles []string
	flagSynthPerIntent  int
	flagSynthOutput    string
	flagSynthSeed      int64
)

func init() {
	rootCmd.AddCommand(synthCmd)
	synthCmd.Flags().StringVar(&flagSynthPreset, "preset", "", "Path to the synth preset YAML (required)")
	synthCmd.Flags().StringSliceVar(&flagSynthRealFiles, "real", nil, "Path(s) to real_prompts.yaml corpora (can be passed multiple times)")
	synthCmd.Flags().IntVar(&flagSynthPerIntent, "per-intent", 1000, "Number of synthetic examples per intent")
	synthCmd.Flags().StringVar(&flagSynthOutput, "output", "", "Path to write the multi-label training data (required)")
	synthCmd.Flags().Int64Var(&flagSynthSeed, "seed", 42, "Random seed")
	_ = synthCmd.MarkFlagRequired("preset")
	_ = synthCmd.MarkFlagRequired("output")
}

func runSynth(cmd *cobra.Command, _ []string) error {
	examples, err := gatherExamples(cmd)
	if err != nil {
		return err
	}
	return writeSynthOutput(cmd, examples)
}

func gatherExamples(cmd *cobra.Command) ([]synth.Example, error) {
	preset, err := synth.LoadPresetFile(flagSynthPreset)
	if err != nil {
		return nil, err
	}
	var examples []synth.Example
	if len(preset.Intents) > 0 {
		examples, err = addSynthetic(cmd, preset, examples)
		if err != nil {
			return nil, err
		}
	}
	return addRealExamples(cmd, examples)
}

func addSynthetic(cmd *cobra.Command, preset synth.Preset, examples []synth.Example) ([]synth.Example, error) {
	seed := flagSynthSeed
	if preset.Seed != 0 {
		seed = preset.Seed
	}
	gen := synth.New(seed)
	syn, err := gen.GenerateIntents(preset.Intents, flagSynthPerIntent)
	if err != nil {
		return nil, fmt.Errorf("synth: generate intents: %w", err)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "synth: generated %d synthetic examples (%d intents x %d)\n", len(syn), len(preset.Intents), flagSynthPerIntent)
	return append(examples, syn...), nil
}

func addRealExamples(cmd *cobra.Command, examples []synth.Example) ([]synth.Example, error) {
	for _, path := range flagSynthRealFiles {
		real, err := synth.LoadRealPromptsFile(path)
		if err != nil {
			return nil, fmt.Errorf("synth: load real %q: %w", path, err)
		}
		for _, r := range real {
			examples = append(examples, r.ToExample())
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "synth: loaded %d real prompts from %s\n", len(real), path)
	}
	return examples, nil
}

func writeSynthOutput(cmd *cobra.Command, examples []synth.Example) error {
	if len(examples) == 0 {
		return fmt.Errorf("synth: no examples to write (check preset has intents and/or pass --real)")
	}
	if err := os.MkdirAll(filepath.Dir(flagSynthOutput), 0o755); err != nil {
		return fmt.Errorf("synth: mkdir: %w", err)
	}
	out, err := os.Create(flagSynthOutput)
	if err != nil {
		return fmt.Errorf("synth: create output: %w", err)
	}
	defer out.Close()
	gen := synth.New(flagSynthSeed)
	if err := gen.WriteMultiLabel(out, examples); err != nil {
		return err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "synth: wrote %d examples (%d multi-label lines) to %s\n", len(examples), len(examples)*8, flagSynthOutput)
	return nil
}
