package main

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MateoSegura/.claude-test/internal/metrics"
	"github.com/MateoSegura/.claude-test/internal/orchestrator"
)

// ProgressReporter prints progress lines as benchmark runs complete.
type ProgressReporter struct {
	w       io.Writer
	total   int
	counter atomic.Int32
	verbose bool
	mu      sync.Mutex
}

// NewProgressReporter creates a reporter that tracks progress toward total runs.
func NewProgressReporter(w io.Writer, total int, verbose bool) *ProgressReporter {
	return &ProgressReporter{
		w:       w,
		total:   total,
		verbose: verbose,
	}
}

// OnResult formats and prints a single run result. It is safe for concurrent use.
func (p *ProgressReporter) OnResult(spec orchestrator.RunSpec, result *metrics.RunResult) {
	n := p.counter.Add(1)
	duration := time.Duration(result.DurationMS) * time.Millisecond
	durationStr := fmt.Sprintf("%ds", int(duration.Seconds()))

	var line string
	if result.Success {
		line = fmt.Sprintf("[%d/%d] %s (%s, attempt %d) ... OK (%.2f, $%.2f, %s)",
			n, p.total,
			spec.Issue.ID,
			spec.Variant,
			spec.Attempt,
			result.Score,
			result.CostUSD,
			durationStr,
		)
	} else {
		line = fmt.Sprintf("[%d/%d] %s (%s, attempt %d) ... FAIL ($%.2f, %s)",
			n, p.total,
			spec.Issue.ID,
			spec.Variant,
			spec.Attempt,
			result.CostUSD,
			durationStr,
		)
	}

	if p.verbose {
		toolCount := len(result.ToolCalls)
		line += fmt.Sprintf(" [turns=%d, tools=%d]", result.NumTurns, toolCount)
	}

	p.mu.Lock()
	fmt.Fprintln(p.w, line)
	p.mu.Unlock()
}
