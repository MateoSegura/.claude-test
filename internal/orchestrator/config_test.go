package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWithDefaults_AllZero(t *testing.T) {
	cfg := &BenchConfig{}
	got := cfg.WithDefaults()

	if got.Execution.Attempts != 5 {
		t.Errorf("Attempts = %d, want 5", got.Execution.Attempts)
	}
	if got.Execution.Parallelism != 2 {
		t.Errorf("Parallelism = %d, want 2", got.Execution.Parallelism)
	}
	if got.Execution.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want 10m", got.Execution.Timeout)
	}
	if got.Execution.Baseline == nil || !*got.Execution.Baseline {
		t.Errorf("Baseline = %v, want true", got.Execution.Baseline)
	}
	if got.Execution.MaxTurns != 25 {
		t.Errorf("MaxTurns = %d, want 25", got.Execution.MaxTurns)
	}
	if got.Docker.Image != "ghcr.io/mateosegura/dev-env:latest" {
		t.Errorf("Image = %q, want default", got.Docker.Image)
	}
	if got.Analysis.SignificanceThreshold != 0.05 {
		t.Errorf("SignificanceThreshold = %f, want 0.05", got.Analysis.SignificanceThreshold)
	}
	if got.Analysis.ImprovementThreshold != 0.1 {
		t.Errorf("ImprovementThreshold = %f, want 0.1", got.Analysis.ImprovementThreshold)
	}
	if len(got.Output.Formats) != 1 || got.Output.Formats[0] != "terminal" {
		t.Errorf("Formats = %v, want [terminal]", got.Output.Formats)
	}
}

func TestWithDefaults_Partial(t *testing.T) {
	cfg := &BenchConfig{
		Execution: ExecSpec{
			Attempts:    3,
			Parallelism: 8,
		},
	}
	got := cfg.WithDefaults()

	if got.Execution.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3 (preserved)", got.Execution.Attempts)
	}
	if got.Execution.Parallelism != 8 {
		t.Errorf("Parallelism = %d, want 8 (preserved)", got.Execution.Parallelism)
	}
	if got.Execution.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want 10m (default)", got.Execution.Timeout)
	}
	if got.Execution.Baseline == nil || !*got.Execution.Baseline {
		t.Errorf("Baseline should default to true")
	}
	if got.Execution.MaxTurns != 25 {
		t.Errorf("MaxTurns = %d, want 25 (default)", got.Execution.MaxTurns)
	}
}

func TestWithDefaults_FullySpecified(t *testing.T) {
	bl := false
	cfg := &BenchConfig{
		Config: ConfigSpec{Path: "/my/config"},
		Docker: DockerSpec{Image: "myimage:v1"},
		Corpus: CorpusSpec{Path: "/my/corpus"},
		Output: OutputSpec{Formats: []string{"json", "csv"}},
		Analysis: AnalysisSpec{
			SignificanceThreshold: 0.01,
			ImprovementThreshold:  0.2,
		},
		Execution: ExecSpec{
			Attempts:    10,
			Parallelism: 4,
			Timeout:     5 * time.Minute,
			Baseline:    &bl,
			MaxTurns:    50,
		},
	}
	got := cfg.WithDefaults()

	if got.Execution.Attempts != 10 {
		t.Errorf("Attempts changed")
	}
	if got.Execution.Parallelism != 4 {
		t.Errorf("Parallelism changed")
	}
	if got.Execution.Timeout != 5*time.Minute {
		t.Errorf("Timeout changed")
	}
	if got.Execution.Baseline == nil || *got.Execution.Baseline {
		t.Errorf("Baseline should remain false")
	}
	if got.Execution.MaxTurns != 50 {
		t.Errorf("MaxTurns changed")
	}
	if got.Docker.Image != "myimage:v1" {
		t.Errorf("Image changed")
	}
	if got.Analysis.SignificanceThreshold != 0.01 {
		t.Errorf("SignificanceThreshold changed")
	}
	if got.Analysis.ImprovementThreshold != 0.2 {
		t.Errorf("ImprovementThreshold changed")
	}
	if len(got.Output.Formats) != 2 {
		t.Errorf("Formats changed")
	}
}

func validConfig() *BenchConfig {
	return &BenchConfig{
		Config: ConfigSpec{Path: "/some/config"},
		Docker: DockerSpec{Image: "myimage:latest"},
		Corpus: CorpusSpec{Path: "/some/corpus.yaml"},
		Output: OutputSpec{Formats: []string{"terminal"}},
		Analysis: AnalysisSpec{
			SignificanceThreshold: 0.05,
			ImprovementThreshold:  0.1,
		},
		Execution: ExecSpec{
			Attempts:    5,
			Parallelism: 2,
			Timeout:     10 * time.Minute,
			Baseline:    BoolPtr(true),
			MaxTurns:    25,
		},
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := validConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

func TestValidate_MissingConfigPath(t *testing.T) {
	cfg := validConfig()
	cfg.Config.Path = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "config") {
		t.Errorf("error %q should mention config", err.Error())
	}
}

func TestValidate_MissingCorpusPath(t *testing.T) {
	cfg := validConfig()
	cfg.Corpus.Path = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "corpus") {
		t.Errorf("error %q should mention corpus", err.Error())
	}
}

func TestValidate_ZeroAttempts(t *testing.T) {
	cfg := validConfig()
	cfg.Execution.Attempts = 0
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "attempts") {
		t.Errorf("error %q should mention attempts", err.Error())
	}
}

func TestValidate_NegativeParallelism(t *testing.T) {
	cfg := validConfig()
	cfg.Execution.Parallelism = -1
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "parallelism") {
		t.Errorf("error %q should mention parallelism", err.Error())
	}
}

func TestValidate_ZeroTimeout(t *testing.T) {
	cfg := validConfig()
	cfg.Execution.Timeout = 0
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "timeout") {
		t.Errorf("error %q should mention timeout", err.Error())
	}
}

func TestValidate_SignificanceOutOfRange(t *testing.T) {
	cfg := validConfig()
	cfg.Analysis.SignificanceThreshold = 1.5
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "significance") {
		t.Errorf("error %q should mention significance", err.Error())
	}
}

func TestValidate_SignificanceZero(t *testing.T) {
	cfg := validConfig()
	cfg.Analysis.SignificanceThreshold = 0.0
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "significance") {
		t.Errorf("error %q should mention significance", err.Error())
	}
}

func TestValidate_MultipleViolations(t *testing.T) {
	cfg := validConfig()
	cfg.Config.Path = ""
	cfg.Execution.Attempts = 0
	cfg.Analysis.SignificanceThreshold = 2.0
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "config") {
		t.Errorf("error should mention config")
	}
	if !strings.Contains(msg, "attempts") {
		t.Errorf("error should mention attempts")
	}
	if !strings.Contains(msg, "significance") {
		t.Errorf("error should mention significance")
	}
}

func TestLoadConfig_Valid(t *testing.T) {
	yaml := `
config:
  path: /my/config
  description: "test config"
docker:
  image: "myimage:v2"
execution:
  attempts: 3
  parallelism: 4
  timeout: 5m
  baseline: false
  max_turns: 15
corpus:
  path: /my/corpus.yaml
  filters:
    difficulty: [easy, medium]
    language: [go]
    task_type: [bugfix]
analysis:
  significance_threshold: 0.01
  improvement_threshold: 0.2
output:
  path: /tmp/results
  formats: [json, csv]
`
	path := writeTempFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Config.Path != "/my/config" {
		t.Errorf("Config.Path = %q", cfg.Config.Path)
	}
	if cfg.Config.Description != "test config" {
		t.Errorf("Config.Description = %q", cfg.Config.Description)
	}
	if cfg.Docker.Image != "myimage:v2" {
		t.Errorf("Docker.Image = %q", cfg.Docker.Image)
	}
	if cfg.Execution.Attempts != 3 {
		t.Errorf("Attempts = %d", cfg.Execution.Attempts)
	}
	if cfg.Execution.Parallelism != 4 {
		t.Errorf("Parallelism = %d", cfg.Execution.Parallelism)
	}
	if cfg.Execution.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v", cfg.Execution.Timeout)
	}
	if cfg.Execution.Baseline == nil || *cfg.Execution.Baseline {
		t.Errorf("Baseline should be false")
	}
	if cfg.Execution.MaxTurns != 15 {
		t.Errorf("MaxTurns = %d", cfg.Execution.MaxTurns)
	}
	if cfg.Corpus.Path != "/my/corpus.yaml" {
		t.Errorf("Corpus.Path = %q", cfg.Corpus.Path)
	}
	if len(cfg.Corpus.Filters.Difficulty) != 2 {
		t.Errorf("Filters.Difficulty = %v", cfg.Corpus.Filters.Difficulty)
	}
	if cfg.Analysis.SignificanceThreshold != 0.01 {
		t.Errorf("SignificanceThreshold = %f", cfg.Analysis.SignificanceThreshold)
	}
	if cfg.Output.Path != "/tmp/results" {
		t.Errorf("Output.Path = %q", cfg.Output.Path)
	}
	if len(cfg.Output.Formats) != 2 {
		t.Errorf("Output.Formats = %v", cfg.Output.Formats)
	}
}

func TestLoadConfig_MinimalYAML(t *testing.T) {
	yaml := `
config:
  path: /my/config
corpus:
  path: /my/corpus.yaml
`
	path := writeTempFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Defaults should be applied
	if cfg.Execution.Attempts != 5 {
		t.Errorf("Attempts = %d, want 5", cfg.Execution.Attempts)
	}
	if cfg.Execution.Parallelism != 2 {
		t.Errorf("Parallelism = %d, want 2", cfg.Execution.Parallelism)
	}
	if cfg.Docker.Image != "ghcr.io/mateosegura/dev-env:latest" {
		t.Errorf("Image = %q, want default", cfg.Docker.Image)
	}
	if cfg.Execution.Baseline == nil || !*cfg.Execution.Baseline {
		t.Errorf("Baseline should default to true")
	}
}

func TestLoadConfig_InvalidValues(t *testing.T) {
	yaml := `
config:
  path: /my/config
corpus:
  path: /my/corpus.yaml
execution:
  attempts: -1
`
	path := writeTempFile(t, yaml)
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid attempts")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "attempts") {
		t.Errorf("error %q should mention attempts", err.Error())
	}
}

func TestLoadConfig_FileNotExist(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadConfig_MalformedYAML(t *testing.T) {
	content := `
config:
  path: [invalid
  broken yaml here
`
	path := writeTempFile(t, content)
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestLoadConfig_WithFilters(t *testing.T) {
	yaml := `
config:
  path: /my/config
corpus:
  path: /my/corpus.yaml
  filters:
    difficulty: [easy, hard]
    language: [go, python]
    task_type: [bugfix, refactor]
`
	path := writeTempFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(cfg.Corpus.Filters.Difficulty) != 2 {
		t.Errorf("Difficulty = %v", cfg.Corpus.Filters.Difficulty)
	}
	if cfg.Corpus.Filters.Difficulty[0] != "easy" || cfg.Corpus.Filters.Difficulty[1] != "hard" {
		t.Errorf("Difficulty values wrong: %v", cfg.Corpus.Filters.Difficulty)
	}
	if len(cfg.Corpus.Filters.Language) != 2 {
		t.Errorf("Language = %v", cfg.Corpus.Filters.Language)
	}
	if len(cfg.Corpus.Filters.TaskType) != 2 {
		t.Errorf("TaskType = %v", cfg.Corpus.Filters.TaskType)
	}
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}
