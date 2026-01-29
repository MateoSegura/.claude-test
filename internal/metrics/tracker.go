package metrics

import (
	"strings"
	"time"
)

// ClassifyTool determines the ToolCategory for a given tool name.
func ClassifyTool(name string) ToolCategory {
	if strings.HasPrefix(name, "mcp__") {
		return MCP
	}
	if name == "Skill" {
		return Skill
	}
	if name == "Task" {
		return Agent
	}
	return BuiltIn
}

// Tracker records tool calls during a benchmark run.
type Tracker struct {
	calls []ToolCall
}

// NewTracker returns a fresh empty tracker.
func NewTracker() *Tracker {
	return &Tracker{}
}

// Record appends a ToolCall with the current timestamp and auto-classified category.
func (t *Tracker) Record(name string, input map[string]any) {
	t.calls = append(t.calls, ToolCall{
		Timestamp: time.Now(),
		Name:      name,
		Input:     input,
		Category:  ClassifyTool(name),
	})
}

// Calls returns all recorded tool calls in order.
func (t *Tracker) Calls() []ToolCall {
	return t.calls
}

// Summary returns a new map with tool-name-to-count.
// The returned map is a copy so mutations do not affect internal state.
func (t *Tracker) Summary() map[string]int {
	m := make(map[string]int)
	for _, c := range t.calls {
		m[c.Name]++
	}
	return m
}
