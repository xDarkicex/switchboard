package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xDarkicex/steady"
)

var classifyCmd = &cobra.Command{
	Use:   "classify",
	Short: "Classify a single query against a trained model.",
	Long: `Loads the trained model, reads a query (from the argument or stdin),
and prints the chosen label and confidences. For debugging routing decisions
in production.`,
	RunE: runClassify,
}

var (
	flagClassifyModel  string
	flagClassifyLabels string
	flagClassifyQuery  string
)

func init() {
	rootCmd.AddCommand(classifyCmd)
	classifyCmd.Flags().StringVar(&flagClassifyModel, "model", "./models/video.bin", "Path to the trained model .bin")
	classifyCmd.Flags().StringVar(&flagClassifyLabels, "labels", "simple_short,complex_medium,high_quality_long,animation_style,realistic_style,3d_cgi,motion_graphics,fallback", "Comma-separated label names")
	classifyCmd.Flags().StringVar(&flagClassifyQuery, "query", "", "Query to classify (default: stdin)")
}

func runClassify(cmd *cobra.Command, _ []string) error {
	model, err := steady.Load(flagClassifyModel)
	if err != nil {
		return err
	}
	defer model.Close()
	labels := strings.Split(flagClassifyLabels, ",")
	model.SetLabelNames(labels)

	query := strings.TrimSpace(flagClassifyQuery)
	if query == "" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		query = strings.TrimSpace(string(data))
	}
	if query == "" {
		return fmt.Errorf("classify: no query provided (use --query or pipe on stdin)")
	}

	ps := model.Classify(query)
	if ps.IsEmpty() {
		fmt.Fprintln(cmd.OutOrStdout(), "no prediction (out of distribution)")
		return nil
	}
	for i, kind := range ps.Kinds {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%.4f\n", kind, ps.Confidences[i])
	}
	return nil
}
