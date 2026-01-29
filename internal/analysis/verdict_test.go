package analysis

import (
	"strings"
	"testing"
)

func TestVerdict_HighConfidence(t *testing.T) {
	comparisons := []PairComparison{
		{
			IssueID:          "issue-1",
			SuccessRateDelta: 0.4,
			ScoreDelta:       0.2,
			CostDelta:        0.02,
			Significant:      true,
			Configured:       AttemptStats{N: 5},
			Baseline:         AttemptStats{N: 5},
		},
		{
			IssueID:          "issue-2",
			SuccessRateDelta: 0.2,
			ScoreDelta:       0.1,
			CostDelta:        0.01,
			Significant:      true,
			Configured:       AttemptStats{N: 5},
			Baseline:         AttemptStats{N: 5},
		},
		{
			IssueID:          "issue-3",
			SuccessRateDelta: 0.6,
			ScoreDelta:       0.3,
			CostDelta:        0.03,
			Significant:      true,
			Configured:       AttemptStats{N: 5},
			Baseline:         AttemptStats{N: 5},
		},
	}
	config := ConfigAnalysis{
		ConfigUtilization:  0.8,
		BehaviorDivergence: 0.5,
	}

	v := GenerateVerdict(comparisons, config)

	if !v.Helped {
		t.Error("expected Helped=true")
	}
	if v.Confidence != "high" {
		t.Errorf("Confidence = %q, want %q", v.Confidence, "high")
	}
	// Mean of 0.4, 0.2, 0.6 = 0.4
	if !approxEqual(v.SuccessRateGain, 0.4) {
		t.Errorf("SuccessRateGain = %v, want 0.4", v.SuccessRateGain)
	}
	// Mean of 0.2, 0.1, 0.3 = 0.2
	if !approxEqual(v.ScoreGain, 0.2) {
		t.Errorf("ScoreGain = %v, want 0.2", v.ScoreGain)
	}
	if len(v.Recommendations) != 0 {
		t.Errorf("expected no recommendations, got %v", v.Recommendations)
	}
}

func TestVerdict_NoDifference(t *testing.T) {
	comparisons := []PairComparison{
		{
			IssueID:          "issue-1",
			SuccessRateDelta: 0.0,
			ScoreDelta:       0.0,
			CostDelta:        0.0,
			Significant:      false,
			Configured:       AttemptStats{N: 5},
			Baseline:         AttemptStats{N: 5},
		},
	}
	config := ConfigAnalysis{
		ConfigUtilization:  0.0,
		BehaviorDivergence: 0.02,
	}

	v := GenerateVerdict(comparisons, config)

	if v.Helped {
		t.Error("expected Helped=false")
	}
	if v.Confidence != "low" {
		t.Errorf("Confidence = %q, want %q", v.Confidence, "low")
	}

	hasUtilization := false
	hasDivergence := false
	for _, r := range v.Recommendations {
		if strings.Contains(r, "utilization") {
			hasUtilization = true
		}
		if strings.Contains(r, "divergence") {
			hasDivergence = true
		}
	}
	if !hasUtilization {
		t.Error("expected recommendation about utilization")
	}
	if !hasDivergence {
		t.Error("expected recommendation about divergence")
	}
}

func TestVerdict_HurtsPerformance(t *testing.T) {
	comparisons := []PairComparison{
		{
			IssueID:          "issue-1",
			SuccessRateDelta: -0.2,
			ScoreDelta:       -0.1,
			CostDelta:        0.05,
			Significant:      true,
			Configured:       AttemptStats{N: 5},
			Baseline:         AttemptStats{N: 5},
		},
	}
	config := ConfigAnalysis{
		ConfigUtilization:  0.5,
		BehaviorDivergence: 0.5,
	}

	v := GenerateVerdict(comparisons, config)

	if v.Helped {
		t.Error("expected Helped=false")
	}

	hasHurting := false
	for _, r := range v.Recommendations {
		if strings.Contains(r, "hurting") {
			hasHurting = true
		}
	}
	if !hasHurting {
		t.Error("expected recommendation about hurting performance")
	}
}

func TestVerdict_EmptyComparisons(t *testing.T) {
	v := GenerateVerdict(nil, ConfigAnalysis{})

	if v.Helped {
		t.Error("expected Helped=false")
	}
	if v.Confidence != "low" {
		t.Errorf("Confidence = %q, want %q", v.Confidence, "low")
	}
}

func TestVerdict_MediumConfidence(t *testing.T) {
	comparisons := []PairComparison{
		{SuccessRateDelta: 0.3, ScoreDelta: 0.1, Significant: true, Configured: AttemptStats{N: 4}, Baseline: AttemptStats{N: 4}},
		{SuccessRateDelta: 0.2, ScoreDelta: 0.15, Significant: true, Configured: AttemptStats{N: 4}, Baseline: AttemptStats{N: 4}},
		{SuccessRateDelta: 0.1, ScoreDelta: 0.05, Significant: true, Configured: AttemptStats{N: 4}, Baseline: AttemptStats{N: 4}},
		{SuccessRateDelta: 0.1, ScoreDelta: 0.02, Significant: false, Configured: AttemptStats{N: 4}, Baseline: AttemptStats{N: 4}},
		{SuccessRateDelta: 0.05, ScoreDelta: 0.01, Significant: false, Configured: AttemptStats{N: 4}, Baseline: AttemptStats{N: 4}},
	}
	config := ConfigAnalysis{
		ConfigUtilization:  0.8,
		BehaviorDivergence: 0.5,
	}

	v := GenerateVerdict(comparisons, config)

	if !v.Helped {
		t.Error("expected Helped=true")
	}
	// sigRatio = 3/5 = 0.6 > 0.4 but <= 0.7, minAttempts = 4 >= 3
	if v.Confidence != "medium" {
		t.Errorf("Confidence = %q, want %q", v.Confidence, "medium")
	}
}

func TestVerdict_PositiveSuccessNegativeScore(t *testing.T) {
	comparisons := []PairComparison{
		{
			SuccessRateDelta: 0.2,
			ScoreDelta:       -0.1,
			Significant:      true,
			Configured:       AttemptStats{N: 5},
			Baseline:         AttemptStats{N: 5},
		},
	}
	config := ConfigAnalysis{
		ConfigUtilization:  0.8,
		BehaviorDivergence: 0.5,
	}

	v := GenerateVerdict(comparisons, config)

	if v.Helped {
		t.Error("expected Helped=false when ScoreGain < 0")
	}
}
