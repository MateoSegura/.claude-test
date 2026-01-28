package benchmark

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// EvalResult contains the evaluation outcome.
type EvalResult struct {
	Success bool    `json:"success"`
	Score   float64 `json:"score"`   // 0.0 - 1.0
	Details string  `json:"details"` // Explanation
}

// evaluate determines if Claude successfully solved the issue.
func (r *BenchmarkRunner) evaluate(ctx context.Context, issue *Issue, repoDir, output string) EvalResult {
	switch issue.EvalMethod {
	case EvalTestSuite:
		return r.evalTestSuite(ctx, issue, repoDir)
	case EvalLLMJudge:
		return r.evalLLMJudge(ctx, issue, repoDir, output)
	case EvalCustomCheck:
		return r.evalCustomCheck(ctx, issue, repoDir)
	case EvalHybrid:
		return r.evalHybrid(ctx, issue, repoDir, output)
	default:
		return r.evalLLMJudge(ctx, issue, repoDir, output)
	}
}

// evalTestSuite runs the repository's test suite.
func (r *BenchmarkRunner) evalTestSuite(ctx context.Context, issue *Issue, repoDir string) EvalResult {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	testCmd := issue.TestCommand
	if testCmd == "" {
		// Default test commands by language
		switch issue.Language {
		case "go":
			testCmd = "go test ./..."
		case "typescript", "javascript":
			testCmd = "npm test"
		case "python":
			testCmd = "pytest"
		case "rust":
			testCmd = "cargo test"
		default:
			testCmd = "make test"
		}
	}

	// Parse command into parts
	parts := strings.Fields(testCmd)
	if len(parts) == 0 {
		return EvalResult{
			Success: false,
			Score:   0.0,
			Details: "No test command specified",
		}
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = repoDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	combinedOutput := stdout.String() + "\n" + stderr.String()

	if err != nil {
		// Check if it's a partial pass (some tests passed)
		score := parseTestScore(combinedOutput, issue.Language)
		return EvalResult{
			Success: false,
			Score:   score,
			Details: fmt.Sprintf("Tests failed: %v\n%s", err, truncateOutput(combinedOutput, 500)),
		}
	}

	return EvalResult{
		Success: true,
		Score:   1.0,
		Details: "All tests passed",
	}
}

// evalLLMJudge uses Claude to evaluate the solution.
func (r *BenchmarkRunner) evalLLMJudge(ctx context.Context, issue *Issue, repoDir, output string) EvalResult {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Get the git diff of changes made
	diffOutput := getGitDiff(repoDir)

	criteria := issue.SuccessCriteria
	if criteria == "" {
		criteria = fmt.Sprintf("The code changes successfully address: %s", issue.Description)
	}

	// Build judge prompt
	judgePrompt := fmt.Sprintf(`You are evaluating if an AI assistant successfully solved a coding task.

## Task Description
%s

## Success Criteria
%s

## Changes Made (git diff)
%s

## Assistant's Output
%s

## Evaluation Instructions
1. Did the assistant make appropriate code changes?
2. Do the changes address the problem described?
3. Are there any obvious bugs or issues in the solution?

Rate the solution on a scale of 0-100 and explain your reasoning.
Format your response as:
SCORE: [number]
REASON: [explanation]
`, issue.Description, criteria, truncateOutput(diffOutput, 3000), truncateOutput(output, 2000))

	cmd := exec.CommandContext(ctx, r.ClaudeBinary, "--print", judgePrompt)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		return EvalResult{
			Success: false,
			Score:   0.0,
			Details: fmt.Sprintf("LLM judge failed: %v", err),
		}
	}

	response := stdout.String()
	score, reason := parseLLMJudgeResponse(response)

	return EvalResult{
		Success: score >= 0.7,
		Score:   score,
		Details: reason,
	}
}

// evalCustomCheck runs a custom validation script.
func (r *BenchmarkRunner) evalCustomCheck(ctx context.Context, issue *Issue, repoDir string) EvalResult {
	if issue.CheckScript == "" {
		return EvalResult{
			Success: false,
			Score:   0.0,
			Details: "No check script specified",
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Write script to temp file
	scriptPath := filepath.Join(repoDir, ".benchmark_check.sh")
	if err := os.WriteFile(scriptPath, []byte(issue.CheckScript), 0755); err != nil {
		return EvalResult{
			Success: false,
			Score:   0.0,
			Details: fmt.Sprintf("Failed to write check script: %v", err),
		}
	}
	defer os.Remove(scriptPath)

	cmd := exec.CommandContext(ctx, "bash", scriptPath)
	cmd.Dir = repoDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return EvalResult{
			Success: false,
			Score:   0.0,
			Details: fmt.Sprintf("Check failed: %v\n%s", err, stderr.String()),
		}
	}

	return EvalResult{
		Success: true,
		Score:   1.0,
		Details: stdout.String(),
	}
}

// evalHybrid combines test suite and LLM judge.
func (r *BenchmarkRunner) evalHybrid(ctx context.Context, issue *Issue, repoDir, output string) EvalResult {
	testResult := r.evalTestSuite(ctx, issue, repoDir)
	llmResult := r.evalLLMJudge(ctx, issue, repoDir, output)

	// Weight: 60% tests, 40% LLM judgment
	combinedScore := (testResult.Score * 0.6) + (llmResult.Score * 0.4)

	return EvalResult{
		Success: combinedScore >= 0.7,
		Score:   combinedScore,
		Details: fmt.Sprintf("Test score: %.0f%%, LLM score: %.0f%%\nTests: %s\nLLM: %s",
			testResult.Score*100, llmResult.Score*100,
			testResult.Details, llmResult.Details),
	}
}

// getGitDiff returns the diff of uncommitted changes.
func getGitDiff(repoDir string) string {
	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = repoDir

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		// Try diff against initial state
		cmd = exec.Command("git", "diff")
		cmd.Dir = repoDir
		cmd.Stdout = &stdout
		cmd.Run()
	}

	return stdout.String()
}

// parseTestScore attempts to extract a partial score from test output.
func parseTestScore(output, language string) float64 {
	// Simple heuristic: count passed vs failed
	// This is language-specific and could be improved

	output = strings.ToLower(output)

	switch language {
	case "go":
		// Go: look for "PASS" and "FAIL"
		passed := strings.Count(output, "--- pass")
		failed := strings.Count(output, "--- fail")
		if passed+failed > 0 {
			return float64(passed) / float64(passed+failed)
		}

	case "python":
		// Pytest: look for "X passed, Y failed"
		if strings.Contains(output, "passed") {
			// Very basic parsing
			if strings.Contains(output, "failed") {
				return 0.5 // Partial
			}
			return 1.0
		}

	case "javascript", "typescript":
		// Jest/Mocha: similar patterns
		if strings.Contains(output, "passing") && !strings.Contains(output, "failing") {
			return 1.0
		}
		if strings.Contains(output, "passing") {
			return 0.5
		}
	}

	return 0.0
}

// parseLLMJudgeResponse extracts score and reason from LLM response.
func parseLLMJudgeResponse(response string) (float64, string) {
	lines := strings.Split(response, "\n")

	var score float64 = 0.0
	var reason string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(strings.ToUpper(line), "SCORE:") {
			scoreStr := strings.TrimPrefix(strings.ToUpper(line), "SCORE:")
			scoreStr = strings.TrimSpace(scoreStr)
			// Parse as percentage
			var s float64
			fmt.Sscanf(scoreStr, "%f", &s)
			if s > 1.0 {
				score = s / 100.0 // Assume it's a percentage
			} else {
				score = s
			}
		}

		if strings.HasPrefix(strings.ToUpper(line), "REASON:") {
			reason = strings.TrimPrefix(line, "REASON:")
			reason = strings.TrimPrefix(reason, "reason:")
			reason = strings.TrimSpace(reason)
		}
	}

	// If no structured response, try to infer
	if score == 0 && reason == "" {
		upper := strings.ToUpper(response)
		if strings.Contains(upper, "SUCCESS") || strings.Contains(upper, "CORRECT") {
			score = 0.8
		} else if strings.Contains(upper, "PARTIAL") {
			score = 0.5
		}
		reason = truncateOutput(response, 200)
	}

	return score, reason
}

// truncateOutput limits output length.
func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... [truncated]"
}
