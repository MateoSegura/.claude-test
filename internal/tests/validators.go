package tests

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	claude "github.com/MateoSegura/claudesdk-go"
)

// ContainsText checks if output contains specific text.
func ContainsText(text string) Validator {
	return func(output string, _ *TestResult) Validation {
		found := strings.Contains(output, text)
		return Validation{
			Name:    fmt.Sprintf("contains: %s", truncate(text, 30)),
			Passed:  found,
			Score:   boolToScore(found),
			Message: fmt.Sprintf("Looking for '%s': %v", truncate(text, 50), found),
		}
	}
}

// MatchesRegex checks if output matches a regex pattern.
func MatchesRegex(pattern string) Validator {
	return func(output string, _ *TestResult) Validation {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return Validation{
				Name:    fmt.Sprintf("regex: %s", truncate(pattern, 30)),
				Passed:  false,
				Score:   0.0,
				Message: fmt.Sprintf("Invalid regex: %v", err),
			}
		}

		found := re.MatchString(output)
		return Validation{
			Name:    fmt.Sprintf("regex: %s", truncate(pattern, 30)),
			Passed:  found,
			Score:   boolToScore(found),
			Message: fmt.Sprintf("Pattern match: %v", found),
		}
	}
}

// ContainsCode checks for code blocks in the output.
func ContainsCode(lang string) Validator {
	return func(output string, _ *TestResult) Validation {
		pattern := fmt.Sprintf("```%s", lang)
		found := strings.Contains(output, pattern)
		return Validation{
			Name:    fmt.Sprintf("code block: %s", lang),
			Passed:  found,
			Score:   boolToScore(found),
			Message: fmt.Sprintf("Found %s code block: %v", lang, found),
		}
	}
}

// FileCreated checks if a file was created (mentioned in tool calls).
func FileCreated(filename string) Validator {
	return func(output string, _ *TestResult) Validation {
		// Look for Write tool usage with this filename
		pattern := fmt.Sprintf(`(?i)(Write|created|wrote).*%s`, regexp.QuoteMeta(filename))
		re := regexp.MustCompile(pattern)
		found := re.MatchString(output)
		return Validation{
			Name:    fmt.Sprintf("file created: %s", filename),
			Passed:  found,
			Score:   boolToScore(found),
			Message: fmt.Sprintf("File %s creation: %v", filename, found),
		}
	}
}

// RuleFollowed checks if a specific rule was followed.
func RuleFollowed(ruleID, description string) Validator {
	return func(output string, _ *TestResult) Validation {
		return Validation{
			Name:    fmt.Sprintf("rule: %s", ruleID),
			Passed:  true, // Default to true, specific rules override
			Score:   1.0,
			Message: fmt.Sprintf("Rule %s: %s - check manually", ruleID, description),
		}
	}
}

// NoErrors checks that output doesn't contain error indicators.
func NoErrors() Validator {
	return func(output string, _ *TestResult) Validation {
		errorPatterns := []string{
			"error:",
			"Error:",
			"ERROR",
			"failed:",
			"Failed:",
			"FAILED",
			"panic:",
			"exception:",
		}

		for _, pattern := range errorPatterns {
			if strings.Contains(output, pattern) {
				return Validation{
					Name:    "no errors",
					Passed:  false,
					Score:   0.0,
					Message: fmt.Sprintf("Found error indicator: %s", pattern),
				}
			}
		}

		return Validation{
			Name:    "no errors",
			Passed:  true,
			Score:   1.0,
			Message: "No error indicators found",
		}
	}
}

// OutputLength checks output is within expected length range.
func OutputLength(minLen, maxLen int) Validator {
	return func(output string, _ *TestResult) Validation {
		length := len(output)
		passed := length >= minLen && length <= maxLen
		return Validation{
			Name:    fmt.Sprintf("length: %d-%d", minLen, maxLen),
			Passed:  passed,
			Score:   boolToScore(passed),
			Message: fmt.Sprintf("Output length %d (expected %d-%d)", length, minLen, maxLen),
		}
	}
}

// CustomValidator wraps a custom validation function.
func CustomValidator(name string, fn func(output string) (bool, string)) Validator {
	return func(output string, _ *TestResult) Validation {
		passed, msg := fn(output)
		return Validation{
			Name:    name,
			Passed:  passed,
			Score:   boolToScore(passed),
			Message: msg,
		}
	}
}

// LLMValidator uses Claude to evaluate if output meets criteria.
// This enables nuanced validation that regex/string matching can't handle.
func LLMValidator(name, criteria string) Validator {
	return func(output string, _ *TestResult) Validation {
		// Build the judge prompt
		judgePrompt := fmt.Sprintf(`You are evaluating test output. Answer only YES or NO, followed by a brief reason.

CRITERIA: %s

OUTPUT TO EVALUATE:
%s

Does the output meet the criteria? (YES/NO + reason)`, criteria, truncate(output, 4000))

		// Call Claude to judge using SDK
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		session, err := claude.NewSession(claude.SessionConfig{
			LaunchOptions: claude.LaunchOptions{
				SkipPermissions: true,
				Timeout:         30 * time.Second,
			},
		})
		if err != nil {
			return Validation{
				Name:    name,
				Passed:  false,
				Score:   0.0,
				Message: fmt.Sprintf("LLM judge session error: %v", err),
			}
		}

		response, err := session.CollectAll(ctx, judgePrompt)
		if err != nil {
			return Validation{
				Name:    name,
				Passed:  false,
				Score:   0.0,
				Message: fmt.Sprintf("LLM judge error: %v", err),
			}
		}

		response = strings.TrimSpace(response)
		passed := strings.HasPrefix(strings.ToUpper(response), "YES")

		return Validation{
			Name:    name,
			Passed:  passed,
			Score:   boolToScore(passed),
			Message: truncate(response, 200),
		}
	}
}

func boolToScore(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
