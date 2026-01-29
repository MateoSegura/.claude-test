package main

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/MateoSegura/.claude-test/internal/corpus"
	"github.com/MateoSegura/.claude-test/internal/metrics"
	"github.com/MateoSegura/.claude-test/internal/orchestrator"
)

func TestProgressFormat(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressReporter(&buf, 10, false)

	spec := orchestrator.RunSpec{
		Issue: corpus.Issue{
			ID: "go-nil-check",
		},
		Variant: orchestrator.Configured,
		Attempt: 3,
	}

	result := &metrics.RunResult{
		Success:    true,
		Score:      0.85,
		CostUSD:    0.12,
		DurationMS: 12000,
		NumTurns:   5,
	}

	p.OnResult(spec, result)
	output := buf.String()

	if !strings.Contains(output, "[1/10]") {
		t.Errorf("output should contain counter [1/10], got: %s", output)
	}
	if !strings.Contains(output, "go-nil-check") {
		t.Errorf("output should contain issue ID, got: %s", output)
	}
	if !strings.Contains(output, "configured") {
		t.Errorf("output should contain variant, got: %s", output)
	}
	if !strings.Contains(output, "OK") {
		t.Errorf("output should contain OK, got: %s", output)
	}
	if !strings.Contains(output, "0.85") {
		t.Errorf("output should contain score, got: %s", output)
	}
	if !strings.Contains(output, "$0.12") {
		t.Errorf("output should contain cost, got: %s", output)
	}
	if !strings.Contains(output, "12s") {
		t.Errorf("output should contain duration, got: %s", output)
	}
}

func TestProgressFailure(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressReporter(&buf, 10, false)

	spec := orchestrator.RunSpec{
		Issue: corpus.Issue{
			ID: "ts-auth",
		},
		Variant: orchestrator.Baseline,
		Attempt: 1,
	}

	result := &metrics.RunResult{
		Success:    false,
		CostUSD:    0.08,
		DurationMS: 5000,
	}

	p.OnResult(spec, result)
	output := buf.String()

	if !strings.Contains(output, "FAIL") {
		t.Errorf("output should contain FAIL, got: %s", output)
	}
	if strings.Contains(output, "OK") {
		t.Errorf("output should not contain OK for failure, got: %s", output)
	}
	// Score should not appear in failure output.
	if strings.Contains(output, "0.85") {
		t.Errorf("output should not contain a score for failure, got: %s", output)
	}
}

func TestProgressConcurrent(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressReporter(&buf, 10, false)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(attempt int) {
			defer wg.Done()
			spec := orchestrator.RunSpec{
				Issue: corpus.Issue{
					ID: fmt.Sprintf("issue-%d", attempt),
				},
				Variant: orchestrator.Configured,
				Attempt: attempt,
			}
			result := &metrics.RunResult{
				Success:    true,
				Score:      0.9,
				CostUSD:    0.10,
				DurationMS: 1000,
			}
			p.OnResult(spec, result)
		}(i + 1)
	}
	wg.Wait()

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 10 {
		t.Errorf("expected 10 lines, got %d:\n%s", len(lines), output)
	}

	// Verify all counters 1-10 are present.
	for i := 1; i <= 10; i++ {
		counter := fmt.Sprintf("[%d/10]", i)
		if !strings.Contains(output, counter) {
			t.Errorf("output should contain counter %s, got:\n%s", counter, output)
		}
	}
}

func TestProgressVerbose(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressReporter(&buf, 5, true)

	spec := orchestrator.RunSpec{
		Issue: corpus.Issue{
			ID: "go-nil-check",
		},
		Variant: orchestrator.Configured,
		Attempt: 1,
	}

	result := &metrics.RunResult{
		Success:    true,
		Score:      0.9,
		CostUSD:    0.15,
		DurationMS: 8000,
		NumTurns:   7,
		ToolCalls: []metrics.ToolCall{
			{Name: "read_file"},
			{Name: "write_file"},
			{Name: "bash"},
		},
	}

	p.OnResult(spec, result)
	output := buf.String()

	if !strings.Contains(output, "turns=7") {
		t.Errorf("verbose output should contain turns, got: %s", output)
	}
	if !strings.Contains(output, "tools=3") {
		t.Errorf("verbose output should contain tool count, got: %s", output)
	}
}
