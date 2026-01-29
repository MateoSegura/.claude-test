package analysis

import (
	"sort"

	"github.com/MateoSegura/.claude-test/internal/metrics"
)

// PairComparison holds the comparison between configured and baseline runs
// for a single issue.
type PairComparison struct {
	IssueID           string       `json:"issue_id"`
	Configured        AttemptStats `json:"configured"`
	Baseline          AttemptStats `json:"baseline"`
	SuccessRateDelta  float64      `json:"success_rate_delta"`
	ScoreDelta        float64      `json:"score_delta"`
	CostDelta         float64      `json:"cost_delta"`
	TurnsDelta        float64      `json:"turns_delta"`
	SuccessRatePValue float64      `json:"success_rate_p_value"`
	ScorePValue       float64      `json:"score_p_value"`
	Significant       bool         `json:"significant"`
}

// ComparePair computes stats for configured and baseline runs, then calculates
// deltas and statistical significance. Deltas are configured - baseline.
// Significant is true if any p-value is below the threshold.
func ComparePair(issueID string, configured, baseline []*metrics.RunResult, threshold float64) PairComparison {
	cfgStats := ComputeStats(configured)
	baseStats := ComputeStats(baseline)

	pc := PairComparison{
		IssueID:          issueID,
		Configured:       cfgStats,
		Baseline:         baseStats,
		SuccessRateDelta: cfgStats.SuccessRate - baseStats.SuccessRate,
		ScoreDelta:       cfgStats.ScoreMean - baseStats.ScoreMean,
		CostDelta:        cfgStats.CostMean - baseStats.CostMean,
		TurnsDelta:       cfgStats.TurnsMean - baseStats.TurnsMean,
	}

	// Success rate p-value via z-test on proportions.
	pc.SuccessRatePValue = ZTestTwoProportions(
		cfgStats.SuccessRate, float64(cfgStats.N),
		baseStats.SuccessRate, float64(baseStats.N),
	)

	// Score p-value via paired t-test on score slices.
	cfgScores := extractScores(configured)
	baseScores := extractScores(baseline)

	// Use min length if different.
	minLen := len(cfgScores)
	if len(baseScores) < minLen {
		minLen = len(baseScores)
	}
	if minLen > 0 {
		pc.ScorePValue = TTestPaired(cfgScores[:minLen], baseScores[:minLen])
	} else {
		pc.ScorePValue = 1.0
	}

	pc.Significant = pc.SuccessRatePValue < threshold || pc.ScorePValue < threshold

	return pc
}

// extractScores returns the Score values from a slice of RunResults.
func extractScores(results []*metrics.RunResult) []float64 {
	scores := make([]float64, len(results))
	for i, r := range results {
		scores[i] = r.Score
	}
	return scores
}

// CompareAll groups results by IssueID and Variant, then calls ComparePair
// for each issue that has both "configured" and "baseline" variants.
func CompareAll(results []*metrics.RunResult, threshold float64) []PairComparison {
	// Group by IssueID then Variant.
	type key struct {
		IssueID string
		Variant string
	}
	groups := make(map[key][]*metrics.RunResult)
	issueSet := make(map[string]bool)

	for _, r := range results {
		k := key{IssueID: r.IssueID, Variant: r.Variant}
		groups[k] = append(groups[k], r)
		issueSet[r.IssueID] = true
	}

	// Sort issue IDs for deterministic output.
	var issueIDs []string
	for id := range issueSet {
		issueIDs = append(issueIDs, id)
	}
	sort.Strings(issueIDs)

	var comparisons []PairComparison
	for _, id := range issueIDs {
		configured := groups[key{IssueID: id, Variant: "configured"}]
		baseline := groups[key{IssueID: id, Variant: "baseline"}]

		if len(configured) == 0 || len(baseline) == 0 {
			continue
		}

		comparisons = append(comparisons, ComparePair(id, configured, baseline, threshold))
	}

	return comparisons
}
