//go:build live

package configtest

import (
	"context"
	"strings"
	"testing"
	"time"

	claude "github.com/MateoSegura/claudesdk-go"
)

func skipIfNoCLI(t *testing.T) {
	t.Helper()
	if !claude.CLIAvailable() {
		t.Skip("claude CLI not available")
	}
}

func runPrompt(t *testing.T, prompt string) string {
	t.Helper()
	skipIfNoCLI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	session, err := claude.NewSession(claude.SessionConfig{
		LaunchOptions: claude.LaunchOptions{
			PermissionMode: claude.PermissionBypass,
			MaxTurns:       1,
		},
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	text, err := session.CollectAll(ctx, prompt)
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}

	return text
}

func TestLive_MetaSkillCreate_RecommendsHook(t *testing.T) {
	text := runPrompt(t,
		"I want to create a skill that automatically formats Go files after editing. "+
			"What Claude Code feature should I use? Answer in one paragraph.")

	lower := strings.ToLower(text)
	if !strings.Contains(lower, "hook") {
		t.Errorf("expected response to mention hooks, got:\n%s", text)
	}
}

func TestLive_MetaSkillCreate_RecommendsMCP(t *testing.T) {
	text := runPrompt(t,
		"I want to create a skill that queries a PostgreSQL database for schema information. "+
			"What Claude Code feature should I use for database access? Answer in one paragraph.")

	lower := strings.ToLower(text)
	if !strings.Contains(lower, "mcp") && !strings.Contains(lower, "model context protocol") {
		t.Errorf("expected response to mention MCP, got:\n%s", text)
	}
}

func TestLive_NewSkillCommand_ShowsStructure(t *testing.T) {
	text := runPrompt(t,
		"Show me the directory structure for a new Claude Code skill called 'terraform'. "+
			"Just show the file tree, nothing else.")

	lower := strings.ToLower(text)
	if !strings.Contains(lower, "skill.md") && !strings.Contains(lower, "skill") {
		t.Errorf("expected response to show SKILL.md in structure, got:\n%s", text)
	}
}

func TestLive_NewHookCommand_ShowsTemplate(t *testing.T) {
	text := runPrompt(t,
		"Show me a settings.json PostToolUse hook template for running gofmt after Write/Edit tools. "+
			"Just show the JSON, nothing else.")

	lower := strings.ToLower(text)
	if !strings.Contains(lower, "posttooluse") && !strings.Contains(lower, "post_tool_use") {
		t.Errorf("expected response to contain PostToolUse hook, got:\n%s", text)
	}
}
