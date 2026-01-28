// Package benchmark provides a framework for testing .claude configurations
// against real-world GitHub repositories and issues.
//
// This is a meta-evaluation system: instead of testing if Claude follows
// extension instructions, it tests if your .claude configuration actually
// makes Claude more effective at solving real problems.
package benchmark

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Difficulty categorizes problem complexity.
type Difficulty string

const (
	DifficultyEasy   Difficulty = "easy"
	DifficultyMedium Difficulty = "medium"
	DifficultyHard   Difficulty = "hard"
)

// TaskType categorizes what Claude needs to do.
type TaskType string

const (
	TaskBugFix        TaskType = "bug_fix"
	TaskFeature       TaskType = "feature"
	TaskRefactor      TaskType = "refactor"
	TaskTest          TaskType = "test"
	TaskDocumentation TaskType = "documentation"
)

// EvalMethod defines how to verify the solution.
type EvalMethod string

const (
	EvalTestSuite   EvalMethod = "test_suite"   // Run existing tests
	EvalLLMJudge    EvalMethod = "llm_judge"    // Claude evaluates
	EvalCustomCheck EvalMethod = "custom_check" // Custom validation script
	EvalHybrid      EvalMethod = "hybrid"       // Tests + LLM
)

// Issue represents a single benchmark task from a GitHub repo.
type Issue struct {
	// Identity
	ID          string     `yaml:"id"`          // Unique identifier (e.g., "express-auth-bug-001")
	Title       string     `yaml:"title"`       // Human-readable title
	Description string     `yaml:"description"` // What needs to be done
	Difficulty  Difficulty `yaml:"difficulty"`
	TaskType    TaskType   `yaml:"task_type"`
	Language    string     `yaml:"language"` // Primary language (go, typescript, python)

	// Source
	RepoURL  string `yaml:"repo_url"`  // GitHub URL to clone
	RepoRef  string `yaml:"repo_ref"`  // Branch/tag/commit to checkout
	IssueURL string `yaml:"issue_url"` // Optional: link to actual GitHub issue

	// The actual prompt given to Claude
	Prompt string `yaml:"prompt"`

	// Evaluation
	EvalMethod EvalMethod `yaml:"eval_method"`

	// For test_suite evaluation
	TestCommand string `yaml:"test_command,omitempty"` // e.g., "go test ./..."

	// For llm_judge evaluation
	SuccessCriteria string `yaml:"success_criteria,omitempty"` // What defines success

	// For custom_check evaluation
	CheckScript string `yaml:"check_script,omitempty"` // Script to run for validation

	// Files that should be modified (for partial credit)
	ExpectedFiles []string `yaml:"expected_files,omitempty"`

	// Optional: files to focus on (passed as context)
	ContextFiles []string `yaml:"context_files,omitempty"`

	// Tags for filtering
	Tags []string `yaml:"tags,omitempty"`
}

// Corpus is a collection of benchmark issues.
type Corpus struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Version     string   `yaml:"version"`
	Issues      []*Issue `yaml:"issues"`
}

// LoadCorpus loads a corpus from a YAML file.
func LoadCorpus(path string) (*Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read corpus: %w", err)
	}

	var corpus Corpus
	if err := yaml.Unmarshal(data, &corpus); err != nil {
		return nil, fmt.Errorf("parse corpus: %w", err)
	}

	return &corpus, nil
}

// LoadCorpusDir loads all corpus files from a directory.
func LoadCorpusDir(dir string) (*Corpus, error) {
	combined := &Corpus{
		Name:        "combined",
		Description: "Combined corpus from multiple files",
		Version:     "1.0.0",
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("glob corpus: %w", err)
	}

	for _, file := range files {
		corpus, err := LoadCorpus(file)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", file, err)
		}
		combined.Issues = append(combined.Issues, corpus.Issues...)
	}

	return combined, nil
}

// Filter returns issues matching the given criteria.
func (c *Corpus) Filter(difficulty *Difficulty, taskType *TaskType, language *string, tags []string) []*Issue {
	var result []*Issue

	for _, issue := range c.Issues {
		if difficulty != nil && issue.Difficulty != *difficulty {
			continue
		}
		if taskType != nil && issue.TaskType != *taskType {
			continue
		}
		if language != nil && issue.Language != *language {
			continue
		}
		if len(tags) > 0 && !hasAnyTag(issue.Tags, tags) {
			continue
		}
		result = append(result, issue)
	}

	return result
}

func hasAnyTag(issueTags, filterTags []string) bool {
	tagSet := make(map[string]bool)
	for _, t := range issueTags {
		tagSet[t] = true
	}
	for _, t := range filterTags {
		if tagSet[t] {
			return true
		}
	}
	return false
}

// Stats returns corpus statistics.
func (c *Corpus) Stats() CorpusStats {
	stats := CorpusStats{
		Total:        len(c.Issues),
		ByDifficulty: make(map[Difficulty]int),
		ByTaskType:   make(map[TaskType]int),
		ByLanguage:   make(map[string]int),
	}

	for _, issue := range c.Issues {
		stats.ByDifficulty[issue.Difficulty]++
		stats.ByTaskType[issue.TaskType]++
		stats.ByLanguage[issue.Language]++
	}

	return stats
}

// CorpusStats holds aggregate statistics about a corpus.
type CorpusStats struct {
	Total        int
	ByDifficulty map[Difficulty]int
	ByTaskType   map[TaskType]int
	ByLanguage   map[string]int
}
