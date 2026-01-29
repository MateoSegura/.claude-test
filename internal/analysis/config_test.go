package analysis

import (
	"testing"

	claude "github.com/MateoSegura/claudesdk-go"
	"github.com/MateoSegura/.claude-test/internal/metrics"
)

func TestJaccardDistance(t *testing.T) {
	tests := []struct {
		name string
		a, b map[string]bool
		want float64
	}{
		{
			"partial overlap",
			map[string]bool{"A": true, "B": true, "C": true},
			map[string]bool{"B": true, "C": true, "D": true},
			0.5,
		},
		{
			"identical",
			map[string]bool{"A": true, "B": true},
			map[string]bool{"A": true, "B": true},
			0.0,
		},
		{
			"disjoint",
			map[string]bool{"A": true, "B": true},
			map[string]bool{"C": true, "D": true},
			1.0,
		},
		{
			"both empty",
			map[string]bool{},
			map[string]bool{},
			0.0,
		},
		{
			"one empty",
			map[string]bool{"A": true},
			map[string]bool{},
			1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JaccardDistance(tt.a, tt.b)
			if !approxEqual(got, tt.want) {
				t.Errorf("JaccardDistance(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestAnalyzeConfig(t *testing.T) {
	cfgInitTools := []string{"Read", "Write", "Edit", "Bash", "mcp__ctx7__resolve", "Skill"}
	baseInitTools := []string{"Read", "Write", "Edit", "Bash"}

	configured := []*metrics.RunResult{
		{
			IssueID:   "issue-1",
			Variant:   "configured",
			InitTools: cfgInitTools,
			ToolCalls: []metrics.ToolCall{
				{Name: "Read", Category: metrics.BuiltIn},
				{Name: "Write", Category: metrics.BuiltIn},
				{Name: "Edit", Category: metrics.BuiltIn},
				{Name: "Bash", Category: metrics.BuiltIn},
				{Name: "mcp__ctx7__resolve", Category: metrics.MCP},
				{Name: "mcp__ctx7__resolve", Category: metrics.MCP},
				{Name: "Skill", Category: metrics.Skill},
				{Name: "Task", Category: metrics.Agent},
			},
			ToolCallSummary: map[string]int{
				"Read":               1,
				"Write":              1,
				"Edit":               1,
				"Bash":               1,
				"mcp__ctx7__resolve": 2,
				"Skill":              1,
				"Task":               1,
			},
			Usage: &claude.Usage{InputTokens: 500, OutputTokens: 200},
		},
		{
			IssueID:   "issue-1",
			Variant:   "configured",
			InitTools: cfgInitTools,
			ToolCalls: []metrics.ToolCall{
				{Name: "Read", Category: metrics.BuiltIn},
				{Name: "Bash", Category: metrics.BuiltIn},
				{Name: "mcp__ctx7__resolve", Category: metrics.MCP},
				{Name: "Skill", Category: metrics.Skill},
			},
			ToolCallSummary: map[string]int{
				"Read":               1,
				"Bash":               1,
				"mcp__ctx7__resolve": 1,
				"Skill":              1,
			},
			Usage: &claude.Usage{InputTokens: 400, OutputTokens: 150},
		},
	}

	baseline := []*metrics.RunResult{
		{
			IssueID:   "issue-1",
			Variant:   "baseline",
			InitTools: baseInitTools,
			ToolCalls: []metrics.ToolCall{
				{Name: "Read", Category: metrics.BuiltIn},
				{Name: "Write", Category: metrics.BuiltIn},
				{Name: "Bash", Category: metrics.BuiltIn},
				{Name: "Bash", Category: metrics.BuiltIn},
			},
			ToolCallSummary: map[string]int{
				"Read":  1,
				"Write": 1,
				"Bash":  2,
			},
			Usage: &claude.Usage{InputTokens: 300, OutputTokens: 100},
		},
	}

	ca := AnalyzeConfig(configured, baseline)

	// ConfigAddedTools: tools in configured but not baseline.
	wantAdded := []string{"Skill", "mcp__ctx7__resolve"}
	if len(ca.ConfigAddedTools) != len(wantAdded) {
		t.Fatalf("ConfigAddedTools = %v, want %v", ca.ConfigAddedTools, wantAdded)
	}
	for i, tool := range wantAdded {
		if ca.ConfigAddedTools[i] != tool {
			t.Errorf("ConfigAddedTools[%d] = %q, want %q", i, ca.ConfigAddedTools[i], tool)
		}
	}

	// ConfigRemovedTools: tools in baseline but not configured.
	if len(ca.ConfigRemovedTools) != 0 {
		t.Errorf("ConfigRemovedTools = %v, want empty", ca.ConfigRemovedTools)
	}

	// MCP tool calls across configured runs.
	if ca.MCPToolCalls != 3 {
		t.Errorf("MCPToolCalls = %d, want 3", ca.MCPToolCalls)
	}

	// Skill invocations.
	if ca.SkillInvocations != 2 {
		t.Errorf("SkillInvocations = %d, want 2", ca.SkillInvocations)
	}

	// Agent invocations.
	if ca.AgentInvocations != 1 {
		t.Errorf("AgentInvocations = %d, want 1", ca.AgentInvocations)
	}

	// ConfigUtilization: both added tools (mcp__ctx7__resolve, Skill) were called.
	if !approxEqual(ca.ConfigUtilization, 1.0) {
		t.Errorf("ConfigUtilization = %v, want 1.0", ca.ConfigUtilization)
	}

	// BehaviorDivergence: configured called {Read, Write, Edit, Bash, mcp__ctx7__resolve, Skill, Task}
	// baseline called {Read, Write, Bash}. Union=7, intersection=3.
	// Distance = 1 - 3/7 = 4/7 ~ 0.571
	wantDivergence := 4.0 / 7.0
	if !approxEqual(ca.BehaviorDivergence, wantDivergence) {
		t.Errorf("BehaviorDivergence = %v, want %v", ca.BehaviorDivergence, wantDivergence)
	}
}
