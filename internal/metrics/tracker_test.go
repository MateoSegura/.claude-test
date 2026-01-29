package metrics

import (
	"testing"
)

func TestClassifyTool(t *testing.T) {
	tests := []struct {
		name     string
		expected ToolCategory
	}{
		{"Read", BuiltIn},
		{"Write", BuiltIn},
		{"Bash", BuiltIn},
		{"mcp__context7__resolve", MCP},
		{"mcp__github__create_pr", MCP},
		{"Skill", Skill},
		{"Task", Agent},
		{"Glob", BuiltIn},
		{"Grep", BuiltIn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyTool(tt.name)
			if got != tt.expected {
				t.Errorf("ClassifyTool(%q) = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

func TestTrackerRecordAndSummary(t *testing.T) {
	tr := NewTracker()

	tr.Record("Bash", map[string]any{"command": "ls"})
	tr.Record("mcp__github__create_pr", map[string]any{"title": "fix"})
	tr.Record("Bash", map[string]any{"command": "go test"})

	calls := tr.Calls()
	if len(calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(calls))
	}

	// Verify order and categories
	if calls[0].Name != "Bash" || calls[0].Category != BuiltIn {
		t.Errorf("call[0]: got name=%q category=%q, want Bash/built_in", calls[0].Name, calls[0].Category)
	}
	if calls[1].Name != "mcp__github__create_pr" || calls[1].Category != MCP {
		t.Errorf("call[1]: got name=%q category=%q, want mcp__github__create_pr/mcp", calls[1].Name, calls[1].Category)
	}
	if calls[2].Name != "Bash" || calls[2].Category != BuiltIn {
		t.Errorf("call[2]: got name=%q category=%q, want Bash/built_in", calls[2].Name, calls[2].Category)
	}

	// Verify summary
	summary := tr.Summary()
	if summary["Bash"] != 2 {
		t.Errorf("summary[Bash] = %d, want 2", summary["Bash"])
	}
	if summary["mcp__github__create_pr"] != 1 {
		t.Errorf("summary[mcp__github__create_pr] = %d, want 1", summary["mcp__github__create_pr"])
	}
}

func TestTrackerSummaryReturnsCopy(t *testing.T) {
	tr := NewTracker()

	tr.Record("Bash", map[string]any{"command": "ls"})
	tr.Record("Read", map[string]any{"path": "/tmp/file"})
	tr.Record("mcp__context7__resolve", map[string]any{"query": "docs"})
	tr.Record("Skill", map[string]any{"skill": "deploy"})
	tr.Record("Task", map[string]any{"task": "research"})

	first := tr.Summary()
	if len(first) != 5 {
		t.Fatalf("expected 5 entries in first summary, got %d", len(first))
	}

	// Mutate the returned map
	delete(first, "Bash")

	// Second call should still have all entries
	second := tr.Summary()
	if len(second) != 5 {
		t.Errorf("expected 5 entries in second summary after mutation, got %d", len(second))
	}
	if second["Bash"] != 1 {
		t.Errorf("second summary[Bash] = %d, want 1", second["Bash"])
	}
}

func TestNewTrackerEmpty(t *testing.T) {
	tr := NewTracker()

	calls := tr.Calls()
	if len(calls) != 0 {
		t.Errorf("expected 0 calls, got %d", len(calls))
	}

	summary := tr.Summary()
	if len(summary) != 0 {
		t.Errorf("expected empty summary, got %d entries", len(summary))
	}
}

func TestToolCallTimestamp(t *testing.T) {
	tr := NewTracker()
	tr.Record("Bash", map[string]any{"command": "echo hello"})

	calls := tr.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}

	if calls[0].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp on recorded tool call")
	}
}
