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

## Two Types of Testing

### 1. Extension Tests (`cmd/runtest`)

Tests that your extensions work as documented:
- Does the skill respond appropriately?
- Does the command show the right template?
- Does the hook fire correctly?

```bash
go run ./cmd/runtest --config ./.claude
```

### 2. Benchmark Tests (`cmd/bench`)

Tests whether your config makes Claude better at real tasks:
- Clone real repos with real issues
- Run Claude with/without your config
- Measure success rate difference

```bash
go run ./cmd/bench --config ~/.claude
```

## Project Structure

```
.claude-test/
├── .claude/                    # Submodule: your config being tested
├── cmd/
│   ├── bench/                  # Benchmark CLI
│   └── runtest/                # Extension test CLI
├── internal/
│   ├── benchmark/              # Benchmark framework
│   │   ├── corpus.go           # Issue/corpus loading
│   │   ├── evaluator.go        # Test/LLM evaluation
│   │   ├── results.go          # Result aggregation
│   │   └── runner.go           # Execution engine
│   └── tests/                  # Extension test framework
│       ├── runner.go           # Test execution
│       └── validators.go       # Output validators
├── corpus/                     # Benchmark test cases
│   └── sample.yaml             # Sample corpus
├── go.mod
└── README.md
```

## Quick Start

```bash
# Clone with submodule
git clone --recurse-submodules https://github.com/MateoSegura/.claude-test

# Run extension tests
go run ./cmd/runtest --config ./.claude

# Run benchmarks against your config
go run ./cmd/bench --config ~/.claude --baseline

# Run specific difficulty
go run ./cmd/bench --difficulty medium --language go
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
=============================================================
BENCHMARK REPORT: sample-corpus
Version: 1.0.0 | Run: 2024-01-15 14:30 | Duration: 45m
=============================================================

## Summary by Configuration

Config               Success    Score   Duration
--------------------------------------------------
baseline                54%      58%       20m
my-config               73%      78%       22m

## Improvement vs Baseline

  my-config: +19.0 percentage points
```

A +5 percentage point improvement is considered significant.

## Requirements

- Go 1.23+
- Claude CLI installed and authenticated
- Git (for cloning test repos)

## License

MIT
