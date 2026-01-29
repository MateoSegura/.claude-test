package orchestrator

import (
	"fmt"
	"testing"
	"time"

	"github.com/MateoSegura/.claude-test/internal/corpus"
)

func makeIssues(n int) []corpus.Issue {
	issues := make([]corpus.Issue, n)
	for i := range issues {
		issues[i] = corpus.Issue{
			ID:     fmt.Sprintf("issue-%d", i),
			Title:  fmt.Sprintf("Issue %d", i),
			Prompt: fmt.Sprintf("Fix issue %d", i),
		}
	}
	return issues
}

func makeConfig(attempts, parallelism int, baseline *bool) *BenchConfig {
	return &BenchConfig{
		Config: ConfigSpec{Path: "/test/config"},
		Docker: DockerSpec{Image: "test:latest"},
		Execution: ExecSpec{
			Attempts:    attempts,
			Parallelism: parallelism,
			Timeout:     5 * time.Minute,
			Baseline:    baseline,
			MaxTurns:    25,
		},
	}
}

func TestGeneratePlan_Standard(t *testing.T) {
	cfg := makeConfig(5, 2, BoolPtr(true))
	issues := makeIssues(3)

	pairs := GeneratePlan(cfg, issues)
	if len(pairs) != 3 {
		t.Fatalf("got %d pairs, want 3", len(pairs))
	}

	total := 0
	for _, p := range pairs {
		if len(p.Configured) != 5 {
			t.Errorf("configured specs = %d, want 5", len(p.Configured))
		}
		if len(p.Baseline) != 5 {
			t.Errorf("baseline specs = %d, want 5", len(p.Baseline))
		}
		total += len(p.Configured) + len(p.Baseline)
	}
	if total != 30 {
		t.Errorf("total specs = %d, want 30", total)
	}
}

func TestGeneratePlan_BaselineDisabled(t *testing.T) {
	cfg := makeConfig(3, 2, BoolPtr(false))
	issues := makeIssues(2)

	pairs := GeneratePlan(cfg, issues)
	if len(pairs) != 2 {
		t.Fatalf("got %d pairs, want 2", len(pairs))
	}

	total := 0
	for _, p := range pairs {
		if len(p.Configured) != 3 {
			t.Errorf("configured specs = %d, want 3", len(p.Configured))
		}
		if len(p.Baseline) != 0 {
			t.Errorf("baseline specs = %d, want 0", len(p.Baseline))
		}
		total += len(p.Configured) + len(p.Baseline)
	}
	if total != 6 {
		t.Errorf("total specs = %d, want 6", total)
	}
}

func TestGeneratePlan_SingleAttempt(t *testing.T) {
	cfg := makeConfig(1, 1, BoolPtr(true))
	issues := makeIssues(1)

	pairs := GeneratePlan(cfg, issues)
	if len(pairs) != 1 {
		t.Fatalf("got %d pairs, want 1", len(pairs))
	}
	if len(pairs[0].Configured) != 1 {
		t.Errorf("configured = %d, want 1", len(pairs[0].Configured))
	}
	if len(pairs[0].Baseline) != 1 {
		t.Errorf("baseline = %d, want 1", len(pairs[0].Baseline))
	}
}

func TestGeneratePlan_EmptyCorpus(t *testing.T) {
	cfg := makeConfig(5, 2, BoolPtr(true))
	pairs := GeneratePlan(cfg, []corpus.Issue{})
	if len(pairs) != 0 {
		t.Errorf("got %d pairs, want 0", len(pairs))
	}
}

func TestGeneratePlan_IssueFieldsPropagate(t *testing.T) {
	cfg := makeConfig(2, 1, BoolPtr(true))
	issues := []corpus.Issue{
		{ID: "test-123", Prompt: "fix the bug", Title: "Bug Fix"},
	}

	pairs := GeneratePlan(cfg, issues)
	for _, spec := range pairs[0].Configured {
		if spec.Issue.ID != "test-123" {
			t.Errorf("Issue.ID = %q, want test-123", spec.Issue.ID)
		}
		if spec.Issue.Prompt != "fix the bug" {
			t.Errorf("Issue.Prompt = %q", spec.Issue.Prompt)
		}
	}
	for _, spec := range pairs[0].Baseline {
		if spec.Issue.ID != "test-123" {
			t.Errorf("Baseline Issue.ID = %q, want test-123", spec.Issue.ID)
		}
	}
}

func TestGeneratePlan_LargePlan(t *testing.T) {
	cfg := makeConfig(7, 2, BoolPtr(true))
	issues := makeIssues(10)

	pairs := GeneratePlan(cfg, issues)
	if len(pairs) != 10 {
		t.Fatalf("got %d pairs, want 10", len(pairs))
	}

	total := 0
	for _, p := range pairs {
		total += len(p.Configured) + len(p.Baseline)
	}
	if total != 140 {
		t.Errorf("total specs = %d, want 140", total)
	}
}

func TestFlattenPlan_Standard(t *testing.T) {
	cfg := makeConfig(3, 1, BoolPtr(true))
	issues := makeIssues(2)
	pairs := GeneratePlan(cfg, issues)

	specs := FlattenPlan(pairs)
	if len(specs) != 12 {
		t.Fatalf("got %d specs, want 12", len(specs))
	}

	// First 3: pair[0] configured
	for i := 0; i < 3; i++ {
		if specs[i].Variant != Configured || specs[i].Issue.ID != "issue-0" {
			t.Errorf("spec[%d] = %v/%s, want configured/issue-0", i, specs[i].Variant, specs[i].Issue.ID)
		}
	}
	// Next 3: pair[0] baseline
	for i := 3; i < 6; i++ {
		if specs[i].Variant != Baseline || specs[i].Issue.ID != "issue-0" {
			t.Errorf("spec[%d] = %v/%s, want baseline/issue-0", i, specs[i].Variant, specs[i].Issue.ID)
		}
	}
	// Next 3: pair[1] configured
	for i := 6; i < 9; i++ {
		if specs[i].Variant != Configured || specs[i].Issue.ID != "issue-1" {
			t.Errorf("spec[%d] = %v/%s, want configured/issue-1", i, specs[i].Variant, specs[i].Issue.ID)
		}
	}
	// Last 3: pair[1] baseline
	for i := 9; i < 12; i++ {
		if specs[i].Variant != Baseline || specs[i].Issue.ID != "issue-1" {
			t.Errorf("spec[%d] = %v/%s, want baseline/issue-1", i, specs[i].Variant, specs[i].Issue.ID)
		}
	}
}

func TestFlattenPlan_BaselineDisabled(t *testing.T) {
	cfg := makeConfig(3, 1, BoolPtr(false))
	issues := makeIssues(2)
	pairs := GeneratePlan(cfg, issues)

	specs := FlattenPlan(pairs)
	if len(specs) != 6 {
		t.Fatalf("got %d specs, want 6", len(specs))
	}
	for _, s := range specs {
		if s.Variant != Configured {
			t.Errorf("expected all Configured, got %v", s.Variant)
		}
	}
}

func TestFlattenPlan_Empty(t *testing.T) {
	specs := FlattenPlan([]RunPair{})
	if len(specs) != 0 {
		t.Errorf("got %d specs, want 0", len(specs))
	}
}

func TestFlattenPlan_Nil(t *testing.T) {
	specs := FlattenPlan(nil)
	if len(specs) != 0 {
		t.Errorf("got %d specs, want 0", len(specs))
	}
}

func TestFlattenPlan_SinglePairSingleAttempt(t *testing.T) {
	cfg := makeConfig(1, 1, BoolPtr(true))
	issues := makeIssues(1)
	pairs := GeneratePlan(cfg, issues)

	specs := FlattenPlan(pairs)
	if len(specs) != 2 {
		t.Fatalf("got %d specs, want 2", len(specs))
	}
	if specs[0].Variant != Configured {
		t.Errorf("first spec should be Configured, got %v", specs[0].Variant)
	}
	if specs[1].Variant != Baseline {
		t.Errorf("second spec should be Baseline, got %v", specs[1].Variant)
	}
}
