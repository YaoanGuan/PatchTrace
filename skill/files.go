package skill

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// FileChunk is an incremental view of a skill file, measured in runes.
type FileChunk struct {
	SkillName          string `json:"skill_name"`
	RelativePath       string `json:"relative_path"`
	Offset             int    `json:"offset"`
	Limit              int    `json:"limit"`
	NextOffset         int    `json:"next_offset"`
	TotalChars         int    `json:"total_chars"`
	HasMore            bool   `json:"has_more"`
	Content            string `json:"content"`
	Summarized         bool   `json:"summarized,omitempty"`
	SummaryMethod      string `json:"summary_method,omitempty"`
	OriginalTotalChars int    `json:"original_total_chars,omitempty"`
}

// FileDocument is a full skill file.
type FileDocument struct {
	SkillName    string `json:"skill_name"`
	RelativePath string `json:"relative_path"`
	Editable     bool   `json:"editable"`
	Content      string `json:"content"`
	TotalChars   int    `json:"total_chars"`
}

// FileEntry is one entry of a skill directory listing.
type FileEntry struct {
	Path  string `json:"path"`
	Type  string `json:"type"`
	Size  int64  `json:"size,omitempty"`
	Depth int    `json:"depth"`
}

// FileList is the payload returned by ListFiles.
type FileList struct {
	SkillName  string      `json:"skill_name"`
	BasePath   string      `json:"base_path"`
	MaxDepth   int         `json:"max_depth"`
	Truncated  bool        `json:"truncated"`
	EntryCount int         `json:"entry_count"`
	Entries    []FileEntry `json:"entries"`
}

const (
	defaultChunkLimit = 12000
	maxChunkLimit     = 30000
	maxListEntries    = 2000
)

// ReadFile returns a rune-addressed chunk of a skill file.
func ReadFile(skillName string, relativePath string, offset int, limit int) (*FileChunk, error) {
	summary, err := Get(skillName)
	if err != nil {
		return nil, err
	}
	rel, err := cleanRelativePath(relativePath)
	if err != nil {
		return nil, err
	}
	if rel == "" {
		rel = skillFileName
	}
	abs, err := joinInside(summary.RootPath, rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%q is a directory", rel)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	return buildChunk(summary.Name, rel, string(data), offset, limit), nil
}

// ReadDocument returns the full content of a skill file.
func ReadDocument(skillName string, relativePath string) (*FileDocument, error) {
	summary, err := Get(skillName)
	if err != nil {
		return nil, err
	}
	rel, err := cleanRelativePath(relativePath)
	if err != nil {
		return nil, err
	}
	if rel == "" {
		rel = skillFileName
	}
	abs, err := joinInside(summary.RootPath, rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%q is a directory", rel)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	content := string(data)
	return &FileDocument{
		SkillName:    summary.Name,
		RelativePath: rel,
		Editable:     summary.Editable,
		Content:      content,
		TotalChars:   utf8.RuneCountInString(content),
	}, nil
}

// SaveDocument writes a skill file. Only skills in the managed root are
// writable.
func SaveDocument(skillName string, relativePath string, content string) (*FileDocument, error) {
	summary, err := Get(skillName)
	if err != nil {
		return nil, err
	}
	if !summary.Editable {
		return nil, fmt.Errorf("skill %q is read-only", summary.Name)
	}
	rel, err := cleanRelativePath(relativePath)
	if err != nil {
		return nil, err
	}
	if rel == "" {
		return nil, fmt.Errorf("relative path is required")
	}
	abs, err := joinInside(summary.RootPath, rel)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return nil, err
	}
	return &FileDocument{
		SkillName:    summary.Name,
		RelativePath: rel,
		Editable:     true,
		Content:      content,
		TotalChars:   utf8.RuneCountInString(content),
	}, nil
}

// ListFiles lists entries under a skill directory without reading contents.
func ListFiles(skillName string, relativePath string, maxDepth int) (*FileList, error) {
	summary, err := Get(skillName)
	if err != nil {
		return nil, err
	}
	rel, err := cleanRelativePath(relativePath)
	if err != nil {
		return nil, err
	}
	if rel == "." {
		rel = ""
	}
	if maxDepth <= 0 {
		maxDepth = 2
	}
	if maxDepth > 5 {
		maxDepth = 5
	}
	base := summary.RootPath
	if rel != "" {
		base, err = joinInside(summary.RootPath, rel)
		if err != nil {
			return nil, err
		}
	}
	info, err := os.Stat(base)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", rel)
	}
	result := &FileList{
		SkillName: summary.Name,
		BasePath:  rel,
		MaxDepth:  maxDepth,
		Entries:   make([]FileEntry, 0, 32),
	}
	err = filepath.WalkDir(base, func(abs string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if abs == base {
			return nil
		}
		childRel, err := filepath.Rel(base, abs)
		if err != nil {
			return nil
		}
		childRel = filepath.ToSlash(childRel)
		if strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		depth := strings.Count(childRel, "/") + 1
		if depth > maxDepth {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		entryType := "file"
		var size int64
		if entry.IsDir() {
			entryType = "dir"
		} else if info, err := entry.Info(); err == nil {
			size = info.Size()
		}
		fullPath := childRel
		if rel != "" {
			fullPath = path.Join(rel, childRel)
		}
		result.Entries = append(result.Entries, FileEntry{
			Path:  fullPath,
			Type:  entryType,
			Size:  size,
			Depth: depth,
		})
		if len(result.Entries) >= maxListEntries {
			result.Truncated = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(result.Entries, func(i, j int) bool {
		return result.Entries[i].Path < result.Entries[j].Path
	})
	result.EntryCount = len(result.Entries)
	return result, nil
}

// CreateSkill creates a new skill directory with a default SKILL.md in the
// managed root.
func CreateSkill(name string, description string) (*SkillSummary, error) {
	name = strings.TrimSpace(name)
	if err := validateSkillName(name); err != nil {
		return nil, err
	}
	if existing, _ := Get(name); existing != nil {
		return nil, fmt.Errorf("skill %q already exists", name)
	}
	description = strings.TrimSpace(description)
	dir := filepath.Join(ManagedRoot(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	title := name
	body := "# " + title + "\n"
	if description != "" {
		body += "\n" + description + "\n"
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: " + name + "\n")
	if description != "" {
		b.WriteString("description: " + description + "\n")
	}
	b.WriteString("---\n\n")
	b.WriteString(body)
	if err := os.WriteFile(filepath.Join(dir, skillFileName), []byte(b.String()), 0o644); err != nil {
		return nil, err
	}
	return &SkillSummary{
		Name:          name,
		Description:   description,
		RootPath:      dir,
		SkillFilePath: filepath.Join(dir, skillFileName),
		Editable:      true,
	}, nil
}

// DeleteSkill removes a managed skill directory.
func DeleteSkill(name string) error {
	summary, err := Get(name)
	if err != nil {
		return err
	}
	if !summary.Editable {
		return fmt.Errorf("skill %q is read-only", summary.Name)
	}
	if filepath.Clean(filepath.Dir(summary.RootPath)) != filepath.Clean(ManagedRoot()) {
		return fmt.Errorf("refusing to delete skill outside the managed root")
	}
	return os.RemoveAll(summary.RootPath)
}

// DeleteDocument removes a single file from a managed skill. SKILL.md cannot be
// deleted on its own.
func DeleteDocument(skillName string, relativePath string) error {
	summary, err := Get(skillName)
	if err != nil {
		return err
	}
	if !summary.Editable {
		return fmt.Errorf("skill %q is read-only", summary.Name)
	}
	rel, err := cleanRelativePath(relativePath)
	if err != nil {
		return err
	}
	if rel == "" || rel == skillFileName {
		return fmt.Errorf("SKILL.md cannot be deleted; delete the whole skill instead")
	}
	abs, err := joinInside(summary.RootPath, rel)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%q is a directory", rel)
	}
	if err := os.Remove(abs); err != nil {
		return err
	}
	pruneEmptyDirs(filepath.Dir(abs), summary.RootPath)
	return nil
}

func validateSkillName(name string) error {
	if name == "" {
		return fmt.Errorf("skill name is required")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid skill name")
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("skill name must not contain path separators")
	}
	first := rune(name[0])
	if !isAlphaNumeric(first) {
		return fmt.Errorf("skill name must start with a letter or digit")
	}
	for _, r := range name {
		if isAlphaNumeric(r) || r == '.' || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("skill name may only contain letters, digits, dot, underscore and dash")
	}
	return nil
}

func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// cleanRelativePath normalises a client-supplied path and rejects escapes.
func cleanRelativePath(rel string) (string, error) {
	rel = strings.ReplaceAll(strings.TrimSpace(rel), "\\", "/")
	if strings.ContainsRune(rel, '\x00') {
		return "", fmt.Errorf("invalid path")
	}
	if rel == "" {
		return "", nil
	}
	if strings.HasPrefix(rel, "/") {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	parts := strings.Split(rel, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", fmt.Errorf("path escapes the skill directory")
		default:
			out = append(out, part)
		}
	}
	return strings.Join(out, "/"), nil
}

// joinInside joins rel onto root and verifies the result stays inside root.
func joinInside(root string, rel string) (string, error) {
	root = filepath.Clean(root)
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if abs != root {
		relToRoot, err := filepath.Rel(root, abs)
		if err != nil {
			return "", err
		}
		if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("path escapes the skill directory")
		}
	}
	return abs, nil
}

func buildChunk(skillName string, rel string, content string, offset int, limit int) *FileChunk {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = defaultChunkLimit
	}
	if limit > maxChunkLimit {
		limit = maxChunkLimit
	}
	runes := []rune(content)
	if offset > len(runes) {
		offset = len(runes)
	}
	end := offset + limit
	if end > len(runes) {
		end = len(runes)
	}
	return &FileChunk{
		SkillName:    skillName,
		RelativePath: rel,
		Offset:       offset,
		Limit:        limit,
		NextOffset:   end,
		TotalChars:   len(runes),
		HasMore:      end < len(runes),
		Content:      string(runes[offset:end]),
	}
}

func pruneEmptyDirs(dir string, stop string) {
	stop = filepath.Clean(stop)
	for {
		dir = filepath.Clean(dir)
		if dir == stop || !strings.HasPrefix(dir, stop+string(filepath.Separator)) {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
