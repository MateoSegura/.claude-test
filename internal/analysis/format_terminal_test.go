package analysis

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFormatTerminal_Helped(t *testing.T) {
	r := &Report{
		Timestamp:  time.Now(),
		CorpusSize: 5,
		TotalRuns:  50,
		Configured: AttemptStats{
			N:            25,
			SuccessRate:  0.8,
			ScoreMean:    0.85,
			CostMean:     0.42,
			TurnsMean:    8.2,
			DurationMean: 45000,
		},
		Baseline: AttemptStats{
			N:            25,
			SuccessRate:  0.6,
			ScoreMean:    0.72,
			CostMean:     0.38,
			TurnsMean:    9.4,
			DurationMean: 52000,
		},
		Comparisons: []PairComparison{
			{
				SuccessRateDelta:  0.2,
				ScoreDelta:        0.13,
				SuccessRatePValue: 0.01,
				ScorePValue:       0.02,
				Significant:       true,
			},
		},
		ConfigAnalysis: ConfigAnalysis{
			ConfigAddedTools:   []string{"mcp__ctx7__resolve", "Skill"},
			ConfigUtilization:  0.75,
			BehaviorDivergence: 0.34,
			MCPToolCalls:       3,
			SkillInvocations:   2,
		},
		Verdict: Verdict{
			Helped:          true,
			Confidence:      "high",
			SuccessRateGain: 0.2,
			ScoreGain:       0.13,
			Summary:         "Configuration improved success rate by 20%.",
		},
		ByDifficulty: make(map[string]PairComparison),
		ByTaskType:   make(map[string]PairComparison),
		ByLanguage:   make(map[string]PairComparison),
	}

	var buf bytes.Buffer
	err := FormatTerminal(r, &buf)
	if err != nil {
		t.Fatalf("FormatTerminal error: %v", err)
	}

	output := buf.String()

	checks := []string{
		"HELPED",
		"high confidence",
		"Success",
		"80%",
		"60%",
		"$",
		"*",
		"75%",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("output missing %q", check)
		}
	}
}

func TestFormatTerminal_EmptyReport(t *testing.T) {
	r := &Report{
		ByDifficulty: make(map[string]PairComparison),
		ByTaskType:   make(map[string]PairComparison),
		ByLanguage:   make(map[string]PairComparison),
	}

	var buf bytes.Buffer
	err := FormatTerminal(r, &buf)
	if err != nil {
		t.Fatalf("FormatTerminal error: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected non-empty output for empty report")
	}
}

func TestFormatTerminal_Hurt(t *testing.T) {
	r := &Report{
		Verdict: Verdict{
			Helped:          false,
			Confidence:      "high",
			SuccessRateGain: -0.2,
			Summary:         "Configuration decreased success rate by 20%.",
		},
		ByDifficulty: make(map[string]PairComparison),
		ByTaskType:   make(map[string]PairComparison),
		ByLanguage:   make(map[string]PairComparison),
	}

	var buf bytes.Buffer
	err := FormatTerminal(r, &buf)
	if err != nil {
		t.Fatalf("FormatTerminal error: %v", err)
	}

	if !strings.Contains(buf.String(), "HURT") {
		t.Error("expected output to contain HURT")
	}
}

func TestFormatTerminal_Neutral(t *testing.T) {
	r := &Report{
		Verdict: Verdict{
			Helped:          false,
			Confidence:      "low",
			SuccessRateGain: 0,
			Summary:         "Configuration had no meaningful impact on performance.",
		},
		ByDifficulty: make(map[string]PairComparison),
		ByTaskType:   make(map[string]PairComparison),
		ByLanguage:   make(map[string]PairComparison),
	}

	var buf bytes.Buffer
	err := FormatTerminal(r, &buf)
	if err != nil {
		t.Fatalf("FormatTerminal error: %v", err)
	}

	if !strings.Contains(buf.String(), "NEUTRAL") {
		t.Error("expected output to contain NEUTRAL")
	}
}
