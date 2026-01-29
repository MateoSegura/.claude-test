# Phase 3: Orchestration

> **Status:** `NOT_STARTED`
> **Depends on:** Phase 2
> **Produces:** `internal/orchestrator` (config, abcontroller, pool, orchestrator)

## Overview

This phase builds the coordination layer that takes a benchmark configuration and corpus, generates an execution plan of A/B paired runs, distributes work across a bounded worker pool, and collects results. The orchestrator is the "brain" that ties Docker management, execution, and metrics collection together.

## Packages

### 3.1 `internal/orchestrator/config.go`

Benchmark configuration loading:

```go
type BenchConfig struct {
    ConfigDir   string        `yaml:"-"`
    Config      ConfigSpec    `yaml:"config"`
    Docker      DockerSpec    `yaml:"docker"`
    Execution   ExecSpec      `yaml:"execution"`
    Corpus      CorpusSpec    `yaml:"corpus"`
    Analysis    AnalysisSpec  `yaml:"analysis"`
    Output      OutputSpec    `yaml:"output"`
}

type ConfigSpec struct {
    Path        string `yaml:"path"`
    Description string `yaml:"description"`
}

type DockerSpec struct {
    Image string `yaml:"image"`
}

type ExecSpec struct {
    Attempts    int           `yaml:"attempts"`
    Parallelism int           `yaml:"parallelism"`
    Timeout     time.Duration `yaml:"timeout"`
    Baseline    *bool         `yaml:"baseline"`    // nil = default true, use BoolPtr()
    MaxTurns    int           `yaml:"max_turns"`
}

// ... other spec types

func LoadConfig(path string) (*BenchConfig, error)
func (c *BenchConfig) Validate() error
func (c *BenchConfig) WithDefaults() *BenchConfig
```

### 3.2 `internal/orchestrator/abcontroller.go`

A/B variant pairing:

```go
type Variant string
const (
    Configured Variant = "configured"
    Baseline   Variant = "baseline"
)

type RunSpec struct {
    Issue     corpus.Issue
    Variant   Variant
    Attempt   int
    ConfigDir string
    Image     string
    Timeout   time.Duration
    MaxTurns  int
}

type RunPair struct {
    Issue      corpus.Issue
    Configured []RunSpec
    Baseline   []RunSpec
}

func GeneratePlan(cfg *BenchConfig, issues []corpus.Issue) []RunPair
func FlattenPlan(pairs []RunPair) []RunSpec
```

### 3.3 `internal/orchestrator/pool.go`

Bounded worker pool:

```go
type WorkerFunc func(ctx context.Context, spec RunSpec) (*metrics.RunResult, error)

type Pool struct {
    parallelism int
    workerFn    WorkerFunc
}

func NewPool(parallelism int, workerFn WorkerFunc) *Pool
func (p *Pool) Execute(ctx context.Context, specs []RunSpec, progress func(RunSpec, *metrics.RunResult)) ([]*metrics.RunResult, error)
```

### 3.4 `internal/orchestrator/orchestrator.go`

Top-level coordinator:

```go
type Orchestrator struct {
    config  *BenchConfig
    corpus  *corpus.Corpus
    manager ContainerManager  // interface, not concrete
    runner  TaskRunner         // interface, not concrete
    pool    *Pool
}

func New(cfg *BenchConfig) (*Orchestrator, error)
func NewWithDeps(cfg *BenchConfig, manager ContainerManager, runner TaskRunner) (*Orchestrator, error) // for testing
func (o *Orchestrator) Run(ctx context.Context) ([]*metrics.RunResult, error)
func (o *Orchestrator) DryRun() []RunPair
```

## Stages

### Stage 3.1: BenchConfig Type and Defaults

**What to build:**
- File: `internal/orchestrator/config.go`
- Types: `BenchConfig`, `ConfigSpec`, `DockerSpec`, `ExecSpec`, `CorpusSpec`, `CorpusFilterSpec`, `AnalysisSpec`, `OutputSpec`
- Functions: `(c *BenchConfig) WithDefaults() *BenchConfig`

**Full type definitions:**
```go
type CorpusSpec struct {
    Path    string           `yaml:"path"`
    Filters CorpusFilterSpec `yaml:"filters"`
}

type CorpusFilterSpec struct {
    Difficulty []string `yaml:"difficulty"`
    Language   []string `yaml:"language"`
    TaskType   []string `yaml:"task_type"`
}

type AnalysisSpec struct {
    SignificanceThreshold float64 `yaml:"significance_threshold"`
    ImprovementThreshold  float64 `yaml:"improvement_threshold"`
}

type OutputSpec struct {
    Path    string   `yaml:"path"`
    Formats []string `yaml:"formats"`
}
```

**Details:**
- `BenchConfig` is the top-level struct with YAML tags as shown in the phase overview (Config, Docker, Execution, Corpus, Analysis, Output sub-structs).
- `WithDefaults()` returns a new `BenchConfig` with zero-value fields replaced by defaults:
  - `Execution.Attempts` defaults to `5`
  - `Execution.Parallelism` defaults to `2`
  - `Execution.Timeout` defaults to `10 * time.Minute`
  - `Execution.Baseline` defaults to `true`
  - `Execution.MaxTurns` defaults to `25`
  - `Docker.Image` defaults to `"ghcr.io/mateosegura/dev-env:latest"`
  - `Analysis.SignificanceThreshold` defaults to `0.05`
  - `Analysis.ImprovementThreshold` defaults to `0.1`
  - `Output.Formats` defaults to `[]string{"terminal"}`
- Fields that already have non-zero values are not overwritten.

**Pass/fail criteria:**
1. `BenchConfig{}` with all zero values, after `WithDefaults()`, has `Execution.Attempts == 5`, `Execution.Parallelism == 2`, `Execution.Timeout == 10*time.Minute`, `Execution.Baseline == true`, `Execution.MaxTurns == 25`.
2. `BenchConfig{Execution: ExecSpec{Attempts: 3}}` after `WithDefaults()` retains `Attempts == 3` (non-zero preserved) but fills other fields with defaults.
3. All sub-struct types compile and have correct YAML struct tags matching the benchmark.yaml schema from the POC doc.

**Test cases:**

| # | Scenario | Input | Expected |
|---|----------|-------|----------|
| 1 | All-zero config gets full defaults | `BenchConfig{}` | After `WithDefaults()`: Attempts=5, Parallelism=2, Timeout=10m, Baseline=true, MaxTurns=25, Image="ghcr.io/mateosegura/dev-env:latest", SignificanceThreshold=0.05, ImprovementThreshold=0.1, Formats=["terminal"] |
| 2 | Partial config preserves user values | `BenchConfig{Execution: ExecSpec{Attempts: 3, Parallelism: 8}}` | After `WithDefaults()`: Attempts=3, Parallelism=8, Timeout=10m, Baseline=true (bool zero-value is false, so default applies only if we use a pointer or explicit flag -- see note below) |
| 3 | Fully specified config is unchanged | A `BenchConfig` with every field set | `WithDefaults()` returns identical values |

> **Design decision on `Baseline` bool default:** Use a `*bool` pointer for `Baseline` in `ExecSpec`. `nil` means "not set" (default to `true`), `BoolPtr(false)` means explicitly disabled, `BoolPtr(true)` means explicitly enabled. The SDK already provides `claude.BoolPtr()` for this pattern. `WithDefaults()` sets `Baseline = BoolPtr(true)` only when the pointer is nil.

---

### Stage 3.2: BenchConfig Validation

**What to build:**
- File: `internal/orchestrator/config.go` (same file, add method)
- Function: `(c *BenchConfig) Validate() error`

**Details:**
- Validation checks after defaults are applied:
  - `Config.Path` must be non-empty.
  - `Corpus.Path` must be non-empty.
  - `Execution.Attempts` must be >= 1.
  - `Execution.Parallelism` must be >= 1.
  - `Execution.Timeout` must be > 0.
  - `Execution.MaxTurns` must be >= 1.
  - `Docker.Image` must be non-empty.
  - `Analysis.SignificanceThreshold` must be in (0.0, 1.0) exclusive.
  - `Analysis.ImprovementThreshold` must be >= 0.0.
- Return a single error describing the first violation, or wrap multiple violations with `errors.Join` (Go 1.20+).

**Pass/fail criteria:**
1. A fully valid config returns `nil` from `Validate()`.
2. Each invalid field produces a non-nil error whose message contains the field name.
3. Multiple invalid fields produce an error that mentions all of them (if using `errors.Join`).

**Test cases:**

| # | Scenario | Input | Expected |
|---|----------|-------|----------|
| 1 | Valid config passes | Config with all required fields set, Attempts=5, Parallelism=2, Timeout=10m, MaxTurns=25, Image set, SignificanceThreshold=0.05 | `Validate()` returns `nil` |
| 2 | Missing config path | Config.Path="" | Error contains "config" and "path" |
| 3 | Missing corpus path | Corpus.Path="" | Error contains "corpus" and "path" |
| 4 | Zero attempts | Execution.Attempts=0 | Error contains "attempts" |
| 5 | Negative parallelism | Execution.Parallelism=-1 | Error contains "parallelism" |
| 6 | Zero timeout | Execution.Timeout=0 | Error contains "timeout" |
| 7 | Significance threshold out of range | Analysis.SignificanceThreshold=1.5 | Error contains "significance" |
| 8 | Significance threshold zero | Analysis.SignificanceThreshold=0.0 | Error contains "significance" |
| 9 | Multiple violations at once | Config.Path="", Execution.Attempts=0, Docker.Image="" | Error mentions all three fields |

---

### Stage 3.3: BenchConfig YAML Loading

**What to build:**
- File: `internal/orchestrator/config.go` (same file, add function)
- Function: `LoadConfig(path string) (*BenchConfig, error)`

**Details:**
- Reads the file at `path`, unmarshals YAML into `BenchConfig`.
- Calls `WithDefaults()` on the result.
- Calls `Validate()` on the defaulted result.
- Returns the validated config or the first error.
- Uses `os.ReadFile` + `yaml.Unmarshal` (from `gopkg.in/yaml.v3` — Go does not have a standard library YAML package).

**Pass/fail criteria:**
1. A valid YAML file round-trips correctly: all fields match expected values.
2. A YAML file with only partial fields gets defaults filled in.
3. A YAML file with invalid values (e.g., `attempts: 0`) returns a validation error.
4. A non-existent file path returns an OS-level error.
5. Malformed YAML (e.g., invalid indentation) returns a parse error.

**Test cases:**

| # | Scenario | Input | Expected |
|---|----------|-------|----------|
| 1 | Full valid YAML | YAML with all fields set (config.path=".claude", corpus.path="tasks.yaml", execution.attempts=5, etc.) | `LoadConfig` returns `*BenchConfig` with all values matching the YAML, `err == nil` |
| 2 | Minimal YAML with defaults | YAML with only `config: {path: ".claude"}` and `corpus: {path: "tasks.yaml"}` | Returns config with Attempts=5, Parallelism=2, Timeout=10m, etc. (all defaults applied) |
| 3 | Invalid values in YAML | `execution: {attempts: 0}` | Returns error from `Validate()` containing "attempts" |
| 4 | File does not exist | Path `/nonexistent/benchmark.yaml` | Returns error wrapping `os.ErrNotExist` |
| 5 | Malformed YAML | File containing `{{{not yaml` | Returns YAML parse error |
| 6 | YAML with filters | `corpus: {filters: {difficulty: [easy, medium], language: [go]}}` | `CorpusFilterSpec.Difficulty == ["easy", "medium"]`, `Language == ["go"]` |

---

### Stage 3.4: A/B Controller -- GeneratePlan

**What to build:**
- File: `internal/orchestrator/abcontroller.go`
- Types: `Variant`, `RunSpec`, `RunPair`
- Constants: `Configured`, `Baseline`
- Function: `GeneratePlan(cfg *BenchConfig, issues []corpus.Issue) []RunPair`

**Details:**
- For each issue in `issues`, create one `RunPair`.
- Each `RunPair.Configured` contains `cfg.Execution.Attempts` `RunSpec` entries, each with:
  - `Variant = Configured`
  - `Attempt` numbered `1..N`
  - `ConfigDir = cfg.Config.Path`
  - `Image`, `Timeout`, `MaxTurns` from config
- Each `RunPair.Baseline` contains `cfg.Execution.Attempts` `RunSpec` entries, each with:
  - `Variant = Baseline`
  - `Attempt` numbered `1..N`
  - `ConfigDir = ""` (empty -- no config for baseline)
  - `Image`, `Timeout`, `MaxTurns` from config
- If `cfg.Execution.Baseline` is `false`, `RunPair.Baseline` is an empty slice.
- The `Issue` field on each `RunSpec` is a copy of the source `corpus.Issue`.

**The math must be exact:**
- `len(result) == len(issues)`
- For each pair: `len(pair.Configured) == cfg.Execution.Attempts`
- For each pair (baseline enabled): `len(pair.Baseline) == cfg.Execution.Attempts`
- Total RunSpecs across all pairs = `len(issues) * attempts * 2` (when baseline enabled)
- Total RunSpecs across all pairs = `len(issues) * attempts` (when baseline disabled)

**Pass/fail criteria:**
1. Given 3 issues and attempts=5 with baseline=true, `GeneratePlan` returns exactly 3 `RunPair` values, each with 5 Configured and 5 Baseline `RunSpec` entries.
2. Every Configured `RunSpec` has `Variant == Configured`, `ConfigDir == cfg.Config.Path`, and `Attempt` in `[1,2,3,4,5]`.
3. Every Baseline `RunSpec` has `Variant == Baseline`, `ConfigDir == ""`, and `Attempt` in `[1,2,3,4,5]`.
4. With baseline=false, each `RunPair.Baseline` is empty (length 0).
5. An empty issues slice returns an empty `[]RunPair`.

**Test cases:**

| # | Scenario | Input | Expected |
|---|----------|-------|----------|
| 1 | Standard A/B plan | 3 issues, attempts=5, parallelism=2, baseline=true, image="test-img", timeout=10m, maxTurns=25, configPath=".claude" | 3 RunPairs. Each has 5 Configured specs (Variant=Configured, ConfigDir=".claude", Attempt=1..5, Image="test-img", Timeout=10m, MaxTurns=25) and 5 Baseline specs (Variant=Baseline, ConfigDir="", Attempt=1..5, same Image/Timeout/MaxTurns). Total specs across all pairs = 30. |
| 2 | Baseline disabled | 2 issues, attempts=3, baseline=false | 2 RunPairs. Each has 3 Configured specs and 0 Baseline specs. Total specs = 6. |
| 3 | Single issue, single attempt | 1 issue, attempts=1, baseline=true | 1 RunPair with 1 Configured and 1 Baseline RunSpec. Total = 2. |
| 4 | Empty corpus | 0 issues, attempts=5, baseline=true | 0 RunPairs returned. |
| 5 | Issue fields propagate | Issue with ID="go-nil-check", Prompt="Fix the nil pointer" | Every RunSpec in the pair has `Issue.ID == "go-nil-check"` and `Issue.Prompt == "Fix the nil pointer"`. |
| 6 | Large plan math | 10 issues, attempts=7, baseline=true | 10 RunPairs. Total Configured specs = 70. Total Baseline specs = 70. Grand total = 140. |

---

### Stage 3.5: A/B Controller -- FlattenPlan

**What to build:**
- File: `internal/orchestrator/abcontroller.go` (same file)
- Function: `FlattenPlan(pairs []RunPair) []RunSpec`

**Details:**
- Takes the structured `[]RunPair` and returns a flat `[]RunSpec` slice suitable for submission to the worker pool.
- Order: for each pair, emit all Configured specs first, then all Baseline specs. Pairs are emitted in input order.
- This deterministic ordering makes test assertions straightforward.

**Pass/fail criteria:**
1. `len(FlattenPlan(pairs)) == totalConfigured + totalBaseline` across all pairs.
2. The order is deterministic: pair[0].Configured, pair[0].Baseline, pair[1].Configured, pair[1].Baseline, ...
3. `FlattenPlan(nil)` and `FlattenPlan([]RunPair{})` both return an empty slice (not nil panic).

**Test cases:**

| # | Scenario | Input | Expected |
|---|----------|-------|----------|
| 1 | Standard flatten | 2 RunPairs, each with 3 Configured and 3 Baseline | 12 RunSpecs. First 3 are pair[0].Configured, next 3 are pair[0].Baseline, next 3 are pair[1].Configured, last 3 are pair[1].Baseline. |
| 2 | Baseline disabled | 2 RunPairs, each with 3 Configured and 0 Baseline | 6 RunSpecs, all Configured. |
| 3 | Empty input | `[]RunPair{}` | Empty `[]RunSpec` (length 0), no panic. |
| 4 | Nil input | `nil` | Empty `[]RunSpec` (length 0), no panic. |
| 5 | Single pair, single attempt, with baseline | 1 RunPair with 1 Configured and 1 Baseline | 2 RunSpecs: first is Configured, second is Baseline. |

---

### Stage 3.6: Worker Pool -- Bounded Concurrency

**What to build:**
- File: `internal/orchestrator/pool.go`
- Types: `Pool`, `WorkerFunc`
- Function: `NewPool(parallelism int, workerFn WorkerFunc) *Pool`
- Method: `(p *Pool) Execute(ctx context.Context, specs []RunSpec, progress func(RunSpec, *metrics.RunResult)) ([]*metrics.RunResult, error)`
- Type: `WorkerFunc func(ctx context.Context, spec RunSpec) (*metrics.RunResult, error)`

**Details:**
- The pool accepts a `WorkerFunc` (a closure over docker.Manager + docker.Runner) rather than concrete dependencies. This makes the pool testable with mock workers.
- Uses `golang.org/x/sync/errgroup` with `SetLimit(parallelism)` to bound concurrency. This dependency must be added to `go.mod` (see overview.md External Dependencies section).
- For each `RunSpec` in `specs`, submit to errgroup. The worker function executes the spec and returns a result.
- The optional `progress` callback is called after each spec completes (for live progress reporting). It must be safe to call from multiple goroutines -- the pool serializes calls with a mutex.
- Results are collected in a thread-safe manner (pre-allocated slice indexed by position, or mutex-guarded append).
- If context is cancelled, in-flight workers finish but no new specs are started.
- If a single worker returns an error, it is recorded in the result's `ErrorMsg` field and execution continues (the pool does NOT abort on individual failures). The pool-level error is only returned for catastrophic failures (context cancellation).

**Pass/fail criteria:**
1. Given 10 specs and parallelism=3, all 10 specs are executed and 10 results are returned.
2. At no point during execution are more than `parallelism` workers running simultaneously. Verified by an atomic counter incremented on worker entry and decremented on worker exit, with a max-value assertion.
3. Context cancellation stops new specs from starting. In-flight specs complete.
4. A worker that returns an error does not abort other workers. The result for that spec has `ErrorMsg` set.
5. The `progress` callback is called exactly once per spec, with the correct `RunSpec` and `*RunResult`.
6. An empty specs slice returns an empty results slice and no error.

**Test cases:**

| # | Scenario | Input | Expected |
|---|----------|-------|----------|
| 1 | All succeed | 6 specs, parallelism=2, worker sleeps 10ms and returns success | 6 results, all with `Success=true`, no error. |
| 2 | Concurrency bound respected | 20 specs, parallelism=3, worker uses atomic counter to track active goroutines | `maxConcurrent` never exceeds 3. Verified by goroutine-safe max tracker inside the mock worker. |
| 3 | Partial failure | 5 specs, parallelism=2, worker fails on spec index 2 (returns error) | 5 results returned. Result[2].ErrorMsg is non-empty. Other 4 results are successful. Pool returns nil error. |
| 4 | Context cancellation | 10 specs, parallelism=1, cancel context after 3rd spec completes | Fewer than 10 results or remaining results have context error. Pool returns context.Canceled. |
| 5 | Empty specs | 0 specs, parallelism=2 | 0 results, nil error. |
| 6 | Progress callback | 4 specs, parallelism=2, track callback invocations | Callback called exactly 4 times. Each call receives the matching RunSpec and RunResult. |
| 7 | Parallelism=1 is serial | 5 specs, parallelism=1, worker tracks active goroutines | `maxConcurrent` is always exactly 1. |

---

### Stage 3.7: Orchestrator Wiring

**What to build:**
- File: `internal/orchestrator/orchestrator.go`
- Type: `Orchestrator`
- Functions: `New(cfg *BenchConfig) (*Orchestrator, error)`, `(o *Orchestrator) Run(ctx context.Context) ([]*metrics.RunResult, error)`, `(o *Orchestrator) DryRun() []RunPair`

**Details:**
- `New` constructs the orchestrator:
  1. Loads the corpus from `cfg.Corpus.Path` using `corpus.Load()`.
  2. Applies filters from `cfg.Corpus.Filters` using `corpus.Filter()`.
  3. Creates a `docker.Manager` with `cfg.Docker.Image`.
  4. Creates a `docker.DockerExecutor` wrapping the manager.
  5. Creates a `docker.Runner` from the executor (via `docker.NewRunner(executor)`).
  6. Creates a `Pool` with `cfg.Execution.Parallelism` and a worker function that wraps the manager + runner.
  7. Returns the assembled `Orchestrator`.
- `NewWithDeps` accepts pre-built `ContainerManager` and `TaskRunner` interfaces for testing.
- `DryRun` calls `GeneratePlan` and returns the `[]RunPair` without executing anything. This is used by the `--dry-run` CLI flag.
- `Run` orchestrates the full benchmark:
  1. Calls `GeneratePlan` to produce `[]RunPair`.
  2. Calls `FlattenPlan` to produce `[]RunSpec`.
  3. Calls `pool.Execute` with the flattened specs.
  4. Returns `[]*metrics.RunResult`.
- The orchestrator should accept interfaces (not concrete types) for Manager and Runner so it can be tested with mocks.

**Dependency injection approach:**
```go
// ContainerManager handles container lifecycle (create/copy/remove).
// Does NOT include exec — that is handled by the Executor interface (Phase 2)
// which is internal to the Runner.
type ContainerManager interface {
    CreateContainer(ctx context.Context, opts docker.ContainerOpts) (string, error)
    CopyToContainer(ctx context.Context, containerID, src, dst string) error
    RemoveContainer(ctx context.Context, id string) error
    IsAvailable(ctx context.Context) error
}

// TaskRunner executes Claude inside a container and returns a RunResult.
// Wraps the Runner from Phase 2 which internally uses the Executor interface.
type TaskRunner interface {
    Run(ctx context.Context, cfg docker.RunConfig) (*metrics.RunResult, error)
}
```

**Pass/fail criteria:**
1. `New` with a valid config and a loadable corpus returns a non-nil `*Orchestrator` and nil error.
2. `New` with an invalid corpus path returns an error.
3. `DryRun` returns the correct `[]RunPair` without executing any containers (mock manager/runner receive zero calls).
4. `Run` with a mock runner executes every flattened spec and returns the correct number of results.
5. `Run` propagates context cancellation to the pool.

**Test cases:**

| # | Scenario | Input | Expected |
|---|----------|-------|----------|
| 1 | DryRun with 3 issues, 5 attempts | Config with attempts=5, baseline=true, corpus with 3 issues | `DryRun()` returns 3 RunPairs. Each pair has 5 Configured + 5 Baseline. Total specs (if flattened) = 30. Mock runner is never called. |
| 2 | Full Run with mock runner | Config with attempts=2, baseline=true, corpus with 2 issues, mock runner returns success for all | `Run()` returns 8 `*RunResult` (2 issues * 2 attempts * 2 variants). Each result has correct IssueID, Variant, and Attempt. |
| 3 | Run with baseline disabled | Config with attempts=3, baseline=false, corpus with 2 issues | `Run()` returns 6 results, all with Variant=Configured. |
| 4 | Corpus load failure | Config with corpus path pointing to non-existent file | `New()` returns error. |
| 5 | Corpus filter reduces issues | Corpus with 5 issues (3 easy, 2 hard), filter difficulty=["easy"], attempts=2, baseline=true | `DryRun()` returns 3 RunPairs (only the 3 easy issues). `Run()` returns 12 results (3 * 2 * 2). |
| 6 | Context cancellation during Run | Config with 10 issues, attempts=5, cancel context immediately | `Run()` returns context.Canceled error. Number of results is less than 100 (some specs were skipped). |
| 7 | End-to-end integration with mock | Full config, 2 issues, attempts=1, baseline=true. Mock runner returns RunResult with CostUSD=0.05 and NumTurns=8. | `Run()` returns 4 results. Every result has CostUSD=0.05 and NumTurns=8. Results can be grouped: 2 per issue, 1 Configured + 1 Baseline per issue. |

---

### Stage 3.8: Orchestrator WorkerFunc Wiring

**What to build:**
- File: `internal/orchestrator/orchestrator.go` (internal function)
- Function: `(o *Orchestrator) makeWorkerFunc() WorkerFunc`

**Details:**
- The worker function returned by `makeWorkerFunc` is the closure that the pool calls for each `RunSpec`. It encapsulates the full container lifecycle:
  1. Create container via manager (`CreateContainer`).
  2. If `spec.Variant == Configured`, copy config dir into container (`CopyToContainer`).
  3. Build `docker.RunConfig` from the `RunSpec` (map Prompt, MaxTurns, Timeout, SetupCmds, Evaluation from the Issue).
  4. Call `runner.Run(ctx, runConfig)` to execute Claude and get `*metrics.RunResult`.
  5. Set `result.IssueID`, `result.Variant`, `result.Attempt` from the spec.
  5a. Set `result.Difficulty`, `result.Language`, `result.TaskType` from `spec.Issue` — required by Phase 4 for breakdown reports.
  6. Remove container via manager (`RemoveContainer`) in a defer.
  7. Return the result.
- On container creation failure, return a `RunResult` with `Success=false` and `ErrorMsg` set.
- Container removal is always attempted (defer), even if the run fails.

**Pass/fail criteria:**
1. For a Configured spec, `CopyToContainer` is called with the config dir. For a Baseline spec, it is not called.
2. `RemoveContainer` is called exactly once per spec, even when `runner.Run` returns an error.
3. The returned `RunResult` has `IssueID`, `Variant`, and `Attempt` matching the input `RunSpec`.
4. Container creation failure produces a result with `Success=false` and a meaningful `ErrorMsg`.

**Test cases:**

| # | Scenario | Input | Expected |
|---|----------|-------|----------|
| 1 | Configured variant lifecycle | RunSpec{Variant=Configured, ConfigDir=".claude", Issue.ID="test-1", Attempt=1} | Mock manager sees: CreateContainer called, CopyToContainer called with ".claude", RemoveContainer called. Result has IssueID="test-1", Variant="configured", Attempt=1. |
| 2 | Baseline variant lifecycle | RunSpec{Variant=Baseline, ConfigDir="", Issue.ID="test-1", Attempt=2} | Mock manager sees: CreateContainer called, CopyToContainer NOT called, RemoveContainer called. Result has Variant="baseline", Attempt=2. |
| 3 | Container creation failure | Mock manager.CreateContainer returns error | Result has Success=false, ErrorMsg contains "create container". RemoveContainer is NOT called (no container to remove). |
| 4 | Runner failure with cleanup | Mock runner.Run returns error | Result has Success=false, ErrorMsg set. RemoveContainer IS called (container was created). |
| 5 | Panic safety | Mock runner.Run panics | RemoveContainer is still called (defer). Worker function recovers and returns error result. |
| 6 | Issue metadata propagated | RunSpec{Issue.Difficulty="easy", Issue.Language="go", Issue.TaskType="bug_fix"} | Result has Difficulty="easy", Language="go", TaskType="bug_fix" — needed by Phase 4 breakdowns. |

## Test Strategy

- **Config tests:** Load valid/invalid YAML, verify defaults, validate constraints.
- **A/B Controller tests:** Given N issues and M attempts, verify correct number of RunSpecs generated, proper pairing, variant assignment.
- **Pool tests:** Use mock runner, verify bounded concurrency (no more than N goroutines active), verify all specs executed, verify error propagation.
- **Orchestrator tests:** Integration test using mock Docker manager and runner. Verify end-to-end from config to results list.
