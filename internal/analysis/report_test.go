package analysis

import (
	"testing"

	claude "github.com/MateoSegura/claudesdk-go"
	"github.com/MateoSegura/.claude-test/internal/metrics"
)

func makeFullResult(issueID, variant string, attempt int, success bool, score float64, difficulty, language, taskType string) *metrics.RunResult {
	return &metrics.RunResult{
		IssueID:    issueID,
		Variant:    variant,
		Attempt:    attempt,
		Success:    success,
		Score:      score,
		CostUSD:    0.10,
		NumTurns:   5,
		DurationMS: 1000,
		Usage: &claude.Usage{
			InputTokens:  100,
			OutputTokens: 50,
		},
		Difficulty: difficulty,
		Language:   language,
		TaskType:   taskType,
	}
}

func TestBuildReport_Full(t *testing.T) {
	var results []*metrics.RunResult
	for _, issueID := range []string{"issue-1", "issue-2"} {
		for i := 0; i < 5; i++ {
			results = append(results,
				makeFullResult(issueID, "configured", i, true, 0.9, "easy", "go", "bug_fix"),
				makeFullResult(issueID, "baseline", i, i < 3, 0.5, "easy", "go", "bug_fix"),
			)
		}
	}

	r := BuildReport(results, 0.05)

	if r.CorpusSize != 2 {
		t.Errorf("CorpusSize = %d, want 2", r.CorpusSize)
	}
	if r.TotalRuns != 20 {
		t.Errorf("TotalRuns = %d, want 20", r.TotalRuns)
	}
	if len(r.Comparisons) != 2 {
		t.Errorf("len(Comparisons) = %d, want 2", len(r.Comparisons))
	}
	if r.Verdict.Confidence == "" {
		t.Error("Verdict.Confidence is empty")
	}
	if r.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}
	if r.ByDifficulty == nil {
		t.Error("ByDifficulty is nil")
	}
	if r.ByTaskType == nil {
		t.Error("ByTaskType is nil")
	}
	if r.ByLanguage == nil {
		t.Error("ByLanguage is nil")
	}
}

func TestBuildReport_Empty(t *testing.T) {
	r := BuildReport(nil, 0.05)

	if r.TotalRuns != 0 {
		t.Errorf("TotalRuns = %d, want 0", r.TotalRuns)
	}
	if r.CorpusSize != 0 {
		t.Errorf("CorpusSize = %d, want 0", r.CorpusSize)
	}
	if len(r.Comparisons) != 0 {
		t.Errorf("len(Comparisons) = %d, want 0", len(r.Comparisons))
	}
}

func TestBuildReport_Breakdowns(t *testing.T) {
	var results []*metrics.RunResult
	for _, cfg := range []struct {
		issueID    string
		difficulty string
		language   string
		taskType   string
	}{
		{"issue-1", "easy", "go", "bug_fix"},
		{"issue-2", "hard", "typescript", "feature"},
	} {
		for i := 0; i < 3; i++ {
			results = append(results,
				makeFullResult(cfg.issueID, "configured", i, true, 0.9, cfg.difficulty, cfg.language, cfg.taskType),
				makeFullResult(cfg.issueID, "baseline", i, true, 0.6, cfg.difficulty, cfg.language, cfg.taskType),
			)
		}
	}

	r := BuildReport(results, 0.05)

	if len(r.ByDifficulty) != 2 {
		t.Errorf("len(ByDifficulty) = %d, want 2", len(r.ByDifficulty))
	}
	if len(r.ByLanguage) != 2 {
		t.Errorf("len(ByLanguage) = %d, want 2", len(r.ByLanguage))
	}
	if len(r.ByTaskType) != 2 {
		t.Errorf("len(ByTaskType) = %d, want 2", len(r.ByTaskType))
	}
}
