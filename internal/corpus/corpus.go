package corpus

// Corpus represents a collection of benchmark issues loaded from YAML.
type Corpus struct {
	Name    string  `yaml:"name"`
	Version string  `yaml:"version"`
	Issues  []Issue `yaml:"issues"`
}

// Issue represents a single benchmark task.
type Issue struct {
	ID            string     `yaml:"id"`
	Title         string     `yaml:"title"`
	Description   string     `yaml:"description"`
	Repo          string     `yaml:"repo"`
	Ref           string     `yaml:"ref"`
	Difficulty    string     `yaml:"difficulty"`
	Language      string     `yaml:"language"`
	TaskType      string     `yaml:"task_type"`
	Prompt        string     `yaml:"prompt"`
	MaxTurns      int        `yaml:"max_turns,omitempty"`
	SetupCmds     []string   `yaml:"setup_commands,omitempty"`
	Evaluation    Evaluation `yaml:"evaluation"`
	ExpectedTools []string   `yaml:"expected_tools,omitempty"`
}

// Evaluation defines how an issue's resolution is verified.
type Evaluation struct {
	Method           string `yaml:"method"`
	Command          string `yaml:"command,omitempty"`
	ExpectedExitCode int    `yaml:"expected_exit_code,omitempty"`
	FilePath         string `yaml:"file_path,omitempty"`
	Contains         string `yaml:"contains,omitempty"`
}

// CorpusFilter controls which issues are included after filtering.
// OR semantics within each field, AND across fields.
// Empty/nil slices match everything.
type CorpusFilter struct {
	Difficulty []string
	Language   []string
	TaskType   []string
}
