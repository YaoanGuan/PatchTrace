// Package skill implements the local skill library used by the agent tools and
// the web settings panel.
//
// A skill is a directory that contains a SKILL.md file. Skills are discovered
// from a configurable list of search roots (config [skills] SEARCH_PATHS). The
// managed root (<DATA_DIR>/skills) is writable through this package; skills
// discovered from other roots are read-only.
package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"AIxVuln/misc"
)

// SkillSummary is the metadata exposed to the prompt builder and the web UI.
type SkillSummary struct {
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	RootPath      string `json:"root_path"`
	SkillFilePath string `json:"skill_file_path"`
	HasScripts    bool   `json:"has_scripts"`
	HasAssets     bool   `json:"has_assets"`
	HasReferences bool   `json:"has_references"`
	Editable      bool   `json:"editable"`
}

// SettingsSnapshot is returned by GetSettingsSnapshot for the settings panel.
type SettingsSnapshot struct {
	SearchPaths   string         `json:"search_paths"`
	ResolvedRoots []string       `json:"resolved_roots"`
	ManagedRoot   string         `json:"managed_root"`
	Skills        []SkillSummary `json:"skills"`
}

const (
	skillFileName = "SKILL.md"
	configSection = "skills"
	configKey     = "SEARCH_PATHS"
)

// ManagedRoot returns the writable skill root: <DATA_DIR>/skills.
func ManagedRoot() string {
	return filepath.Join(misc.GetDataDir(), "skills")
}

// SearchPaths returns the configured raw search path string.
func SearchPaths() string {
	return strings.TrimSpace(misc.GetConfigValueDefault(configSection, configKey, ""))
}

// ResolvedRoots returns the absolute search roots in priority order. The first
// root that contains a skill wins when names collide.
func ResolvedRoots() []string {
	raw := SearchPaths()
	var candidates []string
	if raw == "" {
		candidates = []string{"./skills", ManagedRoot()}
	} else {
		candidates = splitSearchPaths(raw)
	}
	seen := make(map[string]struct{}, len(candidates)+1)
	roots := make([]string, 0, len(candidates)+1)
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = filepath.Clean(p)
		}
		if _, ok := seen[abs]; ok {
			return
		}
		seen[abs] = struct{}{}
		roots = append(roots, abs)
	}
	for _, c := range candidates {
		add(c)
	}
	// The managed root must always be discoverable so newly created skills show
	// up even when the user configured custom search paths.
	add(ManagedRoot())
	return roots
}

func splitSearchPaths(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ';' || r == ','
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		out = append(out, field)
	}
	return out
}

// SaveSearchPaths persists the raw search path configuration.
func SaveSearchPaths(raw string) error {
	return misc.SetConfigValue(configSection, configKey, strings.TrimSpace(raw))
}

// GetSettingsSnapshot returns the settings panel payload.
func GetSettingsSnapshot() (*SettingsSnapshot, error) {
	items, err := List()
	if err != nil {
		return nil, err
	}
	return &SettingsSnapshot{
		SearchPaths:   SearchPaths(),
		ResolvedRoots: ResolvedRoots(),
		ManagedRoot:   ManagedRoot(),
		Skills:        items,
	}, nil
}

// List discovers all skills across the resolved roots. When two roots contain a
// skill with the same name, the higher-priority root wins.
func List() ([]SkillSummary, error) {
	managed := filepath.Clean(ManagedRoot())
	seen := make(map[string]struct{})
	out := make([]SkillSummary, 0)
	for _, root := range ResolvedRoots() {
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) || os.IsPermission(err) {
				continue
			}
			return nil, err
		}
		editable := filepath.Clean(root) == managed
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			name := entry.Name()
			if _, ok := seen[name]; ok {
				continue
			}
			dir := filepath.Join(root, name)
			skillFile := filepath.Join(dir, skillFileName)
			info, err := os.Stat(skillFile)
			if err != nil || info.IsDir() {
				continue
			}
			desc, _ := readDescription(skillFile)
			seen[name] = struct{}{}
			out = append(out, SkillSummary{
				Name:          name,
				Description:   desc,
				RootPath:      dir,
				SkillFilePath: skillFile,
				HasScripts:    isDir(filepath.Join(dir, "scripts")),
				HasAssets:     isDir(filepath.Join(dir, "assets")),
				HasReferences: isDir(filepath.Join(dir, "references")),
				Editable:      editable,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Get returns a single skill by name.
func Get(name string) (*SkillSummary, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("skill name is required")
	}
	items, err := List()
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Name == name {
			return &items[i], nil
		}
	}
	return nil, fmt.Errorf("skill %q not found", name)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// readDescription extracts the description from SKILL.md. It understands a
// leading YAML frontmatter block and falls back to the first markdown heading.
func readDescription(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	if desc := frontmatterDescription(content); desc != "" {
		return desc, nil
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# ")), nil
		}
	}
	return "", nil
}

func frontmatterDescription(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return ""
	}
	rest := content[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return ""
	}
	block := rest[:end]
	for _, line := range strings.Split(block, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) != "description" {
			continue
		}
		return strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return ""
}
