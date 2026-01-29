# Phase 1: Foundation

> **Status:** `NOT_STARTED`
> **Depends on:** Nothing
> **Produces:** `internal/corpus`, `internal/metrics` (collector + tracker), `internal/docker` (manager)

## Overview

This phase establishes the core data types and lowest-level infrastructure. Everything else builds on these packages. No Docker containers are actually run in this phase — only the manager's interface and mock-testable implementation are built.

## Packages

### 1.1 `internal/corpus/corpus.go`

Core types for task definitions:

```go
type Corpus struct {
    Name    string  `yaml:"name"`
    Version string  `yaml:"version"`
    Issues  []Issue `yaml:"issues"`
}

type Issue struct {
    ID          string     `yaml:"id"`
    Title       string     `yaml:"title"`
    Description string     `yaml:"description"`
    Repo        string     `yaml:"repo"`
    Ref         string     `yaml:"ref"`
    Difficulty  string     `yaml:"difficulty"`
    Language    string     `yaml:"language"`
    TaskType    string     `yaml:"task_type"`
    Prompt      string     `yaml:"prompt"`
    MaxTurns    int        `yaml:"max_turns,omitempty"`
    SetupCmds   []string   `yaml:"setup_commands,omitempty"`
    Evaluation  Evaluation `yaml:"evaluation"`
    ExpectedTools []string `yaml:"expected_tools,omitempty"`
}

type Evaluation struct {
    Method           string `yaml:"method"`           // command | file_contains | test_passes | llm_judge
    Command          string `yaml:"command,omitempty"`
    ExpectedExitCode int    `yaml:"expected_exit_code,omitempty"`
    FilePath         string `yaml:"file_path,omitempty"`
    Contains         string `yaml:"contains,omitempty"`
}
```

### 1.2 `internal/corpus/loader.go`

YAML loading with validation:

```go
func Load(path string) (*Corpus, error)
func (c *Corpus) Filter(f CorpusFilter) *Corpus
func (c *Corpus) Validate() error

type CorpusFilter struct {
    Difficulty []string
    Language   []string
    TaskType   []string
}
```

### 1.3 `internal/metrics/collector.go`

Run result and tool call types:

```go
type RunResult struct {
    IssueID   string
    Variant   string
    Attempt   int
    Success   bool
    Score     float64
    ErrorMsg  string
    CostUSD   float64
    DurationMS int64
    NumTurns  int
    Usage     *claude.Usage
    InitTools     []string
    ToolCalls     []ToolCall
    ToolCallSummary map[string]int
    FinalText  string

    // Issue metadata — set by orchestrator, used by Phase 4 for breakdowns
    Difficulty string
    Language   string
    TaskType   string
}

type ToolCall struct {
    Timestamp time.Time
    Name      string
    Input     map[string]any
    Category  ToolCategory
}

type ToolCategory string
const (
    BuiltIn ToolCategory = "built_in"
    MCP     ToolCategory = "mcp"
    Skill   ToolCategory = "skill"
    Agent   ToolCategory = "agent"
)
```

### 1.4 `internal/metrics/tracker.go`

Tool call tracking and classification:

```go
type Tracker struct { ... }
func NewTracker() *Tracker
func (t *Tracker) Record(name string, input map[string]any)
func (t *Tracker) Calls() []ToolCall
func (t *Tracker) Summary() map[string]int
func ClassifyTool(name string) ToolCategory
```

### 1.5 `internal/docker/manager.go`

Container lifecycle via `os/exec`:

```go
type ContainerOpts struct {
    Image     string
    ConfigDir string
    Env       map[string]string
    WorkDir   string
}

type Manager struct {
    Image string
}

func NewManager(image string) *Manager
func (m *Manager) CreateContainer(ctx context.Context, opts ContainerOpts) (string, error)
func (m *Manager) CopyToContainer(ctx context.Context, containerID, src, dst string) error
func (m *Manager) ExecInContainer(ctx context.Context, id string, cmd []string) (stdout io.Reader, stderr io.Reader, exitCode int, err error)
func (m *Manager) RemoveContainer(ctx context.Context, id string) error
func (m *Manager) IsAvailable(ctx context.Context) error
```

## Stages

### Stage 1.1: Corpus Types and YAML Loading

**What to build:**

- `internal/corpus/corpus.go` — Define `Corpus`, `Issue`, `Evaluation`, and `CorpusFilter` types with YAML struct tags.
- `internal/corpus/loader.go` — Implement `Load(path string) (*Corpus, error)` that reads and unmarshals a YAML file, `Validate() error` that checks required fields (every issue must have `id`, `title`, `repo`, `prompt`, and a valid `evaluation.method`), and `Filter(f CorpusFilter) *Corpus` that returns a new Corpus containing only issues matching the filter criteria. `Difficulty`, `Language`, and `TaskType` filter fields use OR semantics within each field and AND semantics across fields (e.g., difficulty=["easy","medium"] AND language=["go"] means: issues that are easy-or-medium AND written in Go).
- `internal/corpus/corpus_test.go` — Tests using embedded YAML fixtures via `os.WriteFile` to temp files.

**Files:**
```
internal/corpus/corpus.go
internal/corpus/loader.go
internal/corpus/corpus_test.go
```

**Pass/fail criteria:**
- `Load()` returns a non-nil `*Corpus` with the correct number of issues when given valid YAML.
- `Load()` returns a non-nil error when the file does not exist.
- `Load()` returns a non-nil error when the YAML is malformed (invalid syntax).
- `Validate()` returns nil for a corpus where every issue has all required fields populated.
- `Validate()` returns a non-nil error that names the offending issue ID when a required field is missing.
- `Filter()` returns a new `Corpus` with only the issues matching the filter; the original corpus is unmodified.
- `Filter()` with an empty/zero-value `CorpusFilter` returns all issues unchanged.

**Test cases:**

1. "Write a YAML fixture with 4 issues: two easy Go bug_fix issues, one medium TypeScript feature issue, and one hard Python refactor issue. Load it, then filter to difficulty ['easy', 'medium'] -- should get back 3 issues and none of them should be the hard Python one."

2. "Create a corpus YAML where one issue is missing the `prompt` field entirely and another issue is missing the `evaluation` block. Call Validate() and confirm the error message mentions the ID of the first invalid issue it finds."

3. "Load a valid 3-issue corpus, filter by language=['go'] and task_type=['feature']. Only one issue is a Go feature -- make sure the filtered corpus has exactly 1 issue and the original corpus still has 3."

4. "Try to Load() a path that points to a directory instead of a file -- should get an error, not a panic."

---

### Stage 1.2: Metrics Types and Tool Classification

**What to build:**

- `internal/metrics/collector.go` — Define `RunResult`, `ToolCall`, and `ToolCategory` types. `RunResult` holds all per-run data: identity fields (`IssueID`, `Variant`, `Attempt`), outcome fields (`Success`, `Score`, `ErrorMsg`), SDK metrics (`CostUSD`, `DurationMS`, `NumTurns`, `Usage`), configuration metrics (`InitTools`, `ToolCalls`, `ToolCallSummary`), output (`FinalText`), and issue metadata (`Difficulty`, `Language`, `TaskType` — set by orchestrator in Phase 3 for Phase 4 breakdowns). `ToolCategory` is a string enum with constants `BuiltIn`, `MCP`, `Skill`, `Agent`.
- `internal/metrics/tracker.go` — Implement `Tracker` struct with `NewTracker() *Tracker`, `Record(name string, input map[string]any)` that appends a `ToolCall` with a timestamp and auto-classified category, `Calls() []ToolCall` that returns all recorded calls in order, `Summary() map[string]int` that returns tool-name-to-count, and the exported `ClassifyTool(name string) ToolCategory` function. Classification rules: prefix `mcp__` maps to `MCP`, exact name `Skill` maps to `Skill`, exact name `Task` maps to `Agent`, everything else maps to `BuiltIn`.
- `internal/metrics/tracker_test.go` — Table-driven tests for classification and tracker state.

**Files:**
```
internal/metrics/collector.go
internal/metrics/tracker.go
internal/metrics/tracker_test.go
```

**Pass/fail criteria:**
- `ClassifyTool("Read")` returns `BuiltIn`, `ClassifyTool("mcp__context7__resolve")` returns `MCP`, `ClassifyTool("Skill")` returns `Skill`, `ClassifyTool("Task")` returns `Agent`.
- `NewTracker()` returns a tracker whose `Calls()` slice has length 0 and `Summary()` map is empty.
- After calling `Record("Bash", ...)` three times and `Record("Read", ...)` twice, `Summary()` returns `{"Bash": 3, "Read": 2}` and `Calls()` has length 5.
- Each `ToolCall` returned by `Calls()` has a non-zero `Timestamp` and the correct `Category` based on the tool name.
- `Record()` with a name like `mcp__github__create_pr` produces a `ToolCall` with `Category == MCP`.
- `Summary()` returns a new map each call (mutating the returned map does not affect the tracker's internal state).

**Test cases:**

1. "Run a table-driven test over these tool names and their expected categories: 'Read' -> BuiltIn, 'Write' -> BuiltIn, 'Bash' -> BuiltIn, 'mcp__context7__resolve' -> MCP, 'mcp__github__create_pr' -> MCP, 'Skill' -> Skill, 'Task' -> Agent, 'Glob' -> BuiltIn, 'Grep' -> BuiltIn. Every single one must match exactly."

2. "Create a fresh tracker. Record 'Bash' with input {\"command\": \"ls\"}, then record 'mcp__github__create_pr' with input {\"title\": \"fix\"}, then record 'Bash' with input {\"command\": \"go test\"}. Calls() should have 3 entries in that exact order, the first and third should have Category BuiltIn, the second MCP. Summary() should show Bash:2, mcp__github__create_pr:1."

3. "Create a tracker, record 5 tool calls of various types, get the Summary() map, mutate it by deleting a key, then call Summary() again -- the second Summary() should still have all 5 entries, proving the tracker returns a copy."

---

### Stage 1.3: Docker Manager (Mock-Testable)

**What to build:**

- `internal/docker/manager.go` — Implement `Manager` struct, `NewManager(image string) *Manager`, and all five methods: `CreateContainer`, `CopyToContainer`, `ExecInContainer`, `RemoveContainer`, `IsAvailable`. Each method shells out via `os/exec.Command` to the `docker` CLI. The `Manager` struct must have an injectable `execCommand` field (type `func(name string, arg ...string) *exec.Cmd`) that defaults to `exec.Command` in production. Tests inject a replacement that routes to `TestHelperProcess`-style test binaries. `CreateContainer` builds the `docker create` argument list from `ContainerOpts` (image, env vars as `-e` flags, workdir as `-w`, config dir volume mount). It returns the container ID from stdout (trimmed). `ExecInContainer` runs `docker exec <id> <cmd...>` and returns stdout/stderr as `io.Reader` plus the exit code (extracted from `exec.ExitError`). `RemoveContainer` runs `docker rm -f <id>`. `IsAvailable` runs `docker info` and returns nil on success. `CopyToContainer` runs `docker cp <src> <id>:<dst>`.
- `internal/docker/manager_test.go` — Tests using the `exec.Command` mock pattern (`TestHelperProcess`). No real Docker daemon required.

**Files:**
```
internal/docker/manager.go
internal/docker/manager_test.go
```

**Pass/fail criteria:**
- `NewManager("myimage")` returns a Manager with `Image == "myimage"`.
- `CreateContainer` invokes `docker create` with the correct image name, `-e` flags for each env var, and `-w` for the working directory. It returns the container ID from the mocked stdout.
- `CreateContainer` returns an error when the underlying command fails (non-zero exit).
- `ExecInContainer` invokes `docker exec <containerID> <cmd>` and the returned stdout reader contains the mocked output. The exit code is extracted from the command's exit status.
- `RemoveContainer` invokes `docker rm -f <containerID>` and returns nil on success.
- `IsAvailable` returns nil when `docker info` exits 0, and a non-nil error when it exits non-zero.
- `CopyToContainer` invokes `docker cp <src> <containerID>:<dst>`.
- No test requires a running Docker daemon. All tests pass in CI without Docker installed.

**Test cases:**

1. "Mock the exec function so that when it receives a command starting with 'docker create', it prints 'abc123def456' to stdout and exits 0. Call CreateContainer with image 'myimage:latest', env {\"ANTHROPIC_API_KEY\": \"sk-test\", \"HOME\": \"/root\"}, and workdir '/workspace'. Confirm the returned container ID is 'abc123def456' and the command args include '-e', 'ANTHROPIC_API_KEY=sk-test', '-e', 'HOME=/root', '-w', '/workspace', and the image name 'myimage:latest'."

2. "Mock the exec function so 'docker create' exits with code 1 and stderr 'image not found'. Call CreateContainer and verify it returns an error whose message contains 'image not found' or the exit code."

3. "Mock exec so 'docker exec' returns 'hello world\n' on stdout and '' on stderr, exit 0. Call ExecInContainer with container 'ctr1' and command ['echo', 'hello world']. Read the returned stdout reader fully and confirm it equals 'hello world\n'. Verify exit code is 0. Verify the assembled command includes 'docker', 'exec', 'ctr1', 'echo', 'hello world'."

4. "Mock exec so 'docker info' exits with code 1. Call IsAvailable and confirm it returns a non-nil error. Then swap the mock so 'docker info' exits 0 and confirm IsAvailable returns nil."

---

### Stage 1.4: Integration Smoke Test

**What to build:**

- `internal/integration_test.go` — A build-level integration test (can be a `_test.go` file with build tag `// go:build integration` or simply a test in the `internal` directory) that imports all three packages (`corpus`, `metrics`, `docker`) and exercises a realistic mini-workflow: load a corpus from YAML, filter it, create a tracker, record tool calls for each issue in the filtered corpus, and verify the resulting summary makes sense. This proves the packages compose correctly and there are no import cycles or type mismatches. This test does NOT require Docker -- the docker manager is only instantiated (NewManager), not invoked.

**Files:**
```
internal/integration_test.go
```

**Pass/fail criteria:**
- The test file compiles and runs with `go test ./internal/...` (no build tag required — it should run as part of the normal test suite since it uses no external services).
- Importing `internal/corpus`, `internal/metrics`, and `internal/docker` together in one test file causes no import cycle errors.
- A corpus loaded from a fixture can be filtered, and for each remaining issue a tracker can record tool calls and produce a summary -- the full pipeline runs without panic or error.
- The test creates a `RunResult` struct populated with real tracker data (`ToolCalls`, `ToolCallSummary`, `InitTools`) and verifies the fields are non-nil/non-empty.

**Test cases:**

1. "Write a 3-issue YAML fixture (one easy Go, one medium TS, one hard Python). Load it, filter to language=['go'], confirm 1 issue remains. Create a tracker, record 'Read', 'Edit', 'Bash' calls for that issue. Build a RunResult with IssueID set to the filtered issue's ID, ToolCalls from the tracker, ToolCallSummary from the tracker's Summary(). Confirm RunResult.ToolCallSummary has 3 keys and RunResult.ToolCalls has 3 entries."

2. "Instantiate a docker.NewManager('test-image') alongside a corpus load and tracker creation in the same test function. This proves all three packages can be imported and used together without import cycle issues. Verify the manager's Image field equals 'test-image' and the corpus loaded successfully."

## Test Strategy

All tests in this phase are unit tests. Docker Manager tests use a mock `execCommand` pattern (inject `os/exec.Command` constructor) so no real Docker daemon is needed.

Corpus loader tests use embedded YAML test fixtures.

Tracker tests are pure logic — table-driven with known inputs/outputs.
