// Package tests provides a framework for testing Claude Code extensions.
// Tests call the Claude CLI directly with extensions loaded and verify outputs.
// Supports all extension types: skills, rules, commands, agents, hooks, and MCPs.
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ExtensionType represents the type of Claude Code extension being tested.
type ExtensionType string

const (
	ExtensionSkill   ExtensionType = "skill"
	ExtensionRule    ExtensionType = "rule"
	ExtensionCommand ExtensionType = "command"
	ExtensionAgent   ExtensionType = "agent"
	ExtensionHook    ExtensionType = "hook"
	ExtensionMCP     ExtensionType = "mcp"
)

// TestRunner executes extension tests against Claude CLI.
type TestRunner struct {
	ClaudeBinary string        // Path to claude binary
	WorkDir      string        // Working directory for tests
	ConfigDir    string        // Path to .claude config directory to test
	OutputDir    string        // Where to save outputs (default /tmp/extension-tests)
	Timeout      time.Duration // Timeout per test
	Verbose      bool          // Print detailed output
	DryRun       bool          // If true, validate structure without calling Claude
}

// NewTestRunner creates a runner with default settings.
// Automatically enables DryRun mode if Claude CLI is not available.
func NewTestRunner() *TestRunner {
	// Check if claude CLI is available
	claudeBinary := "claude"
	dryRun := false

	cmd := exec.Command(claudeBinary, "--version")
	if err := cmd.Run(); err != nil {
		dryRun = true
		fmt.Println("Warning: Claude CLI not available - running in DRY RUN mode (structure validation only)")
		fmt.Println("   To run full tests: install Claude Code CLI")
		fmt.Println()
	}

	return &TestRunner{
		ClaudeBinary: claudeBinary,
		WorkDir:      ".",
		OutputDir:    "/tmp/extension-tests",
		Timeout:      5 * time.Minute,
		Verbose:      false,
		DryRun:       dryRun,
	}
}

// TestCase defines a single extension test.
type TestCase struct {
	Name          string                 // Test name
	ExtensionType ExtensionType          // Type of extension (skill, rule, command, etc.)
	Extension     string                 // Extension name to load
	Skill         string                 // Deprecated: use Extension instead (kept for backward compat)
	Prompt        string                 // Task to give Claude
	Context       string                 // Additional context
	Validators    []Validator            // Functions to validate output
	Setup         func(workDir string)   // Optional setup function
	Teardown      func(workDir string)   // Optional teardown function
	Expected      map[string]interface{} // Expected values for structured validation
	Iterations    int                    // Number of times to run (for consistency testing)
}

// TestResult captures the outcome of a test run.
type TestResult struct {
	Name          string        `json:"name"`
	ExtensionType ExtensionType `json:"extension_type,omitempty"`
	Extension     string        `json:"extension"`
	Skill         string        `json:"skill,omitempty"` // Deprecated
	Passed        bool          `json:"passed"`
	Score         float64       `json:"score"`  // 0.0-1.0
	Output        string        `json:"output"` // Claude's response
	Duration      time.Duration `json:"duration"`
	Validations   []Validation  `json:"validations"`
	Error         error         `json:"error,omitempty"`
	Iteration     int           `json:"iteration"` // Which run this was
}

// Validation is a single validation result.
type Validation struct {
	Name    string  `json:"name"`
	Passed  bool    `json:"passed"`
	Score   float64 `json:"score"`
	Message string  `json:"message"`
}

// Validator checks if output meets expectations.
type Validator func(output string, result *TestResult) Validation

// Suite is a collection of test cases for an extension.
type Suite struct {
	Name          string        // Suite name
	ExtensionType ExtensionType // Type of extension being tested
	Extension     string        // Extension name being tested
	Skill         string        // Deprecated: use Extension instead (kept for backward compat)
	Cases         []*TestCase   // Test cases
	SetupAll      func()        // Run before all tests
	Teardown      func()        // Run after all tests
}

// SuiteResult aggregates results for a suite.
type SuiteResult struct {
	Name          string        `json:"name"`
	ExtensionType ExtensionType `json:"extension_type,omitempty"`
	Extension     string        `json:"extension"`
	Skill         string        `json:"skill,omitempty"` // Deprecated
	TotalTests    int           `json:"total_tests"`
	Passed        int           `json:"passed"`
	Failed        int           `json:"failed"`
	Score         float64       `json:"score"` // Average score
	Results       []*TestResult `json:"results"`
	Duration      time.Duration `json:"duration"`
}

// GradeScale defines the grading criteria.
type GradeScale struct {
	A float64 // >= A is excellent
	B float64 // >= B is good
	C float64 // >= C is acceptable
	D float64 // >= D is poor
	// Below D is failing
}

// DefaultGradeScale returns the standard grading scale.
func DefaultGradeScale() GradeScale {
	return GradeScale{
		A: 0.90, // 90%+ Excellent: Skill consistently works as expected
		B: 0.80, // 80-89% Good: Skill mostly works with minor issues
		C: 0.70, // 70-79% Acceptable: Skill works but has gaps
		D: 0.60, // 60-69% Poor: Skill has significant issues
		// Below 60%: Failing - skill needs major work
	}
}

// Grade returns a letter grade for a score.
func (g GradeScale) Grade(score float64) string {
	switch {
	case score >= g.A:
		return "A"
	case score >= g.B:
		return "B"
	case score >= g.C:
		return "C"
	case score >= g.D:
		return "D"
	default:
		return "F"
	}
}

// Run executes a single test case.
func (r *TestRunner) Run(ctx context.Context, tc *TestCase) (*TestResult, error) {
	start := time.Now()

	// Handle backward compatibility: use Extension if set, otherwise fall back to Skill
	extension := tc.Extension
	if extension == "" {
		extension = tc.Skill
	}
	extensionType := tc.ExtensionType
	if extensionType == "" {
		extensionType = ExtensionSkill // Default to skill for backward compat
	}

	result := &TestResult{
		Name:          tc.Name,
		ExtensionType: extensionType,
		Extension:     extension,
		Skill:         tc.Skill, // Keep for backward compat
		Iteration:     1,
	}

	// Create test workspace
	workDir, err := r.createWorkspace(tc.Name)
	if err != nil {
		result.Error = fmt.Errorf("create workspace: %w", err)
		return result, err
	}
	defer os.RemoveAll(workDir)

	// Run setup if provided
	if tc.Setup != nil {
		tc.Setup(workDir)
	}
	defer func() {
		if tc.Teardown != nil {
			tc.Teardown(workDir)
		}
	}()

	// Build Claude command
	output, err := r.runClaude(ctx, workDir, extensionType, extension, tc.Prompt, tc.Context)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, err
	}

	result.Output = output
	result.Duration = time.Since(start)

	// Run validators
	totalScore := 0.0
	for _, validator := range tc.Validators {
		v := validator(output, result)
		result.Validations = append(result.Validations, v)
		if v.Passed {
			totalScore += v.Score
		}
	}

	// Calculate overall score
	if len(tc.Validators) > 0 {
		result.Score = totalScore / float64(len(tc.Validators))
		result.Passed = result.Score >= 0.7 // 70% threshold
	} else {
		result.Score = 1.0
		result.Passed = true
	}

	// Save output for inspection
	r.saveOutput(tc.Name, output)

	return result, nil
}

// RunSuite executes all tests in a suite.
func (r *TestRunner) RunSuite(ctx context.Context, suite *Suite) (*SuiteResult, error) {
	start := time.Now()

	// Handle backward compatibility
	extension := suite.Extension
	if extension == "" {
		extension = suite.Skill
	}
	extensionType := suite.ExtensionType
	if extensionType == "" {
		extensionType = ExtensionSkill
	}

	result := &SuiteResult{
		Name:          suite.Name,
		ExtensionType: extensionType,
		Extension:     extension,
		Skill:         suite.Skill, // Keep for backward compat
	}

	if suite.SetupAll != nil {
		suite.SetupAll()
	}
	defer func() {
		if suite.Teardown != nil {
			suite.Teardown()
		}
	}()

	totalScore := 0.0
	for _, tc := range suite.Cases {
		iterations := tc.Iterations
		if iterations == 0 {
			iterations = 1
		}

		for i := 1; i <= iterations; i++ {
			testCtx, cancel := context.WithTimeout(ctx, r.Timeout)
			testResult, err := r.Run(testCtx, tc)
			cancel()

			testResult.Iteration = i
			if err != nil && r.Verbose {
				fmt.Printf("Test %s (iteration %d) error: %v\n", tc.Name, i, err)
			}

			result.Results = append(result.Results, testResult)
			result.TotalTests++

			if testResult.Passed {
				result.Passed++
			} else {
				result.Failed++
			}

			totalScore += testResult.Score
		}
	}

	if result.TotalTests > 0 {
		result.Score = totalScore / float64(result.TotalTests)
	}

	result.Duration = time.Since(start)
	return result, nil
}

// runClaude executes the Claude CLI with an extension loaded.
func (r *TestRunner) runClaude(ctx context.Context, workDir string, extType ExtensionType, extension, prompt, ctxStr string) (string, error) {
	// In dry run mode, return a simulated response for structure validation
	if r.DryRun {
		return r.simulateResponse(extension, prompt), nil
	}

	args := []string{
		"--print",                        // Non-interactive mode
		"--dangerously-skip-permissions", // Skip prompts for testing
	}

	// Copy extension to test workspace based on type
	if extension != "" {
		configDir := r.ConfigDir
		if configDir == "" {
			configDir = filepath.Join(r.WorkDir, ".claude")
		}

		var srcPath, dstPath string
		switch extType {
		case ExtensionSkill:
			srcPath = filepath.Join(configDir, "skills", extension)
			dstPath = filepath.Join(workDir, ".claude", "skills", extension)
		case ExtensionRule:
			srcPath = filepath.Join(configDir, "rules", extension)
			dstPath = filepath.Join(workDir, ".claude", "rules", extension)
		case ExtensionCommand:
			srcPath = filepath.Join(configDir, "commands", extension+".md")
			dstPath = filepath.Join(workDir, ".claude", "commands", extension+".md")
		case ExtensionAgent:
			srcPath = filepath.Join(configDir, "agents", extension+".md")
			dstPath = filepath.Join(workDir, ".claude", "agents", extension+".md")
		case ExtensionHook, ExtensionMCP:
			// Hooks and MCPs are in settings files, copy entire .claude config
			srcPath = configDir
			dstPath = filepath.Join(workDir, ".claude")
		default:
			srcPath = filepath.Join(configDir, "skills", extension)
			dstPath = filepath.Join(workDir, ".claude", "skills", extension)
		}

		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
				return "", fmt.Errorf("create extension dir: %w", err)
			}

			// Copy based on whether source is file or directory
			srcInfo, err := os.Stat(srcPath)
			if err != nil {
				return "", fmt.Errorf("stat extension source: %w", err)
			}

			if srcInfo.IsDir() {
				if err := copyDir(srcPath, dstPath); err != nil {
					return "", fmt.Errorf("copy extension: %w", err)
				}
			} else {
				data, err := os.ReadFile(srcPath)
				if err != nil {
					return "", fmt.Errorf("read extension: %w", err)
				}
				if err := os.WriteFile(dstPath, data, srcInfo.Mode()); err != nil {
					return "", fmt.Errorf("write extension: %w", err)
				}
			}
		}
	}

	// Build the full prompt
	fullPrompt := prompt
	if ctxStr != "" {
		fullPrompt = fmt.Sprintf("Context:\n%s\n\nTask:\n%s", ctxStr, prompt)
	}
	args = append(args, fullPrompt)

	cmd := exec.CommandContext(ctx, r.ClaudeBinary, args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stdout.String(), fmt.Errorf("claude: %w: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// createWorkspace creates an isolated test directory.
func (r *TestRunner) createWorkspace(testName string) (string, error) {
	if err := os.MkdirAll(r.OutputDir, 0755); err != nil {
		return "", err
	}

	safeName := regexp.MustCompile(`[^a-zA-Z0-9-]`).ReplaceAllString(testName, "-")
	dir, err := os.MkdirTemp(r.OutputDir, fmt.Sprintf("test-%s-*", safeName))
	if err != nil {
		return "", err
	}

	// Create .claude directory structure for all extension types
	dirs := []string{
		filepath.Join(dir, ".claude", "skills"),
		filepath.Join(dir, ".claude", "rules"),
		filepath.Join(dir, ".claude", "commands"),
		filepath.Join(dir, ".claude", "agents"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return "", err
		}
	}

	// Initialize as git repo (extensions often expect this)
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		return "", err
	}

	return dir, nil
}

// saveOutput saves test output for inspection.
func (r *TestRunner) saveOutput(testName, output string) error {
	safeName := regexp.MustCompile(`[^a-zA-Z0-9-]`).ReplaceAllString(testName, "-")
	outputPath := filepath.Join(r.OutputDir, fmt.Sprintf("%s-output.txt", safeName))
	return os.WriteFile(outputPath, []byte(output), 0644)
}

// SaveSuiteResults saves suite results as JSON.
func (r *TestRunner) SaveSuiteResults(result *SuiteResult, filename string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.OutputDir, filename), data, 0644)
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

// simulateResponse generates a mock response for dry-run testing.
func (r *TestRunner) simulateResponse(skill, prompt string) string {
	var sb strings.Builder

	sb.WriteString("[DRY RUN MODE - Simulated Response]\n\n")

	promptLower := strings.ToLower(prompt)

	if strings.Contains(promptLower, "hook") {
		sb.WriteString("This uses the PostToolUse hook with the Write matcher.\n")
	}
	if strings.Contains(promptLower, "skill") {
		sb.WriteString("Skills are stored in .claude/skills/[name]/SKILL.md with rules/ and reference/ subdirectories.\n")
	}
	if strings.Contains(promptLower, "mcp") || strings.Contains(promptLower, "database") {
		sb.WriteString("This requires an MCP server for external API integration.\n")
	}
	if strings.Contains(promptLower, "rule") {
		sb.WriteString("A simple rule would be the best approach for this - no complex workflow needed.\n")
	}

	return sb.String()
}
