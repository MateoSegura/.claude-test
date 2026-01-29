package analysis

import (
	"testing"

	claude "github.com/MateoSegura/claudesdk-go"
	"github.com/MateoSegura/.claude-test/internal/metrics"
)

func makeVariantResult(issueID, variant string, success bool, score float64) *metrics.RunResult {
	return &metrics.RunResult{
		IssueID:    issueID,
		Variant:    variant,
		Success:    success,
		Score:      score,
		CostUSD:    0.10,
		NumTurns:   5,
		DurationMS: 1000,
		Usage: &claude.Usage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}
}

func TestComparePair_ClearImprovement(t *testing.T) {
	configured := []*metrics.RunResult{
		makeVariantResult("issue-1", "configured", true, 0.9),
		makeVariantResult("issue-1", "configured", true, 0.85),
		makeVariantResult("issue-1", "configured", true, 0.88),
		makeVariantResult("issue-1", "configured", true, 0.92),
		makeVariantResult("issue-1", "configured", true, 0.87),
	}
	baseline := []*metrics.RunResult{
		makeVariantResult("issue-1", "baseline", true, 0.5),
		makeVariantResult("issue-1", "baseline", false, 0.3),
		makeVariantResult("issue-1", "baseline", true, 0.4),
		makeVariantResult("issue-1", "baseline", false, 0.2),
		makeVariantResult("issue-1", "baseline", true, 0.45),
	}

	pc := ComparePair("issue-1", configured, baseline, 0.05)

	if pc.IssueID != "issue-1" {
		t.Errorf("IssueID = %q, want %q", pc.IssueID, "issue-1")
	}
	if !approxEqual(pc.Configured.SuccessRate, 1.0) {
		t.Errorf("Configured.SuccessRate = %v, want 1.0", pc.Configured.SuccessRate)
	}
	if !approxEqual(pc.Baseline.SuccessRate, 0.6) {
		t.Errorf("Baseline.SuccessRate = %v, want 0.6", pc.Baseline.SuccessRate)
	}
	if pc.SuccessRateDelta <= 0 {
		t.Errorf("SuccessRateDelta = %v, want > 0", pc.SuccessRateDelta)
	}
	if pc.ScoreDelta <= 0 {
		t.Errorf("ScoreDelta = %v, want > 0", pc.ScoreDelta)
	}
	if !pc.Significant {
		t.Error("expected Significant=true for clear improvement")
	}
}

func TestComparePair_IdenticalResults(t *testing.T) {
	a := []*metrics.RunResult{
		makeVariantResult("issue-2", "configured", true, 0.7),
		makeVariantResult("issue-2", "configured", true, 0.7),
		makeVariantResult("issue-2", "configured", true, 0.7),
	}
	b := []*metrics.RunResult{
		makeVariantResult("issue-2", "baseline", true, 0.7),
		makeVariantResult("issue-2", "baseline", true, 0.7),
		makeVariantResult("issue-2", "baseline", true, 0.7),
	}

	pc := ComparePair("issue-2", a, b, 0.05)

	if !approxEqual(pc.SuccessRateDelta, 0.0) {
		t.Errorf("SuccessRateDelta = %v, want 0.0", pc.SuccessRateDelta)
	}
	if !approxEqual(pc.ScoreDelta, 0.0) {
		t.Errorf("ScoreDelta = %v, want 0.0", pc.ScoreDelta)
	}
	if pc.Significant {
		t.Error("expected Significant=false for identical results")
	}
}

func TestCompareAll_Grouping(t *testing.T) {
	var results []*metrics.RunResult
	for i := 0; i < 5; i++ {
		results = append(results, makeVariantResult("issue-1", "configured", true, 0.9))
	}
	for i := 0; i < 5; i++ {
		results = append(results, makeVariantResult("issue-1", "baseline", true, 0.5))
	}

	comparisons := CompareAll(results, 0.05)

	if len(comparisons) != 1 {
		t.Fatalf("len(comparisons) = %d, want 1", len(comparisons))
	}
	if comparisons[0].IssueID != "issue-1" {
		t.Errorf("IssueID = %q, want %q", comparisons[0].IssueID, "issue-1")
	}
	if comparisons[0].Configured.N != 5 {
		t.Errorf("Configured.N = %d, want 5", comparisons[0].Configured.N)
	}
	if comparisons[0].Baseline.N != 5 {
		t.Errorf("Baseline.N = %d, want 5", comparisons[0].Baseline.N)
	}
}

func TestCompareAll_MultipleIssues(t *testing.T) {
	var results []*metrics.RunResult
	for _, id := range []string{"issue-A", "issue-B", "issue-C"} {
		for i := 0; i < 3; i++ {
			results = append(results, makeVariantResult(id, "configured", true, 0.8))
			results = append(results, makeVariantResult(id, "baseline", true, 0.6))
		}
	}

	comparisons := CompareAll(results, 0.05)

	if len(comparisons) != 3 {
		t.Fatalf("len(comparisons) = %d, want 3", len(comparisons))
	}
}

func TestCompareAll_SingleVariant(t *testing.T) {
	results := []*metrics.RunResult{
		makeVariantResult("issue-X", "configured", true, 0.9),
		makeVariantResult("issue-X", "configured", true, 0.8),
	}

	comparisons := CompareAll(results, 0.05)

	if len(comparisons) != 0 {
		t.Errorf("len(comparisons) = %d, want 0 (missing baseline)", len(comparisons))
	}
}
