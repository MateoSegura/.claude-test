package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MateoSegura/.claude-test/internal/corpus"
	"github.com/MateoSegura/.claude-test/internal/docker"
	"github.com/MateoSegura/.claude-test/internal/metrics"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type copyCall struct {
	ContainerID, Src, Dst string
}

type mockManager struct {
	createFunc func(ctx context.Context, opts docker.ContainerOpts) (string, error)
	copyFunc   func(ctx context.Context, containerID, src, dst string) error
	removeFunc func(ctx context.Context, id string) error
	availFunc  func(ctx context.Context) error

	createCalls []docker.ContainerOpts
	copyCalls   []copyCall
	removeCalls []string
	mu          sync.Mutex

	nextID atomic.Int64
}

func (m *mockManager) CreateContainer(ctx context.Context, opts docker.ContainerOpts) (string, error) {
	m.mu.Lock()
	m.createCalls = append(m.createCalls, opts)
	m.mu.Unlock()

	if m.createFunc != nil {
		return m.createFunc(ctx, opts)
	}
	id := fmt.Sprintf("container-%d", m.nextID.Add(1))
	return id, nil
}

func (m *mockManager) CopyToContainer(ctx context.Context, containerID, src, dst string) error {
	m.mu.Lock()
	m.copyCalls = append(m.copyCalls, copyCall{ContainerID: containerID, Src: src, Dst: dst})
	m.mu.Unlock()

	if m.copyFunc != nil {
		return m.copyFunc(ctx, containerID, src, dst)
	}
	return nil
}

func (m *mockManager) RemoveContainer(ctx context.Context, id string) error {
	m.mu.Lock()
	m.removeCalls = append(m.removeCalls, id)
	m.mu.Unlock()

	if m.removeFunc != nil {
		return m.removeFunc(ctx, id)
	}
	return nil
}

func (m *mockManager) IsAvailable(ctx context.Context) error {
	if m.availFunc != nil {
		return m.availFunc(ctx)
	}
	return nil
}

type mockRunner struct {
	runFunc func(ctx context.Context, cfg docker.RunConfig) (*metrics.RunResult, error)
	calls   []docker.RunConfig
	mu      sync.Mutex
}

func (r *mockRunner) Run(ctx context.Context, cfg docker.RunConfig) (*metrics.RunResult, error) {
	r.mu.Lock()
	r.calls = append(r.calls, cfg)
	r.mu.Unlock()

	if r.runFunc != nil {
		return r.runFunc(ctx, cfg)
	}
	return &metrics.RunResult{Success: true}, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestOrchestrator_DryRun(t *testing.T) {
	cfg := makeConfig(5, 2, BoolPtr(true))
	issues := makeIssues(3)
	mgr := &mockManager{}
	runner := &mockRunner{}

	orch := NewWithDeps(cfg, issues, mgr, runner)
	pairs := orch.DryRun()

	if len(pairs) != 3 {
		t.Fatalf("got %d pairs, want 3", len(pairs))
	}
	for _, p := range pairs {
		if len(p.Configured) != 5 {
			t.Errorf("configured = %d, want 5", len(p.Configured))
		}
		if len(p.Baseline) != 5 {
			t.Errorf("baseline = %d, want 5", len(p.Baseline))
		}
	}

	runner.mu.Lock()
	callCount := len(runner.calls)
	runner.mu.Unlock()
	if callCount != 0 {
		t.Errorf("runner called %d times during DryRun, want 0", callCount)
	}
}

func TestOrchestrator_Run_FullMock(t *testing.T) {
	cfg := makeConfig(2, 4, BoolPtr(true))
	issues := makeIssues(2)
	mgr := &mockManager{}
	runner := &mockRunner{
		runFunc: func(_ context.Context, _ docker.RunConfig) (*metrics.RunResult, error) {
			return &metrics.RunResult{
				Success:  true,
				CostUSD:  0.05,
				NumTurns: 8,
			}, nil
		},
	}

	orch := NewWithDeps(cfg, issues, mgr, runner)
	results, err := orch.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// 2 issues * 2 attempts * 2 variants = 8 results
	if len(results) != 8 {
		t.Fatalf("got %d results, want 8", len(results))
	}

	for _, r := range results {
		if !r.Success {
			t.Errorf("result for %s/%s/%d not successful", r.IssueID, r.Variant, r.Attempt)
		}
		if r.IssueID == "" {
			t.Error("result missing IssueID")
		}
		if r.Variant == "" {
			t.Error("result missing Variant")
		}
		if r.Attempt < 1 {
			t.Errorf("result has invalid Attempt: %d", r.Attempt)
		}
	}

	mgr.mu.Lock()
	createCount := len(mgr.createCalls)
	removeCount := len(mgr.removeCalls)
	mgr.mu.Unlock()

	if createCount != 8 {
		t.Errorf("CreateContainer called %d times, want 8", createCount)
	}
	if removeCount != 8 {
		t.Errorf("RemoveContainer called %d times, want 8", removeCount)
	}
}

func TestOrchestrator_Run_BaselineDisabled(t *testing.T) {
	cfg := makeConfig(3, 4, BoolPtr(false))
	issues := makeIssues(2)
	mgr := &mockManager{}
	runner := &mockRunner{
		runFunc: func(_ context.Context, _ docker.RunConfig) (*metrics.RunResult, error) {
			return &metrics.RunResult{Success: true}, nil
		},
	}

	orch := NewWithDeps(cfg, issues, mgr, runner)
	results, err := orch.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// 2 issues * 3 attempts * 1 variant = 6
	if len(results) != 6 {
		t.Fatalf("got %d results, want 6", len(results))
	}
	for _, r := range results {
		if r.Variant != string(Configured) {
			t.Errorf("expected all Configured, got %s", r.Variant)
		}
	}
}

func TestOrchestrator_New_CorpusLoadFailure(t *testing.T) {
	cfg := &BenchConfig{
		Config:    ConfigSpec{Path: "/test/config"},
		Docker:    DockerSpec{Image: "test:latest"},
		Corpus:    CorpusSpec{Path: "/nonexistent/corpus.yaml"},
		Execution: ExecSpec{Attempts: 1, Parallelism: 1, Timeout: time.Minute, Baseline: BoolPtr(true), MaxTurns: 10},
	}

	_, err := New(cfg)
	if err == nil {
		t.Fatal("expected error for non-existent corpus, got nil")
	}
}

func TestOrchestrator_Run_WithFilter(t *testing.T) {
	cfg := makeConfig(2, 4, BoolPtr(true))
	// Pre-filter: only include go issues.
	goIssues := []corpus.Issue{
		{ID: "go-1", Prompt: "Fix go bug 1", Language: "go"},
		{ID: "go-2", Prompt: "Fix go bug 2", Language: "go"},
		{ID: "go-3", Prompt: "Fix go bug 3", Language: "go"},
	}

	mgr := &mockManager{}
	runner := &mockRunner{
		runFunc: func(_ context.Context, _ docker.RunConfig) (*metrics.RunResult, error) {
			return &metrics.RunResult{Success: true}, nil
		},
	}

	orch := NewWithDeps(cfg, goIssues, mgr, runner)
	results, err := orch.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// 3 issues * 2 attempts * 2 variants = 12
	if len(results) != 12 {
		t.Fatalf("got %d results, want 12", len(results))
	}

	for _, r := range results {
		if r.Language != "go" {
			t.Errorf("expected language=go, got %q", r.Language)
		}
	}
}

func TestOrchestrator_Run_ContextCancellation(t *testing.T) {
	cfg := makeConfig(5, 2, BoolPtr(true))
	issues := makeIssues(10)
	mgr := &mockManager{}
	runner := &mockRunner{
		runFunc: func(ctx context.Context, _ docker.RunConfig) (*metrics.RunResult, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
				return &metrics.RunResult{Success: true}, nil
			}
		},
	}

	orch := NewWithDeps(cfg, issues, mgr, runner)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := orch.Run(ctx)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}

func TestOrchestrator_WorkerFunc_ConfiguredVariant(t *testing.T) {
	cfg := makeConfig(1, 1, BoolPtr(true))
	mgr := &mockManager{}
	runner := &mockRunner{
		runFunc: func(_ context.Context, _ docker.RunConfig) (*metrics.RunResult, error) {
			return &metrics.RunResult{Success: true, CostUSD: 0.10}, nil
		},
	}

	orch := NewWithDeps(cfg, nil, mgr, runner)
	workerFn := orch.makeWorkerFunc()

	spec := RunSpec{
		Issue:     corpus.Issue{ID: "test-1", Prompt: "fix it", Difficulty: "medium", Language: "go", TaskType: "bug_fix"},
		Variant:   Configured,
		Attempt:   1,
		ConfigDir: ".claude",
		Image:     "test:latest",
		Timeout:   5 * time.Minute,
		MaxTurns:  25,
	}

	result, err := workerFn(context.Background(), spec)
	if err != nil {
		t.Fatalf("worker returned error: %v", err)
	}

	if !result.Success {
		t.Error("expected success")
	}
	if result.IssueID != "test-1" {
		t.Errorf("IssueID = %q, want test-1", result.IssueID)
	}
	if result.Variant != string(Configured) {
		t.Errorf("Variant = %q, want configured", result.Variant)
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if len(mgr.createCalls) != 1 {
		t.Fatalf("CreateContainer called %d times, want 1", len(mgr.createCalls))
	}
	if len(mgr.copyCalls) != 1 {
		t.Fatalf("CopyToContainer called %d times, want 1", len(mgr.copyCalls))
	}
	if mgr.copyCalls[0].Src != ".claude" {
		t.Errorf("CopyToContainer src = %q, want .claude", mgr.copyCalls[0].Src)
	}
	if mgr.copyCalls[0].Dst != "/home/user/.claude" {
		t.Errorf("CopyToContainer dst = %q, want /home/user/.claude", mgr.copyCalls[0].Dst)
	}
	if len(mgr.removeCalls) != 1 {
		t.Errorf("RemoveContainer called %d times, want 1", len(mgr.removeCalls))
	}
}

func TestOrchestrator_WorkerFunc_BaselineVariant(t *testing.T) {
	cfg := makeConfig(1, 1, BoolPtr(true))
	mgr := &mockManager{}
	runner := &mockRunner{
		runFunc: func(_ context.Context, _ docker.RunConfig) (*metrics.RunResult, error) {
			return &metrics.RunResult{Success: true}, nil
		},
	}

	orch := NewWithDeps(cfg, nil, mgr, runner)
	workerFn := orch.makeWorkerFunc()

	spec := RunSpec{
		Issue:     corpus.Issue{ID: "base-1", Prompt: "fix it"},
		Variant:   Baseline,
		Attempt:   1,
		ConfigDir: "",
		Image:     "test:latest",
		Timeout:   5 * time.Minute,
		MaxTurns:  25,
	}

	result, err := workerFn(context.Background(), spec)
	if err != nil {
		t.Fatalf("worker returned error: %v", err)
	}

	if !result.Success {
		t.Error("expected success")
	}
	if result.Variant != string(Baseline) {
		t.Errorf("Variant = %q, want baseline", result.Variant)
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if len(mgr.copyCalls) != 0 {
		t.Errorf("CopyToContainer called %d times for baseline, want 0", len(mgr.copyCalls))
	}
	if len(mgr.removeCalls) != 1 {
		t.Errorf("RemoveContainer called %d times, want 1", len(mgr.removeCalls))
	}
}

func TestOrchestrator_WorkerFunc_ContainerCreateFailure(t *testing.T) {
	cfg := makeConfig(1, 1, BoolPtr(true))
	mgr := &mockManager{
		createFunc: func(_ context.Context, _ docker.ContainerOpts) (string, error) {
			return "", fmt.Errorf("docker daemon not running")
		},
	}
	runner := &mockRunner{}

	orch := NewWithDeps(cfg, nil, mgr, runner)
	workerFn := orch.makeWorkerFunc()

	spec := RunSpec{
		Issue:   corpus.Issue{ID: "fail-1", Prompt: "fix it"},
		Variant: Configured,
		Attempt: 2,
		Image:   "test:latest",
	}

	result, err := workerFn(context.Background(), spec)
	if err != nil {
		t.Fatalf("worker should not return pool-level error, got: %v", err)
	}

	if result.Success {
		t.Error("expected failure result")
	}
	if result.ErrorMsg == "" {
		t.Error("expected error message in result")
	}
	if result.IssueID != "fail-1" {
		t.Errorf("IssueID = %q, want fail-1", result.IssueID)
	}
	if result.Attempt != 2 {
		t.Errorf("Attempt = %d, want 2", result.Attempt)
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if len(mgr.removeCalls) != 0 {
		t.Errorf("RemoveContainer called %d times after create failure, want 0", len(mgr.removeCalls))
	}

	runner.mu.Lock()
	defer runner.mu.Unlock()

	if len(runner.calls) != 0 {
		t.Errorf("Runner called %d times after create failure, want 0", len(runner.calls))
	}
}

func TestOrchestrator_WorkerFunc_RunnerFailure(t *testing.T) {
	cfg := makeConfig(1, 1, BoolPtr(true))
	mgr := &mockManager{}
	runner := &mockRunner{
		runFunc: func(_ context.Context, _ docker.RunConfig) (*metrics.RunResult, error) {
			return nil, fmt.Errorf("claude crashed")
		},
	}

	orch := NewWithDeps(cfg, nil, mgr, runner)
	workerFn := orch.makeWorkerFunc()

	spec := RunSpec{
		Issue:   corpus.Issue{ID: "crash-1", Prompt: "fix it"},
		Variant: Configured,
		Attempt: 1,
		Image:   "test:latest",
	}

	result, err := workerFn(context.Background(), spec)
	if err != nil {
		t.Fatalf("worker should not return pool-level error, got: %v", err)
	}

	if result.Success {
		t.Error("expected failure result")
	}
	if result.ErrorMsg == "" {
		t.Error("expected error message in result")
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if len(mgr.removeCalls) != 1 {
		t.Errorf("RemoveContainer called %d times, want 1", len(mgr.removeCalls))
	}
}

func TestOrchestrator_WorkerFunc_MetadataPropagation(t *testing.T) {
	cfg := makeConfig(1, 1, BoolPtr(true))
	mgr := &mockManager{}
	runner := &mockRunner{
		runFunc: func(_ context.Context, _ docker.RunConfig) (*metrics.RunResult, error) {
			return &metrics.RunResult{Success: true}, nil
		},
	}

	orch := NewWithDeps(cfg, nil, mgr, runner)
	workerFn := orch.makeWorkerFunc()

	spec := RunSpec{
		Issue: corpus.Issue{
			ID:         "meta-1",
			Prompt:     "fix it",
			Difficulty: "easy",
			Language:   "go",
			TaskType:   "bug_fix",
		},
		Variant:  Configured,
		Attempt:  3,
		Image:    "test:latest",
		Timeout:  5 * time.Minute,
		MaxTurns: 25,
	}

	result, err := workerFn(context.Background(), spec)
	if err != nil {
		t.Fatalf("worker returned error: %v", err)
	}

	if result.Difficulty != "easy" {
		t.Errorf("Difficulty = %q, want easy", result.Difficulty)
	}
	if result.Language != "go" {
		t.Errorf("Language = %q, want go", result.Language)
	}
	if result.TaskType != "bug_fix" {
		t.Errorf("TaskType = %q, want bug_fix", result.TaskType)
	}
	if result.IssueID != "meta-1" {
		t.Errorf("IssueID = %q, want meta-1", result.IssueID)
	}
	if result.Variant != string(Configured) {
		t.Errorf("Variant = %q, want configured", result.Variant)
	}
	if result.Attempt != 3 {
		t.Errorf("Attempt = %d, want 3", result.Attempt)
	}
}
