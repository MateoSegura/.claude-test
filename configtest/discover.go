package configtest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// DefaultConfigDir returns the path to the .claude configuration directory.
//
// Resolution order:
//  1. CLAUDE_CONFIG_DIR environment variable (if set)
//  2. ../.claude relative to the current working directory (go test runs in the package dir)
func DefaultConfigDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	return filepath.Join("..", ".claude")
}

// ConfigInventory holds the discovered contents of a .claude configuration directory.
type ConfigInventory struct {
	Root     string
	Commands []CommandInfo
	Skills   []SkillInfo
	Settings *SettingsInfo
}

// CommandInfo describes a discovered command file.
type CommandInfo struct {
	Name        string            // command name derived from filename (e.g., "skill-learn")
	Path        string            // absolute path to the .md file
	RelPath     string            // path relative to commands/ (e.g., "stack/cloud.md")
	Description string            // from YAML frontmatter
	Frontmatter map[string]string // all frontmatter key-value pairs
	Content     string            // full file content including frontmatter
	Body        string            // content after frontmatter
}

// SkillInfo describes a discovered skill directory.
type SkillInfo struct {
	Name        string            // skill directory name (e.g., "go")
	Dir         string            // absolute path to the skill directory
	RelDir      string            // path relative to skills/ (e.g., "cloud/implement/go")
	SkillMDPath string            // absolute path to SKILL.md
	Description string            // from YAML frontmatter
	Frontmatter map[string]string // all frontmatter key-value pairs
	Content     string            // SKILL.md content
	Body        string            // SKILL.md content after frontmatter
}

// SettingsInfo describes the parsed settings.json.
type SettingsInfo struct {
	Path   string
	Raw    json.RawMessage
	Parsed SettingsJSON
}

// SettingsJSON represents the structure of settings.json.
type SettingsJSON struct {
	Permissions *PermissionsBlock      `json:"permissions,omitempty"`
	Hooks       map[string]interface{} `json:"hooks,omitempty"`
	Env         map[string]string      `json:"env,omitempty"`
}

// PermissionsBlock holds allow/deny/ask permission lists.
type PermissionsBlock struct {
	Allow []string `json:"allow,omitempty"`
	Deny  []string `json:"deny,omitempty"`
	Ask   []string `json:"ask,omitempty"`
}

// Discover scans a .claude configuration directory and returns a ConfigInventory.
//
// It discovers:
//   - Commands from commands/**/*.md
//   - Skills from skills/**/SKILL.md
//   - Settings from settings.json
func Discover(configDir string) (*ConfigInventory, error) {
	absDir, err := filepath.Abs(configDir)
	if err != nil {
		return nil, fmt.Errorf("configtest: resolve path %q: %w", configDir, err)
	}

	info, err := os.Stat(absDir)
	if err != nil {
		return nil, fmt.Errorf("configtest: config dir %q: %w", absDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("configtest: %q is not a directory", absDir)
	}

	inv := &ConfigInventory{Root: absDir}

	if err := discoverCommands(absDir, inv); err != nil {
		return nil, err
	}
	if err := discoverSkills(absDir, inv); err != nil {
		return nil, err
	}
	if err := discoverSettings(absDir, inv); err != nil {
		return nil, err
	}

	return inv, nil
}

func discoverCommands(root string, inv *ConfigInventory) error {
	cmdDir := filepath.Join(root, "commands")
	if _, err := os.Stat(cmdDir); os.IsNotExist(err) {
		return nil // no commands directory is valid
	}

	return filepath.Walk(cmdDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("configtest: read command %q: %w", path, err)
		}

		relPath, _ := filepath.Rel(cmdDir, path)
		name := strings.TrimSuffix(filepath.Base(path), ".md")

		fm, body := parseFrontmatter(string(content))

		cmd := CommandInfo{
			Name:        name,
			Path:        path,
			RelPath:     relPath,
			Description: fm["description"],
			Frontmatter: fm,
			Content:     string(content),
			Body:        body,
		}
		inv.Commands = append(inv.Commands, cmd)
		return nil
	})
}

func discoverSkills(root string, inv *ConfigInventory) error {
	skillDir := filepath.Join(root, "skills")
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		return nil // no skills directory is valid
	}

	return filepath.Walk(skillDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Base(path) != "SKILL.md" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("configtest: read skill %q: %w", path, err)
		}

		dir := filepath.Dir(path)
		relDir, _ := filepath.Rel(skillDir, dir)
		name := filepath.Base(dir)

		fm, body := parseFrontmatter(string(content))

		skill := SkillInfo{
			Name:        name,
			Dir:         dir,
			RelDir:      relDir,
			SkillMDPath: path,
			Description: fm["description"],
			Frontmatter: fm,
			Content:     string(content),
			Body:        body,
		}
		inv.Skills = append(inv.Skills, skill)
		return nil
	})
}

func discoverSettings(root string, inv *ConfigInventory) error {
	settingsPath := filepath.Join(root, "settings.json")
	data, err := os.ReadFile(settingsPath)
	if os.IsNotExist(err) {
		return nil // no settings.json is valid
	}
	if err != nil {
		return fmt.Errorf("configtest: read settings: %w", err)
	}

	var parsed SettingsJSON
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("configtest: parse settings.json: %w", err)
	}

	inv.Settings = &SettingsInfo{
		Path:   settingsPath,
		Raw:    json.RawMessage(data),
		Parsed: parsed,
	}
	return nil
}

// parseFrontmatter extracts YAML frontmatter delimited by --- from markdown content.
// Returns the frontmatter as a map and the body after frontmatter.
// Falls back to line-by-line key: value parsing if YAML parsing fails
// (e.g., when values contain YAML-special characters like brackets).
func parseFrontmatter(content string) (map[string]string, string) {
	fm := make(map[string]string)

	if !strings.HasPrefix(content, "---") {
		return fm, content
	}

	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return fm, content
	}

	yamlBlock := rest[:idx]
	body := strings.TrimLeft(rest[idx+4:], "\n")

	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlBlock), &raw); err != nil {
		// Fallback: line-by-line "key: value" parsing for files with
		// YAML-unfriendly values (e.g., argument-hint: [board] [app-path])
		for _, line := range strings.Split(yamlBlock, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if k, v, ok := strings.Cut(line, ":"); ok {
				fm[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
		return fm, body
	}

	for k, v := range raw {
		fm[k] = fmt.Sprintf("%v", v)
	}

	return fm, body
}

// commandRefPattern matches /command-name references in markdown content.
var commandRefPattern = regexp.MustCompile(`/([a-z][a-z0-9-]+)`)

// ExtractCommandRefs finds /command-name references in text content.
func ExtractCommandRefs(content string) []string {
	matches := commandRefPattern.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	var refs []string
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			refs = append(refs, name)
		}
	}
	return refs
}

// AssertCommandsValid runs structural assertions on all discovered commands.
func AssertCommandsValid(t *testing.T, inv *ConfigInventory) {
	t.Helper()

	for _, cmd := range inv.Commands {
		t.Run("Command/"+cmd.Name, func(t *testing.T) {
			// File exists
			if _, err := os.Stat(cmd.Path); err != nil {
				t.Errorf("command file does not exist: %s", cmd.Path)
			}

			// Has frontmatter
			if !strings.HasPrefix(cmd.Content, "---") {
				t.Errorf("command %q missing YAML frontmatter (no --- delimiter)", cmd.Name)
			}

			// Has description
			if cmd.Description == "" {
				t.Errorf("command %q missing description in frontmatter", cmd.Name)
			}
		})
	}
}

// AssertSkillsValid runs structural assertions on all discovered skills.
func AssertSkillsValid(t *testing.T, inv *ConfigInventory) {
	t.Helper()

	for _, skill := range inv.Skills {
		t.Run("Skill/"+skill.RelDir, func(t *testing.T) {
			// SKILL.md exists
			if _, err := os.Stat(skill.SkillMDPath); err != nil {
				t.Errorf("SKILL.md does not exist: %s", skill.SkillMDPath)
			}

			// Has frontmatter
			if !strings.HasPrefix(skill.Content, "---") {
				t.Errorf("skill %q missing YAML frontmatter", skill.Name)
			}

			// Content not empty (>100 bytes)
			if len(skill.Content) < 100 {
				t.Errorf("skill %q SKILL.md too small (%d bytes, want >100)", skill.Name, len(skill.Content))
			}

			// Follows naming convention: lowercase, hyphens only
			if !isValidSkillName(skill.Name) {
				t.Errorf("skill name %q doesn't follow naming convention (lowercase alphanumeric with hyphens)", skill.Name)
			}
		})
	}
}

// AssertSettingsValid runs structural assertions on the settings.json.
func AssertSettingsValid(t *testing.T, inv *ConfigInventory) {
	t.Helper()

	if inv.Settings == nil {
		t.Skip("no settings.json found")
		return
	}

	// Valid JSON (already parsed during discovery, but verify raw)
	var raw interface{}
	if err := json.Unmarshal(inv.Settings.Raw, &raw); err != nil {
		t.Errorf("settings.json is not valid JSON: %v", err)
	}

	// Has permissions
	if inv.Settings.Parsed.Permissions == nil {
		t.Error("settings.json missing permissions block")
	} else {
		if len(inv.Settings.Parsed.Permissions.Allow) == 0 {
			t.Error("settings.json has no allowed permissions")
		}
	}
}

var validSkillName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func isValidSkillName(name string) bool {
	return validSkillName.MatchString(name)
}
