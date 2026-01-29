package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MateoSegura/.claude-test/internal/corpus"
	"github.com/MateoSegura/.claude-test/internal/metrics"
)

func makeSpecs(n int) []RunSpec {
	specs := make([]RunSpec, n)
	for i := range specs {
		specs[i] = RunSpec{
			Issue:   corpus.Issue{ID: fmt.Sprintf("spec-%d", i)},
			Variant: Configured,
			Attempt: i + 1,
		}
	}
	return specs
}

func TestPool_AllSucceed(t *testing.T) {
	worker := func(ctx context.Context, spec RunSpec) (*metrics.RunResult, error) {
		return &metrics.RunResult{
			IssueID: spec.Issue.ID,
			Success: true,
		}, nil
	}

	pool := NewPool(2, worker)
	specs := makeSpecs(6)

	results, err := pool.Execute(context.Background(), specs, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(results) != 6 {
		t.Fatalf("got %d results, want 6", len(results))
	}
	for i, r := range results {
		if !r.Success {
			t.Errorf("result[%d] not successful", i)
		}
	}
}

func TestPool_ConcurrencyBound(t *testing.T) {
	var active, maxActive atomic.Int32
	worker := func(ctx context.Context, spec RunSpec) (*metrics.RunResult, error) {
		cur := active.Add(1)
		for {
			old := maxActive.Load()
			if cur <= old || maxActive.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		active.Add(-1)
		return &metrics.RunResult{Success: true, IssueID: spec.Issue.ID}, nil
	}

	pool := NewPool(3, worker)
	specs := makeSpecs(20)

	results, err := pool.Execute(context.Background(), specs, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(results) != 20 {
		t.Fatalf("got %d results, want 20", len(results))
	}
	if maxActive.Load() > 3 {
		t.Errorf("max concurrent = %d, want <= 3", maxActive.Load())
	}
}

func TestPool_PartialFailure(t *testing.T) {
	worker := func(ctx context.Context, spec RunSpec) (*metrics.RunResult, error) {
		if spec.Attempt == 3 {
			return nil, fmt.Errorf("worker error on attempt 3")
		}
		return &metrics.RunResult{
			IssueID: spec.Issue.ID,
			Success: true,
		}, nil
	}

	pool := NewPool(2, worker)
	specs := makeSpecs(5)

	results, err := pool.Execute(context.Background(), specs, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("got %d results, want 5", len(results))
	}

	// Index 2 corresponds to attempt 3 (0-indexed spec, attempt = i+1)
	if results[2].ErrorMsg == "" {
		t.Error("result[2] should have error message")
	}
	if results[2].Success {
		t.Error("result[2] should not be successful")
	}

	for i, r := range results {
		if i == 2 {
			continue
		}
		if !r.Success {
			t.Errorf("result[%d] should be successful", i)
		}
	}
}

func TestPool_ContextCancellation(t *testing.T) {
	worker := func(ctx context.Context, spec RunSpec) (*metrics.RunResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return &metrics.RunResult{Success: true}, nil
		}
	}

	pool := NewPool(2, worker)
	specs := makeSpecs(10)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := pool.Execute(ctx, specs, nil)
	if err == nil {
		t.Fatal("expected context error")
	}
}

func TestPool_EmptySpecs(t *testing.T) {
	worker := func(ctx context.Context, spec RunSpec) (*metrics.RunResult, error) {
		t.Fatal("worker should not be called")
		return nil, nil
	}

	pool := NewPool(2, worker)
	results, err := pool.Execute(context.Background(), []RunSpec{}, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestPool_ProgressCallback(t *testing.T) {
	worker := func(ctx context.Context, spec RunSpec) (*metrics.RunResult, error) {
		return &metrics.RunResult{
			IssueID: spec.Issue.ID,
			Success: true,
		}, nil
	}

	pool := NewPool(2, worker)
	specs := makeSpecs(4)

	var mu sync.Mutex
	var callCount int
	progress := func(spec RunSpec, result *metrics.RunResult) {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	results, err := pool.Execute(context.Background(), specs, progress)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("got %d results, want 4", len(results))
	}
	if callCount != 4 {
		t.Errorf("progress called %d times, want 4", callCount)
	}
}

func TestPool_Parallelism1(t *testing.T) {
	var active, maxActive atomic.Int32
	worker := func(ctx context.Context, spec RunSpec) (*metrics.RunResult, error) {
		cur := active.Add(1)
		for {
			old := maxActive.Load()
			if cur <= old || maxActive.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		active.Add(-1)
		return &metrics.RunResult{Success: true, IssueID: spec.Issue.ID}, nil
	}

	pool := NewPool(1, worker)
	specs := makeSpecs(5)

	results, err := pool.Execute(context.Background(), specs, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("got %d results, want 5", len(results))
	}
	if maxActive.Load() != 1 {
		t.Errorf("max concurrent = %d, want 1", maxActive.Load())
	}
}
