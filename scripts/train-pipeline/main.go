// scripts/train-pipeline/main.go trains one steady model per dimension from
// the multi-label training data. Each model learns to predict one of the
// values for a dimension (e.g. length:short, length:medium, ...).
//
// Reads:    presets/video/training.txt (multi-label format)
// Writes:   models/<dimension>.bin (one per dimension)
//
// Run from the project root:
//
//	go run ./scripts/train-pipeline
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/xDarkicex/steady"
)

var dimensions = []string{
	"length", "complexity", "style", "quality",
	"camera", "physics", "refs", "cost",
}

const (
	trainingFile = "presets/video/training.txt"
	modelsDir    = "models"
)

func main() {
	// Group lines by dimension.
	byDim := make(map[string][]labeledText)
	for _, dim := range dimensions {
		byDim[dim] = nil
	}

	f, err := os.Open(trainingFile)
	if err != nil {
		log.Fatalf("open %s: %v", trainingFile, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	var totalLines int
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// Format: __label__<dim>:<value> <text...>
		const prefix = "__label__"
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		rest := line[len(prefix):]
		space := strings.IndexByte(rest, ' ')
		if space < 0 {
			continue
		}
		dimValue := rest[:space]
		text := strings.TrimSpace(rest[space+1:])
		if text == "" {
			continue
		}
		// Split dim:value
		colon := strings.IndexByte(dimValue, ':')
		if colon < 0 {
			continue
		}
		dim := dimValue[:colon]
		// Validate dimension and text not empty
		if !validDim(dim) || text == "" {
			continue
		}
		byDim[dim] = append(byDim[dim], labeledText{
			Label: dimValue,
			Text:  text,
		})
		totalLines++
	}
	if err := sc.Err(); err != nil {
		log.Fatalf("scan: %v", err)
	}

	fmt.Printf("scanned %d multi-label lines\n", totalLines)

	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", modelsDir, err)
	}

	// Train each dimension model.
	for _, dim := range dimensions {
		examples := byDim[dim]
		if len(examples) == 0 {
			fmt.Printf("  %s: 0 examples, skipping\n", dim)
			continue
		}

		// Use canonical label order matching the pipeline config. Steady maps
		// LabelNames[i] → index i during training; SetLabelNames must match.
		labels := canonicalLabels(dim)

		// Write a per-dimension training file so we can feed steady directly.
		dimTrainingFile := filepath.Join(modelsDir, dim+"_training.txt")
		if err := writePerDim(dimTrainingFile, examples); err != nil {
			log.Fatalf("write %s: %v", dimTrainingFile, err)
		}

		// Train steady model with optimized settings from steady's QUICKSTART:
		// bucket=2M, dim=64, epochs=20. These avoid byte n-gram hash collisions
		// and produce calibrated confidences at Q≈0.08.
		// Train with bucket=200K, dim=64. Large enough to avoid byte n-gram
		// hash collisions at our ~42K-example data volume, small enough to be
		// practical for shipping (~51MB per model, ~400MB total).
		sc := steady.DefaultTrainConfig()
		sc.Input = dimTrainingFile
		sc.Output = filepath.Join(modelsDir, dim+".bin")
		sc.Bucket = 200_000
		sc.Dim = 64
		sc.Epochs = 20
		sc.LR = 0.1
		sc.Alpha = 0.05
		sc.CalibSplit = 0.2
		sc.Seed = 42
		sc.LabelNames = labels

		fmt.Printf("  %s: %d examples, %d labels -> %s\n", dim, len(examples), len(labels), sc.Output)
		if err := steady.Train(sc); err != nil {
			log.Fatalf("train %s: %v", dim, err)
		}
	}

	fmt.Println("pipeline trained")
}

type labeledText struct {
	Label string
	Text  string
}

func validDim(dim string) bool {
	for _, d := range dimensions {
		if d == dim {
			return true
		}
	}
	return false
}

// canonicalLabels returns the label order for a dimension using the full
// dim:value format (e.g. "length:short"). Must match the order used in
// the pipeline config's labels field and SetLabelNames.
func canonicalLabels(dim string) []string {
	var values []string
	switch dim {
	case "length":
		values = []string{"short", "medium", "long", "multi_stage"}
	case "complexity":
		values = []string{"simple", "multi_subject", "multi_stage", "medium"}
	case "style":
		values = []string{"photorealistic", "cinematic", "animation", "3d", "motion_graphics"}
	case "quality":
		values = []string{"basic", "4k", "8k", "production-grade"}
	case "camera":
		values = []string{"static", "dolly", "tracking", "orbital", "fpv"}
	case "physics":
		values = []string{"none", "basic", "particle", "fluid", "cloth"}
	case "refs":
		values = []string{"none", "image", "video", "audio", "multi"}
	case "cost":
		values = []string{"cheap", "medium", "expensive"}
	}
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = dim + ":" + v
	}
	return out
}

func writePerDim(path string, examples []labeledText) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	for _, ex := range examples {
		if _, err := fmt.Fprintf(bw, "__label__%s %s\n", ex.Label, ex.Text); err != nil {
			return err
		}
	}
	return bw.Flush()
}
