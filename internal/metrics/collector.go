package metrics

import (
	"time"

	claude "github.com/MateoSegura/claudesdk-go"
)

// RunResult captures the full outcome of a single benchmark run.
type RunResult struct {
	IssueID         string
	Variant         string
	Attempt         int
	Success         bool
	Score           float64
	ErrorMsg        string
	CostUSD         float64
	DurationMS      int64
	NumTurns        int
	Usage           *claude.Usage
	InitTools       []string
	ToolCalls       []ToolCall
	ToolCallSummary map[string]int
	FinalText       string

	// Issue metadata — set by orchestrator, used by Phase 4 for breakdowns
	Difficulty string
	Language   string
	TaskType   string
}

// ToolCall represents a single tool invocation recorded during a run.
type ToolCall struct {
	Timestamp time.Time
	Name      string
	Input     map[string]any
	Category  ToolCategory
}

// ToolCategory classifies a tool by its origin.
type ToolCategory string

const (
	BuiltIn ToolCategory = "built_in"
	MCP     ToolCategory = "mcp"
	Skill   ToolCategory = "skill"
	Agent   ToolCategory = "agent"
)
