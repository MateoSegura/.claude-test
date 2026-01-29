package orchestrator

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/MateoSegura/.claude-test/internal/metrics"
)

// WorkerFunc executes a single RunSpec and returns its result.
type WorkerFunc func(ctx context.Context, spec RunSpec) (*metrics.RunResult, error)

// Pool manages concurrent execution of benchmark runs.
type Pool struct {
	parallelism int
	workerFn    WorkerFunc
}

// NewPool creates a pool with the given concurrency limit and worker function.
func NewPool(parallelism int, workerFn WorkerFunc) *Pool {
	return &Pool{
		parallelism: parallelism,
		workerFn:    workerFn,
	}
}

// Execute runs all specs through the worker pool, respecting the concurrency
// limit. Results are returned in the same order as the input specs. Worker
// errors are captured in the result rather than aborting the pool; only
// context errors propagate as the return error.
func (p *Pool) Execute(ctx context.Context, specs []RunSpec, progress func(RunSpec, *metrics.RunResult)) ([]*metrics.RunResult, error) {
	if len(specs) == 0 {
		return []*metrics.RunResult{}, nil
	}

	results := make([]*metrics.RunResult, len(specs))
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(p.parallelism)

	for i, spec := range specs {
		i, spec := i, spec
		g.Go(func() error {
			// Check context before starting work
			if err := ctx.Err(); err != nil {
				return err
			}

			result, err := p.workerFn(ctx, spec)
			if err != nil {
				result = &metrics.RunResult{
					IssueID:  spec.Issue.ID,
					Variant:  string(spec.Variant),
					Attempt:  spec.Attempt,
					Success:  false,
					ErrorMsg: err.Error(),
				}
			}

			results[i] = result

			if progress != nil {
				mu.Lock()
				progress(spec, result)
				mu.Unlock()
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return results, err
	}

	return results, nil
}
