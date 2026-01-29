# Phase 5: CLI + Integration

> **Status:** `NOT_STARTED`
> **Depends on:** Phase 4
> **Produces:** `cmd/configbench/main.go`, end-to-end tests, working binary

## Overview

This phase wires everything together into a single binary. It implements flag parsing, progress reporting, and end-to-end tests. The binary should be fully functional after this phase — `go build ./cmd/configbench` produces a working tool.

## Packages

### 5.1 `cmd/configbench/main.go`

CLI entry point:

```go
func main() {
    // Flag parsing
    // Load config (file or flags)
    // Load corpus
    // Create orchestrator
    // Run (or dry-run)
    // Build report
    // Format and output
}
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `.claude` | Config directory path |
| `--corpus` | string | required | Corpus YAML path |
| `--attempts` | int | `5` | Attempts per variant per task |
| `--parallelism` | int | `2` | Max concurrent containers |
| `--baseline` | bool | `true` | Run baseline variant |
| `--image` | string | `ghcr.io/mateosegura/dev-env:latest` | Docker image |
| `--output` | string | stdout | Report output path |
| `--timeout` | duration | `10m` | Per-task timeout |
| `--max-turns` | int | `25` | Max agentic turns |
| `--filter-difficulty` | string | all | Comma-separated |
| `--filter-language` | string | all | Comma-separated |
| `--filter-task` | string | all | Comma-separated |
| `--dry-run` | bool | `false` | Print plan, don't execute |
| `--verbose` | bool | `false` | Verbose output |
| `--format` | string | `terminal` | Output format: terminal, json |

### 5.2 Progress Reporting

During execution, print progress to stderr:

```
[1/20] go-nil-check (configured, attempt 1/5) ... OK (0.85, $0.03, 12s)
[2/20] go-nil-check (baseline, attempt 1/5) ... OK (0.70, $0.02, 15s)
[3/20] go-nil-check (configured, attempt 2/5) ... FAIL ($0.05, 18s)
...
```

### 5.3 End-to-End Tests

- **Dry-run test:** Load a test corpus, verify plan generation without Docker.
- **Mock Docker test:** Replace `docker` binary with a shell script that returns fake stream-json. Verify the full pipeline from CLI flags to report output.

## Stages

### Stage 5.1: Flag Parsing and Validation

**What to build:**
- File: `cmd/configbench/main.go`
- Define all flags using `flag` package (standard library, no third-party)
- `type cliFlags struct` holding all parsed flag values with their correct types
- `func parseFlags() (*cliFlags, error)` — parses `os.Args`, applies defaults, runs validation
- Validation rules:
  - `--corpus` is required; error if missing
  - `--attempts` must be >= 1
  - `--parallelism` must be >= 1
  - `--timeout` must parse as `time.Duration` and be > 0
  - `--max-turns` must be >= 1
  - `--format` must be one of `terminal` or `json`; error on unknown values
  - `--filter-difficulty`, `--filter-language`, `--filter-task` parse as comma-separated lists into `[]string`
  - `--config` path must exist on disk when not in `--dry-run` mode (skip check if `--dry-run`)
  - `--corpus` path must exist on disk (always required)
- Default values: `--config=.claude`, `--attempts=5`, `--parallelism=2`, `--baseline=true`, `--image=ghcr.io/mateosegura/dev-env:latest`, `--timeout=10m`, `--max-turns=25`, `--format=terminal`, `--dry-run=false`, `--verbose=false`
- `func flagsToOrchConfig(f *cliFlags) *orchestrator.BenchConfig` — converts parsed flags into the orchestrator config type
- Test file: `cmd/configbench/main_test.go`

**Pass/fail criteria:**
1. `go vet ./cmd/configbench/...` exits 0
2. `go build ./cmd/configbench` exits 0 and produces a binary
3. Running the binary with no arguments prints an error containing "required" and "corpus" to stderr, exits non-zero
4. Running with `--help` prints usage text listing all 15 flags, exits 0
5. Running with `--corpus nonexistent.yaml` prints an error containing "no such file" or "does not exist", exits non-zero
6. Running with `--corpus test.yaml --attempts 0` prints an error containing "attempts", exits non-zero
7. Running with `--corpus test.yaml --format xml` prints an error containing "format", exits non-zero
8. Running with `--corpus test.yaml --timeout invalid` prints a parse error, exits non-zero
9. All unit tests pass: `go test ./cmd/configbench/... -run TestParseFlags -v`

**Test cases:**

| # | Prompt | Verify |
|---|--------|--------|
| 1 | Build the binary: `go build -o /tmp/configbench ./cmd/configbench` | Exit code 0, binary exists at `/tmp/configbench` |
| 2 | Run with no args: `/tmp/configbench` | Stderr contains "corpus" and "required". Exit code non-zero. |
| 3 | Run `--help`: `/tmp/configbench --help` | Stdout lists `--corpus`, `--attempts`, `--parallelism`, `--baseline`, `--image`, `--output`, `--timeout`, `--max-turns`, `--filter-difficulty`, `--filter-language`, `--filter-task`, `--dry-run`, `--verbose`, `--format`, `--config`. Exit code 0. |
| 4 | Validate defaults by running `go test ./cmd/configbench/... -run TestDefaults -v`. The test creates a `cliFlags` with only `--corpus` set and asserts: `Attempts==5`, `Parallelism==2`, `Baseline==true`, `Image=="ghcr.io/mateosegura/dev-env:latest"`, `Timeout==10*time.Minute`, `MaxTurns==25`, `Format=="terminal"`, `DryRun==false`, `Verbose==false`. | Test passes. |
| 5 | Run with bad attempts: `/tmp/configbench --corpus test.yaml --attempts -1` | Stderr contains "attempts". Exit code non-zero. |
| 6 | Run with bad format: `/tmp/configbench --corpus test.yaml --format csv` | Stderr contains "format". Exit code non-zero. |
| 7 | Run with comma-separated filters: `go test ./cmd/configbench/... -run TestFilterParsing -v`. The test parses `--filter-difficulty easy,medium,hard` and asserts the result is `[]string{"easy", "medium", "hard"}`. | Test passes. |

---

### Stage 5.2: Flags-to-Orchestrator Wiring

**What to build:**
- File: `cmd/configbench/main.go` (extend `main()`)
- `func run(f *cliFlags) error` — the real entry point called by `main()`, returns an error instead of calling `os.Exit` (testable)
- Wiring sequence inside `run()`:
  1. Convert `cliFlags` to `orchestrator.BenchConfig` via `flagsToOrchConfig()`
  2. Load corpus via `corpus.Load(f.Corpus)`
  3. Apply filters: `loadedCorpus.Filter(corpus.CorpusFilter{...})` — the filter type is from `internal/corpus`, not `internal/orchestrator`. The `CorpusFilterSpec` from `BenchConfig` maps to `corpus.CorpusFilter`.
  4. If `--dry-run`, call `orchestrator.DryRun()` and print plan, then return
  5. Otherwise, call `orchestrator.Run(ctx)`, collect results
  6. Build report via `analysis.BuildReport(results, threshold)`
  7. Format output via `analysis.FormatTerminal()` or `analysis.FormatJSON()` depending on `--format`
  8. Write to `--output` file or stdout
- File output: when `--output report.json` is set, write to that file. When unset, write to stdout.
- Test file: `cmd/configbench/main_test.go`

**Pass/fail criteria:**
1. `flagsToOrchConfig()` correctly maps every CLI flag to the corresponding `BenchConfig` field — verified by unit test
2. `run()` returns an error (not panics) when corpus load fails
3. `run()` returns an error when the corpus is empty after filtering
4. `run()` with `--dry-run` never calls `orchestrator.Run()` — verified with a mock that records calls
5. `go test ./cmd/configbench/... -run TestFlagsToConfig -v` passes

**Test cases:**

| # | Prompt | Verify |
|---|--------|--------|
| 1 | Run `go test ./cmd/configbench/... -run TestFlagsToConfig -v`. The test creates a `cliFlags{Corpus: "x.yaml", Attempts: 10, Parallelism: 4, Baseline: false, Image: "myimage:v1", Timeout: 5*time.Minute, MaxTurns: 30, FilterDifficulty: "easy,medium", Format: "json"}` and calls `flagsToOrchConfig()`. Assert: `BenchConfig.Execution.Attempts == 10`, `BenchConfig.Execution.Parallelism == 4`, `BenchConfig.Execution.Baseline == false`, `BenchConfig.Docker.Image == "myimage:v1"`, `BenchConfig.Execution.Timeout == 5*time.Minute`, `BenchConfig.Execution.MaxTurns == 30`, `BenchConfig.Corpus.Filters.Difficulty == []string{"easy", "medium"}`, `BenchConfig.Output.Formats contains "json"`. | Test passes. All fields map correctly. |
| 2 | Run `go test ./cmd/configbench/... -run TestRunCorpusNotFound -v`. The test calls `run()` with a non-existent corpus path. | Returns error containing "corpus". Does not panic. |
| 3 | Run `go test ./cmd/configbench/... -run TestRunEmptyAfterFilter -v`. The test loads a valid corpus, then applies a filter that matches zero issues (e.g., `--filter-language brainfuck`). | Returns error containing "no issues" or "empty". |

---

### Stage 5.3: Dry-Run Mode

**What to build:**
- File: `cmd/configbench/main.go` (inside `run()`)
- `func printDryRun(w io.Writer, pairs []orchestrator.RunPair, cfg *cliFlags)` — formats the execution plan as human-readable text to the given writer
- The dry-run output must include:
  - Header: `Execution Plan (dry-run)`
  - Total issues count
  - Variants being tested (e.g., "configured + baseline" or "configured only")
  - Attempts per variant
  - Total runs = issues x variants x attempts
  - Per-issue listing with ID, difficulty, language, and task type
  - Image name
  - Parallelism setting
  - Timeout per task
- Dry-run must NOT call `docker.Manager`, `docker.Runner`, `orchestrator.Run()`, or any container operation
- Test file: `cmd/configbench/main_test.go`
- Test fixture: `cmd/configbench/testdata/test_corpus.yaml` — a minimal corpus with 3 issues of varying difficulty/language

**Pass/fail criteria:**
1. Running with `--dry-run` exits 0 and produces output to stdout
2. Output contains the correct total run count: `issues * variants * attempts`
3. Output lists every issue ID from the corpus
4. No Docker daemon is contacted (test runs without Docker installed)
5. When `--baseline=false`, the total run count halves (only configured variant)
6. `go test ./cmd/configbench/... -run TestDryRun -v` passes

**Test cases:**

| # | Prompt | Verify |
|---|--------|--------|
| 1 | Create `cmd/configbench/testdata/test_corpus.yaml` with 3 issues: `go-nil-check` (easy/go/bug_fix), `ts-auth` (hard/typescript/feature), `py-refactor` (medium/python/refactor). Then run: `/tmp/configbench --corpus cmd/configbench/testdata/test_corpus.yaml --dry-run` | Output contains "Execution Plan". Output contains "3 issues". Output contains "30 total runs" (3 issues x 2 variants x 5 attempts). Output lists `go-nil-check`, `ts-auth`, `py-refactor`. Exit code 0. |
| 2 | Run: `/tmp/configbench --corpus cmd/configbench/testdata/test_corpus.yaml --dry-run --baseline=false` | Output contains "15 total runs" (3 issues x 1 variant x 5 attempts). Output does NOT contain "baseline". |
| 3 | Run: `/tmp/configbench --corpus cmd/configbench/testdata/test_corpus.yaml --dry-run --attempts 3` | Output contains "18 total runs" (3 issues x 2 variants x 3 attempts). |
| 4 | Run: `/tmp/configbench --corpus cmd/configbench/testdata/test_corpus.yaml --dry-run --filter-language go` | Output contains "1 issue" (only `go-nil-check`). Output contains "10 total runs" (1 issue x 2 variants x 5 attempts). Does NOT list `ts-auth` or `py-refactor`. |
| 5 | Run `go test ./cmd/configbench/... -run TestDryRunOutput -v`. The test calls `printDryRun()` with 3 issues, baseline=true, attempts=5, captures the output string, and asserts it contains "30 total runs", all 3 issue IDs, the image name, parallelism, and timeout. | Test passes. |

---

### Stage 5.4: Progress Reporting

**What to build:**
- File: `cmd/configbench/progress.go`
- `type ProgressReporter struct` — thread-safe progress printer
- `func NewProgressReporter(w io.Writer, total int, verbose bool) *ProgressReporter`
- `func (p *ProgressReporter) OnResult(spec orchestrator.RunSpec, result *metrics.RunResult)` — called by the orchestrator's pool progress callback
- Output format to stderr (one line per completed run):
  ```
  [1/30] go-nil-check (configured, attempt 1/5) ... OK (0.85, $0.03, 12s)
  [2/30] go-nil-check (baseline, attempt 1/5) ... FAIL ($0.05, 18s)
  ```
- Fields in each line:
  - Counter: `[current/total]`
  - Issue ID
  - Variant name in parentheses
  - Attempt number: `attempt N/M`
  - Status: `OK` or `FAIL`
  - On success: score, cost, duration
  - On failure: cost, duration (no score)
- The counter must be atomic (safe for concurrent pool workers calling `OnResult`)
- In `--verbose` mode, also print: tool call count, token usage, number of turns
- Test file: `cmd/configbench/progress_test.go`

**Pass/fail criteria:**
1. `go vet ./cmd/configbench/...` exits 0
2. Progress output writes to the provided `io.Writer`, not hardcoded to `os.Stderr`
3. Counter increments atomically — concurrent calls from multiple goroutines never produce duplicate or skipped numbers
4. Output line matches the specified format exactly (parseable by regex)
5. Verbose mode includes extra fields (turns, tokens, tool calls)
6. `go test ./cmd/configbench/... -run TestProgress -v` passes

**Test cases:**

| # | Prompt | Verify |
|---|--------|--------|
| 1 | Run `go test ./cmd/configbench/... -run TestProgressFormat -v`. The test creates a `ProgressReporter` with `total=10`, calls `OnResult` with a successful result (score=0.85, cost=0.03, duration=12s), and asserts the output line matches `\[1/10\] .+ \(configured, attempt 1/5\) \.\.\. OK \(0\.85, \$0\.03, 12s\)`. | Test passes. Output matches regex. |
| 2 | Run `go test ./cmd/configbench/... -run TestProgressFailure -v`. The test calls `OnResult` with a failed result and asserts the line contains `FAIL` and does NOT contain a score value. | Test passes. |
| 3 | Run `go test ./cmd/configbench/... -run TestProgressConcurrent -v`. The test launches 10 goroutines each calling `OnResult` once (total=10). Collect all output lines. Assert: exactly 10 lines produced, counter values `[1/10]` through `[10/10]` all appear (in any order), no duplicates. | Test passes. No races. |
| 4 | Run `go test -race ./cmd/configbench/... -run TestProgressConcurrent`. | Exit code 0 — no race conditions detected. |
| 5 | Run `go test ./cmd/configbench/... -run TestProgressVerbose -v`. The test creates a verbose reporter, calls `OnResult`, asserts the output line includes turns, tokens, and tool call count. | Test passes. |

---

### Stage 5.5: End-to-End Dry-Run Integration Test

**What to build:**
- Test file: `cmd/configbench/integration_test.go` (build tag: `//go:build integration`)
- `func TestE2EDryRun(t *testing.T)` — builds the binary via `go build`, creates a temporary corpus YAML file, runs the binary with `--dry-run`, captures stdout/stderr, asserts correctness
- Test fixture: embedded corpus YAML in the test (or reuse `testdata/test_corpus.yaml`)
- This test does NOT require Docker — it validates the full binary pipeline from flag parsing through plan generation
- The test must use `exec.Command` to run the actual compiled binary (not call `run()` directly) to verify the real CLI behavior

**Pass/fail criteria:**
1. The test compiles and runs: `go test ./cmd/configbench/... -run TestE2EDryRun -tags integration -v`
2. The binary exit code is 0 for valid dry-run
3. Stdout contains the execution plan with correct counts
4. Stderr is empty (no progress output in dry-run mode)
5. The test cleans up its temporary files

**Test cases:**

| # | Prompt | Verify |
|---|--------|--------|
| 1 | Run `go test ./cmd/configbench/... -run TestE2EDryRun -tags integration -v`. The test writes a corpus with 2 issues to a temp file, builds the binary via `go build`, runs `/tmp/configbench --corpus <temp>.yaml --dry-run --attempts 3`, captures stdout. | Stdout contains "2 issues", "12 total runs" (2 x 2 x 3). Exit code 0. |
| 2 | Same test but with `--baseline=false --attempts 7`. | Stdout contains "14 total runs" (2 x 1 x 7). |
| 3 | Same test but with `--format json --dry-run`. | Stdout is valid JSON. Contains issue IDs and run counts. |

---

### Stage 5.6: Mock Docker End-to-End Test

**What to build:**
- Test file: `cmd/configbench/integration_test.go` (extend with `//go:build integration`)
- `func TestE2EMockDocker(t *testing.T)` — the full pipeline test with a fake Docker
- Mock `docker` script: `cmd/configbench/testdata/mock_docker.sh`
  - A shell script that handles the following subcommands:
    - `docker create ...` — prints a fake container ID (e.g., `abc123`) to stdout, exits 0
    - `docker cp ...` — exits 0 (no-op)
    - `docker exec ...` — when the command contains `claude`, prints canned stream-json lines to stdout (init message with tools, assistant message with tool use blocks, result message with metrics), exits 0. When the command is a setup or eval command, exits 0.
    - `docker rm ...` — exits 0
  - The canned stream-json must be valid `claude.StreamMessage` JSON lines, including:
    - An init system message with `tools` field listing `["Read", "Write", "Edit", "Bash"]`
    - An assistant message with a `tool_use` content block (e.g., tool name `Read`)
    - A result message with `cost_usd: 0.05`, `duration_ms: 8000`, `num_turns: 3`
- Test procedure:
  1. Build `configbench` binary
  2. Write `mock_docker.sh` to a temp directory
  3. Make it executable and name it `docker`
  4. Prepend the temp directory to `PATH` so the binary finds the mock instead of real Docker
  5. Write a test corpus YAML with 1 issue, run with `--attempts 1 --baseline=false --format json --output <tempfile>`
  6. Read the output JSON, unmarshal into `analysis.Report`
  7. Assert: `TotalRuns == 1`, `Configured.N == 1`, report contains the issue ID, cost and duration values are from the canned stream-json
- Test file for the mock script correctness: verify the script handles all four subcommands

**Pass/fail criteria:**
1. `mock_docker.sh` is executable and handles `create`, `cp`, `exec`, `rm` subcommands
2. The test runs without a real Docker daemon: `go test ./cmd/configbench/... -run TestE2EMockDocker -tags integration -v`
3. The output file exists and contains valid JSON
4. The JSON deserializes into a `Report` struct without error
5. Report fields match the canned values from the mock script
6. The mock docker script is called the expected number of times (create, exec for setup, exec for claude, exec for eval, rm = at least 4 calls)
7. No real containers are created (test works on CI without Docker)

**Test cases:**

| # | Prompt | Verify |
|---|--------|--------|
| 1 | Run `go test ./cmd/configbench/... -run TestE2EMockDocker -tags integration -v`. The test injects a mock `docker` script via PATH, runs the binary with `--corpus test.yaml --attempts 1 --baseline=false --format json --output /tmp/report.json`. | Exit code 0. `/tmp/report.json` exists. |
| 2 | Parse the output: `cat /tmp/report.json \| python3 -c "import json,sys; r=json.load(sys.stdin); print(r['total_runs'])"`. | Prints `1`. |
| 3 | Assert the mock was used: the test checks that `mock_docker.sh` logged its invocations to a temp file, and the log contains `create`, `exec`, and `rm` entries. | Log file contains all expected Docker subcommands. |
| 4 | Run the same test with `--baseline=true --attempts 2` and a 2-issue corpus. | Output JSON has `total_runs == 8` (2 issues x 2 variants x 2 attempts). Report contains both issue IDs. |

---

### Stage 5.7: Binary Compilation and Smoke Test

**What to build:**
- Ensure `cmd/configbench/main.go` has a valid `package main` with `func main()`
- No build errors, no unresolved imports
- All imports use the module path: `github.com/MateoSegura/claudesdk-go/internal/...`
- Smoke test in `cmd/configbench/main_test.go`: `func TestMainCompiles(t *testing.T)` — runs `go build ./cmd/configbench` from within the test
- Add `cmd/configbench/` to any CI build matrix

**Pass/fail criteria:**
1. `go build ./cmd/configbench` exits 0 on Linux and macOS (no platform-specific code)
2. `go vet ./cmd/configbench/...` exits 0
3. The resulting binary runs: `./configbench --help` exits 0
4. The resulting binary prints version info: `./configbench --version` prints `configbench v0.1.0` (or similar)
5. `go test ./cmd/configbench/...` exits 0 (all non-integration tests pass)
6. `go test -race ./cmd/configbench/...` exits 0 (no data races in unit tests)

**Test cases:**

| # | Prompt | Verify |
|---|--------|--------|
| 1 | Run `go build -o /tmp/configbench ./cmd/configbench && echo "BUILD OK"`. | Prints "BUILD OK". Binary exists at `/tmp/configbench`. |
| 2 | Run `/tmp/configbench --help 2>&1 \| head -1`. | First line contains "configbench" or "Usage". |
| 3 | Run `go vet ./cmd/configbench/...`. | Exit code 0. No output (no warnings). |
| 4 | Run `go test ./cmd/configbench/... -v`. | All tests pass. No failures. |
| 5 | Run `go test -race ./cmd/configbench/... -v`. | All tests pass. No race conditions. |
| 6 | Run `file /tmp/configbench`. | Output contains "ELF" (Linux) or "Mach-O" (macOS) — confirms it is a compiled binary, not a script. |

## Test Strategy

- **Flag parsing tests:** Verify defaults, overrides, validation errors.
- **Dry-run test:** End-to-end with real corpus YAML, no Docker, verify plan output.
- **Mock integration test:** Use `PATH` manipulation to inject a mock `docker` script that returns canned stream-json. Run full binary, verify report JSON output matches expected structure.
- **Help text test:** `configbench --help` prints usage without error.
