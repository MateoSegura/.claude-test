package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/MateoSegura/.claude-test/internal/corpus"
	"github.com/MateoSegura/.claude-test/internal/orchestrator"
)

func TestParseFlags_Defaults(t *testing.T) {
	f, err := parseFlags([]string{"--corpus", "test.yaml"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if f.Config != ".claude" {
		t.Errorf("Config = %q, want %q", f.Config, ".claude")
	}
	if f.Corpus != "test.yaml" {
		t.Errorf("Corpus = %q, want %q", f.Corpus, "test.yaml")
	}
	if f.Attempts != 5 {
		t.Errorf("Attempts = %d, want 5", f.Attempts)
	}
	if f.Parallelism != 2 {
		t.Errorf("Parallelism = %d, want 2", f.Parallelism)
	}
	if f.Baseline != true {
		t.Errorf("Baseline = %v, want true", f.Baseline)
	}
	if f.Image != "ghcr.io/mateosegura/dev-env:latest" {
		t.Errorf("Image = %q, want %q", f.Image, "ghcr.io/mateosegura/dev-env:latest")
	}
	if f.Output != "" {
		t.Errorf("Output = %q, want empty", f.Output)
	}
	if f.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want 10m", f.Timeout)
	}
	if f.MaxTurns != 25 {
		t.Errorf("MaxTurns = %d, want 25", f.MaxTurns)
	}
	if f.FilterDifficulty != "" {
		t.Errorf("FilterDifficulty = %q, want empty", f.FilterDifficulty)
	}
	if f.FilterLanguage != "" {
		t.Errorf("FilterLanguage = %q, want empty", f.FilterLanguage)
	}
	if f.FilterTask != "" {
		t.Errorf("FilterTask = %q, want empty", f.FilterTask)
	}
	if f.DryRun != false {
		t.Errorf("DryRun = %v, want false", f.DryRun)
	}
	if f.Verbose != false {
		t.Errorf("Verbose = %v, want false", f.Verbose)
	}
	if f.Format != "terminal" {
		t.Errorf("Format = %q, want %q", f.Format, "terminal")
	}
}

func TestParseFlags_AllFlags(t *testing.T) {
	args := []string{
		"--config", "/custom/config",
		"--corpus", "/path/to/corpus.yaml",
		"--attempts", "10",
		"--parallelism", "4",
		"--baseline=false",
		"--image", "custom-image:v2",
		"--output", "results.json",
		"--timeout", "30m",
		"--max-turns", "50",
		"--filter-difficulty", "easy,hard",
		"--filter-language", "go,python",
		"--filter-task", "bug_fix",
		"--dry-run",
		"--verbose",
		"--format", "json",
	}

	f, err := parseFlags(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if f.Config != "/custom/config" {
		t.Errorf("Config = %q, want %q", f.Config, "/custom/config")
	}
	if f.Corpus != "/path/to/corpus.yaml" {
		t.Errorf("Corpus = %q, want %q", f.Corpus, "/path/to/corpus.yaml")
	}
	if f.Attempts != 10 {
		t.Errorf("Attempts = %d, want 10", f.Attempts)
	}
	if f.Parallelism != 4 {
		t.Errorf("Parallelism = %d, want 4", f.Parallelism)
	}
	if f.Baseline != false {
		t.Errorf("Baseline = %v, want false", f.Baseline)
	}
	if f.Image != "custom-image:v2" {
		t.Errorf("Image = %q, want %q", f.Image, "custom-image:v2")
	}
	if f.Output != "results.json" {
		t.Errorf("Output = %q, want %q", f.Output, "results.json")
	}
	if f.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want 30m", f.Timeout)
	}
	if f.MaxTurns != 50 {
		t.Errorf("MaxTurns = %d, want 50", f.MaxTurns)
	}
	if f.FilterDifficulty != "easy,hard" {
		t.Errorf("FilterDifficulty = %q, want %q", f.FilterDifficulty, "easy,hard")
	}
	if f.FilterLanguage != "go,python" {
		t.Errorf("FilterLanguage = %q, want %q", f.FilterLanguage, "go,python")
	}
	if f.FilterTask != "bug_fix" {
		t.Errorf("FilterTask = %q, want %q", f.FilterTask, "bug_fix")
	}
	if f.DryRun != true {
		t.Errorf("DryRun = %v, want true", f.DryRun)
	}
	if f.Verbose != true {
		t.Errorf("Verbose = %v, want true", f.Verbose)
	}
	if f.Format != "json" {
		t.Errorf("Format = %q, want %q", f.Format, "json")
	}
}

func TestParseFlags_MissingCorpus(t *testing.T) {
	_, err := parseFlags([]string{})
	if err == nil {
		t.Fatal("expected error for missing corpus flag")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "corpus") || !strings.Contains(errMsg, "required") {
		t.Errorf("error = %q, want it to contain 'corpus' and 'required'", errMsg)
	}
}

func TestParseFlags_BadAttempts(t *testing.T) {
	_, err := parseFlags([]string{"--corpus", "x.yaml", "--attempts", "0"})
	if err == nil {
		t.Fatal("expected error for attempts=0")
	}
	if !strings.Contains(err.Error(), "attempts") {
		t.Errorf("error = %q, want it to contain 'attempts'", err.Error())
	}
}

func TestParseFlags_BadFormat(t *testing.T) {
	_, err := parseFlags([]string{"--corpus", "x.yaml", "--format", "csv"})
	if err == nil {
		t.Fatal("expected error for format=csv")
	}
	if !strings.Contains(err.Error(), "format") {
		t.Errorf("error = %q, want it to contain 'format'", err.Error())
	}
}

func TestParseFlags_FilterParsing(t *testing.T) {
	f, err := parseFlags([]string{"--corpus", "x.yaml", "--filter-difficulty", "easy,medium,hard"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := splitCSV(f.FilterDifficulty)
	want := []string{"easy", "medium", "hard"}
	if len(got) != len(want) {
		t.Fatalf("splitCSV length = %d, want %d", len(got), len(want))
	}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("splitCSV[%d] = %q, want %q", i, v, want[i])
		}
	}
}

func TestFlagsToOrchConfig(t *testing.T) {
	f := &cliFlags{
		Config:           "/my/config",
		Corpus:           "/my/corpus.yaml",
		Attempts:         7,
		Parallelism:      3,
		Baseline:         false,
		Image:            "test-image:v1",
		Output:           "out.json",
		Timeout:          5 * time.Minute,
		MaxTurns:         15,
		FilterDifficulty: "easy,hard",
		FilterLanguage:   "go",
		FilterTask:       "bug_fix,refactor",
		Format:           "json",
	}

	cfg := flagsToOrchConfig(f)

	if cfg.Config.Path != "/my/config" {
		t.Errorf("Config.Path = %q, want %q", cfg.Config.Path, "/my/config")
	}
	if cfg.Corpus.Path != "/my/corpus.yaml" {
		t.Errorf("Corpus.Path = %q, want %q", cfg.Corpus.Path, "/my/corpus.yaml")
	}
	if cfg.Docker.Image != "test-image:v1" {
		t.Errorf("Docker.Image = %q, want %q", cfg.Docker.Image, "test-image:v1")
	}
	if cfg.Execution.Attempts != 7 {
		t.Errorf("Execution.Attempts = %d, want 7", cfg.Execution.Attempts)
	}
	if cfg.Execution.Parallelism != 3 {
		t.Errorf("Execution.Parallelism = %d, want 3", cfg.Execution.Parallelism)
	}
	if cfg.Execution.Baseline == nil || *cfg.Execution.Baseline != false {
		t.Errorf("Execution.Baseline = %v, want false", cfg.Execution.Baseline)
	}
	if cfg.Execution.Timeout != 5*time.Minute {
		t.Errorf("Execution.Timeout = %v, want 5m", cfg.Execution.Timeout)
	}
	if cfg.Execution.MaxTurns != 15 {
		t.Errorf("Execution.MaxTurns = %d, want 15", cfg.Execution.MaxTurns)
	}
	if cfg.Output.Path != "out.json" {
		t.Errorf("Output.Path = %q, want %q", cfg.Output.Path, "out.json")
	}

	// Check filters.
	wantDiff := []string{"easy", "hard"}
	if len(cfg.Corpus.Filters.Difficulty) != len(wantDiff) {
		t.Fatalf("Difficulty filter len = %d, want %d", len(cfg.Corpus.Filters.Difficulty), len(wantDiff))
	}
	for i, v := range cfg.Corpus.Filters.Difficulty {
		if v != wantDiff[i] {
			t.Errorf("Difficulty[%d] = %q, want %q", i, v, wantDiff[i])
		}
	}

	wantLang := []string{"go"}
	if len(cfg.Corpus.Filters.Language) != len(wantLang) {
		t.Fatalf("Language filter len = %d, want %d", len(cfg.Corpus.Filters.Language), len(wantLang))
	}

	wantTask := []string{"bug_fix", "refactor"}
	if len(cfg.Corpus.Filters.TaskType) != len(wantTask) {
		t.Fatalf("TaskType filter len = %d, want %d", len(cfg.Corpus.Filters.TaskType), len(wantTask))
	}
}

func TestDryRunOutput(t *testing.T) {
	issues := []corpus.Issue{
		{ID: "go-nil-check", Difficulty: "easy", Language: "go", TaskType: "bug_fix"},
		{ID: "ts-auth", Difficulty: "hard", Language: "typescript", TaskType: "feature"},
		{ID: "py-refactor", Difficulty: "medium", Language: "python", TaskType: "refactor"},
	}

	cfg := &orchestrator.BenchConfig{
		Config:    orchestrator.ConfigSpec{Path: ".claude"},
		Docker:    orchestrator.DockerSpec{Image: "test-image:latest"},
		Execution: orchestrator.ExecSpec{Attempts: 5, Parallelism: 2, Timeout: 10 * time.Minute, Baseline: orchestrator.BoolPtr(true), MaxTurns: 25},
	}

	pairs := orchestrator.GeneratePlan(cfg, issues)

	f := &cliFlags{
		Baseline:    true,
		Attempts:    5,
		Image:       "test-image:latest",
		Parallelism: 2,
		Timeout:     10 * time.Minute,
	}

	var buf bytes.Buffer
	printDryRun(&buf, pairs, f)
	output := buf.String()

	// 3 issues * 5 attempts * 2 variants = 30 total runs.
	if !strings.Contains(output, "30") {
		t.Errorf("output should contain '30' total runs, got:\n%s", output)
	}
	if !strings.Contains(output, "go-nil-check") {
		t.Errorf("output should contain 'go-nil-check'")
	}
	if !strings.Contains(output, "ts-auth") {
		t.Errorf("output should contain 'ts-auth'")
	}
	if !strings.Contains(output, "py-refactor") {
		t.Errorf("output should contain 'py-refactor'")
	}
	if !strings.Contains(output, "test-image:latest") {
		t.Errorf("output should contain image name")
	}
	if !strings.Contains(output, "2") {
		t.Errorf("output should contain parallelism '2'")
	}
}

func TestDryRunBaselineDisabled(t *testing.T) {
	issues := []corpus.Issue{
		{ID: "go-nil-check", Difficulty: "easy", Language: "go", TaskType: "bug_fix"},
		{ID: "ts-auth", Difficulty: "hard", Language: "typescript", TaskType: "feature"},
		{ID: "py-refactor", Difficulty: "medium", Language: "python", TaskType: "refactor"},
	}

	cfg := &orchestrator.BenchConfig{
		Config:    orchestrator.ConfigSpec{Path: ".claude"},
		Docker:    orchestrator.DockerSpec{Image: "test-image:latest"},
		Execution: orchestrator.ExecSpec{Attempts: 5, Parallelism: 2, Timeout: 10 * time.Minute, Baseline: orchestrator.BoolPtr(false), MaxTurns: 25},
	}

	pairs := orchestrator.GeneratePlan(cfg, issues)

	f := &cliFlags{
		Baseline:    false,
		Attempts:    5,
		Image:       "test-image:latest",
		Parallelism: 2,
		Timeout:     10 * time.Minute,
	}

	var buf bytes.Buffer
	printDryRun(&buf, pairs, f)
	output := buf.String()

	// 3 issues * 5 attempts * 1 variant = 15 total runs.
	if !strings.Contains(output, "15") {
		t.Errorf("output should contain '15' total runs, got:\n%s", output)
	}
	if !strings.Contains(output, "configured only") {
		t.Errorf("output should contain 'configured only', got:\n%s", output)
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "empty", input: "", want: nil},
		{name: "single", input: "go", want: []string{"go"}},
		{name: "multiple", input: "easy,medium,hard", want: []string{"easy", "medium", "hard"}},
		{name: "with spaces", input: " go , python ", want: []string{"go", "python"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitCSV(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("splitCSV(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("splitCSV(%q) len = %d, want %d", tt.input, len(got), len(tt.want))
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("splitCSV(%q)[%d] = %q, want %q", tt.input, i, v, tt.want[i])
				}
			}
		})
	}
}

// --- Stage 5.7: Smoke Tests ---

// smokeModuleRoot returns the absolute path to the Go module root.
func smokeModuleRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

func TestBinaryBuilds(t *testing.T) {
	root := smokeModuleRoot(t)
	cmd := exec.Command("go", "build", "./cmd/configbench")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(out))
	}
}

func TestBinaryHelp(t *testing.T) {
	root := smokeModuleRoot(t)
	binaryPath := filepath.Join(t.TempDir(), "configbench")

	// Build the binary first.
	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/configbench")
	build.Dir = root
	buildOut, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(buildOut))
	}

	// Run --help.
	cmd := exec.Command(binaryPath, "--help")
	out, err := cmd.CombinedOutput()
	// --help via flag.FlagSet with ContinueOnError returns ErrHelp,
	// which causes a non-zero exit. We accept either exit code.
	output := string(out)

	_ = err // --help may return exit code 2 with flag package

	if !strings.Contains(output, "corpus") {
		t.Errorf("--help output should contain 'corpus', got:\n%s", output)
	}
	if !strings.Contains(output, "configbench") {
		t.Errorf("--help output should contain 'configbench', got:\n%s", output)
	}
}
