// scripts/train/main.go trains the steady model on the synthetic video
// training data. Run it once:
//
//	go run ./scripts/train
//
// Writes models/video.bin. Lives outside cmd/ so it's a script, not a
// shipped binary.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/xDarkicex/steady"
)

func main() {
	root, err := projectRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "locate project root:", err)
		os.Exit(1)
	}
	if err := os.Chdir(root); err != nil {
		fmt.Fprintln(os.Stderr, "chdir:", err)
		os.Exit(1)
	}

	cfg := steady.DefaultTrainConfig()
	cfg.Input = "presets/video/training.txt"
	cfg.Output = "models/video.bin"
	cfg.Bucket = 20000
	cfg.Dim = 128
	cfg.Epochs = 100
	cfg.LR = 0.1
	cfg.Alpha = 0.01
	cfg.CalibSplit = 0.2
	cfg.Seed = 42
	cfg.LabelNames = []string{
		"simple_short",
		"complex_medium",
		"high_quality_long",
		"animation_style",
		"realistic_style",
		"3d_cgi",
		"motion_graphics",
		"fallback",
	}
	if err := steady.Train(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "train:", err)
		os.Exit(1)
	}
	fmt.Println("trained ->", cfg.Output)
}

// projectRoot returns the switchboard project root by walking up from this
// file's location. scripts/train/main.go -> ../../.
func projectRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot determine caller location")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")), nil
}
