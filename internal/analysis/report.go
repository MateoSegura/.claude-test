package analysis

import (
	"sort"
	"time"

	"github.com/MateoSegura/.claude-test/internal/metrics"
)

// Report is the top-level structure for a configbench benchmark report.
type Report struct {
	Timestamp      time.Time                 `json:"timestamp"`
	CorpusSize     int                       `json:"corpus_size"`
	TotalRuns      int                       `json:"total_runs"`
	Configured     AttemptStats              `json:"configured"`
	Baseline       AttemptStats              `json:"baseline"`
	Comparisons    []PairComparison          `json:"comparisons"`
	ConfigAnalysis ConfigAnalysis            `json:"config_analysis"`
	Verdict        Verdict                   `json:"verdict"`
	ByDifficulty   map[string]PairComparison `json:"by_difficulty"`
	ByTaskType     map[string]PairComparison `json:"by_task_type"`
	ByLanguage     map[string]PairComparison `json:"by_language"`
}

// BuildReport constructs a full Report from a set of RunResults.
func BuildReport(results []*metrics.RunResult, threshold float64) *Report {
	r := &Report{
		Timestamp:    time.Now(),
		ByDifficulty: make(map[string]PairComparison),
		ByTaskType:   make(map[string]PairComparison),
		ByLanguage:   make(map[string]PairComparison),
	}

	if len(results) == 0 {
		return r
	}

	// Split results by variant.
	var configuredResults, baselineResults []*metrics.RunResult
	for _, res := range results {
		switch res.Variant {
		case "configured":
			configuredResults = append(configuredResults, res)
		case "baseline":
			baselineResults = append(baselineResults, res)
		}
	}

	r.Configured = ComputeStats(configuredResults)
	r.Baseline = ComputeStats(baselineResults)
	r.Comparisons = CompareAll(results, threshold)
	r.ConfigAnalysis = AnalyzeConfig(configuredResults, baselineResults)
	r.Verdict = GenerateVerdict(r.Comparisons, r.ConfigAnalysis)

	// Count distinct IssueIDs.
	issueSet := make(map[string]bool)
	for _, res := range results {
		issueSet[res.IssueID] = true
	}
	r.CorpusSize = len(issueSet)
	r.TotalRuns = len(results)

	// Build breakdowns by grouping results, then comparing within each group.
	r.ByDifficulty = buildBreakdown(results, func(res *metrics.RunResult) string { return res.Difficulty }, threshold)
	r.ByTaskType = buildBreakdown(results, func(res *metrics.RunResult) string { return res.TaskType }, threshold)
	r.ByLanguage = buildBreakdown(results, func(res *metrics.RunResult) string { return res.Language }, threshold)

	return r
}

// buildBreakdown groups results by the key returned from keyFn, splits each
// group by variant, and produces a PairComparison for each group.
func buildBreakdown(results []*metrics.RunResult, keyFn func(*metrics.RunResult) string, threshold float64) map[string]PairComparison {
	type variantKey struct {
		group   string
		variant string
	}

	groups := make(map[variantKey][]*metrics.RunResult)
	groupSet := make(map[string]bool)

	for _, res := range results {
		k := keyFn(res)
		if k == "" {
			continue
		}
		groupSet[k] = true
		vk := variantKey{group: k, variant: res.Variant}
		groups[vk] = append(groups[vk], res)
	}

	// Sort group keys for deterministic iteration.
	var keys []string
	for k := range groupSet {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	breakdown := make(map[string]PairComparison)
	for _, k := range keys {
		configured := groups[variantKey{group: k, variant: "configured"}]
		baseline := groups[variantKey{group: k, variant: "baseline"}]
		if len(configured) == 0 || len(baseline) == 0 {
			continue
		}
		breakdown[k] = ComparePair("", configured, baseline, threshold)
	}

	return breakdown
}
