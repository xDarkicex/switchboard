package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/xDarkicex/switchboard/internal/synth"
)

var trainCmd = &cobra.Command{
	Use:   "train",
	Short: "Train the 8-classifier pipeline from intents + real prompts.",
	Long: `Two-step pipeline training:
  1. Run the synth generator against the preset to produce multi-label
     training.txt (combining intent examples and any real prompts).
  2. Run scripts/train-pipeline to train one steady model per dimension,
     producing models/<dimension>.bin for each of the 8 dimensions.

The single training.txt file contains one label per dimension:value pair.
The pipeline script slices it per dimension and trains each in turn.`,
	RunE: runTrain,
}

var (
	flagTrainPreset   string
	flagTrainReal     string
	flagTrainPerIntent int
	flagTrainOutput   string
	flagTrainSeed     int64
)

func init() {
	rootCmd.AddCommand(trainCmd)
	flagTrainPreset = "./presets/video/synth.yaml"
	flagTrainReal = "./presets/video/real_prompts.yaml"
	flagTrainPerIntent = 1500
	flagTrainOutput = "./presets/video/training.txt"
	flagTrainSeed = 42

	trainCmd.Flags().StringVar(&flagTrainPreset, "preset", flagTrainPreset, "Path to the synth preset YAML")
	trainCmd.Flags().StringVar(&flagTrainReal, "real", flagTrainReal, "Path to the real_prompts.yaml corpus")
	trainCmd.Flags().IntVar(&flagTrainPerIntent, "per-intent", flagTrainPerIntent, "Number of synthetic examples per intent")
	trainCmd.Flags().StringVar(&flagTrainOutput, "output", flagTrainOutput, "Path to write the multi-label training data")
	trainCmd.Flags().Int64Var(&flagTrainSeed, "seed", flagTrainSeed, "Random seed")
}

func runTrain(cmd *cobra.Command, _ []string) error {
	// Step 1: run the synth to produce multi-label training data.
	fmt.Fprintln(cmd.ErrOrStderr(), "train: generating multi-label training data...")
	if err := runSynthForTrain(cmd); err != nil {
		return err
	}

	// Step 2: invoke the train-pipeline script.
	fmt.Fprintln(cmd.ErrOrStderr(), "train: training 8 dimension models...")
	root, err := projectRoot()
	if err != nil {
		return err
	}
	tp := exec.Command("go", "run", filepath.Join(root, "scripts/train-pipeline"))
	tp.Stdout = cmd.OutOrStdout()
	tp.Stderr = cmd.ErrOrStderr()
	if err := tp.Run(); err != nil {
		return fmt.Errorf("train: pipeline failed: %w", err)
	}
	return nil
}

func runSynthForTrain(cmd *cobra.Command) error {
	examples, err := gatherTrainExamples(cmd)
	if err != nil {
		return err
	}
	return writeTrainExamples(cmd, examples)
}

func gatherTrainExamples(cmd *cobra.Command) ([]synth.Example, error) {
	preset, err := synth.LoadPresetFile(flagTrainPreset)
	if err != nil {
		return nil, err
	}
	var examples []synth.Example
	if len(preset.Intents) > 0 {
		examples, err = addTrainSynthetic(cmd, preset)
		if err != nil {
			return nil, err
		}
	}
	if flagTrainReal != "" {
		examples = addTrainReal(cmd, examples)
	}
	return examples, nil
}

func addTrainSynthetic(cmd *cobra.Command, preset synth.Preset) ([]synth.Example, error) {
	seed := flagTrainSeed
	if preset.Seed != 0 {
		seed = preset.Seed
	}
	gen := synth.New(seed)
	syn, err := gen.GenerateIntents(preset.Intents, flagTrainPerIntent)
	if err != nil {
		return nil, fmt.Errorf("train: synth: %w", err)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "train: %d synthetic examples\n", len(syn))
	return syn, nil
}

func addTrainReal(cmd *cobra.Command, examples []synth.Example) []synth.Example {
	if _, err := os.Stat(flagTrainReal); err != nil {
		return examples
	}
	real, err := synth.LoadRealPromptsFile(flagTrainReal)
	if err != nil {
		return examples
	}
	for _, r := range real {
		examples = append(examples, r.ToExample())
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "train: %d real prompts\n", len(real))
	return examples
}

func writeTrainExamples(cmd *cobra.Command, examples []synth.Example) error {
	if err := os.MkdirAll(filepath.Dir(flagTrainOutput), 0o755); err != nil {
		return err
	}
	out, err := os.Create(flagTrainOutput)
	if err != nil {
		return err
	}
	defer out.Close()
	gen := synth.New(flagTrainSeed)
	if err := gen.WriteMultiLabel(out, examples); err != nil {
		return err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "train: wrote %d examples (%d lines) to %s\n", len(examples), len(examples)*8, flagTrainOutput)
	return nil
}

// projectRoot returns the absolute path to the switchboard project root by
// walking up from the current working directory.
func projectRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", wd)
		}
		dir = parent
	}
}
