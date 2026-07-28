// scripts/import-vidprom/main.go converts VidProM's example CSV into
// our real_prompts.yaml format with auto-tags. Filters NSFW and very-short
// prompts. All tags are best-effort heuristics. Output is YAML-safe.
//
// Usage:
//   curl -L -o /tmp/vidprom_example.csv \
//     "https://huggingface.co/databases/WenhaoWang/VidProM/resolve/main/example/VidProM_unique_example.csv?download=true"
//   go run ./scripts/import-vidprom --input /tmp/vidprom_example.csv \
//     --output presets/video/vidprom_prompts.yaml
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/xDarkicex/switchboard/internal/dimensions"
)

func main() {
	input := flag.String("input", "/tmp/vidprom_example.csv", "Path to VidProM CSV file")
	output := flag.String("output", "presets/video/vidprom_prompts.yaml", "Output YAML path")
	maxN := flag.Int("max", 5000, "Maximum number of prompts to output")
	flag.Parse()

	rows, err := readCSV(*input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}

	out, err := os.Create(*output)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create:", err)
		os.Exit(1)
	}
	defer out.Close()

	fmt.Fprintf(out, "# Auto-tagged VidProM subset. License: CC BY-NC 4.0.\n")
	fmt.Fprintf(out, "# Filters: word count >= 6, toxicity < 0.3.\n")
	fmt.Fprintf(out, "prompts:\n")

	kept := 0
	for _, row := range rows {
		if kept >= *maxN {
			break
		}
		prompt := strings.TrimSpace(row.prompt)
		prompt = sanitizePrompt(prompt)
		toxicity := parseFloat(row.toxicity)
		words := strings.Fields(prompt)
		if len(words) < 6 || toxicity >= 0.3 {
			continue
		}
		tags := autoTag(prompt, words)
		fmt.Fprintf(out, "  - id: vidprom-%s\n", row.uuid[:8])
		fmt.Fprintf(out, "    source: \"vidprom-imported\"\n")
		// Write as YAML double-quoted string to avoid parser issues with
		// commas, colons, and raw symbols in real-world user prompts.
		fmt.Fprintf(out, "    text: \"%s\"\n", escapeYAMLString(prompt))
		fmt.Fprintf(out, "    tags:\n")
		fmt.Fprintf(out, "      length: %s\n", tags.Length)
		fmt.Fprintf(out, "      complexity: %s\n", tags.Complexity)
		fmt.Fprintf(out, "      style: %s\n", tags.Style)
		fmt.Fprintf(out, "      quality: %s\n", tags.Quality)
		fmt.Fprintf(out, "      camera: %s\n", tags.Camera)
		fmt.Fprintf(out, "      physics: %s\n", tags.Physics)
		fmt.Fprintf(out, "      refs: %s\n", tags.Refs)
		fmt.Fprintf(out, "      cost: %s\n", tags.Cost)
		kept++
	}

	fmt.Fprintf(os.Stderr, "imported %d prompts to %s\n", kept, *output)
}

type vidpromRow struct {
	uuid           string
	prompt         string
	time           string
	toxicity       string
	obscene        string
	identityAttack string
	insult         string
	threat         string
	sexualExplicit string
}

func readCSV(path string) ([]vidpromRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("csv has no rows")
	}
	header := records[0]
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[h] = i
	}
	out := make([]vidpromRow, 0, len(records)-1)
	for _, r := range records[1:] {
		out = append(out, vidpromRow{
			uuid:           get(r, idx, "uuid"),
			prompt:         get(r, idx, "prompt"),
			time:           get(r, idx, "time"),
			toxicity:       get(r, idx, "toxicity"),
			obscene:        get(r, idx, "obscene"),
			identityAttack: get(r, idx, "identity_attack"),
			insult:         get(r, idx, "insult"),
			threat:         get(r, idx, "threat"),
			sexualExplicit: get(r, idx, "sexual_explicit"),
		})
	}
	return out, nil
}

func get(row []string, idx map[string]int, key string) string {
	if i, ok := idx[key]; ok && i < len(row) {
		return row[i]
	}
	return ""
}

func parseFloat(s string) float64 {
	var f float64
	_, _ = fmt.Sscanf(s, "%f", &f)
	return f
}

func sanitizePrompt(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	return s
}

func escapeYAMLString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

func autoTag(prompt string, words []string) dimensions.Tags {
	tags := dimensions.Tags{
		Length: "short", Complexity: "simple", Style: "photorealistic",
		Quality: "basic", Camera: "static", Physics: "none",
		Refs: "none", Cost: "cheap",
	}
	switch {
	case len(words) >= 15:
		tags.Length = "long"
	case len(words) >= 6:
		tags.Length = "medium"
	}
	if containsTimeMarkers(prompt) {
		tags.Length = "multi_stage"
		tags.Complexity = "multi_stage"
	}
	if hasMultiClause(prompt) {
		tags.Complexity = "multi_subject"
	}
	low := strings.ToLower(prompt)
	switch {
	case hasAny(low, "cinematic", "film", "movie", "anamorphic", "arri", "imax"):
		tags.Style = "cinematic"
	case hasAny(low, "anime", "cel-shaded", "cel shaded", "ghibli", "stylized"):
		tags.Style = "animation"
	case hasAny(low, "3d", "cgi", "octane", "redshift", "unreal", "three.js"):
		tags.Style = "3d"
	case hasAny(low, "motion graphics", "infographic", "kinetic typography", "logo reveal"):
		tags.Style = "motion_graphics"
	}
	switch {
	case hasAny(low, "8k", "ultra hd", "uhd", "imax"):
		tags.Quality = "8k"
	case hasAny(low, "4k", "1080p", "high resolution"):
		tags.Quality = "4k"
	case hasAny(low, "production", "cinematic quality", "broadcast"):
		tags.Quality = "production-grade"
	}
	switch {
	case hasAny(low, "fpv", "first-person view", "first person view", "pov ", "drone"):
		tags.Camera = "fpv"
	case hasAny(low, "360", " orbit", "around"):
		tags.Camera = "orbital"
	case hasAny(low, "tracking", "panning", "follow", "dolly", "push-in", "push in"):
		tags.Camera = "tracking"
	case hasAny(low, "slow motion", "close-up", "close up", "macro"):
		tags.Camera = "dolly"
	}
	switch {
	case hasAny(low, "rain", "water", "liquid", "fluid", "wave", " ocean", "river"):
		tags.Physics = "fluid"
	case hasAny(low, "smoke", "fire", "flame", "explosion", "particle", "dust", "snow"):
		tags.Physics = "particle"
	case hasAny(low, "cloth", "fabric", "hair", "fur"):
		tags.Physics = "cloth"
	}
	if strings.Count(prompt, "@") >= 2 {
		tags.Refs = "multi"
	} else if strings.Contains(prompt, "@image") || strings.Contains(prompt, "@Image") {
		tags.Refs = "image"
	} else if strings.Contains(prompt, "@video") || strings.Contains(prompt, "@Video") {
		tags.Refs = "video"
	}
	if tags.Length == "long" || tags.Length == "multi_stage" || tags.Complexity == "multi_stage" {
		tags.Cost = "expensive"
	} else if tags.Length == "medium" || tags.Quality == "4k" || tags.Quality == "8k" || tags.Quality == "production-grade" {
		tags.Cost = "medium"
	}
	return tags
}

func containsTimeMarkers(p string) bool {
	for _, m := range []string{"0:00", "0:01", "0:02", "0:03", "0:04", "0:05", "[0s", "[1s", "[2s", "shot 1", "shot 2"} {
		if strings.Contains(p, m) {
			return true
		}
	}
	return false
}

func hasMultiClause(p string) bool {
	return strings.Count(p, ",")+strings.Count(p, " and ")+strings.Count(p, " while ") >= 2
}

func hasAny(s string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}
