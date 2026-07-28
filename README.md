<p align="center">
  <img src="https://img.shields.io/badge/go-1.25.7-00ADD8?style=flat-square&logo=go" alt="Go 1.25.7">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="MIT License">
  <img src="https://img.shields.io/badge/classifier-steady-8A2BE2?style=flat-square" alt="steady classifier">
  <img src="https://img.shields.io/badge/router-nanite-FF6B35?style=flat-square" alt="nanite router">
</p>

# switchboard

**A zero-alloc, mmap-backed, sub-millisecond AI proxy engine.**

Classify. Tag. Route. Patch.

`switchboard` listens on the wire, classifies the incoming request with a
pre-trained byteSteady model pipeline, and patches the call through to the
right upstream backend — all in pure Go, no Python, no CGo.

Built on the [steady](https://github.com/xDarkicex/steady) classification
framework and [nanite](https://github.com/xDarkicex/nanite) HTTP router. The
design carries a vintage switchboard aesthetic: every internal package is named
after a piece of the operator's equipment. The code is fast, the names are fun.

<br>

---

<p align="center">
  <b>A tool provided by</b><br>
  <a href="https://libravdb.com"><b>libravdb.com</b></a><br>
  <sub>part of the Libra agentic tools ecosystem</sub>
</p>

<p align="center">
  <b>MIT License</b> &mdash; free to use, modify, and ship.<br>
  <sub>Community showcase for the steady classification framework.</sub>
</p>

---

<br>

## Why

Routing AI requests to the right model is a classification problem. You don't
need an LLM to decide which LLM to call. You need a classifier that runs in
microseconds, loads from a file, and never allocates on the hot path.

`switchboard` classifies every request across **8 orthogonal dimensions** —
length, complexity, style, quality, camera, physics, references, and cost —
then routes to the cheapest capable provider using boolean logic rules.
Total overhead: ~2ms for 8 classifiers. Upstream latency: 5–120 seconds.
Nobody notices the router.

## Quick Start

```sh
# Clone and build
git clone https://github.com/xDarkicex/switchboard
cd switchboard
go build -o switchboard ./cmd/switchboard

# Generate training data and train the 8-model pipeline
./switchboard synth --preset ./presets/video/synth.yaml \
  --real ./presets/video/real_prompts.yaml \
  --per-intent 1500 --output ./presets/video/training.txt

go run ./scripts/train-pipeline

# Come on duty
./switchboard serve --config ./switchboard.yaml
```

```sh
# Classify a single query
echo "Make a cinematic video of a dragon over mountains" | \
  ./switchboard classify --model ./models/length.bin
```

## Architecture

```
         ┌──────────┐
  POST  ─│  nanite  │─  middleware
 /v1/...─│  router  │─  classify → tag → route
         └────┬─────┘
              │
    ┌─────────▼─────────┐
    │ 8-classifier      │
    │ pipeline (steady) │   ~2ms total
    │  length             │
    │  complexity         │
    │  style              │─  photorealistic · cinematic · animation · 3d · motion_graphics
    │  quality            │
    │  camera             │
    │  physics            │
    │  refs               │
    │  cost               │
    └─────────┬─────────┘
              │
    ┌─────────▼─────────┐
    │ routing engine    │
    │ (logic.Evaluator) │  AND / OR boolean rules
    │ YAML-configured   │  first-match-wins
    └─────────┬─────────┘
              │
    ┌─────────▼─────────┐
    │ plug (nanite/sse) │
    │ upstream proxy    │  SSE pass-through
    │ ResponseControl   │  write deadline mgmt
    └───────────────────┘
```

## Built With

| Package | Role |
|---|---|
| [steady](https://github.com/xDarkicex/steady) | Zero-alloc byteSteady classifier · mmap'd models · conformal coverage |
| [nanite](https://github.com/xDarkicex/nanite) | High-perf HTTP router · radix tree · context pooling · SSE subpackage |
| [memory](https://github.com/xDarkicex/memory) | Off-heap memory · mmap pools · arena allocator · slab freelists |
| [logic](https://github.com/xDarkicex/logic) | Classical boolean engine · SAT solver · gate chains |
| [cobra](https://github.com/spf13/cobra) | CLI framework |

## API

```go
import "github.com/xDarkicex/switchboard"

// Assemble from config + routing files.
srv, _ := switchboard.NewFromFiles("switchboard.yaml", "routing.yaml")
defer srv.Close()

// Come on duty. Blocks until shutdown.
srv.GoOnDuty()

// Graceful shutdown.
srv.GoOffDuty(ctx)

// The public entry point. server.Patching.Through(w, r).
srv.Patching.Through(w, r)
```

### Internal packages

```go
import "github.com/xDarkicex/switchboard/internal/operator"

// 8-classifier pipeline — one steady model per dimension.
pipeline, _ := operator.NewPipeline(cfg)
tags, _ := pipeline.Classify("Make a video of a sunset")
// tags = { Length: "short", Style: "photorealistic", Cost: "cheap", ... }
```

```go
import "github.com/xDarkicex/switchboard/internal/lines"

// Boolean logic routing engine compiled from YAML.
engine := lines.NewEngine(spec.Rules)
provider, model := engine.Route(tags)
// → ("runway", "gen-3")
```

## Configuration

### switchboard.yaml

```yaml
server:
  listen: ":8080"

pipeline:
  style:  { model_path: "./models/style.bin",  default: cinematic, labels: [cinematic, photorealistic, animation, "3d", motion_graphics] }
  length: { model_path: "./models/length.bin", default: medium,   labels: [short, medium, long, multi_stage] }
  # ... 6 more dimensions

routing: "./presets/video/routing.yaml"

providers:
  - name: runway
    base_url: "https://api.runwayml.com/v1"
    auth: { type: bearer, env: RUNWAY_API_KEY }
    models:
      - name: gen-3
        cost_per_sec: 0.05
        max_duration: 10
        capabilities: [photorealistic, animation, motion_graphics, short, medium]

defaults:
  provider: runway
  model: gen-3
  timeout: 30s
```

### routing.yaml

```yaml
rules:
  - name: sora_multimodal_expensive      # AND — all must match
    match:
      refs: [video, multi, image]
    match_any:                           # OR — any one triggers
      - quality: [production-grade]
      - length: [multi_stage, long]
    provider: openai
    model: sora

  - name: runway_fallback
    provider: runway
    model: gen-3
```

Rules are evaluated in order. First match wins. Missing dimensions match
anything. Empty rule = always matches (fallback).

## CLI

```
switchboard serve        Start the proxy
switchboard synth        Generate training data from intents + real prompts
switchboard train        Synth + train the 8-model pipeline
switchboard classify     One-shot classify against any model
switchboard replay       Replay a request log for benchmarking
```

### serve

```sh
switchboard serve --config ./switchboard.yaml
```

Loads the pipeline, compiles routing rules, and starts the nanite HTTP server.
SIGINT triggers graceful shutdown.

### synth

```sh
switchboard synth \
  --preset ./presets/video/synth.yaml \
  --real ./presets/video/real_prompts.yaml \
  --real ./presets/video/vidprom_prompts.yaml \
  --per-intent 1500 \
  --output ./presets/video/training.txt
```

Combines synthetic examples from intent templates with annotated real-world
prompts. Outputs multi-label steady training format (one line per
dimension:value pair).

### train

```sh
switchboard train \
  --preset ./presets/video/synth.yaml \
  --real ./presets/video/real_prompts.yaml \
  --per-intent 1500
```

Runs `synth` then `scripts/train-pipeline` to produce 8 model files.

### classify

```sh
echo "Make a video of a dragon" | switchboard classify --model ./models/style.bin
```

Loads a single model and prints the prediction set. Useful for debugging
individual dimension classifiers.

### replay

```sh
switchboard replay --input ./testdata/replay/sample.jsonl --model ./models/style.bin
```

Replays a JSONL log of `{"query": "...", "expected": "label"}` records and
reports accuracy.

## Roadmap

- [x] 8-dimension video generation classifier pipeline
- [x] Boolean logic routing engine with cost-aware provider selection
- [x] SSE streaming via nanite/sse
- [x] Synthetic training data generator with intent templates
- [x] Real-world prompt corpus (230+ annotated, 5K+ auto-tagged)
- [ ] LLM router — classify and route text generation requests (OpenAI, Anthropic, Grok, local)
- [ ] Embedding router — classify and route to vector database backends (LibraVDB, Pinecone, Weaviate)
- [ ] Hot-reload routing rules without restart
- [ ] Per-request cost estimation and budget enforcement
- [ ] Request replay and A/B testing mode

<br>

---

<p align="center">
  <sub>Made with ♠ by <a href="https://github.com/xDarkicex">xDarkicex</a></sub><br>
  <sub>A <a href="https://libravdb.com">libravdb.com</a> tool · MIT License · Community showcase for the steady framework</sub><br>
  <sub>Thank you to the Libra agentic tools community</sub>
</p>
