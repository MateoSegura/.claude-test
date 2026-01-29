# Phase 4: Analysis

> **Status:** `NOT_STARTED`
> **Depends on:** Phase 3
> **Produces:** `internal/analysis` (stats, compare, config, verdict, report, format_terminal, format_json)

## Overview

This phase takes raw `RunResult` slices and produces statistical analysis, A/B comparisons, configuration impact analysis, a verdict, and formatted reports. All computation here is pure math and data transformation — no Docker, no I/O beyond writing the final report.

## Packages

### 4.1 `internal/analysis/stats.go`

Statistical aggregation and hypothesis testing:

```go
type AttemptStats struct {
    N            int
    SuccessRate  float64
    ScoreMean    float64
    ScoreStdDev  float64
    ScoreCI95    [2]float64
    CostMean     float64
    CostStdDev   float64
    TurnsMean    float64
    DurationMean float64
    TokensMean   struct{ Input, Output float64 }
}

func ComputeStats(results []*metrics.RunResult) AttemptStats
func ZTestTwoProportions(p1, n1, p2, n2 float64) float64
func TTestPaired(a, b []float64) float64
func Mean(values []float64) float64
func StdDev(values []float64) float64
func CI95(mean, stddev float64, n int) [2]float64
```

### 4.2 `internal/analysis/compare.go`

Pair comparison:

```go
type PairComparison struct {
    IssueID            string
    Configured         AttemptStats
    Baseline           AttemptStats
    SuccessRateDelta   float64
    ScoreDelta         float64
    CostDelta          float64
    TurnsDelta         float64
    SuccessRatePValue  float64
    ScorePValue        float64
    Significant        bool
}

func ComparePair(issueID string, configured, baseline []*metrics.RunResult, threshold float64) PairComparison
func CompareAll(results []*metrics.RunResult, threshold float64) []PairComparison
```

### 4.3 `internal/analysis/config.go`

Configuration impact analysis:

```go
type ConfigAnalysis struct {
    ConfigAddedTools    []string
    ConfigRemovedTools  []string
    ToolUsageDiff       map[string]UsageDelta
    SkillInvocations    int
    AgentInvocations    int
    MCPToolCalls        int
    ConfigUtilization   float64
    BehaviorDivergence  float64
}

type UsageDelta struct {
    Configured int
    Baseline   int
}

func AnalyzeConfig(configured, baseline []*metrics.RunResult) ConfigAnalysis
func JaccardDistance(setA, setB map[string]bool) float64
```

### 4.4 `internal/analysis/verdict.go`

Verdict generation:

```go
type Verdict struct {
    Helped          bool
    Confidence      string
    SuccessRateGain float64
    ScoreGain       float64
    CostImpact      float64
    Summary         string
    Recommendations []string
}

func GenerateVerdict(comparisons []PairComparison, config ConfigAnalysis) Verdict
```

### 4.5 `internal/analysis/report.go`

Report assembly:

```go
type Report struct {
    Timestamp      time.Time
    CorpusSize     int
    TotalRuns      int
    Configured     AttemptStats
    Baseline       AttemptStats
    Comparisons    []PairComparison
    ConfigAnalysis ConfigAnalysis
    Verdict        Verdict
    ByDifficulty   map[string]PairComparison
    ByTaskType     map[string]PairComparison
    ByLanguage     map[string]PairComparison
}

func BuildReport(results []*metrics.RunResult, threshold float64) *Report
```

### 4.6 `internal/analysis/format_terminal.go`

Terminal output:

```go
func FormatTerminal(r *Report, w io.Writer) error
```

### 4.7 `internal/analysis/format_json.go`

JSON output:

```go
func FormatJSON(r *Report, w io.Writer) error
```

## Stages

### Stage 4.1: Statistical Primitives

**What to build:**
- File: `internal/analysis/stats.go`
- Functions: `Mean`, `StdDev`, `CI95`, `ZTestTwoProportions`, `TTestPaired`

These are pure mathematical functions with zero dependencies on any project types. They operate on `[]float64` and scalar inputs only. Each function must handle edge cases: empty slices, single-element slices, zero standard deviation, and division by zero.

**Implementation details:**
- `Mean(values []float64) float64` — Arithmetic mean. Return 0.0 for empty slice.
- `StdDev(values []float64) float64` — Population standard deviation (divide by N, not N-1). Return 0.0 for len < 2. Note: `TTestPaired` uses sample standard deviation (divide by N-1) internally for the t-statistic -- do NOT reuse this `StdDev` function inside `TTestPaired`.
- `CI95(mean, stddev float64, n int) [2]float64` — 95% confidence interval using z=1.96. Return `[mean, mean]` when n < 2 or stddev == 0.
- `ZTestTwoProportions(p1, n1, p2, n2 float64) float64` — Two-proportion z-test, returns p-value (two-tailed). Uses pooled proportion `p = (p1*n1 + p2*n2) / (n1 + n2)`. Return 1.0 when pooled proportion is 0 or 1 (no variance). Approximate normal CDF using the rational approximation from Abramowitz & Stegun (or `math.Erfc`).
- `TTestPaired(a, b []float64) float64` — Paired t-test, returns p-value (two-tailed). Requires `len(a) == len(b)` and `len(a) >= 2`. Return 1.0 when all differences are zero. Use the regularized incomplete beta function for the t-distribution CDF, or a well-known approximation.

**Pass/fail criteria:**
1. `Mean([]float64{2.0, 4.0, 6.0, 8.0, 10.0})` returns exactly `6.0`
2. `Mean([]float64{})` returns `0.0`
3. `Mean([]float64{42.0})` returns `42.0`
4. `StdDev([]float64{2.0, 4.0, 4.0, 4.0, 5.0, 5.0, 7.0, 9.0})` returns approximately `2.0` (exact: 2.0)
5. `StdDev([]float64{})` returns `0.0`
6. `StdDev([]float64{5.0})` returns `0.0`
7. `StdDev([]float64{3.0, 3.0, 3.0})` returns `0.0`
8. `CI95(6.0, 2.0, 25)` returns `[5.216, 6.784]` (6.0 +/- 1.96*2.0/sqrt(25) = 6.0 +/- 0.784)
9. `CI95(10.0, 0.0, 5)` returns `[10.0, 10.0]`
10. `CI95(10.0, 3.0, 1)` returns `[10.0, 10.0]`
11. `ZTestTwoProportions(0.8, 50, 0.6, 50)` returns p < 0.05 (z approx 2.18, p approx 0.029)
12. `ZTestTwoProportions(0.5, 10, 0.5, 10)` returns p == 1.0 (no difference)
13. `ZTestTwoProportions(1.0, 5, 1.0, 5)` returns 1.0 (pooled proportion = 1.0, zero variance)
14. `ZTestTwoProportions(0.0, 5, 0.0, 5)` returns 1.0 (pooled proportion = 0.0, zero variance)
15. `ZTestTwoProportions(1.0, 10, 0.0, 10)` returns p < 0.001 (maximally different)
16. `TTestPaired([]float64{85, 90, 88, 92, 87}, []float64{80, 82, 84, 85, 81})` returns p < 0.01 (clear improvement, differences = [5,8,4,7,6], mean_diff=6.0, sd_diff approx 1.58, t approx 8.49)
17. `TTestPaired([]float64{5.0, 5.0, 5.0}, []float64{5.0, 5.0, 5.0})` returns 1.0 (all differences zero)
18. `TTestPaired` panics or returns 1.0 when `len(a) != len(b)`

**Test cases (table-driven):**

```go
// Mean tests
{"five values", []float64{2.0, 4.0, 6.0, 8.0, 10.0}, 6.0},
{"empty", []float64{}, 0.0},
{"single", []float64{42.0}, 42.0},
{"negative values", []float64{-3.0, -1.0, 1.0, 3.0}, 0.0},

// StdDev tests
{"textbook example", []float64{2, 4, 4, 4, 5, 5, 7, 9}, 2.0},
{"empty", []float64{}, 0.0},
{"single", []float64{5.0}, 0.0},
{"identical values", []float64{3.0, 3.0, 3.0}, 0.0},
{"two values", []float64{0.0, 10.0}, 5.0},

// CI95 tests
{"n=25 sd=2 mean=6", 6.0, 2.0, 25, [2]float64{5.216, 6.784}},
{"zero stddev", 10.0, 0.0, 5, [2]float64{10.0, 10.0}},
{"n=1", 10.0, 3.0, 1, [2]float64{10.0, 10.0}},

// ZTestTwoProportions tests
{"significant difference", 0.8, 50, 0.6, 50, wantPLessThan: 0.05},
{"no difference", 0.5, 10, 0.5, 10, wantP: 1.0},
{"both perfect", 1.0, 5, 1.0, 5, wantP: 1.0},
{"both zero", 0.0, 5, 0.0, 5, wantP: 1.0},
{"max divergence", 1.0, 10, 0.0, 10, wantPLessThan: 0.001},
{"small sample sig", 0.8, 5, 0.2, 5, wantPLessThan: 0.05}, // z approx 2.68

// TTestPaired tests
{"clear improvement", []float64{85,90,88,92,87}, []float64{80,82,84,85,81}, wantPLessThan: 0.01},
{"no difference", []float64{5,5,5}, []float64{5,5,5}, wantP: 1.0},
```

---

### Stage 4.2: ComputeStats Aggregator

**What to build:**
- File: `internal/analysis/stats.go` (continued)
- Type: `AttemptStats` struct (as specified in the Packages section)
- Function: `ComputeStats(results []*metrics.RunResult) AttemptStats`

This function takes a slice of `RunResult` (from Phase 1's `internal/metrics/collector.go`) and produces aggregate statistics. It depends on the `Mean`, `StdDev`, `CI95` functions from Stage 4.1.

**Implementation details:**
- `N` = `len(results)`
- `SuccessRate` = count of `result.Success == true` / N
- `ScoreMean`, `ScoreStdDev` = computed from `result.Score` values
- `ScoreCI95` = `CI95(ScoreMean, ScoreStdDev, N)`
- `CostMean`, `CostStdDev` = computed from `result.CostUSD` values
- `TurnsMean` = mean of `float64(result.NumTurns)` values
- `DurationMean` = mean of `float64(result.DurationMS)` values
- `TokensMean.Input` = mean of `float64(result.Usage.InputTokens)` (skip nil Usage)
- `TokensMean.Output` = mean of `float64(result.Usage.OutputTokens)` (skip nil Usage)
- Return zero-valued `AttemptStats` for empty slice

**Pass/fail criteria:**
1. Given 5 results where 4 succeed (Success=true) with scores [0.9, 0.8, 1.0, 0.7] and 1 fails with score 0.0: `SuccessRate` == 0.8, `ScoreMean` == 0.68
2. Given 3 results with CostUSD [0.10, 0.20, 0.30]: `CostMean` == 0.2, `CostStdDev` approx 0.0816
3. Given results with NumTurns [5, 10, 15]: `TurnsMean` == 10.0
4. Given results with DurationMS [1000, 2000, 3000]: `DurationMean` == 2000.0
5. Given results with Usage{InputTokens: 100, OutputTokens: 50} and Usage{InputTokens: 200, OutputTokens: 150}: `TokensMean.Input` == 150.0, `TokensMean.Output` == 100.0
6. Given results where one has `Usage == nil`: token means computed from non-nil entries only
7. Given an empty slice: returns zero-valued `AttemptStats` with `N == 0`

**Test cases (table-driven):**

```go
// Build synthetic RunResult slices:
func makeResult(success bool, score, cost float64, turns int, durMS int64, inTok, outTok int) *metrics.RunResult {
    return &metrics.RunResult{
        Success: success, Score: score, CostUSD: cost,
        NumTurns: turns, DurationMS: durMS,
        Usage: &claude.Usage{InputTokens: inTok, OutputTokens: outTok},
    }
}

// Test: "five results mixed"
results := []*metrics.RunResult{
    makeResult(true,  0.9, 0.10, 5,  1000, 100, 50),
    makeResult(true,  0.8, 0.20, 8,  2000, 200, 100),
    makeResult(true,  1.0, 0.15, 6,  1500, 150, 75),
    makeResult(true,  0.7, 0.25, 10, 3000, 300, 200),
    makeResult(false, 0.0, 0.30, 12, 4000, 400, 250),
}
stats := ComputeStats(results)
assert stats.N == 5
assert stats.SuccessRate == 0.8
assert stats.ScoreMean == 0.68
assert stats.CostMean == 0.20
assert stats.TurnsMean == 8.2
assert stats.DurationMean == 2300.0
assert stats.TokensMean.Input == 230.0
assert stats.TokensMean.Output == 135.0

// Test: "empty slice"
stats := ComputeStats(nil)
assert stats.N == 0
assert stats.SuccessRate == 0.0
```

---

### Stage 4.3: Pair Comparison

**What to build:**
- File: `internal/analysis/compare.go`
- Type: `PairComparison` struct (as specified in the Packages section)
- Functions: `ComparePair(issueID string, configured, baseline []*metrics.RunResult, threshold float64) PairComparison` and `CompareAll(results []*metrics.RunResult, threshold float64) []PairComparison`

**Implementation details:**
- `ComparePair` calls `ComputeStats` on each slice, then computes deltas (configured minus baseline).
- `SuccessRateDelta` = `Configured.SuccessRate - Baseline.SuccessRate`
- `ScoreDelta` = `Configured.ScoreMean - Baseline.ScoreMean`
- `CostDelta` = `Configured.CostMean - Baseline.CostMean`
- `TurnsDelta` = `Configured.TurnsMean - Baseline.TurnsMean`
- `SuccessRatePValue` = `ZTestTwoProportions(Configured.SuccessRate, N_configured, Baseline.SuccessRate, N_baseline)`
- `ScorePValue` = `TTestPaired(configuredScores, baselineScores)` when both slices have len >= 2, else 1.0. When slices have different lengths, use the shorter length for pairing.
- `Significant` = `SuccessRatePValue < threshold` OR `ScorePValue < threshold`
- `CompareAll` groups results by `IssueID` and `Variant`, then calls `ComparePair` for each issue that has both variants.

**Pass/fail criteria:**
1. Configured: 5 results all succeed (scores [0.9, 0.9, 0.9, 0.9, 0.9]). Baseline: 5 results, 3 succeed (scores [0.5, 0.6, 0.0, 0.5, 0.0]). `SuccessRateDelta` == 0.4, `ScoreDelta` == 0.58 (0.9 - 0.32).
2. Same data as above with threshold=0.05: `Significant` is true (SuccessRatePValue for 1.0 vs 0.6 with n=5 gives z approx 2.36, p approx 0.018).
3. Configured and baseline both have identical success rates (0.6) and scores: `SuccessRateDelta` == 0.0, `ScoreDelta` == 0.0, `Significant` == false.
4. `CompareAll` given 10 results (5 configured + 5 baseline for issue "test-1"): returns a single `PairComparison` with `IssueID == "test-1"`.
5. `CompareAll` given results for 3 issues: returns 3 `PairComparison` entries, one per issue.
6. `CompareAll` given results with only configured variant (no baseline): returns empty slice for that issue.

**Test cases:**

```go
// ComparePair: clear improvement
configured := []*metrics.RunResult{
    {IssueID: "fix-1", Variant: "configured", Success: true, Score: 0.9, CostUSD: 0.40},
    {IssueID: "fix-1", Variant: "configured", Success: true, Score: 0.9, CostUSD: 0.42},
    {IssueID: "fix-1", Variant: "configured", Success: true, Score: 0.9, CostUSD: 0.38},
    {IssueID: "fix-1", Variant: "configured", Success: true, Score: 0.9, CostUSD: 0.41},
    {IssueID: "fix-1", Variant: "configured", Success: true, Score: 0.9, CostUSD: 0.39},
}
baseline := []*metrics.RunResult{
    {IssueID: "fix-1", Variant: "baseline", Success: true,  Score: 0.5, CostUSD: 0.35},
    {IssueID: "fix-1", Variant: "baseline", Success: true,  Score: 0.6, CostUSD: 0.36},
    {IssueID: "fix-1", Variant: "baseline", Success: false, Score: 0.0, CostUSD: 0.34},
    {IssueID: "fix-1", Variant: "baseline", Success: true,  Score: 0.5, CostUSD: 0.37},
    {IssueID: "fix-1", Variant: "baseline", Success: false, Score: 0.0, CostUSD: 0.33},
}
pair := ComparePair("fix-1", configured, baseline, 0.05)
assert pair.SuccessRateDelta == 0.4      // 1.0 - 0.6
assert pair.Significant == true

// CompareAll: groups correctly
results := append(configured, baseline...)  // 10 results, one issue
pairs := CompareAll(results, 0.05)
assert len(pairs) == 1
assert pairs[0].IssueID == "fix-1"
```

---

### Stage 4.4: Configuration Impact Analysis

**What to build:**
- File: `internal/analysis/config.go`
- Types: `ConfigAnalysis`, `UsageDelta` structs (as specified in the Packages section)
- Functions: `AnalyzeConfig(configured, baseline []*metrics.RunResult) ConfigAnalysis` and `JaccardDistance(setA, setB map[string]bool) float64`

**Implementation details:**
- `JaccardDistance(setA, setB)`: Returns `1 - |intersection| / |union|`. Return 0.0 when both sets are empty (identical empty behaviors). Return 1.0 when union is non-empty but intersection is empty.
- `AnalyzeConfig`:
  - `ConfigAddedTools`: tools in configured InitTools but not in baseline InitTools (set difference). Deduplicate across all results — union of all InitTools per variant.
  - `ConfigRemovedTools`: tools in baseline InitTools but not in configured InitTools.
  - `ToolUsageDiff`: for each tool name appearing in either variant's ToolCallSummary, record {Configured: total_count, Baseline: total_count}.
  - `SkillInvocations`: count of tool calls where `Category == Skill` in configured results.
  - `AgentInvocations`: count of tool calls where `Category == Agent` in configured results.
  - `MCPToolCalls`: count of tool calls where `Category == MCP` in configured results.
  - `ConfigUtilization`: `|intersection(ConfigAddedTools, tools_actually_called_configured)| / |ConfigAddedTools|`. Return 0.0 when ConfigAddedTools is empty.
  - `BehaviorDivergence`: `JaccardDistance(configured_tool_set, baseline_tool_set)` where tool sets are the unique tool names actually called.

**Pass/fail criteria:**
1. `JaccardDistance({A,B,C}, {B,C,D})` == 0.5 (intersection={B,C}, union={A,B,C,D}, 1 - 2/4 = 0.5)
2. `JaccardDistance({A,B}, {A,B})` == 0.0 (identical sets)
3. `JaccardDistance({A,B}, {C,D})` == 1.0 (disjoint sets)
4. `JaccardDistance({}, {})` == 0.0 (both empty)
5. `JaccardDistance({A}, {})` == 1.0 (one empty, one not)
6. Given configured results with InitTools=["Read","Write","Edit","Bash","mcp__ctx7__resolve","Skill"] and baseline InitTools=["Read","Write","Edit","Bash"]: `ConfigAddedTools` == ["Skill", "mcp__ctx7__resolve"] (sorted), `ConfigRemovedTools` == [] (empty).
7. Given configured results that call ["Read","Edit","mcp__ctx7__resolve"] and baseline results that call ["Read","Edit","Bash"]: `BehaviorDivergence` == `JaccardDistance({Read,Edit,mcp__ctx7__resolve}, {Read,Edit,Bash})` = 1 - 2/4 = 0.5
8. Given ConfigAddedTools=["mcp__ctx7__resolve","Skill"] and configured calls include "mcp__ctx7__resolve" but NOT "Skill": `ConfigUtilization` == 0.5 (1 of 2 added tools used). Note: this scenario is different from the test data below where both are called.
9. Given ConfigAddedTools is empty: `ConfigUtilization` == 0.0

**Test cases:**

```go
// JaccardDistance table-driven
{"overlapping sets",
    map[string]bool{"A": true, "B": true, "C": true},
    map[string]bool{"B": true, "C": true, "D": true},
    0.5},
{"identical",
    map[string]bool{"A": true, "B": true},
    map[string]bool{"A": true, "B": true},
    0.0},
{"disjoint",
    map[string]bool{"A": true, "B": true},
    map[string]bool{"C": true, "D": true},
    1.0},
{"both empty",
    map[string]bool{},
    map[string]bool{},
    0.0},
{"one empty",
    map[string]bool{"A": true},
    map[string]bool{},
    1.0},

// AnalyzeConfig: full scenario
configured := []*metrics.RunResult{
    {
        InitTools: []string{"Read", "Write", "Edit", "Bash", "mcp__ctx7__resolve", "Skill"},
        ToolCalls: []metrics.ToolCall{
            {Name: "Read", Category: metrics.BuiltIn},
            {Name: "Edit", Category: metrics.BuiltIn},
            {Name: "mcp__ctx7__resolve", Category: metrics.MCP},
            {Name: "Skill", Category: metrics.Skill},
            {Name: "Task", Category: metrics.Agent},
        },
        ToolCallSummary: map[string]int{"Read": 3, "Edit": 2, "mcp__ctx7__resolve": 1, "Skill": 1, "Task": 1},
    },
}
baseline := []*metrics.RunResult{
    {
        InitTools: []string{"Read", "Write", "Edit", "Bash"},
        ToolCalls: []metrics.ToolCall{
            {Name: "Read", Category: metrics.BuiltIn},
            {Name: "Bash", Category: metrics.BuiltIn},
        },
        ToolCallSummary: map[string]int{"Read": 5, "Bash": 3},
    },
}
analysis := AnalyzeConfig(configured, baseline)
assert analysis.MCPToolCalls == 1
assert analysis.SkillInvocations == 1
assert analysis.AgentInvocations == 1
assert analysis.ConfigUtilization == 1.0  // Both added tools (mcp__ctx7__resolve, Skill) were actually called. 2/2 = 1.0
assert len(analysis.ConfigAddedTools) == 2
assert len(analysis.ConfigRemovedTools) == 0
```

---

### Stage 4.5: Verdict Generation

**What to build:**
- File: `internal/analysis/verdict.go`
- Type: `Verdict` struct (as specified in the Packages section)
- Function: `GenerateVerdict(comparisons []PairComparison, config ConfigAnalysis) Verdict`

**Implementation details:**

Verdict logic (from poc.md):
```
Helped = SuccessRateGain > 0 AND ScoreGain >= 0

Confidence:
  high   = sigRatio > 0.7 AND attempts >= 5
  medium = sigRatio > 0.4 AND attempts >= 3
  low    = otherwise

  where sigRatio = (significant comparisons) / (total comparisons)
```

- `SuccessRateGain`: mean of all `PairComparison.SuccessRateDelta` values
- `ScoreGain`: mean of all `PairComparison.ScoreDelta` values
- `CostImpact`: mean of all `PairComparison.CostDelta` values
- `Summary`: human-readable string describing the verdict
- `Recommendations`: generated from analysis thresholds:
  - If `config.ConfigUtilization < 0.3`: add recommendation about unused tools
  - If `CostImpact > 1.5 * mean_baseline_cost`: add recommendation about cost
  - If `config.BehaviorDivergence < 0.1`: add recommendation about redundant config
  - If `Helped == false` and `SuccessRateGain < 0`: add recommendation that config is hurting performance
- `attempts` for confidence calculation: use the minimum N across all comparisons (min of `Configured.N` and `Baseline.N` for each comparison)

**Pass/fail criteria:**
1. All comparisons show improvement (SuccessRateDelta > 0, ScoreDelta > 0), sigRatio=1.0, attempts=5: `Helped` == true, `Confidence` == "high"
2. All comparisons show no difference (deltas == 0): `Helped` == false, `Confidence` == "low"
3. Mixed results — 3 of 5 comparisons significant with positive deltas, attempts=5: `Helped` == true, `Confidence` == "medium" (sigRatio = 0.6 > 0.4, attempts >= 3 but sigRatio < 0.7)
4. Positive SuccessRateGain but negative ScoreGain: `Helped` == false (both conditions must hold)
5. Empty comparisons slice: `Helped` == false, `Confidence` == "low"
6. ConfigUtilization == 0.1: recommendations include "unused tools" message
7. BehaviorDivergence == 0.05: recommendations include "redundant config" message
8. Negative SuccessRateGain: recommendations include "config is hurting" message

**Test cases:**

```go
// Test: "clear improvement, high confidence"
comparisons := []PairComparison{
    {IssueID: "issue-1", SuccessRateDelta: 0.4, ScoreDelta: 0.2,
     Configured: AttemptStats{N: 5}, Baseline: AttemptStats{N: 5}, Significant: true},
    {IssueID: "issue-2", SuccessRateDelta: 0.2, ScoreDelta: 0.1,
     Configured: AttemptStats{N: 5}, Baseline: AttemptStats{N: 5}, Significant: true},
    {IssueID: "issue-3", SuccessRateDelta: 0.6, ScoreDelta: 0.3,
     Configured: AttemptStats{N: 5}, Baseline: AttemptStats{N: 5}, Significant: true},
}
config := ConfigAnalysis{ConfigUtilization: 0.75, BehaviorDivergence: 0.4}
v := GenerateVerdict(comparisons, config)
assert v.Helped == true
assert v.Confidence == "high"    // sigRatio 3/3 = 1.0 > 0.7, attempts=5 >= 5
assert v.SuccessRateGain == 0.4  // mean of [0.4, 0.2, 0.6]
assert v.ScoreGain == 0.2        // mean of [0.2, 0.1, 0.3]
assert len(v.Recommendations) == 0  // all thresholds satisfied

// Test: "no difference"
comparisons := []PairComparison{
    {SuccessRateDelta: 0.0, ScoreDelta: 0.0,
     Configured: AttemptStats{N: 5}, Baseline: AttemptStats{N: 5}, Significant: false},
}
config := ConfigAnalysis{ConfigUtilization: 0.0, BehaviorDivergence: 0.02}
v := GenerateVerdict(comparisons, config)
assert v.Helped == false
assert v.Confidence == "low"
assert containsRecommendation(v.Recommendations, "unused")      // utilization < 0.3
assert containsRecommendation(v.Recommendations, "redundant")   // divergence < 0.1

// Test: "config hurts performance"
comparisons := []PairComparison{
    {SuccessRateDelta: -0.2, ScoreDelta: -0.1,
     Configured: AttemptStats{N: 5}, Baseline: AttemptStats{N: 5}, Significant: true},
}
v := GenerateVerdict(comparisons, ConfigAnalysis{ConfigUtilization: 0.5, BehaviorDivergence: 0.5})
assert v.Helped == false
assert containsRecommendation(v.Recommendations, "hurting")

// Test: "empty comparisons"
v := GenerateVerdict(nil, ConfigAnalysis{})
assert v.Helped == false
assert v.Confidence == "low"

// Test: "mixed results, medium confidence"
comparisons := make([]PairComparison, 5)
// 3 significant positive, 2 not significant
comparisons[0] = PairComparison{SuccessRateDelta: 0.4, ScoreDelta: 0.2, Significant: true,
    Configured: AttemptStats{N: 4}, Baseline: AttemptStats{N: 4}}
comparisons[1] = PairComparison{SuccessRateDelta: 0.2, ScoreDelta: 0.1, Significant: true,
    Configured: AttemptStats{N: 4}, Baseline: AttemptStats{N: 4}}
comparisons[2] = PairComparison{SuccessRateDelta: 0.2, ScoreDelta: 0.15, Significant: true,
    Configured: AttemptStats{N: 4}, Baseline: AttemptStats{N: 4}}
comparisons[3] = PairComparison{SuccessRateDelta: 0.0, ScoreDelta: 0.05, Significant: false,
    Configured: AttemptStats{N: 4}, Baseline: AttemptStats{N: 4}}
comparisons[4] = PairComparison{SuccessRateDelta: 0.0, ScoreDelta: 0.0, Significant: false,
    Configured: AttemptStats{N: 4}, Baseline: AttemptStats{N: 4}}
v := GenerateVerdict(comparisons, ConfigAnalysis{ConfigUtilization: 0.6, BehaviorDivergence: 0.3})
assert v.Helped == true          // mean SuccessRateGain > 0, mean ScoreGain >= 0
assert v.Confidence == "medium"  // sigRatio = 3/5 = 0.6 > 0.4 but < 0.7, attempts=4 >= 3
```

---

### Stage 4.6: Report Assembly

**What to build:**
- File: `internal/analysis/report.go`
- Type: `Report` struct (as specified in the Packages section)
- Function: `BuildReport(results []*metrics.RunResult, threshold float64) *Report`

**Implementation details:**
- Groups all results by variant ("configured" vs "baseline") and computes aggregate `AttemptStats` for each.
- Calls `CompareAll` to produce per-issue `PairComparison` entries.
- Calls `AnalyzeConfig` with the configured and baseline result slices.
- Calls `GenerateVerdict` with the comparisons and config analysis.
- Populates `ByDifficulty`, `ByTaskType`, `ByLanguage` breakdown maps by grouping results according to the `Difficulty`, `Language`, and `TaskType` fields on `RunResult`. These fields are set by the orchestrator's `makeWorkerFunc` (Phase 3, Stage 3.8) from the `Issue` metadata. For each breakdown group, results are split by variant and compared via `ComparePair`. If no metadata is set, the breakdown maps are empty (not nil).
- Sets `Timestamp` to `time.Now()`, `CorpusSize` to number of distinct `IssueID` values, `TotalRuns` to `len(results)`.

**Pass/fail criteria:**
1. Given 20 results (10 configured + 10 baseline for 2 issues, 5 attempts each): `CorpusSize` == 2, `TotalRuns` == 20.
2. `Comparisons` slice has exactly 2 entries (one per issue).
3. `Configured` and `Baseline` aggregate stats are computed from their respective result subsets.
4. `ConfigAnalysis` is populated (non-zero-valued struct).
5. `Verdict` is populated with a valid `Confidence` string (one of "high", "medium", "low").
6. `Timestamp` is not zero.
7. `ByDifficulty`, `ByTaskType`, `ByLanguage` are non-nil maps (may be empty if metadata not set).
8. Given nil/empty results: returns a `*Report` with zero values, no panic.

**Test cases:**

```go
// Build 20 synthetic results: 2 issues x 2 variants x 5 attempts
results := make([]*metrics.RunResult, 0, 20)
for attempt := 0; attempt < 5; attempt++ {
    results = append(results,
        &metrics.RunResult{IssueID: "issue-1", Variant: "configured", Attempt: attempt,
            Success: true, Score: 0.9, CostUSD: 0.40,
            Difficulty: "easy", Language: "go", TaskType: "bug_fix",
            InitTools: []string{"Read", "Write", "mcp__ctx7__resolve"},
            ToolCallSummary: map[string]int{"Read": 3, "mcp__ctx7__resolve": 1}},
        &metrics.RunResult{IssueID: "issue-1", Variant: "baseline", Attempt: attempt,
            Success: attempt < 3, Score: float64(attempt) * 0.2, CostUSD: 0.35,
            Difficulty: "easy", Language: "go", TaskType: "bug_fix",
            InitTools: []string{"Read", "Write"},
            ToolCallSummary: map[string]int{"Read": 5}},
        &metrics.RunResult{IssueID: "issue-2", Variant: "configured", Attempt: attempt,
            Success: true, Score: 0.8, CostUSD: 0.50,
            Difficulty: "hard", Language: "typescript", TaskType: "feature",
            InitTools: []string{"Read", "Write", "mcp__ctx7__resolve"},
            ToolCallSummary: map[string]int{"Read": 2, "mcp__ctx7__resolve": 2}},
        &metrics.RunResult{IssueID: "issue-2", Variant: "baseline", Attempt: attempt,
            Success: attempt < 2, Score: float64(attempt) * 0.15, CostUSD: 0.45,
            Difficulty: "hard", Language: "typescript", TaskType: "feature",
            InitTools: []string{"Read", "Write"},
            ToolCallSummary: map[string]int{"Read": 4}},
    )
}
report := BuildReport(results, 0.05)
assert report.CorpusSize == 2
assert report.TotalRuns == 20
assert len(report.Comparisons) == 2
assert report.Verdict.Confidence != ""
assert !report.Timestamp.IsZero()
assert report.ByDifficulty != nil
assert len(report.ByDifficulty) == 2       // "easy" and "hard"
assert report.ByDifficulty["easy"].IssueID == ""  // aggregated, no single issue ID
assert report.ByTaskType != nil
assert len(report.ByTaskType) == 2          // "bug_fix" and "feature"
assert report.ByLanguage != nil
assert len(report.ByLanguage) == 2          // "go" and "typescript"

// Edge case: empty results
report := BuildReport(nil, 0.05)
assert report.TotalRuns == 0
assert report.CorpusSize == 0
assert len(report.Comparisons) == 0
```

---

### Stage 4.7: Terminal Formatter

**What to build:**
- File: `internal/analysis/format_terminal.go`
- Function: `FormatTerminal(r *Report, w io.Writer) error`

**Implementation details:**

Produces a human-readable terminal report matching the format shown in poc.md. Must include:
1. Header line with tool name and version
2. Verdict line with `HELPED`/`NEUTRAL`/`HURT` and confidence level
3. Quoted summary text from `Verdict.Summary`
4. Summary table with columns: Metric, Configured, Baseline, Delta
5. Rows for: Success, Score, Cost, Turns, Duration
6. Significance markers (`*`) on statistically significant deltas
7. Per-difficulty breakdown (if present)
8. Configuration Impact section showing added tools, utilization, invocation counts, behavior divergence
9. Recommendations list (if any)

Formatting requirements:
- Table columns must be aligned (use `text/tabwriter` or manual padding)
- Cost values prefixed with `$` and formatted to 2 decimal places
- Percentages formatted with `%` suffix
- Duration formatted in human-readable form (e.g., "45s", "1m30s")

**Pass/fail criteria:**
1. Output written to `bytes.Buffer` is non-empty.
2. Output contains "HELPED" when `Verdict.Helped == true`, "HURT" when `Helped == false && SuccessRateGain < 0`, "NEUTRAL" otherwise.
3. Output contains the confidence level string (e.g., "high confidence").
4. Output contains table header row with "Metric", "Configured", "Baseline", "Delta".
5. Output contains "Success" row with correct percentage values.
6. Output contains "$" followed by cost values.
7. Output contains "*" marker on lines where the comparison is statistically significant.
8. Output contains "Utilization" followed by the percentage value.
9. `FormatTerminal` returns nil error for a valid report.
10. `FormatTerminal` does not panic for a zero-valued `*Report`.

**Test cases:**

```go
// Build a report with known values, format it, scan the output
report := &Report{
    Configured: AttemptStats{N: 5, SuccessRate: 0.8, ScoreMean: 0.85, CostMean: 0.42, TurnsMean: 8.2, DurationMean: 45000},
    Baseline:   AttemptStats{N: 5, SuccessRate: 0.6, ScoreMean: 0.72, CostMean: 0.38, TurnsMean: 9.4, DurationMean: 52000},
    Comparisons: []PairComparison{
        {IssueID: "fix-1", SuccessRateDelta: 0.2, ScoreDelta: 0.13, Significant: true},
    },
    ConfigAnalysis: ConfigAnalysis{
        ConfigAddedTools: []string{"mcp__ctx7__resolve", "Skill"},
        ConfigUtilization: 0.75,
        BehaviorDivergence: 0.34,
        SkillInvocations: 2,
        MCPToolCalls: 3,
    },
    Verdict: Verdict{Helped: true, Confidence: "high",
        SuccessRateGain: 0.2, ScoreGain: 0.13, CostImpact: 0.04,
        Summary: "Configuration improved success rate by 20%."},
}

var buf bytes.Buffer
err := FormatTerminal(report, &buf)
assert err == nil
output := buf.String()
assert strings.Contains(output, "HELPED")
assert strings.Contains(output, "high confidence")
assert strings.Contains(output, "Success")
assert strings.Contains(output, "80%")
assert strings.Contains(output, "60%")
assert strings.Contains(output, "+20%")
assert strings.Contains(output, "$0.42")
assert strings.Contains(output, "*")     // significance marker
assert strings.Contains(output, "75%")   // utilization

// Edge case: zero report
var buf2 bytes.Buffer
err = FormatTerminal(&Report{}, &buf2)
assert err == nil
assert buf2.Len() > 0
```

---

### Stage 4.8: JSON Formatter

**What to build:**
- File: `internal/analysis/format_json.go`
- Function: `FormatJSON(r *Report, w io.Writer) error`

**Implementation details:**
- Serializes the entire `Report` struct to JSON using `json.NewEncoder(w).Encode(r)` with indentation (`json.MarshalIndent` or `encoder.SetIndent`).
- All fields must have `json:"..."` tags on the `Report` and all nested structs (`AttemptStats`, `PairComparison`, `ConfigAnalysis`, `UsageDelta`, `Verdict`). The `TokensMean` anonymous struct inside `AttemptStats` also needs json tags on its `Input` and `Output` fields.
- Time fields should serialize in RFC 3339 format (Go's default for `time.Time`).
- Must produce valid JSON that can be deserialized back into a `Report` struct without data loss.
- The `[2]float64` type for `ScoreCI95` should serialize as a JSON array of two numbers.

**Pass/fail criteria:**
1. Output is valid JSON: `json.Valid(buf.Bytes())` returns true.
2. Round-trip: `json.Unmarshal(output, &Report{})` succeeds without error.
3. Round-trip preserves key values: after unmarshal, `report.Verdict.Helped` matches original, `report.CorpusSize` matches, `report.Configured.SuccessRate` matches.
4. Round-trip preserves `Comparisons` slice length.
5. Round-trip preserves `ConfigAnalysis.ConfigAddedTools` contents.
6. Timestamp round-trips correctly (parse back to `time.Time`, compare with tolerance of 1 second).
7. `ScoreCI95` round-trips as a 2-element array.
8. `FormatJSON` returns nil error for valid report.
9. `FormatJSON` does not panic for zero-valued `*Report`.
10. Output is indented (contains newlines and spaces for readability).

**Test cases:**

```go
// Build a full report
original := &Report{
    Timestamp:  time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
    CorpusSize: 3,
    TotalRuns:  30,
    Configured: AttemptStats{
        N: 15, SuccessRate: 0.8, ScoreMean: 0.85, ScoreStdDev: 0.1,
        ScoreCI95: [2]float64{0.799, 0.901}, CostMean: 0.42,
    },
    Baseline: AttemptStats{N: 15, SuccessRate: 0.6, ScoreMean: 0.72},
    Comparisons: []PairComparison{
        {IssueID: "issue-1", SuccessRateDelta: 0.2, Significant: true},
        {IssueID: "issue-2", SuccessRateDelta: 0.3, Significant: true},
        {IssueID: "issue-3", SuccessRateDelta: 0.0, Significant: false},
    },
    ConfigAnalysis: ConfigAnalysis{
        ConfigAddedTools:   []string{"mcp__ctx7__resolve", "Skill"},
        ConfigUtilization:  0.75,
        BehaviorDivergence: 0.34,
    },
    Verdict: Verdict{
        Helped: true, Confidence: "high",
        SuccessRateGain: 0.2, ScoreGain: 0.13,
        Summary: "Config helped.",
        Recommendations: []string{},
    },
    ByDifficulty: map[string]PairComparison{
        "easy": {SuccessRateDelta: 0.3},
    },
}

var buf bytes.Buffer
err := FormatJSON(original, &buf)
assert err == nil
assert json.Valid(buf.Bytes())

// Round-trip
var decoded Report
err = json.Unmarshal(buf.Bytes(), &decoded)
assert err == nil
assert decoded.CorpusSize == 3
assert decoded.TotalRuns == 30
assert decoded.Verdict.Helped == true
assert decoded.Verdict.Confidence == "high"
assert decoded.Configured.SuccessRate == 0.8
assert decoded.Configured.ScoreCI95 == [2]float64{0.799, 0.901}
assert len(decoded.Comparisons) == 3
assert decoded.Comparisons[0].IssueID == "issue-1"
assert len(decoded.ConfigAnalysis.ConfigAddedTools) == 2
assert decoded.Timestamp.Year() == 2025

// Indentation check
assert strings.Contains(buf.String(), "\n")
assert strings.Contains(buf.String(), "  ")  // indented

// Edge case: empty report
var buf2 bytes.Buffer
err = FormatJSON(&Report{}, &buf2)
assert err == nil
assert json.Valid(buf2.Bytes())
```

## Test Strategy

- **Statistics tests:** Table-driven with known mathematical outputs. E.g., z-test for known proportions, t-test for known sample pairs, mean/stddev for known value sets.
- **Comparison tests:** Build synthetic RunResult slices with controlled success/failure, verify deltas and p-values match hand-calculated values.
- **Config analysis tests:** Build RunResults with known tool call lists, verify utilization and Jaccard distance.
- **Verdict tests:** Feed known comparisons and config analysis, verify verdict logic (helped/confidence/recommendations).
- **Format tests:** Verify terminal output contains expected table rows. Verify JSON output round-trips through `json.Unmarshal`.
