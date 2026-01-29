package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/MateoSegura/.claude-test/internal/analysis"
	"github.com/MateoSegura/.claude-test/internal/corpus"
	"github.com/MateoSegura/.claude-test/internal/orchestrator"
)

type cliFlags struct {
	Config           string
	Corpus           string
	Attempts         int
	Parallelism      int
	Baseline         bool
	Image            string
	Output           string
	Timeout          time.Duration
	MaxTurns         int
	FilterDifficulty string
	FilterLanguage   string
	FilterTask       string
	DryRun           bool
	Verbose          bool
	Format           string
}

func parseFlags(args []string) (*cliFlags, error) {
	fs := flag.NewFlagSet("configbench", flag.ContinueOnError)

	f := &cliFlags{}
	fs.StringVar(&f.Config, "config", ".claude", "path to Claude config directory")
	fs.StringVar(&f.Corpus, "corpus", "", "path to corpus YAML file (required)")
	fs.IntVar(&f.Attempts, "attempts", 5, "number of attempts per variant")
	fs.IntVar(&f.Parallelism, "parallelism", 2, "max concurrent containers")
	fs.BoolVar(&f.Baseline, "baseline", true, "run baseline (no-config) variant")
	fs.StringVar(&f.Image, "image", "ghcr.io/mateosegura/dev-env:latest", "Docker image for containers")
	fs.StringVar(&f.Output, "output", "", "output file path (stdout if empty)")
	fs.DurationVar(&f.Timeout, "timeout", 10*time.Minute, "timeout per run")
	fs.IntVar(&f.MaxTurns, "max-turns", 25, "max conversation turns per run")
	fs.StringVar(&f.FilterDifficulty, "filter-difficulty", "", "comma-separated difficulty filter")
	fs.StringVar(&f.FilterLanguage, "filter-language", "", "comma-separated language filter")
	fs.StringVar(&f.FilterTask, "filter-task", "", "comma-separated task type filter")
	fs.BoolVar(&f.DryRun, "dry-run", false, "print execution plan without running")
	fs.BoolVar(&f.Verbose, "verbose", false, "verbose progress output")
	fs.StringVar(&f.Format, "format", "terminal", "output format (terminal or json)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if f.Corpus == "" {
		return nil, fmt.Errorf("required flag --corpus not provided")
	}
	if f.Attempts < 1 {
		return nil, fmt.Errorf("attempts must be >= 1")
	}
	if f.Parallelism < 1 {
		return nil, fmt.Errorf("parallelism must be >= 1")
	}
	if f.MaxTurns < 1 {
		return nil, fmt.Errorf("max-turns must be >= 1")
	}
	if f.Format != "terminal" && f.Format != "json" {
		return nil, fmt.Errorf("format must be \"terminal\" or \"json\"")
	}

	return f, nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		result = append(result, strings.TrimSpace(p))
	}
	return result
}

func flagsToOrchConfig(f *cliFlags) *orchestrator.BenchConfig {
	cfg := &orchestrator.BenchConfig{
		Config: orchestrator.ConfigSpec{
			Path: f.Config,
		},
		Docker: orchestrator.DockerSpec{
			Image: f.Image,
		},
		Execution: orchestrator.ExecSpec{
			Attempts:    f.Attempts,
			Parallelism: f.Parallelism,
			Timeout:     f.Timeout,
			Baseline:    orchestrator.BoolPtr(f.Baseline),
			MaxTurns:    f.MaxTurns,
		},
		Corpus: orchestrator.CorpusSpec{
			Path: f.Corpus,
			Filters: orchestrator.CorpusFilterSpec{
				Difficulty: splitCSV(f.FilterDifficulty),
				Language:   splitCSV(f.FilterLanguage),
				TaskType:   splitCSV(f.FilterTask),
			},
		},
		Output: orchestrator.OutputSpec{
			Path:    f.Output,
			Formats: []string{f.Format},
		},
		Analysis: orchestrator.AnalysisSpec{
			SignificanceThreshold: 0.05,
			ImprovementThreshold:  0.1,
		},
	}
	return cfg
}

func printDryRun(w io.Writer, pairs []orchestrator.RunPair, f *cliFlags) {
	// Count total runs from the pairs.
	totalRuns := 0
	for _, pair := range pairs {
		totalRuns += len(pair.Configured) + len(pair.Baseline)
	}

	variantDesc := "configured + baseline"
	if !f.Baseline {
		variantDesc = "configured only"
	}

	fmt.Fprintf(w, "Execution Plan\n")
	fmt.Fprintf(w, "==============\n")
	fmt.Fprintf(w, "Issues:       %d\n", len(pairs))
	fmt.Fprintf(w, "Variants:     %s\n", variantDesc)
	fmt.Fprintf(w, "Attempts:     %d per variant\n", f.Attempts)
	fmt.Fprintf(w, "Total runs:   %d\n", totalRuns)
	fmt.Fprintf(w, "Image:        %s\n", f.Image)
	fmt.Fprintf(w, "Parallelism:  %d\n", f.Parallelism)
	fmt.Fprintf(w, "Timeout:      %s\n", f.Timeout)
	fmt.Fprintf(w, "\nIssues:\n")
	for _, pair := range pairs {
		fmt.Fprintf(w, "  - %s (difficulty=%s, language=%s, task=%s)\n",
			pair.Issue.ID,
			pair.Issue.Difficulty,
			pair.Issue.Language,
			pair.Issue.TaskType,
		)
	}
}

func run(args []string) error {
	f, err := parseFlags(args)
	if err != nil {
		return err
	}

	cfg := flagsToOrchConfig(f)

	// Load corpus ourselves for pre-filtering and validation.
	c, err := corpus.Load(f.Corpus)
	if err != nil {
		return fmt.Errorf("loading corpus: %w", err)
	}

	if err := c.Validate(); err != nil {
		return fmt.Errorf("validating corpus: %w", err)
	}

	// Apply filters.
	filter := corpus.CorpusFilter{
		Difficulty: splitCSV(f.FilterDifficulty),
		Language:   splitCSV(f.FilterLanguage),
		TaskType:   splitCSV(f.FilterTask),
	}
	filtered := c.Filter(filter)

	if len(filtered.Issues) == 0 {
		return fmt.Errorf("no issues match the specified filters")
	}

	// Generate plan for dry-run or execution.
	pairs := orchestrator.GeneratePlan(cfg, filtered.Issues)

	if f.DryRun {
		printDryRun(os.Stdout, pairs, f)
		return nil
	}

	// Set up context with signal handling.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		cancel()
	}()

	// Create orchestrator with pre-filtered issues.
	orch := orchestrator.NewWithDeps(cfg, filtered.Issues, nil, nil)

	// Run the benchmark.
	results, err := orch.Run(ctx)
	if err != nil {
		return fmt.Errorf("benchmark execution: %w", err)
	}

	// Build report.
	report := analysis.BuildReport(results, cfg.Analysis.SignificanceThreshold)

	// Format output.
	var w io.Writer
	if f.Output != "" {
		file, err := os.Create(f.Output)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer file.Close()
		w = file
	} else {
		w = os.Stdout
	}

	switch f.Format {
	case "json":
		if err := analysis.FormatJSON(report, w); err != nil {
			return fmt.Errorf("formatting JSON: %w", err)
		}
	default:
		if err := analysis.FormatTerminal(report, w); err != nil {
			return fmt.Errorf("formatting terminal: %w", err)
		}
	}

	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
