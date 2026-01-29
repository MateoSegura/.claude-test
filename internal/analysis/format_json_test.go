package analysis

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFormatJSON_Valid(t *testing.T) {
	r := &Report{
		Timestamp:  time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		CorpusSize: 5,
		TotalRuns:  50,
		Configured: AttemptStats{
			N:            25,
			SuccessRate:  0.8,
			ScoreMean:    0.85,
			ScoreStdDev:  0.05,
			ScoreCI95:    [2]float64{0.83, 0.87},
			CostMean:     0.42,
			TurnsMean:    8.2,
			DurationMean: 45000,
		},
		Baseline: AttemptStats{
			N:           25,
			SuccessRate: 0.6,
			ScoreMean:   0.72,
			CostMean:    0.38,
		},
		Comparisons: []PairComparison{
			{
				IssueID:          "issue-1",
				SuccessRateDelta: 0.2,
				ScoreDelta:       0.13,
				Significant:      true,
				Configured:       AttemptStats{N: 5},
				Baseline:         AttemptStats{N: 5},
			},
			{
				IssueID:          "issue-2",
				SuccessRateDelta: 0.1,
				ScoreDelta:       0.05,
				Configured:       AttemptStats{N: 5},
				Baseline:         AttemptStats{N: 5},
			},
		},
		ConfigAnalysis: ConfigAnalysis{
			ConfigAddedTools:   []string{"mcp__ctx7__resolve", "Skill"},
			ConfigUtilization:  0.75,
			BehaviorDivergence: 0.34,
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
	err := FormatJSON(r, &buf)
	if err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}

	data := buf.Bytes()

	// Verify valid JSON.
	if !json.Valid(data) {
		t.Fatal("output is not valid JSON")
	}

	// Round-trip unmarshal.
	var decoded Report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.CorpusSize != 5 {
		t.Errorf("CorpusSize = %d, want 5", decoded.CorpusSize)
	}
	if decoded.Verdict.Helped != true {
		t.Error("Verdict.Helped = false, want true")
	}
	if !approxEqual(decoded.Configured.SuccessRate, 0.8) {
		t.Errorf("Configured.SuccessRate = %v, want 0.8", decoded.Configured.SuccessRate)
	}
	if !approxEqual(decoded.Configured.ScoreCI95[0], 0.83) {
		t.Errorf("Configured.ScoreCI95[0] = %v, want 0.83", decoded.Configured.ScoreCI95[0])
	}
	if len(decoded.Comparisons) != 2 {
		t.Errorf("len(Comparisons) = %d, want 2", len(decoded.Comparisons))
	}
	if len(decoded.ConfigAnalysis.ConfigAddedTools) != 2 {
		t.Errorf("len(ConfigAddedTools) = %d, want 2", len(decoded.ConfigAnalysis.ConfigAddedTools))
	}
	if decoded.Timestamp.Year() != 2026 {
		t.Errorf("Timestamp.Year() = %d, want 2026", decoded.Timestamp.Year())
	}
}

func TestFormatJSON_EmptyReport(t *testing.T) {
	r := &Report{
		ByDifficulty: make(map[string]PairComparison),
		ByTaskType:   make(map[string]PairComparison),
		ByLanguage:   make(map[string]PairComparison),
	}

	var buf bytes.Buffer
	err := FormatJSON(r, &buf)
	if err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}

	if !json.Valid(buf.Bytes()) {
		t.Fatal("output is not valid JSON")
	}
}

func TestFormatJSON_Indented(t *testing.T) {
	r := &Report{
		CorpusSize:   1,
		ByDifficulty: make(map[string]PairComparison),
		ByTaskType:   make(map[string]PairComparison),
		ByLanguage:   make(map[string]PairComparison),
	}

	var buf bytes.Buffer
	err := FormatJSON(r, &buf)
	if err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "\n") {
		t.Error("expected output to contain newlines")
	}
	if !strings.Contains(output, "  ") {
		t.Error("expected output to contain indentation spaces")
	}
}
