# .claude-test

Meta-evaluation framework for testing Claude Code configurations.

## What This Is

This framework tests whether your `.claude` configuration actually makes Claude better at solving real-world problems.

```
┌─────────────────────────────────────────────────────────────────┐
│                    Meta-Evaluation Framework                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   Input: Your .claude config (skills + commands + hooks + ...)   │
│                              ↓                                   │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │  Test Corpus: Real GitHub Repos + Real Issues            │  │
│   │  ├── Easy:   simple-app (bug: nil pointer)               │  │
│   │  ├── Medium: express-api (feature: rate limiting)        │  │
│   │  └── Hard:   async-cache (bug: race condition)           │  │
│   └──────────────────────────────────────────────────────────┘  │
│                              ↓                                   │
│   Measure: Did your .claude config increase P(success)?          │
│                                                                  │
│   Output: "Your Go config solves 73% of medium issues vs 54%     │
│            baseline - your skills are working"                   │
└─────────────────────────────────────────────────────────────────┘
```

## Three Types of Testing

### 1. Config Validation (`configtest/`)

Structural and dry-run validation of your `.claude` directory. Auto-discovers commands, skills, and settings — adding a new extension automatically creates new subtests.

```bash
# Fast structural + dry-run checks
CLAUDE_CONFIG_DIR=./.claude go test ./configtest/...

# Live tests that invoke the real Claude CLI (costs API credits)
CLAUDE_CONFIG_DIR=./.claude go test -tags live ./configtest/...
```

### 2. Extension Tests (`cmd/runtest`)

Tests that your extensions work as documented:
- Does the skill respond appropriately?
- Does the command show the right template?
- Does the hook fire correctly?

```bash
go run ./cmd/runtest --config ./.claude
```

### 3. A/B Benchmark Tests (`cmd/configbench`)

Docker-containerized A/B comparison: runs the same tasks N times with your config and N times without, then produces a statistical verdict.

```bash
go run ./cmd/configbench \
  --config ./.claude \
  --corpus corpus/sample.yaml \
  --attempts 5 \
  --parallelism 2
```

## Project Structure

```
.claude-test/
├── .claude/                    # Git submodule: config being tested
├── claudesdk-go/               # Git submodule: Go SDK for Claude CLI
├── configtest/                 # Config validation (separate Go module)
│   ├── doc.go
│   ├── discover.go             # Auto-discovery engine
│   ├── structural_test.go      # Frontmatter, descriptions, naming
│   ├── dryrun_test.go          # Exercises assertions without CLI
│   └── live_test.go            # Real Claude CLI invocation (build tag: live)
├── cmd/
│   ├── configbench/            # Docker A/B benchmark CLI
│   │   ├── main.go
│   │   └── progress.go
│   └── runtest/                # Extension test CLI
├── internal/
│   ├── analysis/               # Statistical analysis & reporting
│   ├── corpus/                 # Corpus types & YAML loading
│   ├── docker/                 # Container lifecycle management
│   ├── metrics/                # Metrics collection & parsing
│   ├── orchestrator/           # A/B orchestration & worker pool
│   └── tests/                  # Extension test framework
├── corpus/                     # Benchmark test case definitions
│   └── sample.yaml
├── docs/
│   ├── arch/poc.md             # Full architecture document
│   ├── arch/poc/               # Phase implementation details
│   └── diagrams/               # Auto-generated architecture diagrams
├── go.mod
└── README.md
```

## Quick Start

```bash
# Clone with submodules
git clone --recurse-submodules https://github.com/MateoSegura/.claude-test

# Validate config structure (fast, no Claude CLI needed)
CLAUDE_CONFIG_DIR=./.claude go test ./configtest/...

# Run extension tests
go run ./cmd/runtest --config ./.claude

# Run A/B benchmarks
go run ./cmd/configbench \
  --config ./.claude \
  --corpus corpus/sample.yaml \
  --attempts 5

# Dry-run to preview the execution plan
go run ./cmd/configbench \
  --config ./.claude \
  --corpus corpus/sample.yaml \
  --dry-run
```

## Creating Test Corpus

Add YAML files to `corpus/`:

```yaml
name: my-corpus
version: "1.0.0"
issues:
  - id: unique-issue-id
    title: Short description
    description: Full problem description
    difficulty: easy|medium|hard
    task_type: bug_fix|feature|refactor|test
    language: go|typescript|python
    repo_url: https://github.com/owner/repo
    repo_ref: main
    prompt: |
      The exact prompt to give Claude
    eval_method: test_suite|llm_judge|hybrid
    test_command: go test ./...  # for test_suite
    success_criteria: |          # for llm_judge
      What defines success
```

## Evaluation Methods

| Method | How it works | Best for |
|--------|--------------|----------|
| `test_suite` | Runs repo's tests | Repos with good test coverage |
| `llm_judge` | Claude evaluates the diff | Subjective quality |
| `hybrid` | 60% tests + 40% LLM | Balanced evaluation |
| `custom_check` | Your validation script | Special cases |

## Interpreting Results

```
configbench v0.1.0 — Configuration Benchmark Report
════════════════════════════════════════════════════

Verdict: HELPED (high confidence)
  "Configuration improved success rate by 20% with no significant
   cost increase. MCP tools were actively utilized."

Summary
┌────────────┬────────────┬──────────┬────────┐
│ Metric     │ Configured │ Baseline │ Delta  │
├────────────┼────────────┼──────────┼────────┤
│ Success    │ 80%        │ 60%      │ +20%*  │
│ Score      │ 0.85       │ 0.72     │ +0.13* │
│ Cost       │ $0.42      │ $0.38    │ +$0.04 │
│ Turns      │ 8.2        │ 9.4      │ -1.2   │
└────────────┴────────────┴──────────┴────────┘
* statistically significant (p < 0.05)
```

## Requirements

- Go 1.24+
- Claude CLI installed and authenticated
- Docker (for A/B benchmarks)
- Git (for cloning test repos)

## License

MIT
