package analysis

import "fmt"

// Verdict summarizes whether a configuration helped or hurt benchmark
// performance, along with confidence level and recommendations.
type Verdict struct {
	Helped          bool     `json:"helped"`
	Confidence      string   `json:"confidence"`
	SuccessRateGain float64  `json:"success_rate_gain"`
	ScoreGain       float64  `json:"score_gain"`
	CostImpact      float64  `json:"cost_impact"`
	Summary         string   `json:"summary"`
	Recommendations []string `json:"recommendations"`
}

// GenerateVerdict analyzes pair comparisons and config analysis to produce
// a final verdict on whether the configuration helped.
func GenerateVerdict(comparisons []PairComparison, config ConfigAnalysis) Verdict {
	if len(comparisons) == 0 {
		return Verdict{
			Helped:     false,
			Confidence: "low",
			Summary:    "No comparisons available",
		}
	}

	// Compute mean deltas across all comparisons.
	successDeltas := make([]float64, len(comparisons))
	scoreDeltas := make([]float64, len(comparisons))
	costDeltas := make([]float64, len(comparisons))
	for i, c := range comparisons {
		successDeltas[i] = c.SuccessRateDelta
		scoreDeltas[i] = c.ScoreDelta
		costDeltas[i] = c.CostDelta
	}

	v := Verdict{
		SuccessRateGain: Mean(successDeltas),
		ScoreGain:       Mean(scoreDeltas),
		CostImpact:      Mean(costDeltas),
	}

	// Helped requires both positive success rate gain and non-negative score gain.
	v.Helped = v.SuccessRateGain > 0 && v.ScoreGain >= 0

	// Confidence based on significance ratio and minimum sample size.
	var sigCount int
	minAttempts := comparisons[0].Configured.N
	if comparisons[0].Baseline.N < minAttempts {
		minAttempts = comparisons[0].Baseline.N
	}
	for _, c := range comparisons {
		if c.Significant {
			sigCount++
		}
		if c.Configured.N < minAttempts {
			minAttempts = c.Configured.N
		}
		if c.Baseline.N < minAttempts {
			minAttempts = c.Baseline.N
		}
	}

	sigRatio := float64(sigCount) / float64(len(comparisons))

	switch {
	case sigRatio > 0.7 && minAttempts >= 5:
		v.Confidence = "high"
	case sigRatio > 0.4 && minAttempts >= 3:
		v.Confidence = "medium"
	default:
		v.Confidence = "low"
	}

	// Summary.
	if v.Helped {
		v.Summary = fmt.Sprintf("Configuration improved success rate by %.0f%%.", v.SuccessRateGain*100)
	} else if v.SuccessRateGain < 0 {
		v.Summary = fmt.Sprintf("Configuration decreased success rate by %.0f%%.", -v.SuccessRateGain*100)
	} else {
		v.Summary = "Configuration had no meaningful impact on performance."
	}

	// Recommendations.
	if config.ConfigUtilization < 0.3 {
		v.Recommendations = append(v.Recommendations,
			"Low tool utilization: less than 30% of added tools are being used")
	}
	if config.BehaviorDivergence < 0.1 {
		v.Recommendations = append(v.Recommendations,
			"Low behavior divergence: configuration has minimal impact on tool usage patterns")
	}
	if !v.Helped && v.SuccessRateGain < 0 {
		v.Recommendations = append(v.Recommendations,
			"Configuration appears to be hurting performance: success rate decreased")
	}

	return v
}
