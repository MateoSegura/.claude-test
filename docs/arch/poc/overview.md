# configbench — Implementation Overview

> **IMPORTANT FOR CLAUDE CODE AGENTS:** This file is the single source of truth for implementation progress. Every phase MUST:
> 1. Read this file at the start of execution
> 2. Verify all prerequisite phases show `DONE` before proceeding
> 3. Update the phase status to `IN_PROGRESS` when starting work
> 4. Update the phase status to `DONE` with completion notes when finished
> 5. Write this file back using the Edit tool after every status change
>
> This allows the human operator to leave Claude Code running autonomously across all 5 phases.
>
> **ARCHITECTURE SYNC RULE:** If implementation of ANY phase reveals that the high-level architecture (`docs/arch/poc.md`) is incorrect, incomplete, or needs changes:
> 1. Update `docs/arch/poc.md` to reflect the actual implementation
> 2. Identify ALL downstream phase documents that may be affected by the change
> 3. Update those phase documents to stay consistent
> 4. Re-run validation (`go vet`, `go test`) on any already-completed phases that were affected
> 5. If a completed phase's tests break due to the change, set its status back to `IN_PROGRESS` and fix it before continuing
>
> **REGRESSION RULE:** After completing any phase, run `go test ./internal/...` to verify ALL previously-completed phases still pass. If any regression is found, fix it before marking the current phase as `DONE`.

## Project Summary

`configbench` is a single-binary Docker-based benchmarking tool that determines whether a user's `.claude` configuration improves Claude Code's effectiveness. It runs A/B comparisons (configured vs baseline) with multiple attempts, tracks tool calls, and produces a statistical verdict.

**Full architecture:** See `docs/arch/poc.md`

## Phase Status

| Phase | Name | Status | Packages | Depends On |
|-------|------|--------|----------|------------|
| 1 | Foundation | `DONE` | `internal/corpus`, `internal/metrics`, `internal/docker` (manager only) | — |
| 2 | Container Execution | `DONE` | `internal/docker` (runner), stream parsing, evaluation | Phase 1 |
| 3 | Orchestration | `DONE` | `internal/orchestrator` (config, abcontroller, pool, orchestrator) | Phase 2 |
| 4 | Analysis | `DONE` | `internal/analysis` (stats, compare, config, verdict, report, formats) | Phase 3 |
| 5 | CLI + Integration | `DONE` | `cmd/configbench`, progress reporting, end-to-end tests | Phase 4 |

## Phase Completion Criteria

### Phase 1: Foundation
- [x] `internal/corpus/corpus.go` — `Corpus`, `Issue`, `Evaluation` types compile
- [x] `internal/corpus/loader.go` — Loads and validates YAML corpus files
- [x] `internal/metrics/collector.go` — `RunResult`, `ToolCall`, `ToolCategory` types compile
- [x] `internal/metrics/tracker.go` — Tool classification logic with unit tests
- [x] `internal/docker/manager.go` — Container create/exec/remove via `os/exec`, unit tests with mocks
- [x] All packages pass `go vet` and `go test`

### Phase 2: Container Execution
- [x] `internal/docker/runner.go` — Executes Claude inside container, parses stream-json
- [x] `internal/metrics/parser.go` — Extracts metrics from `claude.StreamMessage` stream
- [x] Evaluation execution (command, test_passes, file_contains)
- [x] Integration test: runner produces `RunResult` from mock stream-json
- [x] All packages pass `go vet` and `go test`

### Phase 3: Orchestration
- [x] `internal/orchestrator/config.go` — `BenchConfig` loads from YAML
- [x] `internal/orchestrator/abcontroller.go` — Generates paired `RunSpec` sets
- [x] `internal/orchestrator/pool.go` — Bounded worker pool with `errgroup`
- [x] `internal/orchestrator/orchestrator.go` — Full run coordination
- [x] Unit tests for plan generation, pairing, and pool behavior
- [x] All packages pass `go vet` and `go test`

### Phase 4: Analysis
- [x] `internal/analysis/stats.go` — `AttemptStats`, z-test, t-test
- [x] `internal/analysis/compare.go` — `PairComparison` with deltas and p-values
- [x] `internal/analysis/config.go` — `ConfigAnalysis`, utilization, Jaccard distance
- [x] `internal/analysis/verdict.go` — Verdict generation logic
- [x] `internal/analysis/report.go` — `Report` assembly
- [x] `internal/analysis/format_terminal.go` — Terminal table output
- [x] `internal/analysis/format_json.go` — JSON serialization
- [x] All statistical functions have table-driven tests with known outputs
- [x] All packages pass `go vet` and `go test`

### Phase 5: CLI + Integration
- [x] `cmd/configbench/main.go` — Flag parsing, orchestrator wiring, progress output
- [x] End-to-end dry-run test (no Docker required)
- [x] End-to-end integration test with mock Docker (scripts replacing docker binary)
- [x] `go build ./cmd/configbench` succeeds
- [x] Binary runs `--help` and `--dry-run` correctly

## Completion Log

- **Phase 1 (Foundation):** Completed. 21 tests passing across `internal/corpus` (6), `internal/docker` (8), `internal/metrics` (5), and `internal` integration (2). All packages compile, `go vet` clean.
- **Phase 2 (Container Execution):** Completed. 44 total tests passing. Added `internal/metrics/parser.go` (7 tests), `internal/docker/executor.go` + mock (5 tests), `internal/docker/runner.go` (14 unit + 5 integration tests). All Phase 1 tests still pass.
- **Phase 3 (Orchestration):** Completed. Added `internal/orchestrator/` with config.go (18 tests), abcontroller.go (11 tests), pool.go (7 tests), orchestrator.go (11 tests). All prior phases still pass.
- **Phase 4 (Analysis):** Completed. Added `internal/analysis/` with stats.go, compare.go, config.go, verdict.go, report.go, format_terminal.go, format_json.go. 34 tests total. All prior phases still pass.
- **Phase 5 (CLI + Integration):** Completed. Added `cmd/configbench/` with main.go (flag parsing, wiring, dry-run), progress.go (thread-safe progress reporter). 16 unit tests + 4 integration tests. Binary builds and runs. Full pipeline tested with mock Docker. All phases pass — project complete.

---

## External Dependencies

The following dependencies must be added to `go.mod` before Phase 1 begins:

| Dependency | Used By | Why |
|------------|---------|-----|
| `gopkg.in/yaml.v3` | Phase 1 (corpus loader), Phase 3 (config loader) | YAML parsing for corpus and benchmark config files |
| `golang.org/x/sync` | Phase 3 (worker pool) | `errgroup` with `SetLimit()` for bounded concurrency |

Run: `go get gopkg.in/yaml.v3 golang.org/x/sync`

## Directory Structure

```
.claude-test/
├── .claude/                            # Git submodule: config being tested
├── claudesdk-go/                       # Git submodule: Go SDK (separate module)
├── configtest/                         # Config validation (separate module)
│   ├── doc.go
│   ├── discover.go
│   ├── *_test.go
│   └── go.mod
├── cmd/
│   ├── configbench/
│   │   ├── main.go
│   │   └── progress.go
│   ├── runtest/main.go
│   └── bench/main.go                  # Legacy
├── internal/
│   ├── analysis/
│   │   ├── stats.go
│   │   ├── compare.go
│   │   ├── config.go
│   │   ├── verdict.go
│   │   ├── report.go
│   │   ├── format_terminal.go
│   │   └── format_json.go
│   ├── benchmark/                     # Legacy
│   ├── corpus/
│   │   ├── corpus.go
│   │   └── loader.go
│   ├── docker/
│   │   ├── executor.go
│   │   ├── executor_docker.go
│   │   ├── executor_mock.go
│   │   ├── manager.go
│   │   └── runner.go
│   ├── metrics/
│   │   ├── collector.go
│   │   ├── tracker.go
│   │   └── parser.go
│   ├── orchestrator/
│   │   ├── orchestrator.go
│   │   ├── abcontroller.go
│   │   ├── pool.go
│   │   └── config.go
│   └── tests/
│       ├── runner.go
│       └── validators.go
├── corpus/
│   └── sample.yaml
├── docs/arch/
│   ├── poc.md                         # Full architecture
│   └── poc/
│       ├── overview.md                # This file (progress tracker)
│       ├── phase-1.md
│       ├── phase-2.md
│       ├── phase-3.md
│       ├── phase-4.md
│       └── phase-5.md
├── go.mod                             # Root module
└── go.sum
```
