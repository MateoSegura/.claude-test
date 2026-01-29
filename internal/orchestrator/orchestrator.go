package orchestrator

import (
	"context"
	"fmt"

	"github.com/MateoSegura/.claude-test/internal/corpus"
	"github.com/MateoSegura/.claude-test/internal/docker"
	"github.com/MateoSegura/.claude-test/internal/metrics"
)

// ContainerManager handles container lifecycle.
type ContainerManager interface {
	CreateContainer(ctx context.Context, opts docker.ContainerOpts) (string, error)
	CopyToContainer(ctx context.Context, containerID, src, dst string) error
	RemoveContainer(ctx context.Context, id string) error
	IsAvailable(ctx context.Context) error
}

// TaskRunner executes Claude inside a container and returns a RunResult.
type TaskRunner interface {
	Run(ctx context.Context, cfg docker.RunConfig) (*metrics.RunResult, error)
}

// Orchestrator coordinates benchmark execution across containers.
type Orchestrator struct {
	config  *BenchConfig
	issues  []corpus.Issue
	manager ContainerManager
	runner  TaskRunner
}

// New creates a production Orchestrator by loading the corpus, applying filters,
// and constructing Docker dependencies.
func New(cfg *BenchConfig) (*Orchestrator, error) {
	c, err := corpus.Load(cfg.Corpus.Path)
	if err != nil {
		return nil, fmt.Errorf("loading corpus: %w", err)
	}

	filtered := c.Filter(corpus.CorpusFilter{
		Difficulty: cfg.Corpus.Filters.Difficulty,
		Language:   cfg.Corpus.Filters.Language,
		TaskType:   cfg.Corpus.Filters.TaskType,
	})

	mgr := docker.NewManager(cfg.Docker.Image)
	executor := docker.NewDockerExecutor(mgr)
	runner := docker.NewRunner(executor)

	return &Orchestrator{
		config:  cfg,
		issues:  filtered.Issues,
		manager: mgr,
		runner:  runner,
	}, nil
}

// NewWithDeps creates an Orchestrator with pre-built dependencies and pre-loaded
// issues, enabling unit testing without real Docker or filesystem access.
// When manager or runner are nil, production defaults using Docker are created.
func NewWithDeps(cfg *BenchConfig, issues []corpus.Issue, manager ContainerManager, runner TaskRunner) *Orchestrator {
	if manager == nil || runner == nil {
		mgr := docker.NewManager(cfg.Docker.Image)
		if manager == nil {
			manager = mgr
		}
		if runner == nil {
			executor := docker.NewDockerExecutor(mgr)
			runner = docker.NewRunner(executor)
		}
	}
	return &Orchestrator{
		config:  cfg,
		issues:  issues,
		manager: manager,
		runner:  runner,
	}
}

// DryRun generates the execution plan without running anything.
func (o *Orchestrator) DryRun() []RunPair {
	return GeneratePlan(o.config, o.issues)
}

// Run generates the plan, flattens it, and executes all specs through the pool.
func (o *Orchestrator) Run(ctx context.Context) ([]*metrics.RunResult, error) {
	pairs := GeneratePlan(o.config, o.issues)
	specs := FlattenPlan(pairs)

	pool := NewPool(o.config.Execution.Parallelism, o.makeWorkerFunc())
	return pool.Execute(ctx, specs, nil)
}

// makeWorkerFunc returns a closure that executes a single RunSpec inside a
// container and returns the result with metadata populated.
func (o *Orchestrator) makeWorkerFunc() WorkerFunc {
	return func(ctx context.Context, spec RunSpec) (*metrics.RunResult, error) {
		// Create container.
		containerID, err := o.manager.CreateContainer(ctx, docker.ContainerOpts{
			Image:   spec.Image,
			WorkDir: "/workspace",
		})
		if err != nil {
			// Container creation failure is not a pool-level error.
			return &metrics.RunResult{
				Success:  false,
				ErrorMsg: err.Error(),
				IssueID:  spec.Issue.ID,
				Variant:  string(spec.Variant),
				Attempt:  spec.Attempt,
			}, nil
		}

		// Ensure container cleanup.
		defer o.manager.RemoveContainer(ctx, containerID)

		// Copy config directory for configured variants.
		if spec.Variant == Configured && spec.ConfigDir != "" {
			if err := o.manager.CopyToContainer(ctx, containerID, spec.ConfigDir, "/home/user/.claude"); err != nil {
				return &metrics.RunResult{
					Success:    false,
					ErrorMsg:   fmt.Sprintf("copy config: %s", err.Error()),
					IssueID:    spec.Issue.ID,
					Variant:    string(spec.Variant),
					Attempt:    spec.Attempt,
					Difficulty: spec.Issue.Difficulty,
					Language:   spec.Issue.Language,
					TaskType:   spec.Issue.TaskType,
				}, nil
			}
		}

		// Build run configuration.
		runCfg := docker.RunConfig{
			ContainerID: containerID,
			Prompt:      spec.Issue.Prompt,
			MaxTurns:    spec.MaxTurns,
			Timeout:     spec.Timeout,
			WorkDir:     "/workspace",
			SetupCmds:   spec.Issue.SetupCmds,
			Evaluation:  spec.Issue.Evaluation,
		}

		// Execute the run.
		result, err := o.runner.Run(ctx, runCfg)
		if err != nil {
			return &metrics.RunResult{
				Success:    false,
				ErrorMsg:   err.Error(),
				IssueID:    spec.Issue.ID,
				Variant:    string(spec.Variant),
				Attempt:    spec.Attempt,
				Difficulty: spec.Issue.Difficulty,
				Language:   spec.Issue.Language,
				TaskType:   spec.Issue.TaskType,
			}, nil
		}

		// Populate metadata on the result.
		result.IssueID = spec.Issue.ID
		result.Variant = string(spec.Variant)
		result.Attempt = spec.Attempt
		result.Difficulty = spec.Issue.Difficulty
		result.Language = spec.Issue.Language
		result.TaskType = spec.Issue.TaskType

		return result, nil
	}
}
