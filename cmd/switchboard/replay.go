package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xDarkicex/steady"
)

var replayCmd = &cobra.Command{
	Use:   "replay",
	Short: "Replay a captured request log against a trained model.",
	Long: `Reads a JSONL file of {"query": "...", "expected": "label"} records,
classifies each one, and reports the actual vs. expected label. Useful for
benchmarking classifier accuracy against held-out traffic.`,
	RunE: runReplay,
}

var (
	flagReplayModel  string
	flagReplayLabels string
	flagReplayInput  string
)

func init() {
	rootCmd.AddCommand(replayCmd)
	replayCmd.Flags().StringVar(&flagReplayModel, "model", "./models/video.bin", "Path to the trained model .bin")
	replayCmd.Flags().StringVar(&flagReplayLabels, "labels", "simple_short,complex_medium,high_quality_long,animation_style,realistic_style,3d_cgi,motion_graphics,fallback", "Comma-separated label names")
	replayCmd.Flags().StringVar(&flagReplayInput, "input", "", "Path to the JSONL request log (required)")
	_ = replayCmd.MarkFlagRequired("input")
}

type replayRecord struct {
	Query    string `json:"query"`
	Expected string `json:"expected"`
}

func runReplay(cmd *cobra.Command, _ []string) error {
	model, err := steady.Load(flagReplayModel)
	if err != nil {
		return err
	}
	defer model.Close()
	labels := strings.Split(flagReplayLabels, ",")
	model.SetLabelNames(labels)

	f, err := os.Open(flagReplayInput)
	if err != nil {
		return err
	}
	defer f.Close()

	total, correct, ood := scanReplay(cmd, f, model)
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "\nreplay: %d total, %d correct, %d OOD, accuracy %.1f%%\n", total, correct, ood, pct(correct, total))
	return nil
}

// scanReplay reads the log line-by-line, classifies each record, and writes
// per-record results to cmd's stdout. Returns totals.
func scanReplay(cmd *cobra.Command, r io.Reader, model *steady.Model) (total, correct, ood int) {
	sc := bufio.NewScanner(r)
	replayLine(cmd, sc, model, &total, &correct, &ood)
	return total, correct, ood
}

func replayLine(cmd *cobra.Command, sc *bufio.Scanner, model *steady.Model, total, correct, ood *int) {
	for sc.Scan() {
		var rec replayRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			continue
		}
		*total++
		got, isOOD := classifyRecord(model, rec.Query)
		if isOOD {
			*ood++
			fmt.Fprintf(cmd.OutOrStdout(), "Q: %q -> OOD (expected %q)\n", rec.Query, rec.Expected)
			continue
		}
		hit := got == rec.Expected
		if hit {
			*correct++
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Q: %q -> %s (expected %s) %s\n", rec.Query, got, rec.Expected, hitMark(hit))
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "scan: %v\n", err)
	}
}

// classifyRecord classifies a single query and returns the top label plus
// whether the prediction was empty (out of distribution).
func classifyRecord(model *steady.Model, query string) (string, bool) {
	ps := model.Classify(query)
	if ps.IsEmpty() {
		return "", true
	}
	best := 0
	for i, c := range ps.Confidences {
		if c > ps.Confidences[best] {
			best = i
		}
	}
	return ps.Kinds[best], false
}

func hitMark(ok bool) string {
	if ok {
		return "OK"
	}
	return "MISS"
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(n) / float64(total)
}
