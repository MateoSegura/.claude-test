package orchestrator

import (
	"time"

	"github.com/MateoSegura/.claude-test/internal/corpus"
)

// Variant identifies whether a run uses the custom config or baseline.
type Variant string

const (
	Configured Variant = "configured"
	Baseline   Variant = "baseline"
)

// RunSpec describes a single benchmark execution.
type RunSpec struct {
	Issue     corpus.Issue
	Variant   Variant
	Attempt   int
	ConfigDir string
	Image     string
	Timeout   time.Duration
	MaxTurns  int
}

// RunPair groups the configured and baseline runs for a single issue.
type RunPair struct {
	Issue      corpus.Issue
	Configured []RunSpec
	Baseline   []RunSpec
}

// GeneratePlan creates RunPairs for every issue according to the config.
func GeneratePlan(cfg *BenchConfig, issues []corpus.Issue) []RunPair {
	pairs := make([]RunPair, 0, len(issues))

	runBaseline := true
	if cfg.Execution.Baseline != nil {
		runBaseline = *cfg.Execution.Baseline
	}

	for _, issue := range issues {
		pair := RunPair{
			Issue:      issue,
			Configured: make([]RunSpec, 0, cfg.Execution.Attempts),
		}

		for a := 1; a <= cfg.Execution.Attempts; a++ {
			pair.Configured = append(pair.Configured, RunSpec{
				Issue:     issue,
				Variant:   Configured,
				Attempt:   a,
				ConfigDir: cfg.Config.Path,
				Image:     cfg.Docker.Image,
				Timeout:   cfg.Execution.Timeout,
				MaxTurns:  cfg.Execution.MaxTurns,
			})
		}

		if runBaseline {
			pair.Baseline = make([]RunSpec, 0, cfg.Execution.Attempts)
			for a := 1; a <= cfg.Execution.Attempts; a++ {
				pair.Baseline = append(pair.Baseline, RunSpec{
					Issue:     issue,
					Variant:   Baseline,
					Attempt:   a,
					ConfigDir: "",
					Image:     cfg.Docker.Image,
					Timeout:   cfg.Execution.Timeout,
					MaxTurns:  cfg.Execution.MaxTurns,
				})
			}
		}

		pairs = append(pairs, pair)
	}

	return pairs
}

// FlattenPlan converts RunPairs into a flat slice of RunSpecs in
// deterministic order: pair[0].Configured, pair[0].Baseline, pair[1].Configured, ...
func FlattenPlan(pairs []RunPair) []RunSpec {
	if len(pairs) == 0 {
		return []RunSpec{}
	}

	var specs []RunSpec
	for _, pair := range pairs {
		specs = append(specs, pair.Configured...)
		specs = append(specs, pair.Baseline...)
	}

	if specs == nil {
		return []RunSpec{}
	}
	return specs
}
