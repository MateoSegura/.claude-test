package metrics

import (
	"testing"

	claude "github.com/MateoSegura/claudesdk-go"
)

func Test_StreamParser_InitMessage(t *testing.T) {
	p := NewStreamParser()

	tools := []string{"Read", "Write", "Bash", "Glob", "Grep", "mcp__github__create_pr", "mcp__context7__resolve"}
	msg := &claude.StreamMessage{
		Type:    "system",
		Subtype: "init",
		Tools:   tools,
	}
	p.ProcessMessage(msg)

	result := p.Result()
	if len(result.InitTools) != 7 {
		t.Fatalf("expected 7 init tools, got %d", len(result.InitTools))
	}
	for i, want := range tools {
		if result.InitTools[i] != want {
			t.Errorf("InitTools[%d] = %q, want %q", i, result.InitTools[i], want)
		}
	}
}

func Test_StreamParser_MultipleAssistantMessages(t *testing.T) {
	p := NewStreamParser()

	// Init message
	initMsg := &claude.StreamMessage{
		Type:    "system",
		Subtype: "init",
		Tools:   []string{"Bash", "Read", "Grep", "mcp__github__create_pr"},
	}
	p.ProcessMessage(initMsg)

	// Assistant message 1: single Bash tool_use
	msg1 := &claude.StreamMessage{
		Type: "assistant",
		Message: &claude.MessageContent{
			Role: "assistant",
			Content: []claude.ContentBlock{
				{Type: "tool_use", Name: "Bash", ID: "tool1", Input: map[string]any{"command": "go test"}},
			},
		},
	}
	p.ProcessMessage(msg1)

	// Assistant message 2: two tool_use blocks
	msg2 := &claude.StreamMessage{
		Type: "assistant",
		Message: &claude.MessageContent{
			Role: "assistant",
			Content: []claude.ContentBlock{
				{Type: "tool_use", Name: "Read", ID: "tool2", Input: map[string]any{"file_path": "/tmp/file"}},
				{Type: "tool_use", Name: "Grep", ID: "tool3", Input: map[string]any{"pattern": "TODO"}},
			},
		},
	}
	p.ProcessMessage(msg2)

	// Assistant message 3: MCP tool
	msg3 := &claude.StreamMessage{
		Type: "assistant",
		Message: &claude.MessageContent{
			Role: "assistant",
			Content: []claude.ContentBlock{
				{Type: "tool_use", Name: "mcp__github__create_pr", ID: "tool4", Input: map[string]any{"title": "fix"}},
			},
		},
	}
	p.ProcessMessage(msg3)

	result := p.Result()

	// Verify total tool calls
	if len(result.ToolCalls) != 4 {
		t.Fatalf("expected 4 tool calls, got %d", len(result.ToolCalls))
	}

	// Verify summary counts
	expectedSummary := map[string]int{
		"Bash":                   1,
		"Read":                   1,
		"Grep":                   1,
		"mcp__github__create_pr": 1,
	}
	for name, want := range expectedSummary {
		if got := result.Summary[name]; got != want {
			t.Errorf("Summary[%q] = %d, want %d", name, got, want)
		}
	}

	// Verify categories
	categoryChecks := map[string]ToolCategory{
		"Bash":                   BuiltIn,
		"Read":                   BuiltIn,
		"Grep":                   BuiltIn,
		"mcp__github__create_pr": MCP,
	}
	for _, tc := range result.ToolCalls {
		want, ok := categoryChecks[tc.Name]
		if !ok {
			t.Errorf("unexpected tool call: %q", tc.Name)
			continue
		}
		if tc.Category != want {
			t.Errorf("tool %q category = %q, want %q", tc.Name, tc.Category, want)
		}
	}
}

func Test_StreamParser_ResultMessage(t *testing.T) {
	p := NewStreamParser()

	msg := &claude.StreamMessage{
		Type:       "result",
		CostUSD:    0.087,
		DurationMS: 34200,
		NumTurns:   12,
		Usage: &claude.Usage{
			InputTokens:              15000,
			OutputTokens:             3200,
			CacheCreationInputTokens: 500,
			CacheReadInputTokens:     8000,
		},
		Result: "The fix has been applied",
	}
	p.ProcessMessage(msg)

	result := p.Result()

	if result.CostUSD != 0.087 {
		t.Errorf("CostUSD = %f, want 0.087", result.CostUSD)
	}
	if result.DurationMS != 34200 {
		t.Errorf("DurationMS = %d, want 34200", result.DurationMS)
	}
	if result.NumTurns != 12 {
		t.Errorf("NumTurns = %d, want 12", result.NumTurns)
	}
	if result.FinalText != "The fix has been applied" {
		t.Errorf("FinalText = %q, want %q", result.FinalText, "The fix has been applied")
	}
	if result.Usage == nil {
		t.Fatal("Usage is nil, expected non-nil")
	}
	if result.Usage.InputTokens != 15000 {
		t.Errorf("Usage.InputTokens = %d, want 15000", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 3200 {
		t.Errorf("Usage.OutputTokens = %d, want 3200", result.Usage.OutputTokens)
	}
	if result.Usage.CacheCreationInputTokens != 500 {
		t.Errorf("Usage.CacheCreationInputTokens = %d, want 500", result.Usage.CacheCreationInputTokens)
	}
	if result.Usage.CacheReadInputTokens != 8000 {
		t.Errorf("Usage.CacheReadInputTokens = %d, want 8000", result.Usage.CacheReadInputTokens)
	}
}

func Test_StreamParser_EmptyStream(t *testing.T) {
	p := NewStreamParser()

	result := p.Result()

	if result.InitTools != nil && len(result.InitTools) != 0 {
		t.Errorf("expected InitTools to be nil or empty, got %v", result.InitTools)
	}
	if len(result.ToolCalls) != 0 {
		t.Errorf("expected 0 ToolCalls, got %d", len(result.ToolCalls))
	}
	if len(result.Summary) != 0 {
		t.Errorf("expected empty Summary, got %d entries", len(result.Summary))
	}
	if result.CostUSD != 0 {
		t.Errorf("expected CostUSD 0, got %f", result.CostUSD)
	}
	if result.DurationMS != 0 {
		t.Errorf("expected DurationMS 0, got %d", result.DurationMS)
	}
	if result.NumTurns != 0 {
		t.Errorf("expected NumTurns 0, got %d", result.NumTurns)
	}
	if result.Usage != nil {
		t.Errorf("expected Usage nil, got %v", result.Usage)
	}
	if result.FinalText != "" {
		t.Errorf("expected FinalText empty, got %q", result.FinalText)
	}
}

func Test_StreamParser_IgnoresUnknownTypes(t *testing.T) {
	p := NewStreamParser()

	// Init
	initMsg := &claude.StreamMessage{
		Type:    "system",
		Subtype: "init",
		Tools:   []string{"Bash", "Read"},
	}
	p.ProcessMessage(initMsg)

	// Unknown type "ping" -- should be silently ignored
	pingMsg := &claude.StreamMessage{
		Type: "ping",
	}
	p.ProcessMessage(pingMsg)

	// Assistant with Bash tool_use
	assistMsg := &claude.StreamMessage{
		Type: "assistant",
		Message: &claude.MessageContent{
			Role: "assistant",
			Content: []claude.ContentBlock{
				{Type: "tool_use", Name: "Bash", ID: "tool1", Input: map[string]any{"command": "ls"}},
			},
		},
	}
	p.ProcessMessage(assistMsg)

	result := p.Result()

	if len(result.InitTools) != 2 {
		t.Errorf("expected 2 init tools, got %d", len(result.InitTools))
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Name != "Bash" {
		t.Errorf("expected tool call name Bash, got %q", result.ToolCalls[0].Name)
	}
}

func Test_StreamParser_FullConversation(t *testing.T) {
	p := NewStreamParser()

	// 1. Init with 10 tools
	initMsg := &claude.StreamMessage{
		Type:    "system",
		Subtype: "init",
		Tools:   []string{"Read", "Write", "Edit", "Bash", "Glob", "Grep", "mcp__github__create_pr", "mcp__context7__resolve", "Skill", "Task"},
	}
	p.ProcessMessage(initMsg)

	// 2. Assistant with Bash tool_use
	msg1 := &claude.StreamMessage{
		Type: "assistant",
		Message: &claude.MessageContent{
			Role: "assistant",
			Content: []claude.ContentBlock{
				{Type: "tool_use", Name: "Bash", ID: "tool1", Input: map[string]any{"command": "go test ./..."}},
			},
		},
	}
	p.ProcessMessage(msg1)

	// 3. User with tool_result (should be ignored -- not tracked)
	userMsg := &claude.StreamMessage{
		Type: "user",
		Message: &claude.MessageContent{
			Role: "user",
			Content: []claude.ContentBlock{
				{Type: "tool_result", ToolUseID: "tool1", Content: "PASS"},
			},
		},
	}
	p.ProcessMessage(userMsg)

	// 4. Assistant with Read and Edit tool_use blocks
	msg2 := &claude.StreamMessage{
		Type: "assistant",
		Message: &claude.MessageContent{
			Role: "assistant",
			Content: []claude.ContentBlock{
				{Type: "tool_use", Name: "Read", ID: "tool2", Input: map[string]any{"file_path": "/tmp/main.go"}},
				{Type: "tool_use", Name: "Edit", ID: "tool3", Input: map[string]any{"file_path": "/tmp/main.go", "old_string": "foo", "new_string": "bar"}},
			},
		},
	}
	p.ProcessMessage(msg2)

	// 5. Result
	resultMsg := &claude.StreamMessage{
		Type:       "result",
		CostUSD:    0.045,
		DurationMS: 18000,
		NumTurns:   4,
		Usage: &claude.Usage{
			InputTokens:              10000,
			OutputTokens:             2000,
			CacheCreationInputTokens: 300,
			CacheReadInputTokens:     5000,
		},
		Result: "All tests pass.",
	}
	p.ProcessMessage(resultMsg)

	result := p.Result()

	// Verify init tools count
	if len(result.InitTools) != 10 {
		t.Errorf("expected 10 init tools, got %d", len(result.InitTools))
	}

	// Verify tool calls: Bash, Read, Edit (3 total, user message not tracked)
	if len(result.ToolCalls) != 3 {
		t.Fatalf("expected 3 tool calls, got %d", len(result.ToolCalls))
	}

	expectedNames := []string{"Bash", "Read", "Edit"}
	for i, want := range expectedNames {
		if result.ToolCalls[i].Name != want {
			t.Errorf("ToolCalls[%d].Name = %q, want %q", i, result.ToolCalls[i].Name, want)
		}
	}

	// Verify result metrics
	if result.CostUSD != 0.045 {
		t.Errorf("CostUSD = %f, want 0.045", result.CostUSD)
	}
	if result.NumTurns != 4 {
		t.Errorf("NumTurns = %d, want 4", result.NumTurns)
	}
	if result.DurationMS != 18000 {
		t.Errorf("DurationMS = %d, want 18000", result.DurationMS)
	}
}

func Test_StreamParser_RepeatedInit(t *testing.T) {
	p := NewStreamParser()

	// First init
	msg1 := &claude.StreamMessage{
		Type:    "system",
		Subtype: "init",
		Tools:   []string{"Read", "Write"},
	}
	p.ProcessMessage(msg1)

	// Second init overwrites
	msg2 := &claude.StreamMessage{
		Type:    "system",
		Subtype: "init",
		Tools:   []string{"Read", "Write", "Bash"},
	}
	p.ProcessMessage(msg2)

	result := p.Result()

	expected := []string{"Read", "Write", "Bash"}
	if len(result.InitTools) != 3 {
		t.Fatalf("expected 3 init tools, got %d", len(result.InitTools))
	}
	for i, want := range expected {
		if result.InitTools[i] != want {
			t.Errorf("InitTools[%d] = %q, want %q", i, result.InitTools[i], want)
		}
	}
}
