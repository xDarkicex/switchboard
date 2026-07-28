# switchboard — Agent Skill

You are helping a user operate **switchboard**, a zero-alloc AI proxy that
classifies video generation prompts across 8 dimensions and routes them to the
cheapest capable provider. Every function, type, and variable uses vintage
switchboard operator vocabulary. Match that voice in all output.

## Quick reference

```sh
# Serve (the main thing people do)
./switchboard serve --config ./switchboard.yaml

# One-shot classify (debug a query)
echo "Make a video of a dragon" | ./switchboard classify --model ./models/style.bin

# Regenerate training data from intents + real prompts
./switchboard synth \
  --preset ./presets/video/synth.yaml \
  --real ./presets/video/real_prompts.yaml \
  --per-intent 1500 \
  --output ./presets/video/training.txt

# Retrain all 8 dimension models
go run ./scripts/train-pipeline
```

## Where models live

Trained models are **not** in the repo — they're on Hugging Face.
Download them before serving for the first time:

```sh
hf download LibraVDB/switchboard-video-v1 --local-dir ./models
```

This fetches 8 files into `./models/`:

```
models/length.bin      models/complexity.bin    models/quality.bin
models/style.bin       models/camera.bin        models/physics.bin
models/refs.bin        models/cost.bin
```

Each is ~51 MB. The pipeline mmaps them at startup — only accessed pages hit RAM.

## Configuring switchboard.yaml

The main config declares the server, the pipeline, providers, and defaults.

```yaml
server:
  listen: ":8080"          # port to listen on

pipeline:
  style:
    model_path: "./models/style.bin"
    default: cinematic      # fallback if classifier is OOD
    labels: [cinematic, photorealistic, animation, "3d", motion_graphics]
  length:
    model_path: "./models/length.bin"
    default: medium
    labels: [short, medium, long, multi_stage]
  # ... 6 more dimensions (complexity, quality, camera, physics, refs, cost)

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
      - name: gen-4
        cost_per_sec: 0.12
        max_duration: 30
        capabilities: [photorealistic, cinematic, animation, "3d", short, medium, long]

defaults:
  provider: runway
  model: gen-3
  timeout: 30s
```

### Adding a new provider

1. Add a `providers:` entry with `base_url`, `auth`, and at least one `model`.
2. Set `auth.env` to the env var holding the API key.
3. List `capabilities` — these are dimension values the model supports.
   Use the exact values from the dimension labels.

```yaml
  - name: luma
    base_url: "https://api.lumalabs.ai/dream-machine/v1"
    auth: { type: bearer, env: LUMA_API_KEY }
    models:
      - name: dream-machine
        cost_per_sec: 0.08
        max_duration: 5
        capabilities: [photorealistic, cinematic, short]
```

### Adding a new model to an existing provider

1. Add a new entry under `models:` for that provider.
2. Set `cost_per_sec` and `capabilities`.
3. Update routing rules if this model should be selected for specific cases.

## Configuring routing.yaml

Rules are evaluated in order. **First match wins.** The last rule should be a
fallback with no match conditions (matches everything).

```yaml
rules:
  - name: sora_multimodal_expensive
    match:                   # AND — all must hold
      refs: [video, multi]
    match_any:                # OR — any one triggers
      - quality: [production-grade]
      - length: [multi_stage]
    provider: openai
    model: sora

  - name: runway_animation
    match_any:
      - style: [animation]
      - style: [motion_graphics]
    provider: runway
    model: gen-3

  - name: runway_fallback      # always matches (no conditions)
    provider: runway
    model: gen-3
```

### Rule anatomy

| Field | Behavior |
|---|---|
| `match` | AND across all keys. Tag must be in the listed values. |
| `match_any` | OR across all entries. Any one entry matching is sufficient. |
| No `match` and no `match_any` | Always matches (fallback). |
| `provider` | Must match a `name` in switchboard.yaml providers. |
| `model` | Must match a model name under that provider. |

### Dimension values for match rules

| Dimension | Valid values |
|---|---|
| `length` | `short`, `medium`, `long`, `multi_stage` |
| `complexity` | `simple`, `multi_subject`, `multi_stage` |
| `style` | `photorealistic`, `cinematic`, `animation`, `3d`, `motion_graphics` |
| `quality` | `basic`, `4k`, `8k`, `production-grade` |
| `camera` | `static`, `dolly`, `tracking`, `orbital`, `fpv` |
| `physics` | `none`, `basic`, `particle`, `fluid`, `cloth` |
| `refs` | `none`, `image`, `video`, `audio`, `multi` |
| `cost` | `cheap`, `medium`, `expensive` |

## Training new models

If you add new training data or want to tune the pipeline:

```sh
# 1. Generate multi-label training data
./switchboard synth \
  --preset ./presets/video/synth.yaml \
  --real ./presets/video/real_prompts.yaml \
  --per-intent 1500 \
  --output ./presets/video/training.txt

# 2. Train 8 models (one per dimension)
go run ./scripts/train-pipeline
```

The synth combines:
- **Synthetic examples** from intent templates in `synth.yaml`
- **Real-world prompts** from `real_prompts.yaml` (curated, annotated)
- Optionally VidProM prompts from `vidprom_prompts.yaml` (auto-tagged)

Training hyperparameters are in `scripts/train-pipeline/main.go`:
`bucket=200_000`, `dim=64`, `epochs=20`, `lr=0.1`, `alpha=0.05`

## Project layout

```
switchboard/
├── cmd/switchboard/     # CLI (cobra subcommands)
├── internal/
│   ├── board/           # Config types, YAML loader, env secrets
│   ├── dimensions/      # Shared Tags struct (8 dimensions)
│   ├── lines/           # Routing engine (logic.Evaluator chains)
│   ├── operator/        # 8-classifier Pipeline (steady wrapper)
│   ├── plug/            # Upstream proxy + SSE (nanite/sse)
│   ├── synth/           # Synthetic training data generator
│   └── trunk/           # Call rejection (Drop, NumberChanged)
├── presets/video/       # Training data, routing, real prompts
├── models/              # Trained .bin files (from HF, not in repo)
├── scripts/             # Training and data import scripts
└── switchboard.go       # Public entry point (Server, Patching.Through)
```

## Troubleshooting

**"provider X requires env var Y" at startup:**
Set the env var. The operator comes on duty anyway — calls to that provider
will be dropped with 503 until the key is set.

**"all circuits busy" (503) on every request:**
Upstream is unreachable or API key is missing/wrong. Check the provider's
`base_url` and that the env var is set correctly.

**Model OOD on every request (classifier returns default):**
The training data didn't cover this prompt style. Add more real prompts of
that type to `real_prompts.yaml`, regenerate training data, and retrain.

**Models not found at startup:**
Run `hf download LibraVDB/switchboard-video-v1 --local-dir ./models`.
Verify `model_path` in switchboard.yaml points to the right location.
