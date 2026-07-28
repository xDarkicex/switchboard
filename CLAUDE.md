# switchboard

A general-purpose AI / API gateway and classifier-proxy. It listens on a wire,
classifies the incoming request with a pre-trained `steady` model, and patches
the call through to the right upstream backend — all in pure Go, sub-millisecond
routing overhead, zero allocations on the hot path where it matters.

The motivating use case is routing video generation requests (Runway, Pika,
Kling, Sora, Stable Video, etc.) with auto-determined parameters, but the
architecture is fully domain-agnostic. Any backend — LLM, image, embed, video —
can be plugged in by adding a backend entry plus a steady label.

The design carries a vintage switchboard aesthetic: every internal package is
named after a piece of the operator's equipment. The code is fast, the names
are fun.

---

## Module

- Path: `github.com/xDarkicex/switchboard`
- Go version: 1.25.7 (the floor set by `steady`, `memory`, and `logic`; `nanite` is on 1.22 but 1.25.7 is fine)
- License: MIT — drop a `LICENSE` file at the repo root before the first commit.
- The repo lives at `~/Development/golang/src/github.com/xDarkicex/switchboard`
  and is **not** a git repository yet.

## Architecture (the switchboard vocabulary)

The codebase leans hard into the operator metaphor. Every name should sound
like it came off a 1950s cord board. Don't smuggle in generic CRUD names —
the retro API is the whole point. If you can picture a Bell System training
manual using the same word, you're on the right track.

### Top-level public package

`switchboard` — the assembled gateway. The public entry point.

| Surface | Switchboard equivalent |
|---|---|
| `switchboard.Patching.Through(w, r)` | Master routing call. Listens on the line, asks the operator, patches the cord. |
| `switchboard.GoOnDuty()` | Operator comes on shift. Start the server. |
| `switchboard.GoOffDuty()` | Operator goes off shift. Graceful shutdown. |
| `switchboard.CordTest()` | Operator dials a test line to check the cord. Health check. |
| `switchboard.NightService()` | After-hours routing. |

### Internal packages

Each package is a piece of the operator's equipment.

| Package | Real-world meaning | Switchboard equivalent |
|---|---|---|
| `internal/operator` | The human attendant seated at the cord board. | Wraps `steady`. Listens on the line, classifies in ~260µs, returns the trunk line. Owns the busy lamp (circuit breaker). |
| `internal/trunk` | The long-distance line connecting two switching centers. | Out-of-distribution, rejection, drop-on-disconnect. The "no good answer for you" side. |
| `internal/plug` | The patch cord that bridges two jacks. | The socket to upstream. Owns the HTTP client, header rewriting, and SSE pass-through. |
| `internal/board` | The switchboard itself — the board with jacks, cords, and lamps. | Config types, YAML loader, env-var resolution. The "directory" of stations. |
| `internal/lines` | The patch cord bundle — the lines that connect jacks on the board. | Optional: per-label rule chains expressed via `logic`. Lets us encode "if label is X and station matches Y, override params" without imperative `if` trees. |

### Public surface per package

These are the names you should reach for. New code should slot into this
vocabulary before inventing anything new.

```go
// switchboard — the public entry point
func (s *Server) Patching.Through(w http.ResponseWriter, r *http.Request)  // master routing call
func (s *Server) GoOnDuty() error                                          // start the server
func (s *Server) GoOffDuty(ctx context.Context) error                      // graceful shutdown
func (s *Server) CordTest() error                                          // health check
func (s *Server) Ringing() bool                                            // is the operator on the line?

// internal/operator — the human attendant
func (o *Operator) Listen(call string) (extension string, trouble error)   // ~260µs classify
func (o *Operator) IsBusy() bool                                           // busy lamp
func (o *Operator) IsRinging() bool                                        // the operator is on the line
func (o *Operator) Switchhook(duty bool)                                   // on-hook / off-hook
func (o *Operator) NightService(forwarding string)                         // after-hours routing
func (o *Operator) Pickup(call string)                                     // accept a call

// internal/trunk — the long-distance line
func Drop(w http.ResponseWriter, r *http.Request, reason Reason)           // refuse a call
func NumberChanged(w http.ResponseWriter, r *http.Request, calledParty string)
func AllCircuitsBusy(w http.ResponseWriter, r *http.Request) bool

// internal/plug — the patch cord
func In(trunk *TieLine, w http.ResponseWriter, r *http.Request) (Trouble error)
func NewCord(trunk *TieLine) *Cord
func (c *Cord) CordTest() bool
func (c *Cord) TalkPath(callerID, calledParty string) error
func (c *Cord) Cutoff() error

// internal/board — the switchboard itself
func Load(path string) (Switchboard *Config, trouble error)
func (c *Config) Switchboard() (*Switchboard, trouble error)
func (c *Config) Directory() *Directory

// internal/lines — the patch cord bundle
func New(rules ...Rule) *Bundle
func (b *Bundle) Dial(label string, params map[string]any) (patched map[string]any)
func (b *Bundle) Directory() []string
```

If you find yourself reaching for a generic name, read the
[[Humor style guide (the retro API contract)]] before inventing anything new.

### Hot path

The hot path is intentionally short:

```go
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if operator.IsBusy() {
        trunk.Drop(w, r, trunk.CircuitOpen)
        return
    }
    switchboard.Patching.Through(w, r)
}
```

## Project layout

```
switchboard/
├── CLAUDE.md
├── go.mod
├── cmd/
│   └── switchboard/           # cobra root
│       ├── main.go
│       ├── serve.go           # switchboard serve
│       ├── train.go           # switchboard train (wraps steady CLI)
│       ├── classify.go        # switchboard classify (one-shot debug)
│       └── replay.go          # switchboard replay (replay a request log)
├── internal/
│   ├── operator/              # steady wrapper + circuit breaker
│   ├── trunk/                 # rejection / out-of-distribution
│   ├── plug/                  # upstream transport + SSE
│   ├── board/                 # config types + env wiring
│   └── lines/                 # logic.Rules-based param overrides
├── presets/                   # reference training data + presets
│   └── video/
│       ├── training.txt
│       └── params.yaml
├── examples/
│   └── basic/main.go          # the smallest viable gateway
└── testdata/
    └── replay/                # sample HAR / JSONL request logs
```

## Dependencies (already vendored as siblings)

| Use | Package | Why |
|---|---|---|
| HTTP routing | `github.com/xDarkicex/nanite` | Express-like ergonomics, radix tree, pooled contexts, SSE subpackage. |
| Classification | `github.com/xDarkicex/steady` | mmap'd, zero-alloc, sub-ms text classifier with conformal coverage. |
| Off-heap memory | `github.com/xDarkicex/memory` | mmap pools, arena allocator, slab freelists. Where the mmap'd model lives. |
| Logic rules | `github.com/xDarkicex/logic` | Classical + SAT engine. We use `logic.Evaluator` and gate types for condition chains. |
| CLI | `github.com/spf13/cobra` | Standard Go CLI. |

### Real APIs to lean on (don't invent new ones)

**`nanite`**
- `nanite.New(opts ...Option) *Router`
- `(*Router).Use(...MiddlewareFunc)`, `(*Router).Get/Post/Put/Delete/Patch(path, HandlerFunc, ...MiddlewareFunc) *Route`
- `(*Router).Start(port string) error`, `(*Router).Shutdown(timeout time.Duration) error`, `(*Router).AddShutdownHook(ShutdownHook)`
- `Context` exposes `Writer`, `Request`, `Values`, `Set/Get/GetValue`, `JSON`, `String`, `SetHeader`, `Abort`, `IsAborted`, `IsWritten`. Use `c.Set("key", v)` / `c.Get("key")` to pass routing decisions between middleware and the final handler.
- `nanite/sse` subpackage: `Hub`, `Connection`, `NewHub(ctx, pingInterval)`, `Register`, `Stop`. Use this for streaming pass-through.

**`steady`**
- `steady.Load(path string) (*Model, error)` — opens a trained `.bin` model file.
- `(*Model).Classify(text string) PredictionSet` — returns label set + confidences.
- `(*Model).SetLabelNames(names []string)` — call after `Load` so `Kinds` are human-readable.
- `(*Model).Close() error` — release mmap + pools.
- `PredictionSet { Kinds []string; Confidences []float32; IsEmpty() bool }` — empty means out-of-distribution.
- `steady.DefaultTrainConfig() TrainConfig` — for the `train` wrapper.
- The training CLI binary lives at `../steady/cmd/steady/main.go`; `switchboard train` shells out to it (or imports `steady.Train` directly when present).

**`memory`**
- `memory.NewPool(memory.AllocatorConfig{PoolSize, SlabSize, SlabCount}, align uint64) (*Pool, error)`
- `(*Pool).Allocate(size uint64) ([]byte, error)`, `Reset()`, `Free()`, `Stats()`
- `memory.Munmap`, `memory.MmapFileReadOnly(fd, offset, length)`
- `memory.MustPoolSlice[T](pool, cap)` — typed slice from a pool.

**`logic`**
- `logic.And/Or/Xor/Not/Nand/Nor/Xnor/Implies/Iff(inputs ...bool) bool`
- `logic.SolveSAT(expr string) (*sat.SolverResult, error)` (for anyone who wants to get fancy)
- `logic/classical.Evaluator` — fluent builder: `Eval(initial).And(x).Or(y).Not().Result()`
- `logic/classical.{AndGate,OrGate,NotGate,...}` — composable gate types.

## Build / test / lint

```sh
# Build
go build ./...

# Run the server
go run ./cmd/switchboard serve --config ./switchboard.yaml

# Train (thin wrapper over the steady CLI)
go run ./cmd/switchboard train \
  --input ./presets/video/training.txt \
  --output ./models/video.bin \
  --bucket 10000 --dim 64 --epochs 100

# One-shot classify (debug)
echo "Write a poem about a robot" | go run ./cmd/switchboard classify --model ./models/video.bin

# Tests
go test ./...

# Static analysis — these are gates, not suggestions
go vet ./...                              # The memory package is the ONLY exception (see below).
staticcheck ./...                         # Must be clean.
gocyclo -over 10 ./...                    # Cyclomatic complexity must stay under 10 per function.

# Format
gofmt -s -w .
```

Run all four before opening a PR. `staticcheck` and `gocyclo` are not optional.

## Code rules (the operator's handbook)

These rules are the difference between a working gateway and a working gateway
that hires replacement staff. Run them before claiming a change is done.

1. **`staticcheck ./...` must pass.** No ignored checks. If a rule is wrong for this project, change the code, not the config.
2. **Cyclomatic complexity must be `< 10` per function.** Measured via `gocyclo -over 10`. Don't write a 200-line function with 14 branches. The ~260µs classify target assumes the routing decision is a tree of small, predictable branches.
3. **`go vet ./...` must pass — except the `memory` package.** The `memory` package uses `unsafe`, mmap, and platform-specific assembly; `go vet` is known to flag patterns that are intentional there. Every other package must pass `go vet` cleanly. If a new package needs the same exemption, add it explicitly here with a reason.
4. **No new top-level dependencies without a discussion.** The four listed packages are the foundation. Adding more requires editing this file.
5. **No goroutine leaks.** Every goroutine must have a clear shutdown path wired through `AddShutdownHook`. Operators hang up at end of shift.
6. **No `panic` for control flow.** `panic` is only acceptable at program-init time when a configuration is provably un-recoverable. Mid-call panic is a "the operator fainted" event.
7. **Retro naming — strict.** Every function, type, method, parameter, field, and variable picks a name from the switchboard universe. See the [[Humor style guide (the retro API contract)]] for the full contract. The banned list includes (non-exhaustive): `Manager`, `Handler`, `Service`, `Worker`, `Pool`, `Cache`, `Queue`, `Router`, `Endpoint`, `Request`, `Response`, `Client`, `Server`, `Engine`, `Runtime`, `Dispatcher`, `Controller`, `Adapter`, `Driver`, `Factory`, `Builder`, `Provider`, `Consumer`, `Publisher`, `Subscriber`, `Listener`, `Observer`, `Helper`, `Util`, `Common`, `Base`, `Core`, `Spec`, `Schema`, `Model`, `Entity`, `Data`, `Info`, `Metrics`, `Trace`, `Event`, `Payload`, `Envelope`, `Config`, `Setting`, `Parameter`, `Argument`, `Input`, `Output`. No exceptions without an explicit amendment to this file.

## CLI surface (cobra)

All four subcommands live under `cmd/switchboard`. The steady model comes
**pre-trained** — `switchboard serve` only loads. The `train` subcommand is a
convenience wrapper around the steady training CLI, not the primary path.

| Command | Purpose |
|---|---|
| `switchboard serve` | Load config + model, start the proxy. `--config path` (default `./switchboard.yaml`), `--model path` (default from config). |
| `switchboard train` | Train a model via the steady CLI. Passes through `-input`, `-output`, `-bucket`, `-dim`, `-epochs`. Useful for CI and the `presets/` recipes. |
| `switchboard classify` | Load a model, read a query (arg or stdin), print the chosen label + confidences. For debugging routing decisions in production. |
| `switchboard replay` | Replay a captured request log (HAR or JSONL) against the running proxy. Compares expected vs. actual labels. Benchmark mode. |

The root command has `--version` and `--no-color` (because switchboard aesthetic
is vintage, but **logs** should still be readable in a terminal pipe).

## Configuration

`switchboard.yaml` declares backends and routing. Secrets come from env vars.

```yaml
server:
  listen: ":8080"
  read_timeout: 5s
  write_timeout: 60s

model:
  path: "./models/video.bin"
  label_param: "label"          # optional override for label names

defaults:
  label: "fallback"
  timeout: 30s

backends:
  - name: runway
    base_url: "https://api.runwayml.com/v1"
    api_key_env: RUNWAY_API_KEY
    labels: [runway_gen3, animation_style]
    timeout: 60s
  - name: sora
    base_url: "https://api.openai.com/v1"
    api_key_env: OPENAI_API_KEY
    labels: [sora, realistic_style]
    timeout: 120s
  - name: fallback
    base_url: "https://api.anthropic.com/v1"
    api_key_env: ANTHROPIC_API_KEY
    labels: [fallback]
```

Rules:
- `api_key_env` is **always** an env var name. Never inline a key.
- `labels` is the whitelist of steady labels that route to this backend. A label not listed anywhere falls back to `defaults.label`.
- `timeout` is per-request upstream timeout. SSE streams negotiate their own.
- Hot-reload is **not** in v1. Restart on SIGHUP.

## Streaming (SSE)

Streaming is required for v1. Upstream responses that arrive as `text/event-stream`
must pass through end-to-end without buffering the whole body.

- Detection: if the upstream `Content-Type` starts with `text/event-stream`, the
  plug returns the `nanite/sse.Connection` directly and pipes bytes through.
- The operator does **not** classify streamed bytes — classification happens once,
  on the first request, before the upstream is contacted.
- Use `nanite/sse.NewHub(ctx, 30*time.Second)` per server. Register connections
  on open, deregister on close.
- Backpressure: if the client is slow, drop the upstream connection — do not
  buffer the entire stream in memory.

## Adding a new backend

1. Add a `__label__<name>` line to a training file under `presets/`.
2. `switchboard train` a new `.bin` (or extend the existing one).
3. Add a backend entry to `switchboard.yaml` with `labels: [<name>]` and the
   appropriate `api_key_env`.
4. If the backend needs per-label parameter shaping (e.g. video duration),
   add a rule in `internal/lines` using `logic.Evaluator` or a gate chain.
5. Add a smoke test in `internal/plug/<name>_test.go` that round-trips a canned
   request through a mocked upstream.

## Humor style guide (the retro API contract)

The vintage switchboard aesthetic is the point. This is the part that makes
people reading the code smile. **Don't half-ass it.** The implementation is
serious — the names are the joke. If a name wouldn't make sense on a 1950s
cord board alongside a Bell System training manual, rewrite it.

The API is the product. The retro voice is what separates this from a
generic Go proxy tutorial. Generic CRUD names are a smell.

### The naming law

Every function, type, method, parameter, field, and variable picks a name
from the switchboard universe. The lists below are the contract. If you find
yourself reaching for a generic CRUD name, the operator wants a word with you.

### Banned names (the blacklist)

Banned names apply to functions, types, methods, parameters, fields, and
variables. Any one of these in a PR is a (lovingly) hard ask to rename.

**Plumbing**

- `router`, `routing`, `route` (as a noun) → `switchboard`, `patch`, `patchThrough`
- `service`, `services` → `position`, `attendant`
- `handler`, `handlers` → `position`, `attendant`
- `manager`, `managers` → `chiefOperator`
- `engine`, `runtime` → `cordBoard`, `shift`
- `gateway`, `proxy` → `switchboard`, `patch`, `talkPath`
- `dispatcher`, `controller` → `switchboard`, `switcher`
- `adapter`, `driver` → `patchAdapter`, `(drop the indirection)`
- `factory`, `builder`, `provider` → `patchPanel`, `trunk`
- `consumer`, `subscriber` → `caller`
- `publisher`, `producer` → `patch`, `patchThrough`
- `observer`, `listener` → `supervisor`, `attendant`
- `worker`, `workerPool` → `attendant`, `cordBin`
- `helper`, `util`, `common`, `shared` → `cordKeeper`, `switchRoom`, `cordRoom`
- `base`, `core` (as a package qualifier) → already in `internal/`, skip qualifiers
- `spec`, `schema`, `model`, `entity` → `directory`, `directory entry`
- `dto`, `vo`, `po`, `data`, `info`, `payload`, `envelope` → `record`, `entry`, `message`
- `event`, `msg`, `message` → `ring`, `signal`, `voice`
- `metrics`, `trace`, `audit` → `trafficRegister`, `cordTrail`, `troubleLog`

**HTTP / network**

- `connection`, `conn` → `cord`, `talkPath`, `patch`
- `channel`, `chan` → `trunk`, `tieLine`
- `socket` → `jack`, `plug`
- `port`, `endpoint` → `extension`, `station`
- `backend` → `trunk`, `tieLine`
- `client`, `cl` → `caller`
- `server`, `srv` → `exchange`
- `request`, `req` → `call`, `line`
- `response`, `resp` → `reply`, `voice`
- `id`, `uuid` → `callerID`, `stationNumber`, `cordID`
- `url`, `uri` → `number`, `trunkAddress`
- `hostname`, `host` → `exchange`
- `method` (HTTP) → `lineType`
- `header` → `cordLabel`
- `body` → `voice`, `message`

**Operations**

- `start`, `init`, `initialize`, `bootstrap`, `setup` → `goOnDuty`, `pickup`, `switchUp`
- `stop`, `close`, `shutdown`, `terminate` → `goOffDuty`, `hangup`, `cutoff`, `switchDown`
- `listen`, `accept` → `campOn`, `listenOn`
- `forward`, `route` → `patchThrough`, `transfer`
- `reject`, `refuse`, `deny` → `drop`, `intercept`, `numberChanged`
- `fallback`, `default` → `intercept`, `numberChanged`
- `retry`, `tryagain` → `ringBack`, `flash`
- `timeout`, `expired` → `reorder`, `allCircuitsBusy`
- `cancel`, `abort` → `flash`, `hangup`
- `connect`, `open` → `patch`, `patchUp`
- `disconnect`, `drop` → `cutoff`, `hangup`
- `health`, `ready`, `alive` → `cordTest`, `ringing`, `onDuty`
- `load`, `read` (file) → `plugIn`, `patchUp`
- `save`, `write` (file) → `cordLog`, `tieLineLog`
- `process`, `execute`, `perform`, `handle`, `manage` → `patch`, `patchThrough`, `run`
- `transform`, `convert`, `map` → `translate`, `patch`
- `filter`, `select` → `screen`, `sort`
- `join`, `merge`, `aggregate` → `conference`
- `split`, `branch` → `break`, `branch`
- `lookup`, `search`, `find`, `get` → `lookUp`, `directory`
- `set`, `put` → `plug`, `set` (in the cord sense)
- `create`, `make`, `new` → `make`, `build` (in the cord sense)
- `delete`, `remove` → `cutoff`, `remove`
- `update`, `modify` → `patch`, `repatch`
- `add`, `append` → `plug`, `add`
- `register`, `subscribe` → `list`, `campOn`
- `unregister`, `unsubscribe` → `drop`, `hangup`
- `notify`, `alert` → `ring`, `ringBack`
- `monitor`, `watch` → `watch`, `supervise`
- `track`, `record` → `trail`, `log`
- `validate`, `verify` → `check`, `cordTest`
- `authenticate`, `authorize` → `credentials`, `permitted`, `allowed`
- `resolve`, `calculate`, `compute` → `lookUp`, `tabulate`

**Concurrency primitives**

- `thread`, `goroutine` → `cord`
- `mutex`, `lock` → `latch`
- `unlock` → `unlatch`
- `wait`, `sleep` → `hold`, `wait`
- `signal`, `condition` → `ring`, `signal`
- `buffer` → `buffer`
- `stream` → `trunk`

### What the names look like (the whitelist)

#### Top-level

| Banned | Use |
|---|---|
| `Router` | `Switchboard` |
| `Service` | `Position`, `Attendant` |
| `Handler` | `Position`, `Attendant` |
| `Manager` | `ChiefOperator` |
| `Engine` | `CordBoard` |
| `Runtime` | `Shift` |
| `Gateway` | `Switchboard` |
| `Proxy` | `Patch`, `TalkPath` |
| `Dispatcher` | `Switchboard`, `Switcher` |
| `Controller` | `ChiefOperator` |

#### Calls and stations

| Banned | Use |
|---|---|
| `Request` | `Call`, `Line` |
| `Response` | `Reply`, `Voice` |
| `ID` | `CallerID`, `StationNumber`, `CordID` |
| `URL` | `Number`, `TrunkAddress` |
| `Hostname` | `Exchange` |
| `Method` | `LineType` |
| `Header` | `CordLabel` |
| `Body` | `Voice`, `Message` |
| `Client` | `Caller` |
| `Server` | `Exchange` |
| `Endpoint` | `Station`, `Extension` |
| `Backend` | `Trunk`, `TieLine` |
| `Connection` | `Cord`, `TalkPath`, `Patch` |
| `Channel` | `Trunk`, `TieLine` |
| `Socket` | `Jack`, `Plug` |
| `Port` | `Extension` |
| `Host` | `Exchange` |

#### Operations

| Banned | Use |
|---|---|
| `Start` | `GoOnDuty`, `Pickup`, `SwitchUp` |
| `Stop` | `GoOffDuty`, `Hangup`, `Cutoff`, `SwitchDown` |
| `Listen` | `CampOn`, `ListenOn` |
| `Forward` | `PatchThrough`, `Transfer` |
| `Reject` | `Drop`, `Intercept`, `NumberChanged` |
| `Fallback` | `Intercept`, `NumberChanged` |
| `Retry` | `RingBack`, `Flash` |
| `Timeout` | `Reorder`, `AllCircuitsBusy` |
| `Cancel` | `Flash`, `Hangup` |
| `Connect` | `Patch`, `PatchUp` |
| `Disconnect` | `Cutoff`, `Hangup` |
| `Close` | `Hangup`, `SwitchDown` |
| `Open` | `Pickup`, `SwitchUp` |
| `Shutdown` | `SwitchDown`, `PullThePlug` |
| `Health` | `CordTest`, `Ringing` |
| `Ready` | `OnDuty`, `CampOn` |
| `Load` | `PlugIn`, `PatchUp` |
| `Save` | `CordLog`, `TieLineLog` |
| `Send` | `Send`, `Patch` |
| `Process` | `Patch`, `PatchThrough` |
| `Execute` | `Patch`, `Run` |
| `Perform` | `Run` |
| `Handle` | `Patch`, `PatchThrough` |
| `Initialize` | `SwitchUp`, `GoOnDuty` |
| `Lookup` | `LookUp`, `Directory` |
| `Get` | `LookUp`, `Read` |
| `Set` | `Plug`, `Set` |
| `Create` | `Make`, `Build` |
| `Delete` | `Cutoff`, `Remove` |
| `Update` | `Patch`, `Repatch` |
| `Add` | `Plug`, `Add` |
| `Remove` | `Unplug`, `Cutoff` |
| `Register` | `List`, `CordLog` |
| `Unregister` | `Drop`, `Remove` |
| `Subscribe` | `CampOn` |
| `Unsubscribe` | `Hangup` |
| `Publish` | `Patch`, `Send` |
| `Notify` | `Ring`, `RingBack` |
| `Monitor` | `Watch`, `Supervise` |
| `Track` | `Trail` |
| `Record` | `Log`, `CordLog` |
| `Validate` | `Check`, `CordTest` |
| `Verify` | `Check`, `CordTest` |
| `Authenticate` | `Credentials`, `Code` |
| `Authorize` | `Permitted`, `Allowed` |
| `Resolve` | `LookUp`, `Directory` |
| `Calculate` | `Tabulate` |
| `Transform` | `Patch`, `Translate` |
| `Convert` | `Translate` |
| `Map` | `Translate` |
| `Filter` | `Screen`, `Sort` |
| `Join` | `Conference` |
| `Merge` | `Conference` |
| `Aggregate` | `Conference` |
| `Split` | `Break`, `Branch` |
| `Compute` | `Tabulate` |
| `Read` | `Read` |
| `Write` | `Write` |
| `Backup` | `Backup` |
| `Restore` | `Restore` |
| `Sync` | `Sync` |

#### Reliability and observability

| Banned | Use |
|---|---|
| `Error` | `Trouble`, `Intercept` |
| `HealthCheck` | `CordTest`, `Ringing` |
| `CircuitBreaker` | `BusyLamp` |
| `Cache` | `CallDetailRecord` |
| `Metrics` | `TrafficRegister` |
| `Log` (verb) | `Log`, `CordLog` |
| `Log` (noun) | `CordLog`, `TroubleLog` |
| `Trace` | `CordTrail`, `TrunkTrail` |
| `Queue` | `HoldQueue`, `CampOn` |
| `Pool` | `CordBin`, `JackBank` |
| `Worker` | `Attendant`, `Operator` |
| `Middleware` | `Circuit`, `Link` |
| `Module` | `Position`, `Patch` |
| `Component` | `Position`, `Patch` |
| `Plugin` | `PatchCord` |
| `Config` | `Directory`, `Switchboard` |
| `Setting` | `Directory` entry |
| `Parameter` | `Patch`, `PatchCord` |
| `Argument` | `Number`, `Address` |
| `Input` | `Incoming`, `Call` |
| `Output` | `Outgoing`, `Patch` |

#### Concurrency

| Banned | Use |
|---|---|
| `Thread` | `Cord` |
| `Goroutine` | `Cord` |
| `Mutex` | `Latch` |
| `Lock` | `Latch` |
| `Unlock` | `Unlatch` |
| `Wait` | `Hold`, `Wait` |
| `Signal` | `Ring`, `Signal` |
| `Channel` | `Trunk`, `TieLine` |
| `Buffer` | `Buffer` |
| `Stream` | `Trunk` |

### Variable names

The variable name is the operator's note. Same treatment.

```go
// DO
busyLamp      := operator.IsBusy()
cordID        := uuid.New()
callerID      := "555-1234"
talkPath      := plug.NewCord(station)
ringBack      := time.After(30 * time.Second)
holdQueue     := lines.New(...)
directory     := board.Load(path)
trunkCatalog  := config.Backends
patchPanel    := plug.NewPanel(trunkCatalog)
nightService  := operator.NightMode()
cordKeeper    := switchboard.NewCordKeeper()
calledParty   := r.URL.Path
ringing       := operator.IsRinging()
ringer        := operator.NewRinger()
troubleLog    := log.New()
trouble       := errors.New("the line is busy")
cordRoom      := switchboard.NewCordRoom()
switchRoom    := switchboard.NewSwitchRoom()
cordBin       := plug.NewCordBin()
latch         := sync.NewLatch()
hold          := time.Sleep(30 * time.Second)

// DON'T
isBusy        := operator.IsBusy()
connID        := uuid.New()
reqID         := "555-1234"
isConnected   := plug.NewConnection(station)
retry         := time.After(30 * time.Second)
ruleEngine    := lines.New(...)
config        := board.Load(path)
backends      := config.Backends
proxy         := plug.NewProxy(backends)
nightMode     := operator.NightMode()
helper        := helper.New()
endpoint      := r.URL.Path
isReady       := operator.IsReady()
notifier      := operator.NewNotifier()
logger        := log.New()
err           := errors.New("the line is busy")
common        := common.New()
switchboard2  := switchboard.New()
pool          := pool.New()
mutex         := mutex.New()
sleep         := time.Sleep(30 * time.Second)
```

### Doc comments

Doc comments are the voice of an operator on the line. Short, clipped, often
imperative. They never say "this function" — they say "we" or "the operator".

```go
// Drop refuses a call. Used when the line is busy or the caller is OOD.
func Drop(w, r, reason) {
    ...
}

// Patching.Through routes a call to the right trunk.
// Listens on the line, asks the operator where to send it, patches the cord.
func (s *Server) Patching.Through(w, r) {
    ...
}

// IsBusy returns true when the operator is on another call.
// The circuit is closed and we don't pick up.
func (o *Operator) IsBusy() bool {
    ...
}
```

### Error messages

On the line, errors are short and in the voice. The transport still returns
the real status code.

```go
// Real error from the upstream
errors.New("upstream rate limit exceeded (HTTP 429)")

// Switchboard-flavored log entry
log.Info("the line is busy, try again shortly")
log.Warn("your call cannot be completed as dialed")
log.Error("number, please? (missing label)")
```

### Corporate AI dialect to avoid

These words **do not appear** in this codebase. If you find them in a PR, the
reviewer will (lovingly) push back.

- "leverage", "synergy", "robust", "scalable", "best-in-class"
- "orchestration", "governance", "visibility", "actionable"
- "in order to", "in the event that", "with regard to"
- "deliverable", "stakeholder", "shareholder"
- "enterprise-grade", "production-ready", "battle-tested"
- "doors" (referring to features), "happy path", "edge case"
- "this is a no-op", "just", "simply", "obviously", "clearly"
- "we don't need to worry about" (yes we do)
- "it should be noted that", "please note that", "as you can see"
- "broadly", "broadly speaking", "in general"
- "fundamentally", "essentially", "effectively"
- "at the end of the day", "in the long run"
- "going forward", "in the future"
- "powerful", "flexible", "extensible"
- "modern", "next-generation", "cutting-edge"
- "ecosystem", "tooling", "framework"
- "best practice", "best practices"
- "as per", "per the", "per our"
- "bring to bear", "double down", "circle back"
- "deep dive", "deep dive into", "dive into"
- "unpack", "unpack the"
- "operationalize", "operationalize the"
- "foster", "facilitate", "enable"
- "ensure", "in order to ensure"
- "in this context", "in this case"
- "in summary", "to summarize"
- "holistic", "holistic approach"
- "rock-solid", "battle-hardened"
- "engagement", "engaged"
- "onboard", "onboarding"
- "offboard", "offboarding"
- "value-add", "win-win", "low-hanging fruit"
- "move the needle", "move fast and break things"
- "disrupt", "disruptive"
- "paradigm", "paradigm shift"
- "10x", "100x", "1000x"
- "agile", "lean", "MVP"
- "iteration", "iterate"
- "go-to-market", "GTM"
- "north star", "mission-critical"
- "platform", "platform-agnostic"
- "cloud-native", "cloud-ready"
- "on-prem", "on-premise"
- "hybrid", "multi-cloud"
- "zero-trust", "beyond-trust"
- "AI-powered", "AI-driven", "AI-first"
- "ML-powered", "ML-driven"
- "smart", "intelligent"
- "context-aware", "context-driven"
- "self-service", "self-serve"
- "frictionless", "seamless"
- "turnkey", "drop-in"
- "out of the box", "out-of-the-box"

### Don't overdo it

The implementation is serious. The names are the joke. Zero-alloc, mmap-backed,
sub-millisecond proxy engine — and **also** it calls
`switchboard.Patching.Through()`. That's the whole bit. Don't add more bits.

## Working in this repo

- Code lives under `internal/` except `cmd/switchboard` and the public `switchboard` package.
- Test files use standard `testing` only. No testify.
- Benchmarks go in `*_bench_test.go` alongside the code they measure. Use `testing.B`.
- Don't shadow the four dependency packages with local aliases. `import "github.com/xDarkicex/steady"` always reads as `steady`.
- The `memory` package is the only place `unsafe` is allowed. New packages adopt it only by amendment to this file.
- Run the four checks (`go test ./...`, `go vet ./...`, `staticcheck ./...`, `gocyclo -over 10 ./...`) before claiming a change is done.
