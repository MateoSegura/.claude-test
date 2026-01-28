// Command bench runs benchmark tests against .claude configurations.
//
// Usage:
//
//	go run ./cmd/bench [flags] [corpus-path]
//
// Examples:
//
//	# Run with your .claude config vs baseline
//	go run ./cmd/bench --config /path/to/.claude
//
//	# Run specific difficulty
//	go run ./cmd/bench --difficulty medium
//
//	# Compare multiple configs
//	go run ./cmd/bench --config /path/to/config1 --config /path/to/config2
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MateoSegura/.claude-test/internal/benchmark"
)

type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	// Flags
	var configs stringSlice
	var difficulty string
	var taskType string
	var language string
	var verbose bool
	var dryRun bool
	var baseline bool
	var outputDir string
	var issueID string

	flag.Var(&configs, "config", "Path to .claude config directory (can specify multiple)")
	flag.StringVar(&difficulty, "difficulty", "", "Filter by difficulty (easy, medium, hard)")
	flag.StringVar(&taskType, "task", "", "Filter by task type (bug_fix, feature, refactor, test)")
	flag.StringVar(&language, "language", "", "Filter by language (go, typescript, python)")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&dryRun, "dry-run", false, "Validate corpus without running Claude")
	flag.BoolVar(&baseline, "baseline", true, "Include baseline (no config) for comparison")
	flag.StringVar(&outputDir, "output", "/tmp/claude-benchmark/results", "Output directory for results")
	flag.StringVar(&issueID, "issue", "", "Run only a specific issue by ID")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [corpus-path]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Benchmark .claude configurations against real-world issues.\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  corpus-path    Path to corpus YAML file or directory (default: ./corpus)\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Compare your config against baseline\n")
		fmt.Fprintf(os.Stderr, "  %s --config ~/.claude\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Run only easy Go issues\n")
		fmt.Fprintf(os.Stderr, "  %s --difficulty easy --language go\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Test a single issue\n")
		fmt.Fprintf(os.Stderr, "  %s --issue easy-go-nil-check-001\n\n", os.Args[0])
	}

	flag.Parse()

	// Determine corpus path
	corpusPath := "./corpus"
	if flag.NArg() > 0 {
		corpusPath = flag.Arg(0)
	}

	// Load corpus
	var corpus *benchmark.Corpus
	var err error

	info, statErr := os.Stat(corpusPath)
	if statErr != nil {
		fmt.Fprintf(os.Stderr, "Error: corpus path not found: %s\n", corpusPath)
		os.Exit(1)
	}

	if info.IsDir() {
		corpus, err = benchmark.LoadCorpusDir(corpusPath)
	} else {
		corpus, err = benchmark.LoadCorpus(corpusPath)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading corpus: %v\n", err)
		os.Exit(1)
	}

	// Apply filters
	var diffFilter *benchmark.Difficulty
	var taskFilter *benchmark.TaskType
	var langFilter *string

	if difficulty != "" {
		d := benchmark.Difficulty(difficulty)
		diffFilter = &d
	}
	if taskType != "" {
		t := benchmark.TaskType(taskType)
		taskFilter = &t
	}
	if language != "" {
		langFilter = &language
	}

	// Filter issues
	issues := corpus.Filter(diffFilter, taskFilter, langFilter, nil)

	// Filter by specific issue ID if provided
	if issueID != "" {
		var filtered []*benchmark.Issue
		for _, issue := range issues {
			if issue.ID == issueID {
				filtered = append(filtered, issue)
			}
		}
		if len(filtered) == 0 {
			fmt.Fprintf(os.Stderr, "Error: issue not found: %s\n", issueID)
			os.Exit(1)
		}
		issues = filtered
	}

	// Create filtered corpus
	filteredCorpus := &benchmark.Corpus{
		Name:        corpus.Name,
		Description: corpus.Description,
		Version:     corpus.Version,
		Issues:      issues,
	}

	// Print corpus info
	stats := filteredCorpus.Stats()
	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Printf("CLAUDE CONFIG BENCHMARK\n")
	fmt.Println(strings.Repeat("=", 61))
	fmt.Printf("\nCorpus: %s (v%s)\n", corpus.Name, corpus.Version)
	fmt.Printf("Issues: %d total\n", stats.Total)
	fmt.Printf("  By difficulty: easy=%d, medium=%d, hard=%d\n",
		stats.ByDifficulty[benchmark.DifficultyEasy],
		stats.ByDifficulty[benchmark.DifficultyMedium],
		stats.ByDifficulty[benchmark.DifficultyHard])
	fmt.Printf("  By language: ")
	for lang, count := range stats.ByLanguage {
		fmt.Printf("%s=%d ", lang, count)
	}
	fmt.Println()

	if len(issues) == 0 {
		fmt.Println("\nNo issues match the specified filters.")
		os.Exit(0)
	}

	// Setup runner
	runner := benchmark.NewBenchmarkRunner()
	runner.OutputDir = outputDir
	runner.Verbose = verbose

	// Add baseline if requested
	if baseline {
		runner.AddBaseline()
		fmt.Println("\nConfigs to test:")
		fmt.Println("  - baseline (no .claude config)")
	} else {
		fmt.Println("\nConfigs to test:")
	}

	// Add user-specified configs
	for _, cfgPath := range configs {
		absPath, err := filepath.Abs(cfgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid config path: %s\n", cfgPath)
			os.Exit(1)
		}

		name := filepath.Base(absPath)
		if name == ".claude" {
			name = filepath.Base(filepath.Dir(absPath)) + "-config"
		}

		runner.AddConfig(&benchmark.Config{
			Name:        name,
			Path:        absPath,
			Description: fmt.Sprintf("Config from %s", cfgPath),
		})
		fmt.Printf("  - %s (%s)\n", name, cfgPath)
	}

	if len(runner.Configs) == 0 {
		fmt.Println("\nNo configs specified. Use --config to add configurations to test.")
		fmt.Println("Running with baseline only.")
		runner.AddBaseline()
	}

	fmt.Println()

	// Dry run mode
	if dryRun {
		fmt.Println("DRY RUN MODE - validating corpus structure only\n")
		for _, issue := range issues {
			fmt.Printf("  [%s] %s (%s/%s)\n",
				issue.ID, issue.Title, issue.Difficulty, issue.Language)
		}
		fmt.Printf("\nCorpus validation complete. %d issues ready for benchmarking.\n", len(issues))
		os.Exit(0)
	}

	// Run benchmark
	fmt.Println("Starting benchmark run...")
	fmt.Println("(This may take a while depending on issue count and complexity)\n")

	ctx := context.Background()
	result, err := runner.Run(ctx, filteredCorpus)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Benchmark error: %v\n", err)
		os.Exit(1)
	}

	// Print report
	result.PrintReport()

	// Print comparison if we have baseline and other configs
	if baseline && len(configs) > 0 {
		for _, cfg := range configs {
			name := filepath.Base(cfg)
			if name == ".claude" {
				name = filepath.Base(filepath.Dir(cfg)) + "-config"
			}

			comparison := result.Compare("baseline", name)
			if comparison != nil {
				fmt.Printf("\nYour config '%s' vs baseline:\n", name)
				if comparison.Improvement > 0 {
					fmt.Printf("   +%.1f percentage points improvement\n", comparison.Improvement)
				} else if comparison.Improvement < 0 {
					fmt.Printf("   %.1f percentage points regression\n", comparison.Improvement)
				} else {
					fmt.Printf("   No change in success rate\n")
				}
			}
		}
	}

	fmt.Printf("\nResults saved to: %s\n", outputDir)
}
