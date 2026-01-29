package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	claude "github.com/MateoSegura/claudesdk-go"
	"github.com/MateoSegura/.claude-test/internal/corpus"
)

// buildStream constructs a newline-delimited JSON stream from StreamMessages.
func buildStream(msgs ...claude.StreamMessage) string {
	var lines []string
	for _, msg := range msgs {
		data, _ := json.Marshal(msg)
		lines = append(lines, string(data))
	}
	return strings.Join(lines, "\n") + "\n"
}

func Test_Runner_SetupCommandsExecuteInOrder(t *testing.T) {
	mock := NewMockExecutor()

	stream := buildStream(
		claude.StreamMessage{Type: "system", Subtype: "init", Tools: []string{"Read"}},
		claude.StreamMessage{Type: "result", Result: "done", CostUSD: 0.01, DurationMS: 1000, NumTurns: 1},
	)
	mock.OnCommand("claude", MockResponse{Stdout: stream})
	// "sh" defaults to exit 0 (setup and eval succeed).
	mock.OnCommand("sh", MockResponse{ExitCode: 0})

	runner := NewRunner(mock)
	cfg := RunConfig{
		ContainerID: "c1",
		Prompt:      "fix it",
		MaxTurns:    5,
		Timeout:     30 * time.Second,
		SetupCmds:   []string{"cd /workspace && go mod download", "go generate ./..."},
		Evaluation:  corpus.Evaluation{Method: "command", Command: "go vet ./..."},
	}

	_, err := runner.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cmds := mock.Commands()
	if len(cmds) < 3 {
		t.Fatalf("expected at least 3 commands (2 setup + claude), got %d", len(cmds))
	}

	// First two must be setup commands (sh -c ...).
	for i := 0; i < 2; i++ {
		if cmds[i][0] != "sh" {
			t.Errorf("command[%d][0] = %q, want \"sh\"", i, cmds[i][0])
		}
	}
	if cmds[0][2] != "cd /workspace && go mod download" {
		t.Errorf("setup cmd 0 = %q, want %q", cmds[0][2], "cd /workspace && go mod download")
	}
	if cmds[1][2] != "go generate ./..." {
		t.Errorf("setup cmd 1 = %q, want %q", cmds[1][2], "go generate ./...")
	}

	// Third command must be claude.
	if cmds[2][0] != "claude" {
		t.Errorf("command[2][0] = %q, want \"claude\"", cmds[2][0])
	}
}

func Test_Runner_SetupFailureAborts(t *testing.T) {
	mock := NewMockExecutor()
	mock.OnCommand("sh", MockResponse{ExitCode: 1})

	runner := NewRunner(mock)
	cfg := RunConfig{
		ContainerID: "c1",
		Prompt:      "fix it",
		MaxTurns:    5,
		Timeout:     30 * time.Second,
		SetupCmds:   []string{"failing-command"},
		Evaluation:  corpus.Evaluation{Method: "command", Command: "go vet ./..."},
	}

	_, err := runner.Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error from failed setup, got nil")
	}
	if !strings.Contains(err.Error(), "setup command failed") {
		t.Errorf("error = %q, want it to contain \"setup command failed\"", err.Error())
	}

	// Claude should never have been called.
	for _, cmd := range mock.Commands() {
		if cmd[0] == "claude" {
			t.Error("claude was called despite setup failure")
		}
	}
}

func Test_Runner_StreamParsing(t *testing.T) {
	mock := NewMockExecutor()

	stream := buildStream(
		claude.StreamMessage{
			Type: "system", Subtype: "init",
			Tools: []string{"Read", "Write", "Bash"},
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
			Type: "result", Result: "Fixed the bug",
			CostUSD: 0.034, DurationMS: 8500, NumTurns: 3,
			Usage: &claude.Usage{InputTokens: 5000, OutputTokens: 1200},
		},
	)
	mock.OnCommand("claude", MockResponse{Stdout: stream})
	mock.OnCommand("sh", MockResponse{ExitCode: 0})

	runner := NewRunner(mock)
	result, err := runner.Run(context.Background(), RunConfig{
		ContainerID: "c1",
		Prompt:      "fix it",
		MaxTurns:    10,
		Evaluation:  corpus.Evaluation{Method: "command", Command: "true"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.CostUSD != 0.034 {
		t.Errorf("CostUSD = %f, want 0.034", result.CostUSD)
	}
	if result.DurationMS != 8500 {
		t.Errorf("DurationMS = %d, want 8500", result.DurationMS)
	}
	if result.NumTurns != 3 {
		t.Errorf("NumTurns = %d, want 3", result.NumTurns)
	}
	if result.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if result.Usage.InputTokens != 5000 {
		t.Errorf("InputTokens = %d, want 5000", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 1200 {
		t.Errorf("OutputTokens = %d, want 1200", result.Usage.OutputTokens)
	}
	if result.FinalText != "Fixed the bug" {
		t.Errorf("FinalText = %q, want %q", result.FinalText, "Fixed the bug")
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("ToolCalls count = %d, want 1", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Name != "Bash" {
		t.Errorf("ToolCalls[0].Name = %q, want \"Bash\"", result.ToolCalls[0].Name)
	}
	if len(result.InitTools) != 3 {
		t.Errorf("InitTools count = %d, want 3", len(result.InitTools))
	}
}

func Test_Runner_SkipsGarbageLines(t *testing.T) {
	mock := NewMockExecutor()

	initMsg, _ := json.Marshal(claude.StreamMessage{
		Type: "system", Subtype: "init",
		Tools: []string{"Read"},
	})
	resultMsg, _ := json.Marshal(claude.StreamMessage{
		Type: "result", Result: "ok",
		CostUSD: 0.01, DurationMS: 500, NumTurns: 1,
	})

	// Mix valid JSON with garbage.
	stream := "DEBUG: connecting to server\n" +
		string(initMsg) + "\n" +
		"\n" +
		"{invalid json here\n" +
		string(resultMsg) + "\n"

	mock.OnCommand("claude", MockResponse{Stdout: stream})
	mock.OnCommand("sh", MockResponse{ExitCode: 0})

	runner := NewRunner(mock)
	result, err := runner.Run(context.Background(), RunConfig{
		ContainerID: "c1",
		Prompt:      "fix it",
		MaxTurns:    5,
		Evaluation:  corpus.Evaluation{Method: "command", Command: "true"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FinalText != "ok" {
		t.Errorf("FinalText = %q, want %q", result.FinalText, "ok")
	}
	if result.CostUSD != 0.01 {
		t.Errorf("CostUSD = %f, want 0.01", result.CostUSD)
	}
}

func Test_Runner_BuildsClaudeCommand(t *testing.T) {
	mock := NewMockExecutor()

	stream := buildStream(
		claude.StreamMessage{Type: "system", Subtype: "init", Tools: []string{"Read"}},
		claude.StreamMessage{Type: "result", Result: "done", CostUSD: 0.01, DurationMS: 100, NumTurns: 1},
	)
	mock.OnCommand("claude", MockResponse{Stdout: stream})
	mock.OnCommand("sh", MockResponse{ExitCode: 0})

	runner := NewRunner(mock)
	_, err := runner.Run(context.Background(), RunConfig{
		ContainerID: "c1",
		Prompt:      "Fix the nil pointer",
		MaxTurns:    15,
		Evaluation:  corpus.Evaluation{Method: "command", Command: "true"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the claude command.
	var claudeCmd []string
	for _, cmd := range mock.Commands() {
		if cmd[0] == "claude" {
			claudeCmd = cmd
			break
		}
	}
	if claudeCmd == nil {
		t.Fatal("claude command not found in recorded commands")
	}

	expected := []string{
		"claude", "--print", "--output-format", "stream-json",
		"--verbose", "--dangerously-skip-permissions",
		"--max-turns", "15", "Fix the nil pointer",
	}
	if len(claudeCmd) != len(expected) {
		t.Fatalf("claude cmd length = %d, want %d\ngot:  %v\nwant: %v", len(claudeCmd), len(expected), claudeCmd, expected)
	}
	for i := range expected {
		if claudeCmd[i] != expected[i] {
			t.Errorf("claude cmd[%d] = %q, want %q", i, claudeCmd[i], expected[i])
		}
	}
}

func Test_Evaluate_CommandSuccess(t *testing.T) {
	mock := NewMockExecutor()
	mock.OnCommand("sh", MockResponse{ExitCode: 0})

	runner := NewRunner(mock)
	success, score, err := runner.evaluate(context.Background(), "c1", corpus.Evaluation{
		Method:           "command",
		Command:          "go vet ./...",
		ExpectedExitCode: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !success {
		t.Error("expected success=true")
	}
	if score != 1.0 {
		t.Errorf("score = %f, want 1.0", score)
	}
}

func Test_Evaluate_CommandFailure(t *testing.T) {
	mock := NewMockExecutor()
	mock.OnCommand("sh", MockResponse{ExitCode: 2})

	runner := NewRunner(mock)
	success, score, err := runner.evaluate(context.Background(), "c1", corpus.Evaluation{
		Method:           "command",
		Command:          "go vet ./...",
		ExpectedExitCode: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if success {
		t.Error("expected success=false")
	}
	if score != 0.0 {
		t.Errorf("score = %f, want 0.0", score)
	}
}

func Test_Evaluate_CommandExpectsNonZero(t *testing.T) {
	mock := NewMockExecutor()
	mock.OnCommand("sh", MockResponse{ExitCode: 1})

	runner := NewRunner(mock)
	success, score, err := runner.evaluate(context.Background(), "c1", corpus.Evaluation{
		Method:           "command",
		Command:          "grep -q 'error' output.log",
		ExpectedExitCode: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !success {
		t.Error("expected success=true")
	}
	if score != 1.0 {
		t.Errorf("score = %f, want 1.0", score)
	}
}

func Test_Evaluate_TestPasses(t *testing.T) {
	mock := NewMockExecutor()
	mock.OnCommand("sh", MockResponse{ExitCode: 0})

	runner := NewRunner(mock)
	success, score, err := runner.evaluate(context.Background(), "c1", corpus.Evaluation{
		Method:  "test_passes",
		Command: "go test ./...",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !success {
		t.Error("expected success=true")
	}
	if score != 1.0 {
		t.Errorf("score = %f, want 1.0", score)
	}
}

func Test_Evaluate_TestFails(t *testing.T) {
	mock := NewMockExecutor()
	mock.OnCommand("sh", MockResponse{ExitCode: 1})

	runner := NewRunner(mock)
	success, score, err := runner.evaluate(context.Background(), "c1", corpus.Evaluation{
		Method:  "test_passes",
		Command: "go test ./...",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if success {
		t.Error("expected success=false")
	}
	if score != 0.0 {
		t.Errorf("score = %f, want 0.0", score)
	}
}

func Test_Evaluate_FileContainsFound(t *testing.T) {
	mock := NewMockExecutor()
	mock.OnCommand("grep", MockResponse{ExitCode: 0})

	runner := NewRunner(mock)
	success, score, err := runner.evaluate(context.Background(), "c1", corpus.Evaluation{
		Method:   "file_contains",
		FilePath: "/workspace/main.go",
		Contains: "func handleNilPointer",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !success {
		t.Error("expected success=true")
	}
	if score != 1.0 {
		t.Errorf("score = %f, want 1.0", score)
	}

	// Verify the grep command was called correctly.
	var grepCmd []string
	for _, cmd := range mock.Commands() {
		if cmd[0] == "grep" {
			grepCmd = cmd
			break
		}
	}
	if grepCmd == nil {
		t.Fatal("grep command not found")
	}
	expected := []string{"grep", "-qF", "func handleNilPointer", "/workspace/main.go"}
	if len(grepCmd) != len(expected) {
		t.Fatalf("grep cmd = %v, want %v", grepCmd, expected)
	}
	for i := range expected {
		if grepCmd[i] != expected[i] {
			t.Errorf("grep cmd[%d] = %q, want %q", i, grepCmd[i], expected[i])
		}
	}
}

func Test_Evaluate_FileContainsNotFound(t *testing.T) {
	mock := NewMockExecutor()
	mock.OnCommand("grep", MockResponse{ExitCode: 1})

	runner := NewRunner(mock)
	success, score, err := runner.evaluate(context.Background(), "c1", corpus.Evaluation{
		Method:   "file_contains",
		FilePath: "/workspace/main.go",
		Contains: "func handleNilPointer",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if success {
		t.Error("expected success=false")
	}
	if score != 0.0 {
		t.Errorf("score = %f, want 0.0", score)
	}
}

func Test_Evaluate_UnknownMethod(t *testing.T) {
	mock := NewMockExecutor()
	runner := NewRunner(mock)

	_, _, err := runner.evaluate(context.Background(), "c1", corpus.Evaluation{
		Method: "llm_judge",
	})
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
	if !strings.Contains(err.Error(), "unknown evaluation method") {
		t.Errorf("error = %q, want it to contain \"unknown evaluation method\"", err.Error())
	}
}

func Test_Evaluate_EmptyMethod(t *testing.T) {
	mock := NewMockExecutor()
	runner := NewRunner(mock)

	_, _, err := runner.evaluate(context.Background(), "c1", corpus.Evaluation{
		Method: "",
	})
	if err == nil {
		t.Fatal("expected error for empty method")
	}
	if !strings.Contains(err.Error(), "unknown evaluation method") {
		t.Errorf("error = %q, want it to contain \"unknown evaluation method\"", err.Error())
	}
}

// suppress unused import warning
var _ = fmt.Sprintf
