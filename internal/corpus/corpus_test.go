package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fourIssueFixture = `name: test-corpus
version: "1.0"
issues:
  - id: easy-go-1
    title: Fix nil pointer in handler
    description: Handler crashes on nil input
    repo: org/repo-a
    ref: main
    difficulty: easy
    language: go
    task_type: bug_fix
    prompt: Fix the nil pointer dereference in handler.go
    evaluation:
      method: test_passes
      command: go test ./...

  - id: easy-go-2
    title: Fix off-by-one in loop
    description: Loop iterates one extra time
    repo: org/repo-a
    ref: main
    difficulty: easy
    language: go
    task_type: bug_fix
    prompt: Fix the off-by-one error in loop.go
    evaluation:
      method: command
      command: go test ./...

  - id: medium-ts-1
    title: Add pagination to API
    description: API needs cursor-based pagination
    repo: org/repo-b
    ref: develop
    difficulty: medium
    language: typescript
    task_type: feature
    prompt: Implement cursor-based pagination for the list endpoint
    evaluation:
      method: test_passes
      command: npm test

  - id: hard-py-1
    title: Refactor data pipeline
    description: Pipeline needs to be split into stages
    repo: org/repo-c
    ref: main
    difficulty: hard
    language: python
    task_type: refactor
    prompt: Refactor the monolithic pipeline into composable stages
    evaluation:
      method: llm_judge
`

func writeFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "corpus.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}
	return path
}

func TestLoadAndFilterByDifficulty(t *testing.T) {
	path := writeFixture(t, fourIssueFixture)

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(c.Issues) != 4 {
		t.Fatalf("expected 4 issues, got %d", len(c.Issues))
	}

	filtered := c.Filter(CorpusFilter{
		Difficulty: []string{"easy", "medium"},
	})

	if len(filtered.Issues) != 3 {
		t.Fatalf("expected 3 filtered issues, got %d", len(filtered.Issues))
	}

	for _, issue := range filtered.Issues {
		if issue.ID == "hard-py-1" {
			t.Error("filtered corpus should not contain the hard Python issue")
		}
	}
}

func TestValidateErrors(t *testing.T) {
	const fixture = `name: validate-test
version: "1.0"
issues:
  - id: missing-prompt
    title: Some title
    description: Missing prompt field
    repo: org/repo
    ref: main
    difficulty: easy
    language: go
    task_type: bug_fix
    evaluation:
      method: command
      command: go test ./...

  - id: missing-eval
    title: Another title
    description: Missing evaluation block
    repo: org/repo
    ref: main
    difficulty: easy
    language: go
    task_type: bug_fix
    prompt: Do something
`

	path := writeFixture(t, fixture)

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	err = c.Validate()
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	if !strings.Contains(err.Error(), "missing-prompt") {
		t.Errorf("expected error to mention 'missing-prompt', got: %v", err)
	}
}

func TestFilterPreservesOriginal(t *testing.T) {
	const fixture = `name: preserve-test
version: "1.0"
issues:
  - id: go-feature
    title: Add caching
    description: Add response caching
    repo: org/repo
    ref: main
    difficulty: easy
    language: go
    task_type: feature
    prompt: Add an in-memory cache
    evaluation:
      method: test_passes
      command: go test ./...

  - id: go-bugfix
    title: Fix race condition
    description: Race in concurrent handler
    repo: org/repo
    ref: main
    difficulty: medium
    language: go
    task_type: bug_fix
    prompt: Fix the race condition
    evaluation:
      method: command
      command: go test -race ./...

  - id: py-feature
    title: Add logging
    description: Add structured logging
    repo: org/repo-py
    ref: main
    difficulty: easy
    language: python
    task_type: feature
    prompt: Add structured logging with structlog
    evaluation:
      method: command
      command: pytest
`

	path := writeFixture(t, fixture)

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(c.Issues) != 3 {
		t.Fatalf("expected 3 issues in original, got %d", len(c.Issues))
	}

	filtered := c.Filter(CorpusFilter{
		Language: []string{"go"},
		TaskType: []string{"feature"},
	})

	if len(filtered.Issues) != 1 {
		t.Fatalf("expected 1 filtered issue, got %d", len(filtered.Issues))
	}

	if filtered.Issues[0].ID != "go-feature" {
		t.Errorf("expected filtered issue to be 'go-feature', got %q", filtered.Issues[0].ID)
	}

	if len(c.Issues) != 3 {
		t.Errorf("original corpus was modified: expected 3 issues, got %d", len(c.Issues))
	}
}

func TestLoadDirectoryError(t *testing.T) {
	dir := t.TempDir()

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when loading a directory, got nil")
	}

	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("expected error to mention 'directory', got: %v", err)
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("/nonexistent/path/corpus.yaml")
	if err == nil {
		t.Fatal("expected error for non-existent path, got nil")
	}
}

func TestFilterEmptyFilter(t *testing.T) {
	path := writeFixture(t, fourIssueFixture)

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	filtered := c.Filter(CorpusFilter{})

	if len(filtered.Issues) != len(c.Issues) {
		t.Errorf("empty filter should return all %d issues, got %d", len(c.Issues), len(filtered.Issues))
	}
}
