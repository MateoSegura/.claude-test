package configtest

import (
	"testing"
)

// TestDryRun_Discovery verifies the full discovery pipeline produces
// consistent results across multiple invocations.
func TestDryRun_Discovery(t *testing.T) {
	dir := testConfigDir(t)

	inv1, err := Discover(dir)
	if err != nil {
		t.Fatalf("first Discover: %v", err)
	}
	inv2, err := Discover(dir)
	if err != nil {
		t.Fatalf("second Discover: %v", err)
	}

	if len(inv1.Commands) != len(inv2.Commands) {
		t.Errorf("command count changed: %d vs %d", len(inv1.Commands), len(inv2.Commands))
	}
	if len(inv1.Skills) != len(inv2.Skills) {
		t.Errorf("skill count changed: %d vs %d", len(inv1.Skills), len(inv2.Skills))
	}
}

// TestDryRun_CommandAssertions exercises AssertCommandsValid in dry-run
// mode against the real config without any Claude CLI invocation.
func TestDryRun_CommandAssertions(t *testing.T) {
	inv := mustDiscover(t)
	if len(inv.Commands) == 0 {
		t.Skip("no commands discovered")
	}

	// Run the exported assertion helpers — they should pass on a valid config
	AssertCommandsValid(t, inv)
}

// TestDryRun_SkillAssertions exercises AssertSkillsValid in dry-run mode.
func TestDryRun_SkillAssertions(t *testing.T) {
	inv := mustDiscover(t)
	if len(inv.Skills) == 0 {
		t.Skip("no skills discovered")
	}

	AssertSkillsValid(t, inv)
}

// TestDryRun_SettingsAssertions exercises AssertSettingsValid in dry-run mode.
func TestDryRun_SettingsAssertions(t *testing.T) {
	inv := mustDiscover(t)
	AssertSettingsValid(t, inv)
}

// TestDryRun_CommandSubtests verifies that each discovered command
// generates a named subtest, enabling auto-discovery of new commands.
func TestDryRun_CommandSubtests(t *testing.T) {
	inv := mustDiscover(t)

	for _, cmd := range inv.Commands {
		t.Run(cmd.Name, func(t *testing.T) {
			// Verify basic invariants hold for each command
			if cmd.Path == "" {
				t.Error("command has empty path")
			}
			if cmd.Content == "" {
				t.Error("command has empty content")
			}
			if len(cmd.Frontmatter) == 0 {
				t.Errorf("command %q has no frontmatter", cmd.Name)
			}
			t.Logf("command %q: description=%q, %d bytes",
				cmd.Name, cmd.Description, len(cmd.Content))
		})
	}
}

// TestDryRun_SkillSubtests verifies that each discovered skill
// generates a named subtest, enabling auto-discovery of new skills.
func TestDryRun_SkillSubtests(t *testing.T) {
	inv := mustDiscover(t)
	if len(inv.Skills) == 0 {
		t.Skip("no skills discovered")
	}

	for _, skill := range inv.Skills {
		t.Run(skill.RelDir, func(t *testing.T) {
			if skill.SkillMDPath == "" {
				t.Error("skill has empty SkillMDPath")
			}
			if skill.Content == "" {
				t.Error("skill has empty content")
			}
			if len(skill.Frontmatter) == 0 {
				t.Errorf("skill %q has no frontmatter", skill.Name)
			}
			t.Logf("skill %q: description=%q, %d bytes",
				skill.Name, skill.Description, len(skill.Content))
		})
	}
}

// TestDryRun_InventoryCompleteness checks that the inventory covers
// all expected sections of the config directory.
func TestDryRun_InventoryCompleteness(t *testing.T) {
	inv := mustDiscover(t)

	t.Logf("inventory summary:")
	t.Logf("  root: %s", inv.Root)
	t.Logf("  commands: %d", len(inv.Commands))
	t.Logf("  skills: %d", len(inv.Skills))

	if inv.Settings != nil {
		t.Logf("  settings: present (%s)", inv.Settings.Path)
	} else {
		t.Logf("  settings: absent")
	}

	// At minimum, a valid config should have either commands or settings
	if len(inv.Commands) == 0 && inv.Settings == nil {
		t.Error("config has neither commands nor settings — is CLAUDE_CONFIG_DIR correct?")
	}
}
