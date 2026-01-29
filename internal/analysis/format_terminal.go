package analysis

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// FormatTerminal writes a human-readable benchmark report to w.
func FormatTerminal(r *Report, w io.Writer) error {
	if r == nil {
		r = &Report{}
	}

	// Header.
	line := strings.Repeat("=", 43)
	fmt.Fprintf(w, "%s\n", line)
	fmt.Fprintf(w, "  configbench — Configuration Benchmark Report\n")
	fmt.Fprintf(w, "%s\n\n", line)

	// Verdict line.
	verdictLabel := "NEUTRAL"
	if r.Verdict.Helped {
		verdictLabel = "HELPED"
	} else if r.Verdict.SuccessRateGain < 0 {
		verdictLabel = "HURT"
	}
	fmt.Fprintf(w, "  Verdict: %s (%s confidence)\n", verdictLabel, r.Verdict.Confidence)
	if r.Verdict.Summary != "" {
		fmt.Fprintf(w, "  %q\n", r.Verdict.Summary)
	}
	fmt.Fprintln(w)

	// Metrics table.
	successDelta := r.Configured.SuccessRate - r.Baseline.SuccessRate
	scoreDelta := r.Configured.ScoreMean - r.Baseline.ScoreMean
	costDelta := r.Configured.CostMean - r.Baseline.CostMean
	turnsDelta := r.Configured.TurnsMean - r.Baseline.TurnsMean
	durationDelta := r.Configured.DurationMean - r.Baseline.DurationMean

	// Determine significance markers from comparisons.
	successSig := false
	scoreSig := false
	for _, c := range r.Comparisons {
		if c.SuccessRatePValue < 0.05 {
			successSig = true
		}
		if c.ScorePValue < 0.05 {
			scoreSig = true
		}
	}

	fmt.Fprintf(w, "  %-12s %-12s %-10s %s\n", "Metric", "Configured", "Baseline", "Delta")
	fmt.Fprintf(w, "  %-12s %-12s %-10s %s\n",
		strings.Repeat("-", 12), strings.Repeat("-", 12), strings.Repeat("-", 10), strings.Repeat("-", 10))

	fmt.Fprintf(w, "  %-12s %-12s %-10s %s\n",
		"Success",
		fmt.Sprintf("%.0f%%", r.Configured.SuccessRate*100),
		fmt.Sprintf("%.0f%%", r.Baseline.SuccessRate*100),
		formatDeltaPct(successDelta, successSig))

	fmt.Fprintf(w, "  %-12s %-12s %-10s %s\n",
		"Score",
		fmt.Sprintf("%.2f", r.Configured.ScoreMean),
		fmt.Sprintf("%.2f", r.Baseline.ScoreMean),
		formatDeltaFloat(scoreDelta, scoreSig))

	fmt.Fprintf(w, "  %-12s %-12s %-10s %s\n",
		"Cost",
		fmt.Sprintf("$%.2f", r.Configured.CostMean),
		fmt.Sprintf("$%.2f", r.Baseline.CostMean),
		formatDeltaCost(costDelta))

	fmt.Fprintf(w, "  %-12s %-12s %-10s %s\n",
		"Turns",
		fmt.Sprintf("%.1f", r.Configured.TurnsMean),
		fmt.Sprintf("%.1f", r.Baseline.TurnsMean),
		formatDeltaFloat(turnsDelta, false))

	fmt.Fprintf(w, "  %-12s %-12s %-10s %s\n",
		"Duration",
		formatDuration(r.Configured.DurationMean),
		formatDuration(r.Baseline.DurationMean),
		formatDeltaDuration(durationDelta))

	if successSig || scoreSig {
		fmt.Fprintf(w, "  * statistically significant (p < 0.05)\n")
	}
	fmt.Fprintln(w)

	// Configuration impact.
	fmt.Fprintf(w, "  Configuration Impact:\n")
	if len(r.ConfigAnalysis.ConfigAddedTools) > 0 {
		fmt.Fprintf(w, "    Added tools: %s\n", strings.Join(r.ConfigAnalysis.ConfigAddedTools, ", "))
	} else {
		fmt.Fprintf(w, "    Added tools: none\n")
	}
	fmt.Fprintf(w, "    Utilization: %.0f%%\n", r.ConfigAnalysis.ConfigUtilization*100)
	fmt.Fprintf(w, "    MCP calls: %d | Skill calls: %d | Agent calls: %d\n",
		r.ConfigAnalysis.MCPToolCalls, r.ConfigAnalysis.SkillInvocations, r.ConfigAnalysis.AgentInvocations)
	fmt.Fprintf(w, "    Behavior divergence: %.0f%%\n", r.ConfigAnalysis.BehaviorDivergence*100)
	fmt.Fprintln(w)

	// Recommendations.
	fmt.Fprintf(w, "  Recommendations:\n")
	if len(r.Verdict.Recommendations) == 0 {
		fmt.Fprintf(w, "    None — configuration is performing well.\n")
	} else {
		for _, rec := range r.Verdict.Recommendations {
			fmt.Fprintf(w, "    • %s\n", rec)
		}
	}

	return nil
}

func formatDeltaPct(delta float64, sig bool) string {
	sign := "+"
	if delta < 0 {
		sign = ""
	}
	s := fmt.Sprintf("%s%.0f%%", sign, delta*100)
	if sig {
		s += " *"
	}
	return s
}

func formatDeltaFloat(delta float64, sig bool) string {
	sign := "+"
	if delta < 0 {
		sign = ""
	}
	s := fmt.Sprintf("%s%.2f", sign, delta)
	if sig {
		s += " *"
	}
	return s
}

func formatDeltaCost(delta float64) string {
	if delta >= 0 {
		return fmt.Sprintf("+$%.2f", delta)
	}
	return fmt.Sprintf("-$%.2f", -delta)
}

func formatDeltaDuration(deltaMS float64) string {
	sign := "+"
	if deltaMS < 0 {
		sign = "-"
		deltaMS = -deltaMS
	}
	return sign + formatDurationMS(deltaMS)
}

func formatDuration(ms float64) string {
	return formatDurationMS(ms)
}

func formatDurationMS(ms float64) string {
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return fmt.Sprintf("%dms", int64(ms))
	}
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}
