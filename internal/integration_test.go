package internal_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MateoSegura/.claude-test/internal/corpus"
	"github.com/MateoSegura/.claude-test/internal/docker"
	"github.com/MateoSegura/.claude-test/internal/metrics"
)

const testCorpusYAML = `name: integration-test-corpus
version: "1.0"
issues:
  - id: go-bug-001
    title: "Fix nil pointer in handler"
    description: "The HTTP handler panics on nil body"
    repo: "github.com/example/goapp"
    ref: "main"
    difficulty: easy
    language: go
    task_type: bug_fix
    prompt: "Fix the nil pointer dereference in the request handler"
    evaluation:
      method: test_passes
      command: "go test ./..."
  - id: ts-feature-001
    title: "Add dark mode toggle"
    description: "Implement a dark mode toggle in the settings page"
    repo: "github.com/example/webapp"
    ref: "main"
    difficulty: medium
    language: typescript
    task_type: feature
    prompt: "Add a dark mode toggle component to the settings page"
    evaluation:
      method: file_contains
      file_path: "src/components/Settings.tsx"
      contains: "darkMode"
  - id: py-refactor-001
    title: "Extract database layer"
    description: "Move all SQL queries into a dedicated repository module"
    repo: "github.com/example/pyapi"
    ref: "main"
    difficulty: hard
    language: python
    task_type: refactor
    prompt: "Refactor the codebase to extract all database queries into a repository pattern"
    evaluation:
      method: command
      command: "python -m pytest tests/"
      expected_exit_code: 0
`

func TestIntegrationCorpusFilterAndTrack(t *testing.T) {
	// Write the fixture to a temp file
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "corpus.yaml")
	if err := os.WriteFile(yamlPath, []byte(testCorpusYAML), 0644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	// Load the corpus
	c, err := corpus.Load(yamlPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if len(c.Issues) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(c.Issues))
	}

	// Validate
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}

	// Filter to Go issues only
	filtered := c.Filter(corpus.CorpusFilter{Language: []string{"go"}})
	if len(filtered.Issues) != 1 {
		t.Fatalf("expected 1 Go issue, got %d", len(filtered.Issues))
	}
	if filtered.Issues[0].ID != "go-bug-001" {
		t.Errorf("expected issue ID 'go-bug-001', got %q", filtered.Issues[0].ID)
	}

	// Original unchanged
	if len(c.Issues) != 3 {
		t.Errorf("original corpus modified: expected 3 issues, got %d", len(c.Issues))
	}

	// Create a tracker and record tool calls for the filtered issue
	tracker := metrics.NewTracker()
	tracker.Record("Read", map[string]any{"file_path": "handler.go"})
	tracker.Record("Edit", map[string]any{"file_path": "handler.go", "old_string": "body.Read()", "new_string": "if body != nil { body.Read() }"})
	tracker.Record("Bash", map[string]any{"command": "go test ./..."})

	calls := tracker.Calls()
	if len(calls) != 3 {
		t.Fatalf("expected 3 tool calls, got %d", len(calls))
	}

	summary := tracker.Summary()
	if len(summary) != 3 {
		t.Errorf("expected 3 keys in summary, got %d", len(summary))
	}
	if summary["Read"] != 1 {
		t.Errorf("summary[Read] = %d, want 1", summary["Read"])
	}
	if summary["Edit"] != 1 {
		t.Errorf("summary[Edit] = %d, want 1", summary["Edit"])
	}
	if summary["Bash"] != 1 {
		t.Errorf("summary[Bash] = %d, want 1", summary["Bash"])
	}

	// Build a RunResult with real tracker data
	issue := filtered.Issues[0]
	result := metrics.RunResult{
		IssueID:         issue.ID,
		Variant:         "configured",
		Attempt:         1,
		Success:         true,
		Score:           1.0,
		CostUSD:         0.05,
		DurationMS:      12500,
		NumTurns:        3,
		InitTools:       []string{"Read", "Edit", "Bash", "Write", "Glob", "Grep"},
		ToolCalls:       calls,
		ToolCallSummary: summary,
		FinalText:       "Fixed the nil pointer dereference by adding a nil check.",
		Difficulty:      issue.Difficulty,
		Language:        issue.Language,
		TaskType:        issue.TaskType,
	}

	// Verify RunResult fields are populated
	if result.IssueID != "go-bug-001" {
		t.Errorf("RunResult.IssueID = %q, want 'go-bug-001'", result.IssueID)
	}
	if len(result.ToolCalls) != 3 {
		t.Errorf("RunResult.ToolCalls length = %d, want 3", len(result.ToolCalls))
	}
	if len(result.ToolCallSummary) != 3 {
		t.Errorf("RunResult.ToolCallSummary length = %d, want 3", len(result.ToolCallSummary))
	}
	if len(result.InitTools) != 6 {
		t.Errorf("RunResult.InitTools length = %d, want 6", len(result.InitTools))
	}
	if result.Difficulty != "easy" {
		t.Errorf("RunResult.Difficulty = %q, want 'easy'", result.Difficulty)
	}
	if result.Language != "go" {
		t.Errorf("RunResult.Language = %q, want 'go'", result.Language)
	}
	if result.TaskType != "bug_fix" {
		t.Errorf("RunResult.TaskType = %q, want 'bug_fix'", result.TaskType)
	}
}

func TestIntegrationAllPackagesImport(t *testing.T) {
	// This test proves all three packages can be imported and used together
	// without import cycle issues.

	// Corpus
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "corpus.yaml")
	if err := os.WriteFile(yamlPath, []byte(testCorpusYAML), 0644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}
	c, err := corpus.Load(yamlPath)
	if err != nil {
		t.Fatalf("corpus.Load() failed: %v", err)
	}
	if len(c.Issues) == 0 {
		t.Fatal("corpus loaded with 0 issues")
	}

	// Metrics
	tracker := metrics.NewTracker()
	tracker.Record("Read", nil)
	if len(tracker.Calls()) != 1 {
		t.Error("tracker should have 1 call")
	}

	// Docker
	mgr := docker.NewManager("test-image")
	if mgr.Image != "test-image" {
		t.Errorf("manager.Image = %q, want 'test-image'", mgr.Image)
	}

	// All three packages imported and used successfully
}
