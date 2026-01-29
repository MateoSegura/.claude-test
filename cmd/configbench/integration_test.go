//go:build integration

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/MateoSegura/.claude-test/internal/analysis"
)

// moduleRoot returns the absolute path to the Go module root.
func moduleRoot(t *testing.T) string {
	t.Helper()
	// The test file lives at cmd/configbench/, so module root is two levels up.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// buildBinary compiles the configbench binary into a temp directory and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()
	root := moduleRoot(t)
	binaryPath := filepath.Join(t.TempDir(), "configbench")

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/configbench")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, string(out))
	}
	return binaryPath
}

// writeCorpus writes a YAML corpus to a temp file and returns the path.
func writeCorpus(t *testing.T, yaml string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "corpus.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write corpus: %v", err)
	}
	return path
}

// --- Stage 5.5: End-to-End Dry-Run Tests ---

const twoIssueCorpus = `name: e2e-test
version: "1.0"
issues:
  - id: go-nil-check
    title: "Fix nil pointer dereference"
    description: "Handler panics on nil body"
    repo: "github.com/example/goapp"
    ref: "main"
    difficulty: easy
    language: go
    task_type: bug_fix
    prompt: "Fix the nil pointer dereference"
    evaluation:
      method: test_passes
      command: "go test ./..."
  - id: ts-auth
    title: "Add OAuth2 authentication"
    description: "Implement OAuth2 login flow"
    repo: "github.com/example/webapp"
    ref: "main"
    difficulty: hard
    language: typescript
    task_type: feature
    prompt: "Implement OAuth2 authentication"
    evaluation:
      method: command
      command: "npm test"
      expected_exit_code: 0
`

func TestE2EDryRun(t *testing.T) {
	binary := buildBinary(t)
	corpusPath := writeCorpus(t, twoIssueCorpus)

	cmd := exec.Command(binary, "--corpus", corpusPath, "--dry-run", "--attempts", "3")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary exited with error: %v\n%s", err, string(out))
	}

	output := string(out)

	// 2 issues
	if !strings.Contains(output, "2") {
		t.Errorf("output should contain issue count '2', got:\n%s", output)
	}
	// 2 issues x 2 variants x 3 attempts = 12 total runs
	if !strings.Contains(output, "12") {
		t.Errorf("output should contain '12' total runs, got:\n%s", output)
	}
	// Both issue IDs present
	if !strings.Contains(output, "go-nil-check") {
		t.Errorf("output should contain 'go-nil-check', got:\n%s", output)
	}
	if !strings.Contains(output, "ts-auth") {
		t.Errorf("output should contain 'ts-auth', got:\n%s", output)
	}
}

func TestE2EDryRun_BaselineDisabled(t *testing.T) {
	binary := buildBinary(t)
	corpusPath := writeCorpus(t, twoIssueCorpus)

	cmd := exec.Command(binary, "--corpus", corpusPath, "--dry-run", "--attempts", "3", "--baseline=false")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary exited with error: %v\n%s", err, string(out))
	}

	output := string(out)

	// 2 issues x 1 variant x 3 attempts = 6 total runs
	if !strings.Contains(output, "6") {
		t.Errorf("output should contain '6' total runs, got:\n%s", output)
	}
	if !strings.Contains(output, "configured only") {
		t.Errorf("output should contain 'configured only', got:\n%s", output)
	}
}

const threeIssueCorpus = `name: filter-test
version: "1.0"
issues:
  - id: go-nil-check
    title: "Fix nil pointer dereference"
    description: "Handler panics on nil body"
    repo: "github.com/example/goapp"
    ref: "main"
    difficulty: easy
    language: go
    task_type: bug_fix
    prompt: "Fix the nil pointer dereference"
    evaluation:
      method: test_passes
      command: "go test ./..."
  - id: go-concurrency
    title: "Fix race condition"
    description: "Race in concurrent map access"
    repo: "github.com/example/goapp2"
    ref: "main"
    difficulty: medium
    language: go
    task_type: bug_fix
    prompt: "Fix the race condition"
    evaluation:
      method: test_passes
      command: "go test -race ./..."
  - id: py-refactor
    title: "Extract database layer"
    description: "Move SQL to repository pattern"
    repo: "github.com/example/pyapi"
    ref: "main"
    difficulty: medium
    language: python
    task_type: refactor
    prompt: "Refactor database queries"
    evaluation:
      method: file_contains
      file_path: "src/repository.py"
      contains: "class Repository"
`

func TestE2EDryRun_WithFilter(t *testing.T) {
	binary := buildBinary(t)
	corpusPath := writeCorpus(t, threeIssueCorpus)

	cmd := exec.Command(binary, "--corpus", corpusPath, "--filter-language", "go", "--dry-run")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary exited with error: %v\n%s", err, string(out))
	}

	output := string(out)

	// Only 2 Go issues should be present
	if !strings.Contains(output, "go-nil-check") {
		t.Errorf("output should contain 'go-nil-check', got:\n%s", output)
	}
	if !strings.Contains(output, "go-concurrency") {
		t.Errorf("output should contain 'go-concurrency', got:\n%s", output)
	}
	// Python issue should be filtered out
	if strings.Contains(output, "py-refactor") {
		t.Errorf("output should NOT contain 'py-refactor' (filtered out), got:\n%s", output)
	}
	// Issue count line should show 2
	if !strings.Contains(output, "Issues:       2") {
		t.Errorf("output should show 2 issues, got:\n%s", output)
	}
}

// --- Stage 5.6: Mock Docker End-to-End Test ---

const mockDockerScript = `#!/bin/bash
# Mock docker script for integration testing
# Logs all invocations to $MOCK_DOCKER_LOG

if [ -n "$MOCK_DOCKER_LOG" ]; then
    echo "$@" >> "$MOCK_DOCKER_LOG"
fi

case "$1" in
    create)
        echo "mock-container-id-12345"
        exit 0
        ;;
    cp)
        exit 0
        ;;
    exec)
        # Check if this is the claude command
        if echo "$@" | grep -q "claude"; then
            # Output canned stream-json
            cat <<'STREAM'
{"type":"system","subtype":"init","tools":["Read","Write","Edit","Bash"]}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","id":"t1","input":{"file_path":"handler.go"}}]}}
{"type":"result","result":"Fixed the issue","cost_usd":0.05,"duration_ms":8000,"num_turns":3,"usage":{"input_tokens":5000,"output_tokens":1200}}
STREAM
            exit 0
        else
            # Setup or eval command — just succeed
            exit 0
        fi
        ;;
    rm)
        exit 0
        ;;
    info)
        echo "Mock Docker"
        exit 0
        ;;
    *)
        echo "unknown docker command: $1" >&2
        exit 1
        ;;
esac
`

const singleIssueCorpus = `name: mock-docker-test
version: "1.0"
issues:
  - id: go-nil-check
    title: "Fix nil pointer dereference"
    description: "Handler panics on nil body"
    repo: "github.com/example/goapp"
    ref: "main"
    difficulty: easy
    language: go
    task_type: bug_fix
    prompt: "Fix the nil pointer dereference"
    evaluation:
      method: command
      command: "echo ok"
      expected_exit_code: 0
`

func TestE2EMockDocker(t *testing.T) {
	binary := buildBinary(t)

	// Create temp directory for mock docker binary
	mockDir := t.TempDir()
	mockDockerPath := filepath.Join(mockDir, "docker")
	if err := os.WriteFile(mockDockerPath, []byte(mockDockerScript), 0755); err != nil {
		t.Fatalf("failed to write mock docker script: %v", err)
	}

	// Create a mock docker log file
	dockerLogPath := filepath.Join(mockDir, "docker.log")

	// Write the test corpus
	corpusPath := writeCorpus(t, singleIssueCorpus)

	// Create a minimal .claude config directory
	configDir := filepath.Join(t.TempDir(), ".claude")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	// Write an empty settings file so the config dir is valid
	settingsPath := filepath.Join(configDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write settings: %v", err)
	}

	// Create output file path
	outputPath := filepath.Join(t.TempDir(), "report.json")

	// Build the command with mock docker in PATH
	cmd := exec.Command(binary,
		"--corpus", corpusPath,
		"--attempts", "1",
		"--baseline=false",
		"--format", "json",
		"--output", outputPath,
		"--config", configDir,
		"--timeout", "30s",
	)

	// Prepend mock dir to PATH so the mock docker is found first
	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = "PATH=" + mockDir + string(os.PathListSeparator) + strings.TrimPrefix(e, "PATH=")
			break
		}
	}
	env = append(env, "MOCK_DOCKER_LOG="+dockerLogPath)
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary exited with error: %v\n%s", err, string(out))
	}

	// Read and parse the JSON output
	reportData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var report analysis.Report
	if err := json.Unmarshal(reportData, &report); err != nil {
		t.Fatalf("failed to unmarshal report JSON: %v\nraw: %s", err, string(reportData))
	}

	// Verify the report
	if report.TotalRuns != 1 {
		t.Errorf("TotalRuns = %d, want 1", report.TotalRuns)
	}
	if report.CorpusSize != 1 {
		t.Errorf("CorpusSize = %d, want 1", report.CorpusSize)
	}

	// Verify mock docker was actually called by checking the log
	if logData, err := os.ReadFile(dockerLogPath); err == nil {
		logStr := string(logData)
		if !strings.Contains(logStr, "create") {
			t.Errorf("mock docker log should contain 'create' command, got:\n%s", logStr)
		}
	}
}
