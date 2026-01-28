package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// BenchmarkResult contains the complete results of a benchmark run.
type BenchmarkResult struct {
	Timestamp     time.Time                `json:"timestamp"`
	CorpusName    string                   `json:"corpus_name"`
	CorpusVersion string                   `json:"corpus_version"`
	Duration      time.Duration            `json:"duration"`
	ConfigResults map[string]*ConfigResult `json:"config_results"`
}

// ConfigResult contains results for a single configuration.
type ConfigResult struct {
	ConfigName   string         `json:"config_name"`
	IssueResults []*IssueResult `json:"issue_results"`

	// Aggregate stats
	TotalIssues   int           `json:"total_issues"`
	SuccessCount  int           `json:"success_count"`
	SuccessRate   float64       `json:"success_rate"`
	AverageScore  float64       `json:"average_score"`
	TotalDuration time.Duration `json:"total_duration"`

	// Breakdown by difficulty
	ByDifficulty map[Difficulty]*DifficultyStats `json:"by_difficulty"`

	// Breakdown by task type
	ByTaskType map[TaskType]*TaskTypeStats `json:"by_task_type"`

	// Breakdown by language
	ByLanguage map[string]*LanguageStats `json:"by_language"`
}

// IssueResult contains the result for a single issue.
type IssueResult struct {
	IssueID      string        `json:"issue_id"`
	ConfigName   string        `json:"config_name"`
	Difficulty   Difficulty    `json:"difficulty"`
	TaskType     TaskType      `json:"task_type"`
	Language     string        `json:"language"`
	Success      bool          `json:"success"`
	Score        float64       `json:"score"`
	ClaudeOutput string        `json:"claude_output"`
	EvalDetails  string        `json:"eval_details"`
	Error        string        `json:"error,omitempty"`
	Duration     time.Duration `json:"duration"`
	WorkDir      string        `json:"work_dir"` // For debugging
}

// DifficultyStats contains aggregate stats by difficulty.
type DifficultyStats struct {
	Total       int     `json:"total"`
	Successes   int     `json:"successes"`
	SuccessRate float64 `json:"success_rate"`
	AvgScore    float64 `json:"avg_score"`
}

// TaskTypeStats contains aggregate stats by task type.
type TaskTypeStats struct {
	Total       int     `json:"total"`
	Successes   int     `json:"successes"`
	SuccessRate float64 `json:"success_rate"`
	AvgScore    float64 `json:"avg_score"`
}

// LanguageStats contains aggregate stats by language.
type LanguageStats struct {
	Total       int     `json:"total"`
	Successes   int     `json:"successes"`
	SuccessRate float64 `json:"success_rate"`
	AvgScore    float64 `json:"avg_score"`
}

// calculateStats computes aggregate statistics for a config result.
func (cr *ConfigResult) calculateStats() {
	cr.TotalIssues = len(cr.IssueResults)
	cr.ByDifficulty = make(map[Difficulty]*DifficultyStats)
	cr.ByTaskType = make(map[TaskType]*TaskTypeStats)
	cr.ByLanguage = make(map[string]*LanguageStats)

	var totalScore float64
	var totalDuration time.Duration

	for _, ir := range cr.IssueResults {
		if ir.Success {
			cr.SuccessCount++
		}
		totalScore += ir.Score
		totalDuration += ir.Duration

		// By difficulty
		if cr.ByDifficulty[ir.Difficulty] == nil {
			cr.ByDifficulty[ir.Difficulty] = &DifficultyStats{}
		}
		ds := cr.ByDifficulty[ir.Difficulty]
		ds.Total++
		if ir.Success {
			ds.Successes++
		}
		ds.AvgScore = (ds.AvgScore*float64(ds.Total-1) + ir.Score) / float64(ds.Total)
		ds.SuccessRate = float64(ds.Successes) / float64(ds.Total)

		// By task type
		if cr.ByTaskType[ir.TaskType] == nil {
			cr.ByTaskType[ir.TaskType] = &TaskTypeStats{}
		}
		ts := cr.ByTaskType[ir.TaskType]
		ts.Total++
		if ir.Success {
			ts.Successes++
		}
		ts.AvgScore = (ts.AvgScore*float64(ts.Total-1) + ir.Score) / float64(ts.Total)
		ts.SuccessRate = float64(ts.Successes) / float64(ts.Total)

		// By language
		if cr.ByLanguage[ir.Language] == nil {
			cr.ByLanguage[ir.Language] = &LanguageStats{}
		}
		ls := cr.ByLanguage[ir.Language]
		ls.Total++
		if ir.Success {
			ls.Successes++
		}
		ls.AvgScore = (ls.AvgScore*float64(ls.Total-1) + ir.Score) / float64(ls.Total)
		ls.SuccessRate = float64(ls.Successes) / float64(ls.Total)
	}

	if cr.TotalIssues > 0 {
		cr.SuccessRate = float64(cr.SuccessCount) / float64(cr.TotalIssues)
		cr.AverageScore = totalScore / float64(cr.TotalIssues)
	}
	cr.TotalDuration = totalDuration
}

// Comparison compares two config results.
type Comparison struct {
	BaselineConfig string  `json:"baseline_config"`
	TestConfig     string  `json:"test_config"`
	BaselineRate   float64 `json:"baseline_rate"`
	TestRate       float64 `json:"test_rate"`
	Improvement    float64 `json:"improvement"` // Percentage points
	Significant    bool    `json:"significant"` // Basic significance check
}

// Compare generates a comparison between two configs.
func (br *BenchmarkResult) Compare(baselineName, testName string) *Comparison {
	baseline := br.ConfigResults[baselineName]
	test := br.ConfigResults[testName]

	if baseline == nil || test == nil {
		return nil
	}

	improvement := (test.SuccessRate - baseline.SuccessRate) * 100

	return &Comparison{
		BaselineConfig: baselineName,
		TestConfig:     testName,
		BaselineRate:   baseline.SuccessRate,
		TestRate:       test.SuccessRate,
		Improvement:    improvement,
		Significant:    abs(improvement) >= 5, // Basic threshold
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// PrintReport prints a human-readable report.
func (br *BenchmarkResult) PrintReport() {
	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Printf("BENCHMARK REPORT: %s\n", br.CorpusName)
	fmt.Printf("Version: %s | Run: %s | Duration: %s\n",
		br.CorpusVersion, br.Timestamp.Format("2006-01-02 15:04"), br.Duration.Round(time.Second))
	fmt.Println(strings.Repeat("=", 61))

	// Summary table
	fmt.Println("\n## Summary by Configuration")
	fmt.Printf("%-20s %8s %8s %10s\n", "Config", "Success", "Score", "Duration")
	fmt.Println(strings.Repeat("-", 50))

	for name, cr := range br.ConfigResults {
		fmt.Printf("%-20s %7.0f%% %7.0f%% %10s\n",
			truncateName(name, 20),
			cr.SuccessRate*100,
			cr.AverageScore*100,
			cr.TotalDuration.Round(time.Second))
	}

	// By difficulty breakdown
	fmt.Println("\n## Success Rate by Difficulty")
	fmt.Printf("%-20s %10s %10s %10s\n", "Config", "Easy", "Medium", "Hard")
	fmt.Println(strings.Repeat("-", 55))

	for name, cr := range br.ConfigResults {
		easy := getRate(cr.ByDifficulty[DifficultyEasy])
		medium := getRate(cr.ByDifficulty[DifficultyMedium])
		hard := getRate(cr.ByDifficulty[DifficultyHard])
		fmt.Printf("%-20s %9.0f%% %9.0f%% %9.0f%%\n",
			truncateName(name, 20), easy*100, medium*100, hard*100)
	}

	// Comparison if we have baseline
	if baseline, ok := br.ConfigResults["baseline"]; ok {
		fmt.Println("\n## Improvement vs Baseline")
		for name, cr := range br.ConfigResults {
			if name == "baseline" {
				continue
			}
			improvement := (cr.SuccessRate - baseline.SuccessRate) * 100
			sign := "+"
			if improvement < 0 {
				sign = ""
			}
			fmt.Printf("  %s: %s%.1f percentage points\n", name, sign, improvement)
		}
	}

	fmt.Println()
}

func truncateName(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func getRate(ds *DifficultyStats) float64 {
	if ds == nil {
		return 0
	}
	return ds.SuccessRate
}

// SaveReport saves the report as JSON.
func (br *BenchmarkResult) SaveReport(path string) error {
	data, err := json.MarshalIndent(br, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadResult loads a benchmark result from JSON.
func LoadResult(path string) (*BenchmarkResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result BenchmarkResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
