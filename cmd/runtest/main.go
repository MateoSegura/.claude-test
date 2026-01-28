// Command runtest executes extension tests against Claude CLI.
//
// Usage:
//
//	go run ./cmd/runtest [flags]
//
// Examples:
//
//	# Run all tests against your .claude config
//	go run ./cmd/runtest --config ./.claude
//
//	# Run only command tests
//	go run ./cmd/runtest --suite commands --config ./.claude
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MateoSegura/.claude-test/internal/tests"
)

func main() {
	var configPath string
	var suiteName string
	var verbose bool

	flag.StringVar(&configPath, "config", "./.claude", "Path to .claude config directory to test")
	flag.StringVar(&suiteName, "suite", "all", "Test suite to run: skills, commands, all")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Test Claude Code extensions from a .claude config.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Test your .claude config\n")
		fmt.Fprintf(os.Stderr, "  %s --config ~/.claude\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Test submodule .claude config\n")
		fmt.Fprintf(os.Stderr, "  %s --config ./.claude\n\n", os.Args[0])
	}

	flag.Parse()

	// Resolve config path
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid config path: %s\n", configPath)
		os.Exit(1)
	}

	if _, err := os.Stat(absConfigPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: config directory not found: %s\n", absConfigPath)
		os.Exit(1)
	}

	runner := tests.NewTestRunner()
	runner.ConfigDir = absConfigPath
	runner.Verbose = verbose

	if runner.DryRun {
		fmt.Println("Warning: Running in dry-run mode (Claude CLI not available)")
		fmt.Println("Results will be simulated, not real.")
		fmt.Println()
	}

	fmt.Printf("Testing config: %s\n\n", absConfigPath)

	var suites []*tests.Suite

	switch suiteName {
	case "skills":
		suites = append(suites, skillsSuite())
	case "commands":
		suites = append(suites, commandsSuite())
	case "all":
		suites = append(suites, skillsSuite(), commandsSuite())
	default:
		fmt.Printf("Unknown suite: %s\n", suiteName)
		fmt.Println("Available: skills, commands, all")
		os.Exit(1)
	}

	exitCode := 0
	for _, suite := range suites {
		if !runSuite(runner, suite) {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

func skillsSuite() *tests.Suite {
	return &tests.Suite{
		Name:          "skills",
		ExtensionType: tests.ExtensionSkill,
		Cases: []*tests.TestCase{
			{
				Name:          "meta-skill-recommends-hook",
				ExtensionType: tests.ExtensionSkill,
				Extension:     "meta-skill-create",
				Prompt:        "I want to automatically run gofmt after Claude writes Go files. What type of Claude Code extension should I create?",
				Validators: []tests.Validator{
					tests.LLMValidator(
						"identifies-hook",
						"The response recommends using a HOOK (specifically mentioning PostToolUse or post-tool-use event) for automatically running commands after file writes",
					),
				},
			},
			{
				Name:          "meta-skill-recommends-mcp",
				ExtensionType: tests.ExtensionSkill,
				Extension:     "meta-skill-create",
				Prompt:        "I want Claude to be able to query my PostgreSQL database directly. What extension type should I use?",
				Validators: []tests.Validator{
					tests.LLMValidator(
						"identifies-mcp",
						"The response recommends using an MCP server (Model Context Protocol) for database access",
					),
				},
			},
		},
	}
}

func commandsSuite() *tests.Suite {
	return &tests.Suite{
		Name:          "commands",
		ExtensionType: tests.ExtensionCommand,
		Cases: []*tests.TestCase{
			{
				Name:          "new-skill-template",
				ExtensionType: tests.ExtensionCommand,
				Extension:     "new-skill",
				Prompt:        "Show me how to create a skill called 'language-rust-embedded'. What's the directory structure?",
				Validators: []tests.Validator{
					tests.LLMValidator("shows-structure", "Response shows skill directory structure with SKILL.md"),
					tests.ContainsText("SKILL.md"),
				},
			},
			{
				Name:          "new-hook-template",
				ExtensionType: tests.ExtensionCommand,
				Extension:     "new-hook",
				Prompt:        "Create a hook to run prettier after writing JavaScript files.",
				Validators: []tests.Validator{
					tests.LLMValidator("shows-hook-json", "Response shows JSON hook config with PostToolUse event"),
					tests.ContainsText("PostToolUse"),
				},
			},
		},
	}
}

func runSuite(runner *tests.TestRunner, suite *tests.Suite) bool {
	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Printf("Running test suite: %s\n", suite.Name)
	fmt.Printf("Extension type: %s\n", suite.ExtensionType)
	fmt.Printf("%s\n\n", strings.Repeat("=", 60))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	result, err := runner.RunSuite(ctx, suite)
	if err != nil {
		fmt.Printf("Suite execution failed: %v\n", err)
		return false
	}

	// Print results
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("Suite: %s\n", result.Name)
	fmt.Printf("Tests: %d total, %d passed, %d failed\n", result.TotalTests, result.Passed, result.Failed)
	fmt.Printf("Score: %.0f%% (Grade: %s)\n", result.Score*100, tests.DefaultGradeScale().Grade(result.Score))
	fmt.Printf("Duration: %v\n", result.Duration)
	fmt.Println(strings.Repeat("-", 60))

	for _, r := range result.Results {
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
		}
		fmt.Printf("\n%s: %s (%.0f%%)\n", status, r.Name, r.Score*100)
		for _, v := range r.Validations {
			vStatus := "  +"
			if !v.Passed {
				vStatus = "  -"
			}
			msg := v.Message
			if len(msg) > 150 {
				msg = msg[:147] + "..."
			}
			fmt.Printf("%s %s: %s\n", vStatus, v.Name, msg)
		}
	}

	// Save results
	filename := fmt.Sprintf("%s-results.json", suite.Name)
	if err := runner.SaveSuiteResults(result, filename); err != nil {
		fmt.Printf("\nWarning: couldn't save results: %v\n", err)
	} else {
		fmt.Printf("\nResults saved to %s/%s\n", runner.OutputDir, filename)
	}

	if result.Score < 0.70 {
		fmt.Printf("\nSuite failed: %.0f%% < 70%% threshold\n", result.Score*100)
		return false
	}

	fmt.Printf("\nSuite passed!\n")
	return true
}
