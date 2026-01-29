package analysis

import (
	"sort"

	"github.com/MateoSegura/.claude-test/internal/metrics"
)

// ConfigAnalysis captures the differences in tool availability and usage
// between configured and baseline runs.
type ConfigAnalysis struct {
	ConfigAddedTools   []string              `json:"config_added_tools"`
	ConfigRemovedTools []string              `json:"config_removed_tools"`
	ToolUsageDiff      map[string]UsageDelta `json:"tool_usage_diff"`
	SkillInvocations   int                   `json:"skill_invocations"`
	AgentInvocations   int                   `json:"agent_invocations"`
	MCPToolCalls       int                   `json:"mcp_tool_calls"`
	ConfigUtilization  float64               `json:"config_utilization"`
	BehaviorDivergence float64               `json:"behavior_divergence"`
}

// UsageDelta holds per-tool call counts for each variant.
type UsageDelta struct {
	Configured int `json:"configured"`
	Baseline   int `json:"baseline"`
}

// JaccardDistance computes 1 - |intersection|/|union| of two boolean sets.
// Returns 0.0 when both sets are empty.
func JaccardDistance(setA, setB map[string]bool) float64 {
	if len(setA) == 0 && len(setB) == 0 {
		return 0.0
	}

	union := make(map[string]bool)
	for k := range setA {
		union[k] = true
	}
	for k := range setB {
		union[k] = true
	}

	var intersection int
	for k := range setA {
		if setB[k] {
			intersection++
		}
	}

	return 1.0 - float64(intersection)/float64(len(union))
}

// AnalyzeConfig compares tool configuration and usage between configured
// and baseline runs.
func AnalyzeConfig(configured, baseline []*metrics.RunResult) ConfigAnalysis {
	var ca ConfigAnalysis

	// Union of InitTools per variant.
	cfgInitTools := unionInitTools(configured)
	baseInitTools := unionInitTools(baseline)

	// Added = in configured but not baseline.
	// Removed = in baseline but not configured.
	ca.ConfigAddedTools = sortedDiff(cfgInitTools, baseInitTools)
	ca.ConfigRemovedTools = sortedDiff(baseInitTools, cfgInitTools)

	// Aggregate ToolCallSummary per variant.
	cfgSummary := aggregateSummary(configured)
	baseSummary := aggregateSummary(baseline)

	// Build ToolUsageDiff from the union of all tool names.
	allTools := make(map[string]bool)
	for k := range cfgSummary {
		allTools[k] = true
	}
	for k := range baseSummary {
		allTools[k] = true
	}

	ca.ToolUsageDiff = make(map[string]UsageDelta)
	for tool := range allTools {
		ca.ToolUsageDiff[tool] = UsageDelta{
			Configured: cfgSummary[tool],
			Baseline:   baseSummary[tool],
		}
	}

	// Count Skill/Agent/MCP from configured ToolCalls by Category.
	for _, r := range configured {
		for _, tc := range r.ToolCalls {
			switch tc.Category {
			case metrics.Skill:
				ca.SkillInvocations++
			case metrics.Agent:
				ca.AgentInvocations++
			case metrics.MCP:
				ca.MCPToolCalls++
			}
		}
	}

	// ConfigUtilization = |added intersect actually_called_configured| / |added|.
	addedSet := make(map[string]bool)
	for _, t := range ca.ConfigAddedTools {
		addedSet[t] = true
	}

	cfgCalled := actuallyCalledTools(configured)

	if len(addedSet) > 0 {
		var usedCount int
		for t := range addedSet {
			if cfgCalled[t] {
				usedCount++
			}
		}
		ca.ConfigUtilization = float64(usedCount) / float64(len(addedSet))
	}

	// BehaviorDivergence = JaccardDistance of actually-called tool sets.
	baseCalled := actuallyCalledTools(baseline)
	ca.BehaviorDivergence = JaccardDistance(cfgCalled, baseCalled)

	return ca
}

// unionInitTools returns the union of InitTools across all results.
func unionInitTools(results []*metrics.RunResult) map[string]bool {
	s := make(map[string]bool)
	for _, r := range results {
		for _, t := range r.InitTools {
			s[t] = true
		}
	}
	return s
}

// aggregateSummary sums ToolCallSummary counts across all results.
func aggregateSummary(results []*metrics.RunResult) map[string]int {
	m := make(map[string]int)
	for _, r := range results {
		for tool, count := range r.ToolCallSummary {
			m[tool] += count
		}
	}
	return m
}

// actuallyCalledTools returns the set of tool names that appear in ToolCalls.
func actuallyCalledTools(results []*metrics.RunResult) map[string]bool {
	s := make(map[string]bool)
	for _, r := range results {
		for _, tc := range r.ToolCalls {
			s[tc.Name] = true
		}
	}
	return s
}

// sortedDiff returns sorted elements in a but not in b.
func sortedDiff(a, b map[string]bool) []string {
	var diff []string
	for k := range a {
		if !b[k] {
			diff = append(diff, k)
		}
	}
	sort.Strings(diff)
	return diff
}
