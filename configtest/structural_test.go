package configtest

import (
	"os"
	"strings"
	"testing"
)

func mustDiscover(t *testing.T) *ConfigInventory {
	t.Helper()
	dir := testConfigDir(t)
	inv, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	return inv
}

// --- Command Tests ---

func TestCommands_AllExist(t *testing.T) {
	inv := mustDiscover(t)
	if len(inv.Commands) == 0 {
		t.Skip("no commands discovered")
	}

	for _, cmd := range inv.Commands {
		t.Run(cmd.Name, func(t *testing.T) {
			if _, err := os.Stat(cmd.Path); err != nil {
				t.Errorf("command file does not exist: %s", cmd.Path)
			}
		})
	}
}

func TestCommands_HaveFrontmatter(t *testing.T) {
	inv := mustDiscover(t)
	if len(inv.Commands) == 0 {
		t.Skip("no commands discovered")
	}

	for _, cmd := range inv.Commands {
		t.Run(cmd.Name, func(t *testing.T) {
			if !strings.HasPrefix(cmd.Content, "---") {
				t.Errorf("missing opening --- frontmatter delimiter")
			}
			// Check for closing delimiter
			rest := cmd.Content[3:]
			if !strings.Contains(rest, "\n---") {
				t.Errorf("missing closing --- frontmatter delimiter")
			}
		})
	}
}

func TestCommands_HaveDescription(t *testing.T) {
	inv := mustDiscover(t)
	if len(inv.Commands) == 0 {
		t.Skip("no commands discovered")
	}

	for _, cmd := range inv.Commands {
		t.Run(cmd.Name, func(t *testing.T) {
			if cmd.Description == "" {
				t.Errorf("missing description in frontmatter")
			}
		})
	}
}

func TestCommands_HaveContentSections(t *testing.T) {
	inv := mustDiscover(t)
	if len(inv.Commands) == 0 {
		t.Skip("no commands discovered")
	}

	for _, cmd := range inv.Commands {
		t.Run(cmd.Name, func(t *testing.T) {
			if !strings.Contains(cmd.Body, "## ") {
				t.Errorf("missing any ## section heading — commands should have structured content")
			}
		})
	}
}

// --- Skill Tests ---

func TestSkills_AllHaveSkillMD(t *testing.T) {
	inv := mustDiscover(t)
	if len(inv.Skills) == 0 {
		t.Skip("no skills discovered")
	}

	for _, skill := range inv.Skills {
		t.Run(skill.RelDir, func(t *testing.T) {
			if _, err := os.Stat(skill.SkillMDPath); err != nil {
				t.Errorf("SKILL.md does not exist: %s", skill.SkillMDPath)
			}
		})
	}
}

func TestSkills_HaveFrontmatter(t *testing.T) {
	inv := mustDiscover(t)
	if len(inv.Skills) == 0 {
		t.Skip("no skills discovered")
	}

	for _, skill := range inv.Skills {
		t.Run(skill.RelDir, func(t *testing.T) {
			if !strings.HasPrefix(skill.Content, "---") {
				t.Errorf("missing opening --- frontmatter delimiter")
			}
			rest := skill.Content[3:]
			if !strings.Contains(rest, "\n---") {
				t.Errorf("missing closing --- frontmatter delimiter")
			}
		})
	}
}

func TestSkills_ContentNotEmpty(t *testing.T) {
	inv := mustDiscover(t)
	if len(inv.Skills) == 0 {
		t.Skip("no skills discovered")
	}

	for _, skill := range inv.Skills {
		t.Run(skill.RelDir, func(t *testing.T) {
			if len(skill.Content) < 100 {
				t.Errorf("SKILL.md too small (%d bytes, want >100)", len(skill.Content))
			}
		})
	}
}

func TestSkills_FollowNamingConvention(t *testing.T) {
	inv := mustDiscover(t)
	if len(inv.Skills) == 0 {
		t.Skip("no skills discovered")
	}

	for _, skill := range inv.Skills {
		t.Run(skill.RelDir, func(t *testing.T) {
			if !isValidSkillName(skill.Name) {
				t.Errorf("skill name %q doesn't follow naming convention (lowercase alphanumeric with hyphens)", skill.Name)
			}
		})
	}
}

// --- Settings Tests ---

func TestSettings_ValidJSON(t *testing.T) {
	inv := mustDiscover(t)
	if inv.Settings == nil {
		t.Skip("no settings.json found")
	}

	// Already validated during Discover, but verify raw is non-empty
	if len(inv.Settings.Raw) == 0 {
		t.Error("settings.json raw content is empty")
	}
}

func TestSettings_PluginsEnabled(t *testing.T) {
	inv := mustDiscover(t)
	if inv.Settings == nil {
		t.Skip("no settings.json found")
	}

	perms := inv.Settings.Parsed.Permissions
	if perms == nil {
		t.Skip("no permissions block in settings")
	}

	if len(perms.Allow) == 0 && len(perms.Deny) == 0 && len(perms.Ask) == 0 {
		t.Error("settings.json has no permission rules configured")
	}

	t.Logf("permissions: %d allow, %d deny, %d ask",
		len(perms.Allow), len(perms.Deny), len(perms.Ask))
}

// --- Cross-cutting Tests ---

func TestCommands_ReferencedBySkillsExist(t *testing.T) {
	inv := mustDiscover(t)
	if len(inv.Skills) == 0 {
		t.Skip("no skills discovered")
	}

	// Build set of known command names
	knownCmds := make(map[string]bool)
	for _, cmd := range inv.Commands {
		knownCmds[cmd.Name] = true
	}
	// Also include commands in subdirectories by their base name
	for _, cmd := range inv.Commands {
		parts := strings.Split(cmd.RelPath, string(os.PathSeparator))
		if len(parts) > 1 {
			knownCmds[parts[len(parts)-1]] = true
		}
	}

	for _, skill := range inv.Skills {
		t.Run(skill.RelDir, func(t *testing.T) {
			refs := ExtractCommandRefs(skill.Body)
			for _, ref := range refs {
				// Only flag refs that look like they should be commands
				// (skip common words that happen to match the pattern)
				if len(ref) < 3 {
					continue
				}
				if knownCmds[ref] {
					continue
				}
				// Log as info rather than fail — not all /refs are commands
				t.Logf("reference /%s in skill %q not found in commands/ (may be a CLI built-in)", ref, skill.Name)
			}
		})
	}
}
