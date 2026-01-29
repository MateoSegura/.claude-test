package docker

import (
	"context"
	"testing"
	"time"

	claude "github.com/MateoSegura/claudesdk-go"
	"github.com/MateoSegura/.claude-test/internal/corpus"
	"github.com/MateoSegura/.claude-test/internal/metrics"
)

func Test_Runner_FullPipeline_BugFix(t *testing.T) {
	mock := NewMockExecutor()

	stream := buildStream(
		claude.StreamMessage{
			Type: "system", Subtype: "init",
			Tools: []string{"Read", "Write", "Edit", "Bash", "Glob", "Grep"},
		},
		claude.StreamMessage{
			Type: "assistant",
			Message: &claude.MessageContent{
				Role: "assistant",
				Content: []claude.ContentBlock{
					{Type: "tool_use", Name: "Read", ID: "t1", Input: map[string]any{"file_path": "/workspace/main.go"}},
				},
			},
		},
		claude.StreamMessage{
			Type: "assistant",
			Message: &claude.MessageContent{
				Role: "assistant",
				Content: []claude.ContentBlock{
					{Type: "tool_use", Name: "Edit", ID: "t2", Input: map[string]any{"file_path": "/workspace/main.go", "old_string": "nil", "new_string": "value"}},
				},
			},
		},
		claude.StreamMessage{
			Type: "assistant",
			Message: &claude.MessageContent{
				Role: "assistant",
				Content: []claude.ContentBlock{
					{Type: "tool_use", Name: "Bash", ID: "t3", Input: map[string]any{"command": "go test ./..."}},
				},
			},
		},
		claude.StreamMessage{
			Type: "assistant",
			Message: &claude.MessageContent{
				Role: "assistant",
				Content: []claude.ContentBlock{
					{Type: "text", Text: "Fixed it"},
				},
			},
		},
		claude.StreamMessage{
			Type: "result", Result: "Fixed it",
			CostUSD: 0.023, DurationMS: 15000, NumTurns: 4,
			Usage: &claude.Usage{InputTokens: 8000, OutputTokens: 2000},
		},
	)
	mock.OnCommand("claude", MockResponse{Stdout: stream})
	mock.OnCommand("sh", MockResponse{ExitCode: 0})

	runner := NewRunner(mock)
	result, err := runner.Run(context.Background(), RunConfig{
		ContainerID: "bench-container",
		Prompt:      "Fix the nil pointer dereference in main.go",
		MaxTurns:    10,
		Timeout:     60 * time.Second,
		SetupCmds:   []string{"cd /workspace && go mod download"},
		Evaluation: corpus.Evaluation{
			Method:           "command",
			Command:          "go vet ./...",
			ExpectedExitCode: 0,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected Success=true")
	}
	if result.Score != 1.0 {
		t.Errorf("Score = %f, want 1.0", result.Score)
	}
	if result.CostUSD != 0.023 {
		t.Errorf("CostUSD = %f, want 0.023", result.CostUSD)
	}
	if result.NumTurns != 4 {
		t.Errorf("NumTurns = %d, want 4", result.NumTurns)
	}
	if result.DurationMS != 15000 {
		t.Errorf("DurationMS = %d, want 15000", result.DurationMS)
	}
	if len(result.ToolCalls) != 3 {
		t.Fatalf("ToolCalls count = %d, want 3", len(result.ToolCalls))
	}

	expectedTools := []string{"Read", "Edit", "Bash"}
	for i, want := range expectedTools {
		if result.ToolCalls[i].Name != want {
			t.Errorf("ToolCalls[%d].Name = %q, want %q", i, result.ToolCalls[i].Name, want)
		}
		if result.ToolCalls[i].Category != metrics.BuiltIn {
			t.Errorf("ToolCalls[%d].Category = %q, want %q", i, result.ToolCalls[i].Category, metrics.BuiltIn)
		}
	}

	if len(result.InitTools) != 6 {
		t.Errorf("InitTools count = %d, want 6", len(result.InitTools))
	}
}

func Test_Runner_FullPipeline_WithMCPTools(t *testing.T) {
	mock := NewMockExecutor()

	stream := buildStream(
		claude.StreamMessage{
			Type: "system", Subtype: "init",
			Tools: []string{"Read", "Write", "mcp__github__get_issue", "mcp__github__create_pr"},
		},
		claude.StreamMessage{
			Type: "assistant",
			Message: &claude.MessageContent{
				Role: "assistant",
				Content: []claude.ContentBlock{
					{Type: "tool_use", Name: "mcp__github__get_issue", ID: "t1", Input: map[string]any{"issue": "42"}},
				},
			},
		},
		claude.StreamMessage{
			Type: "assistant",
			Message: &claude.MessageContent{
				Role: "assistant",
				Content: []claude.ContentBlock{
					{Type: "tool_use", Name: "mcp__github__create_pr", ID: "t2", Input: map[string]any{"title": "Fix bug"}},
				},
			},
		},
		claude.StreamMessage{
			Type: "result", Result: "PR created",
			CostUSD: 0.05, DurationMS: 20000, NumTurns: 3,
			Usage: &claude.Usage{InputTokens: 10000, OutputTokens: 3000},
		},
	)
	mock.OnCommand("claude", MockResponse{Stdout: stream})
	mock.OnCommand("sh", MockResponse{ExitCode: 0})

	runner := NewRunner(mock)
	result, err := runner.Run(context.Background(), RunConfig{
		ContainerID: "c1",
		Prompt:      "Create a PR",
		MaxTurns:    10,
		Evaluation:  corpus.Evaluation{Method: "command", Command: "true"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.ToolCalls) != 2 {
		t.Fatalf("ToolCalls count = %d, want 2", len(result.ToolCalls))
	}

	for _, tc := range result.ToolCalls {
		if tc.Category != metrics.MCP {
			t.Errorf("ToolCall %q category = %q, want %q", tc.Name, tc.Category, metrics.MCP)
		}
	}
}

func Test_Runner_FullPipeline_TextOnlyResponse(t *testing.T) {
	mock := NewMockExecutor()

	stream := buildStream(
		claude.StreamMessage{
			Type: "system", Subtype: "init",
			Tools: []string{"Read", "Write"},
		},
		claude.StreamMessage{
			Type: "assistant",
			Message: &claude.MessageContent{
				Role: "assistant",
				Content: []claude.ContentBlock{
					{Type: "text", Text: "The answer is 42."},
				},
			},
		},
		claude.StreamMessage{
			Type: "result", Result: "The answer is 42.",
			CostUSD: 0.005, DurationMS: 2000, NumTurns: 1,
			Usage: &claude.Usage{InputTokens: 500, OutputTokens: 50},
		},
	)
	mock.OnCommand("claude", MockResponse{Stdout: stream})
	mock.OnCommand("sh", MockResponse{ExitCode: 0})

	runner := NewRunner(mock)
	result, err := runner.Run(context.Background(), RunConfig{
		ContainerID: "c1",
		Prompt:      "What is the meaning of life?",
		MaxTurns:    5,
		Evaluation:  corpus.Evaluation{Method: "command", Command: "true"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.ToolCalls) != 0 {
		t.Errorf("ToolCalls count = %d, want 0", len(result.ToolCalls))
	}
	if result.FinalText != "The answer is 42." {
		t.Errorf("FinalText = %q, want %q", result.FinalText, "The answer is 42.")
	}
}

func Test_Runner_FullPipeline_EvaluationFails(t *testing.T) {
	mock := NewMockExecutor()

	stream := buildStream(
		claude.StreamMessage{
			Type: "system", Subtype: "init",
			Tools: []string{"Read", "Bash"},
		},
		claude.StreamMessage{
			Type: "assistant",
			Message: &claude.MessageContent{
				Role: "assistant",
				Content: []claude.ContentBlock{
					{Type: "tool_use", Name: "Bash", ID: "t1", Input: map[string]any{"command": "go test"}},
				},
			},
		},
		claude.StreamMessage{
			Type: "result", Result: "Attempted fix",
			CostUSD: 0.018, DurationMS: 12000, NumTurns: 2,
			Usage: &claude.Usage{InputTokens: 6000, OutputTokens: 1500},
		},
	)
	mock.OnCommand("claude", MockResponse{Stdout: stream})
	// Both setup and eval commands use "sh". For this test, eval should fail.
	// Since there are no setup commands, all "sh" calls are eval calls.
	mock.OnCommand("sh", MockResponse{ExitCode: 1})

	runner := NewRunner(mock)
	result, err := runner.Run(context.Background(), RunConfig{
		ContainerID: "c1",
		Prompt:      "fix it",
		MaxTurns:    10,
		Evaluation: corpus.Evaluation{
			Method:           "command",
			Command:          "go vet ./...",
			ExpectedExitCode: 0,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Success {
		t.Error("expected Success=false")
	}
	if result.Score != 0.0 {
		t.Errorf("Score = %f, want 0.0", result.Score)
	}

	// Metrics should still be populated.
	if result.CostUSD != 0.018 {
		t.Errorf("CostUSD = %f, want 0.018", result.CostUSD)
	}
	if result.NumTurns != 2 {
		t.Errorf("NumTurns = %d, want 2", result.NumTurns)
	}
	if result.DurationMS != 12000 {
		t.Errorf("DurationMS = %d, want 12000", result.DurationMS)
	}
	if len(result.ToolCalls) != 1 {
		t.Errorf("ToolCalls count = %d, want 1", len(result.ToolCalls))
	}
}

func Test_Runner_ClaudeCrash(t *testing.T) {
	mock := NewMockExecutor()

	// Claude crashes: exit code 1 with no stdout.
	mock.OnCommand("claude", MockResponse{Stdout: "", ExitCode: 1})
	mock.OnCommand("sh", MockResponse{ExitCode: 0})

	runner := NewRunner(mock)
	_, err := runner.Run(context.Background(), RunConfig{
		ContainerID: "c1",
		Prompt:      "fix it",
		MaxTurns:    5,
		Evaluation:  corpus.Evaluation{Method: "command", Command: "true"},
	})
	if err == nil {
		t.Fatal("expected error from claude crash")
	}
	if !contains(err.Error(), "crashed") && !contains(err.Error(), "exit code") {
		t.Errorf("error = %q, want it to mention crash or exit code", err.Error())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
