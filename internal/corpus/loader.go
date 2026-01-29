package corpus

import (
	"fmt"
	"os"
	"slices"

	"gopkg.in/yaml.v3"
)

// validEvalMethods enumerates the allowed evaluation methods.
var validEvalMethods = []string{
	"command",
	"file_contains",
	"test_passes",
	"llm_judge",
}

// Load reads a YAML file at path and unmarshals it into a Corpus.
func Load(path string) (*Corpus, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("corpus load: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("corpus load: %s is a directory, not a file", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("corpus load: %w", err)
	}

	var c Corpus
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("corpus load: invalid YAML: %w", err)
	}

	return &c, nil
}

// Validate checks that every issue has the required fields and a valid
// evaluation method. It returns an error naming the first offending issue ID.
func (c *Corpus) Validate() error {
	for _, issue := range c.Issues {
		id := issue.ID
		if id == "" {
			id = "(empty id)"
		}

		if issue.ID == "" {
			return fmt.Errorf("validate issue %s: missing required field 'id'", id)
		}
		if issue.Title == "" {
			return fmt.Errorf("validate issue %s: missing required field 'title'", id)
		}
		if issue.Repo == "" {
			return fmt.Errorf("validate issue %s: missing required field 'repo'", id)
		}
		if issue.Prompt == "" {
			return fmt.Errorf("validate issue %s: missing required field 'prompt'", id)
		}
		if issue.Evaluation.Method == "" {
			return fmt.Errorf("validate issue %s: missing required field 'evaluation.method'", id)
		}
		if !slices.Contains(validEvalMethods, issue.Evaluation.Method) {
			return fmt.Errorf("validate issue %s: invalid evaluation method %q (must be one of %v)", id, issue.Evaluation.Method, validEvalMethods)
		}
	}
	return nil
}

// Filter returns a new Corpus containing only the issues that match the filter.
// OR semantics within each field, AND across fields. Empty/nil filter slices
// match everything. The original corpus is not modified.
func (c *Corpus) Filter(f CorpusFilter) *Corpus {
	filtered := &Corpus{
		Name:    c.Name,
		Version: c.Version,
	}

	for _, issue := range c.Issues {
		if matchesFilter(issue, f) {
			filtered.Issues = append(filtered.Issues, issue)
		}
	}

	return filtered
}

func matchesFilter(issue Issue, f CorpusFilter) bool {
	if len(f.Difficulty) > 0 && !slices.Contains(f.Difficulty, issue.Difficulty) {
		return false
	}
	if len(f.Language) > 0 && !slices.Contains(f.Language, issue.Language) {
		return false
	}
	if len(f.TaskType) > 0 && !slices.Contains(f.TaskType, issue.TaskType) {
		return false
	}
	return true
}
