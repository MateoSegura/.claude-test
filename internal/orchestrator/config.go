package orchestrator

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// BenchConfig holds the full benchmark configuration.
type BenchConfig struct {
	ConfigDir string       `yaml:"-"`
	Config    ConfigSpec   `yaml:"config"`
	Docker    DockerSpec   `yaml:"docker"`
	Execution ExecSpec     `yaml:"execution"`
	Corpus    CorpusSpec   `yaml:"corpus"`
	Analysis  AnalysisSpec `yaml:"analysis"`
	Output    OutputSpec   `yaml:"output"`
}

// ConfigSpec identifies the Claude config under test.
type ConfigSpec struct {
	Path        string `yaml:"path"`
	Description string `yaml:"description"`
}

// DockerSpec configures the Docker environment.
type DockerSpec struct {
	Image string `yaml:"image"`
}

// ExecSpec controls execution parameters.
type ExecSpec struct {
	Attempts    int           `yaml:"attempts"`
	Parallelism int           `yaml:"parallelism"`
	Timeout     time.Duration `yaml:"timeout"`
	Baseline    *bool         `yaml:"baseline"`
	MaxTurns    int           `yaml:"max_turns"`
}

// CorpusSpec locates and filters the issue corpus.
type CorpusSpec struct {
	Path    string           `yaml:"path"`
	Filters CorpusFilterSpec `yaml:"filters"`
}

// CorpusFilterSpec controls which issues are included.
type CorpusFilterSpec struct {
	Difficulty []string `yaml:"difficulty"`
	Language   []string `yaml:"language"`
	TaskType   []string `yaml:"task_type"`
}

// AnalysisSpec configures statistical analysis thresholds.
type AnalysisSpec struct {
	SignificanceThreshold float64 `yaml:"significance_threshold"`
	ImprovementThreshold  float64 `yaml:"improvement_threshold"`
}

// OutputSpec controls where and how results are written.
type OutputSpec struct {
	Path    string   `yaml:"path"`
	Formats []string `yaml:"formats"`
}

// BoolPtr returns a pointer to the given bool value.
func BoolPtr(v bool) *bool { return &v }

// WithDefaults returns a copy of the config with zero-value fields filled
// with sensible defaults.
func (c *BenchConfig) WithDefaults() *BenchConfig {
	out := *c

	if out.Execution.Attempts == 0 {
		out.Execution.Attempts = 5
	}
	if out.Execution.Parallelism == 0 {
		out.Execution.Parallelism = 2
	}
	if out.Execution.Timeout == 0 {
		out.Execution.Timeout = 10 * time.Minute
	}
	if out.Execution.Baseline == nil {
		out.Execution.Baseline = BoolPtr(true)
	}
	if out.Execution.MaxTurns == 0 {
		out.Execution.MaxTurns = 25
	}
	if out.Docker.Image == "" {
		out.Docker.Image = "ghcr.io/mateosegura/dev-env:latest"
	}
	if out.Analysis.SignificanceThreshold == 0 {
		out.Analysis.SignificanceThreshold = 0.05
	}
	if out.Analysis.ImprovementThreshold == 0 {
		out.Analysis.ImprovementThreshold = 0.1
	}
	if len(out.Output.Formats) == 0 {
		out.Output.Formats = []string{"terminal"}
	}

	return &out
}

// Validate checks that all required fields are set and within valid ranges.
// It returns a combined error describing all violations found.
func (c *BenchConfig) Validate() error {
	var errs []error

	if c.Config.Path == "" {
		errs = append(errs, fmt.Errorf("config path must be non-empty"))
	}
	if c.Corpus.Path == "" {
		errs = append(errs, fmt.Errorf("corpus path must be non-empty"))
	}
	if c.Execution.Attempts < 1 {
		errs = append(errs, fmt.Errorf("execution attempts must be >= 1"))
	}
	if c.Execution.Parallelism < 1 {
		errs = append(errs, fmt.Errorf("execution parallelism must be >= 1"))
	}
	if c.Execution.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("execution timeout must be > 0"))
	}
	if c.Execution.MaxTurns < 1 {
		errs = append(errs, fmt.Errorf("execution max_turns must be >= 1"))
	}
	if c.Docker.Image == "" {
		errs = append(errs, fmt.Errorf("docker image must be non-empty"))
	}
	if c.Analysis.SignificanceThreshold <= 0.0 || c.Analysis.SignificanceThreshold >= 1.0 {
		errs = append(errs, fmt.Errorf("analysis significance_threshold must be in (0.0, 1.0)"))
	}
	if c.Analysis.ImprovementThreshold < 0 {
		errs = append(errs, fmt.Errorf("analysis improvement_threshold must be >= 0"))
	}

	return errors.Join(errs...)
}

// LoadConfig reads a YAML file, applies defaults, and validates the result.
func LoadConfig(path string) (*BenchConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg BenchConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config YAML: %w", err)
	}

	result := cfg.WithDefaults()

	if err := result.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return result, nil
}
