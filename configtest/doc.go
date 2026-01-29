// Package configtest validates Claude Code .claude configuration directories.
//
// It provides auto-discovery of commands, skills, and settings, plus a three-layer
// test framework that runs from fast structural checks to live CLI invocations.
//
// # Test Layers
//
// Structural (no build tag, <1s):
//
//	Validates file existence, YAML frontmatter, JSON validity, and naming conventions.
//
// Dry-Run (no build tag, <2s):
//
//	Exercises test infrastructure with DryRun=true. Validates that discovery and
//	structural assertions work end-to-end without external dependencies.
//
// Live (build tag "live", minutes):
//
//	Invokes the real Claude CLI to validate commands and skills produce expected
//	output. Costs API credits. Enable with:
//	  go test -tags live ./configtest/...
//
// # Usage
//
//	go test ./configtest/...              # structural + dry-run (fast, always)
//	go test -tags live ./configtest/...   # + live Claude invocation (slow, costs $)
//
// # Configuration
//
// By default, the config directory is resolved as ../.claude relative to the test's
// working directory (the configtest/ package dir). Override with CLAUDE_CONFIG_DIR:
//
//	CLAUDE_CONFIG_DIR=/path/to/.claude go test ./configtest/...
//
// # Importing
//
// This package is intentionally public. Other projects can import the discovery
// engine and assertion helpers to validate their own .claude configurations:
//
//	import "github.com/MateoSegura/.claude-test/configtest"
//
//	func TestMyConfig(t *testing.T) {
//	    inv, err := configtest.Discover("/path/to/my/.claude")
//	    if err != nil {
//	        t.Fatal(err)
//	    }
//	    configtest.AssertCommandsValid(t, inv)
//	    configtest.AssertSkillsValid(t, inv)
//	    configtest.AssertSettingsValid(t, inv)
//	}
package configtest
