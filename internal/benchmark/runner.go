package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	claude "github.com/MateoSegura/claudesdk-go"
)

// Config represents a .claude configuration to test.
type Config struct {
	Name        string `json:"name"` // e.g., "baseline", "golang-optimized"
	Path        string `json:"path"` // Path to .claude directory (empty = no config)
	Description string `json:"description"`
}

// BenchmarkRunner executes benchmark tests.
type BenchmarkRunner struct {
	// Paths
	WorkDir   string // Base directory for benchmark runs
	OutputDir string // Where to save results

	// Execution
	ClaudeBinary string        // Path to claude binary
	Timeout      time.Duration // Timeout per issue
	Verbose      bool

	// Configs to compare
	Configs []*Config
}

// NewBenchmarkRunner creates a runner with defaults.
func NewBenchmarkRunner() *BenchmarkRunner {
	return &BenchmarkRunner{
		WorkDir:      "/tmp/claude-benchmark",
		OutputDir:    "/tmp/claude-benchmark/results",
		ClaudeBinary: "claude",
		Timeout:      10 * time.Minute,
		Verbose:      false,
		Configs:      []*Config{},
	}
}

// AddConfig adds a configuration to test.
func (r *BenchmarkRunner) AddConfig(cfg *Config) {
	r.Configs = append(r.Configs, cfg)
}

// AddBaseline adds a baseline (no .claude config) for comparison.
func (r *BenchmarkRunner) AddBaseline() {
	r.Configs = append(r.Configs, &Config{
		Name:        "baseline",
		Path:        "", // Empty = no config
		Description: "No .claude configuration (raw Claude)",
	})
}

// Run executes benchmark for all issues with all configs.
func (r *BenchmarkRunner) Run(ctx context.Context, corpus *Corpus) (*BenchmarkResult, error) {
	start := time.Now()

	// Ensure directories exist
	if err := os.MkdirAll(r.WorkDir, 0755); err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}
	if err := os.MkdirAll(r.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	result := &BenchmarkResult{
		Timestamp:     time.Now(),
		CorpusName:    corpus.Name,
		CorpusVersion: corpus.Version,
		ConfigResults: make(map[string]*ConfigResult),
	}

	// Run each config against all issues
	for _, cfg := range r.Configs {
		cfgResult := &ConfigResult{
			ConfigName:   cfg.Name,
			IssueResults: make([]*IssueResult, 0, len(corpus.Issues)),
		}

		for _, issue := range corpus.Issues {
			if r.Verbose {
				fmt.Printf("Running %s with config %s...\n", issue.ID, cfg.Name)
			}

			issueResult, err := r.runIssue(ctx, cfg, issue)
			if err != nil {
				issueResult = &IssueResult{
					IssueID:    issue.ID,
					ConfigName: cfg.Name,
					Success:    false,
					Error:      err.Error(),
				}
			}

			cfgResult.IssueResults = append(cfgResult.IssueResults, issueResult)

			// Save intermediate result
			r.saveIssueResult(issueResult)
		}

		// Calculate aggregate stats
		cfgResult.calculateStats()
		result.ConfigResults[cfg.Name] = cfgResult
	}

	result.Duration = time.Since(start)

	// Save final result
	if err := r.saveResult(result); err != nil {
		return result, fmt.Errorf("save result: %w", err)
	}

	return result, nil
}

// runIssue runs a single issue with a single config.
func (r *BenchmarkRunner) runIssue(ctx context.Context, cfg *Config, issue *Issue) (*IssueResult, error) {
	start := time.Now()

	// Create isolated workspace
	workDir, err := r.createWorkspace(cfg.Name, issue.ID)
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	// Don't cleanup - keep for debugging
	// defer os.RemoveAll(workDir)

	// Clone the repository
	repoDir := filepath.Join(workDir, "repo")
	if err := r.cloneRepo(ctx, issue.RepoURL, issue.RepoRef, repoDir); err != nil {
		return nil, fmt.Errorf("clone repo: %w", err)
	}

	// Apply .claude config if specified
	if cfg.Path != "" {
		if err := r.applyConfig(cfg.Path, repoDir); err != nil {
			return nil, fmt.Errorf("apply config: %w", err)
		}
	}

	// Build prompt with context
	prompt := r.buildPrompt(issue, repoDir)

	// Run Claude
	output, claudeErr := r.runClaude(ctx, repoDir, prompt)

	// Evaluate result
	evalResult := r.evaluate(ctx, issue, repoDir, output)

	result := &IssueResult{
		IssueID:      issue.ID,
		ConfigName:   cfg.Name,
		Difficulty:   issue.Difficulty,
		TaskType:     issue.TaskType,
		Language:     issue.Language,
		Success:      evalResult.Success,
		Score:        evalResult.Score,
		ClaudeOutput: output,
		EvalDetails:  evalResult.Details,
		Duration:     time.Since(start),
		WorkDir:      workDir,
	}

	if claudeErr != nil {
		result.Error = claudeErr.Error()
	}

	return result, nil
}

// cloneRepo clones a git repository.
func (r *BenchmarkRunner) cloneRepo(ctx context.Context, url, ref, dest string) error {
	// Clone
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", url, dest)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %w: %s", err, output)
	}

	// Checkout specific ref if provided
	if ref != "" && ref != "main" && ref != "master" {
		cmd = exec.CommandContext(ctx, "git", "fetch", "origin", ref)
		cmd.Dir = dest
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git fetch ref: %w: %s", err, output)
		}

		cmd = exec.CommandContext(ctx, "git", "checkout", ref)
		cmd.Dir = dest
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git checkout: %w: %s", err, output)
		}
	}

	return nil
}

// applyConfig copies .claude directory to the repo.
func (r *BenchmarkRunner) applyConfig(configPath, repoDir string) error {
	destPath := filepath.Join(repoDir, ".claude")

	// Remove existing .claude if present
	os.RemoveAll(destPath)

	// Copy config
	return copyDir(configPath, destPath)
}

// buildPrompt constructs the full prompt for Claude.
func (r *BenchmarkRunner) buildPrompt(issue *Issue, repoDir string) string {
	var sb strings.Builder

	sb.WriteString(issue.Prompt)

	// Add context files if specified
	if len(issue.ContextFiles) > 0 {
		sb.WriteString("\n\n## Relevant Files\n")
		for _, file := range issue.ContextFiles {
			content, err := os.ReadFile(filepath.Join(repoDir, file))
			if err == nil {
				sb.WriteString(fmt.Sprintf("\n### %s\n```\n%s\n```\n", file, string(content)))
			}
		}
	}

	return sb.String()
}

// runClaude executes Claude CLI using the SDK.
func (r *BenchmarkRunner) runClaude(ctx context.Context, workDir, prompt string) (string, error) {
	session, err := claude.NewSession(claude.SessionConfig{
		WorkDir:         workDir,
		SkipPermissions: true,
		Timeout:         r.Timeout,
	})
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	output, err := session.CollectAll(ctx, prompt)
	if err != nil {
		return output, fmt.Errorf("claude: %w", err)
	}

	return output, nil
}

// createWorkspace creates an isolated directory for a benchmark run.
func (r *BenchmarkRunner) createWorkspace(configName, issueID string) (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	name := fmt.Sprintf("%s_%s_%s", timestamp, configName, issueID)
	dir := filepath.Join(r.WorkDir, "runs", name)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	return dir, nil
}

// saveIssueResult saves individual issue result to disk.
func (r *BenchmarkRunner) saveIssueResult(result *IssueResult) error {
	filename := fmt.Sprintf("%s_%s.json", result.ConfigName, result.IssueID)
	path := filepath.Join(r.OutputDir, "issues", filename)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// saveResult saves the complete benchmark result.
func (r *BenchmarkRunner) saveResult(result *BenchmarkResult) error {
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("benchmark_%s.json", timestamp)
	path := filepath.Join(r.OutputDir, filename)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
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
