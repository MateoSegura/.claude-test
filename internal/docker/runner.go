package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	claude "github.com/MateoSegura/claudesdk-go"
	"github.com/MateoSegura/.claude-test/internal/corpus"
	"github.com/MateoSegura/.claude-test/internal/metrics"
)

// RunConfig holds all parameters needed for a single benchmark run.
type RunConfig struct {
	ContainerID string
	Prompt      string
	MaxTurns    int
	Timeout     time.Duration
	WorkDir     string
	SetupCmds   []string
	Evaluation  corpus.Evaluation
}

// Runner executes benchmark runs inside a container using an Executor.
type Runner struct {
	executor Executor
}

// NewRunner creates a Runner backed by the given Executor.
func NewRunner(executor Executor) *Runner {
	return &Runner{executor: executor}
}

// Run executes a full benchmark run: setup, claude invocation, stream parsing,
// and evaluation. It returns a RunResult with metrics and evaluation outcome.
func (r *Runner) Run(ctx context.Context, cfg RunConfig) (*metrics.RunResult, error) {
	// 1. Run setup commands.
	for _, cmd := range cfg.SetupCmds {
		_, _, exitCode, err := r.executor.Exec(ctx, cfg.ContainerID, []string{"sh", "-c", cmd})
		if err != nil {
			return nil, fmt.Errorf("setup command failed: %q: %w", cmd, err)
		}
		if exitCode != 0 {
			return nil, fmt.Errorf("setup command failed: %q: exit code %d", cmd, exitCode)
		}
	}

	// 2. Build claude command.
	claudeCmd := []string{
		"claude",
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		"--max-turns", strconv.Itoa(cfg.MaxTurns),
		cfg.Prompt,
	}

	// 3. Execute claude command.
	stdout, _, claudeExitCode, claudeErr := r.executor.Exec(ctx, cfg.ContainerID, claudeCmd)

	// 4. Parse stdout line-by-line.
	parser := metrics.NewStreamParser()
	messageCount := 0

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		var msg claude.StreamMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			// Skip non-JSON / garbage lines.
			continue
		}
		parser.ProcessMessage(&msg)
		messageCount++
	}

	// 5. If the executor returned a system error, propagate it.
	if claudeErr != nil {
		return nil, fmt.Errorf("claude execution error: %w", claudeErr)
	}

	// 6. If claude exited non-zero and no messages were captured, it crashed.
	if claudeExitCode != 0 && messageCount == 0 {
		return nil, fmt.Errorf("claude CLI crashed: exit code %d with no output", claudeExitCode)
	}

	// 7. Assemble partial result from parser.
	pr := parser.Result()

	result := &metrics.RunResult{
		CostUSD:         pr.CostUSD,
		DurationMS:      pr.DurationMS,
		NumTurns:        pr.NumTurns,
		Usage:           pr.Usage,
		InitTools:       pr.InitTools,
		ToolCalls:       pr.ToolCalls,
		ToolCallSummary: pr.Summary,
		FinalText:       pr.FinalText,
	}

	// If claude exited non-zero but we captured some messages, note the error.
	if claudeExitCode != 0 {
		result.ErrorMsg = fmt.Sprintf("claude exited with code %d", claudeExitCode)
	}

	// 8. Run evaluation.
	success, score, evalErr := r.evaluate(ctx, cfg.ContainerID, cfg.Evaluation)
	if evalErr != nil {
		return nil, fmt.Errorf("evaluation failed: %w", evalErr)
	}

	result.Success = success
	result.Score = score

	return result, nil
}

// evaluate runs the post-run evaluation to determine if the benchmark task
// was completed successfully.
func (r *Runner) evaluate(ctx context.Context, containerID string, eval corpus.Evaluation) (success bool, score float64, err error) {
	switch eval.Method {
	case "command":
		_, _, exitCode, execErr := r.executor.Exec(ctx, containerID, []string{"sh", "-c", eval.Command})
		if execErr != nil {
			return false, 0.0, fmt.Errorf("evaluation command exec error: %w", execErr)
		}
		if exitCode == eval.ExpectedExitCode {
			return true, 1.0, nil
		}
		return false, 0.0, nil

	case "test_passes":
		_, _, exitCode, execErr := r.executor.Exec(ctx, containerID, []string{"sh", "-c", eval.Command})
		if execErr != nil {
			return false, 0.0, fmt.Errorf("evaluation test exec error: %w", execErr)
		}
		if exitCode == 0 {
			return true, 1.0, nil
		}
		return false, 0.0, nil

	case "file_contains":
		_, _, exitCode, execErr := r.executor.Exec(ctx, containerID, []string{"grep", "-qF", eval.Contains, eval.FilePath})
		if execErr != nil {
			return false, 0.0, fmt.Errorf("evaluation grep exec error: %w", execErr)
		}
		if exitCode == 0 {
			return true, 1.0, nil
		}
		return false, 0.0, nil

	default:
		return false, 0.0, fmt.Errorf("unknown evaluation method: %q", eval.Method)
	}
}
