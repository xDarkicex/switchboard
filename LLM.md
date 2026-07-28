# switchboard — LLM Reference

## Purpose

A zero-allocation, mmap-backed AI proxy that classifies video generation
prompts across 8 orthogonal dimensions using a steady classifier pipeline,
then routes to the cheapest capable provider using boolean logic rules.

Total routing overhead: ~2ms (8 classifiers × ~260µs each).
Upstream latency: 5–120 seconds. The proxy is invisible.

## Build

```sh
go build -o switchboard ./cmd/switchboard
```

Requires Go 1.25.7. All dependencies are pure Go — no CGo, no Python.

```sh
# Fetch pre-trained models (408 MB total, 8 files × 51 MB)
hf download LibraVDB/switchboard-video-v1 --local-dir ./models
```

## Architecture

```
POST /v1/chat/completions
  │
  ├─ nanite middleware → extract user message from JSON body
  │
  ├─ operator.Pipeline.Classify(text) → dimensions.Tags
  │   │
  │   ├─ length.bin     → short | medium | long | multi_stage
  │   ├─ complexity.bin → simple | multi_subject | multi_stage
  │   ├─ style.bin      → photorealistic | cinematic | animation | 3d | motion_graphics
  │   ├─ quality.bin    → basic | 4k | 8k | production-grade
  │   ├─ camera.bin     → static | dolly | tracking | orbital | fpv
  │   ├─ physics.bin    → none | basic | particle | fluid | cloth
  │   ├─ refs.bin       → none | image | video | audio | multi
  │   └─ cost.bin       → cheap | medium | expensive
  │
  ├─ lines.Engine.Route(tags) → (provider, model)
  │   Rules compiled from routing.yaml via logic.Evaluator chains.
  │   AND/OR boolean predicates. First match wins.
  │
  └─ plug.In(tieLine, r) → upstream HTTP proxy
      └─ SSE via nanite/sse.Connection.SendBytes() + ResponseController
```

## Classification pipeline

Each model is a steady byteSteady classifier: byte n-gram hashing → OVA
logistic head → Platt scaling → conformal prediction.

- Embedding table: 200,000 rows × 64 dim = 51 MB per model
- Training data: 42,731 examples × 8 labels = 341,848 lines
- Model format: .bin (mmap'd at load, only accessed pages in RAM)
- Label ordering: canonical order in `scripts/train-pipeline/main.go` → `canonicalLabels()`. Must match `switchboard.yaml` pipeline labels exactly.

## Routing engine

Compiled from `routing.yaml` at startup. Each rule's `match` (AND) and
`match_any` (OR) become `func(Tags) bool` predicates. The `logic.Evaluator`
package provides boolean composition.

```go
// internal/lines/routing.go
func compile(match map[string][]string, matchAny []map[string][]string) func(Tags) bool
```

## Package map

| Package | Role | Key types |
|---|---|---|
| `switchboard` | Public API | `Server`, `Patching`, `NewFromFiles()` |
| `internal/board` | Config + secrets | `Config`, `Provider`, `Model`, `LoadFile()` |
| `internal/dimensions` | Shared types | `Tags` (8 string fields), `Match()` |
| `internal/lines` | Routing engine | `Engine`, `ruleSpec`, `Route()` |
| `internal/operator` | Classifier pipeline | `Pipeline`, `PipelineSlot`, `Classify()` |
| `internal/plug` | Upstream proxy + SSE | `TieLine`, `Cord`, `StreamSSE()`, `In()` |
| `internal/synth` | Training data generator | `Generator`, `Intent`, `Preset`, `WriteMultiLabel()` |
| `internal/trunk` | Call rejection | `Drop()`, `NumberChanged()` |

## Training

```sh
# Step 1: Generate multi-label training file
./switchboard synth \
  --preset ./presets/video/synth.yaml \
  --real ./presets/video/real_prompts.yaml \
  --per-intent 1500 \
  --output ./presets/video/training.txt

# Step 2: Train 8 dimension models
go run ./scripts/train-pipeline
```

Training hyperparameters (in `scripts/train-pipeline/main.go`):
`bucket=200_000`, `dim=64`, `epochs=20`, `lr=0.1`, `alpha=0.05`, `calib_split=0.2`

Training data format (multi-label, one line per dimension:value):
```
__label__length:short Make a video of a sunset
__label__style:photorealistic Make a video of a sunset
__label__cost:cheap Make a video of a sunset
```

The same text appears under 8 labels (one per dimension). The train-pipeline
script slices this per dimension before feeding to steady.Train.

## Config format

### switchboard.yaml

```yaml
server:    { listen, read_timeout, write_timeout }
pipeline:  { length, complexity, style, quality, camera, physics, refs, cost }
           Each: { model_path, default, labels }
routing:   path to routing.yaml
providers: [{ name, base_url, auth: { type, env }, models: [{ name, cost_per_sec, capabilities }] }]
defaults:  { provider, model, timeout }
```

### routing.yaml

```yaml
rules:
  - name: string
    match:     { dimension: [values...] }    # AND
    match_any: [{ dimension: [values...] }]  # OR
    provider:  string
    model:     string
```

## Dependencies (all tagged)

| Module | Version | Use |
|---|---|---|
| `github.com/xDarkicex/nanite` | v0.5.7 | HTTP router |
| `github.com/xDarkicex/nanite/sse` | v0.0.3 | SSE streaming |
| `github.com/xDarkicex/steady` | v0.1.3 | ByteSteady classifier |
| `github.com/xDarkicex/logic` | v0.1.10 | Boolean logic for routing |
| `github.com/xDarkicex/memory` | v1.2.8 | Off-heap memory (transitive) |
| `github.com/spf13/cobra` | v1.10.2 | CLI framework |

## Model artifacts (Hugging Face)

```
LibraVDB/switchboard-video-v1
├── length.bin (51 MB)
├── complexity.bin (51 MB)
├── style.bin (51 MB)
├── quality.bin (51 MB)
├── camera.bin (51 MB)
├── physics.bin (51 MB)
├── refs.bin (51 MB)
└── cost.bin (51 MB)
```

Download: `hf download LibraVDB/switchboard-video-v1 --local-dir ./models`
