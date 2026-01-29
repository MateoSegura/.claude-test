package configtest

import (
	"os"
	"path/filepath"
	"testing"
)

func testConfigDir(t *testing.T) string {
	t.Helper()
	dir := DefaultConfigDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skipf("config dir %q not found, set CLAUDE_CONFIG_DIR", dir)
	}
	return dir
}

func TestDiscover(t *testing.T) {
	dir := testConfigDir(t)

	inv, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover(%q): %v", dir, err)
	}

	if inv.Root == "" {
		t.Error("inventory Root is empty")
	}
	if !filepath.IsAbs(inv.Root) {
		t.Errorf("inventory Root is not absolute: %q", inv.Root)
	}
}

func TestDiscoverCommands(t *testing.T) {
	dir := testConfigDir(t)

	inv, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(inv.Commands) == 0 {
		t.Fatal("expected at least one command, got 0")
	}

	// Verify each command has required fields
	for _, cmd := range inv.Commands {
		if cmd.Name == "" {
			t.Errorf("command at %q has empty name", cmd.Path)
		}
		if cmd.Path == "" {
			t.Errorf("command %q has empty path", cmd.Name)
		}
		if cmd.Content == "" {
			t.Errorf("command %q has empty content", cmd.Name)
		}
	}

	t.Logf("discovered %d commands", len(inv.Commands))
	for _, cmd := range inv.Commands {
		t.Logf("  %s (%s)", cmd.Name, cmd.RelPath)
	}
}

func TestDiscoverSkills(t *testing.T) {
	dir := testConfigDir(t)

	inv, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	// Skills may be empty — that's valid
	t.Logf("discovered %d skills", len(inv.Skills))
	for _, skill := range inv.Skills {
		if skill.Name == "" {
			t.Errorf("skill at %q has empty name", skill.Dir)
		}
		if skill.SkillMDPath == "" {
			t.Errorf("skill %q has empty SkillMDPath", skill.Name)
		}
		t.Logf("  %s (%s)", skill.Name, skill.RelDir)
	}
}

func TestDiscoverSettings(t *testing.T) {
	dir := testConfigDir(t)

	inv, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if inv.Settings == nil {
		t.Skip("no settings.json found")
	}

	if inv.Settings.Path == "" {
		t.Error("settings path is empty")
	}
	if len(inv.Settings.Raw) == 0 {
		t.Error("settings raw JSON is empty")
	}

	// Verify permissions parsed
	if inv.Settings.Parsed.Permissions != nil {
		t.Logf("permissions: %d allow, %d deny, %d ask",
			len(inv.Settings.Parsed.Permissions.Allow),
			len(inv.Settings.Parsed.Permissions.Deny),
			len(inv.Settings.Parsed.Permissions.Ask))
	}
}

func TestDiscoverNonExistent(t *testing.T) {
	_, err := Discover("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for nonexistent directory, got nil")
	}
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantKey string
		wantVal string
		wantLen int
	}{
		{
			name:    "valid frontmatter",
			input:   "---\ndescription: Test command\nmodel: sonnet\n---\n# Body",
			wantKey: "description",
			wantVal: "Test command",
			wantLen: 2,
		},
		{
			name:    "no frontmatter",
			input:   "# Just a heading\nSome content",
			wantKey: "",
			wantVal: "",
			wantLen: 0,
		},
		{
			name:    "incomplete frontmatter",
			input:   "---\ndescription: Test\nNo closing delimiter",
			wantKey: "",
			wantVal: "",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, _ := parseFrontmatter(tt.input)
			if len(fm) != tt.wantLen {
				t.Errorf("got %d keys, want %d", len(fm), tt.wantLen)
			}
			if tt.wantKey != "" && fm[tt.wantKey] != tt.wantVal {
				t.Errorf("fm[%q] = %q, want %q", tt.wantKey, fm[tt.wantKey], tt.wantVal)
			}
		})
	}
}

func TestExtractCommandRefs(t *testing.T) {
	content := `Use /skill-learn to create skills.
Then run /domain-status to check coverage.
Don't confuse /skill-learn with /skill-refresh.`

	refs := ExtractCommandRefs(content)
	if len(refs) < 3 {
		t.Errorf("expected at least 3 refs, got %d: %v", len(refs), refs)
	}

	want := map[string]bool{
		"skill-learn":   true,
		"domain-status": true,
		"skill-refresh": true,
	}
	for _, ref := range refs {
		if !want[ref] {
			t.Logf("unexpected ref: %q", ref)
		}
	}
	for name := range want {
		found := false
		for _, ref := range refs {
			if ref == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected ref %q in %v", name, refs)
		}
	}
}
