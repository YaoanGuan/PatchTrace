// Package resource exposes the bundled tool/docs/PoC corpus shipped next to
// the web binary (the release tarball contains resource/tools, resource/docs
// and resource/pocs).
//
// The root is resolved from AIxVuln_RESOURCE_ROOT first, then common locations
// relative to the working directory and the executable.
package resource

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

// AgentTool is a downloadable helper binary described by tools/manifest.json.
type AgentTool struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Filename    string `json:"filename"`
	File        string `json:"file"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
}

// ResourceDoc is one file under resource/docs.
type ResourceDoc struct {
	RelativePath string `json:"relative_path"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
}

// PocKnowledgeDoc is one file under resource/pocs.
type PocKnowledgeDoc struct {
	RelativePath string `json:"relative_path"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
}

// DocChunk is an incremental rune-addressed view of a resource file.
type DocChunk struct {
	RelativePath string `json:"relative_path"`
	Name         string `json:"name"`
	Offset       int    `json:"offset"`
	Limit        int    `json:"limit"`
	NextOffset   int    `json:"next_offset"`
	TotalChars   int    `json:"total_chars"`
	HasMore      bool   `json:"has_more"`
	Content      string `json:"content"`
}

// ResourceDocSearchHit is a ranked search hit under resource/docs.
type ResourceDocSearchHit struct {
	RelativePath string `json:"relative_path"`
	Name         string `json:"name"`
	Line         int    `json:"line"`
	Snippet      string `json:"snippet"`
	Score        int    `json:"score"`
}

// PocKnowledgeDocSearchHit is a ranked search hit under resource/pocs.
type PocKnowledgeDocSearchHit struct {
	RelativePath string `json:"relative_path"`
	Name         string `json:"name"`
	Line         int    `json:"line"`
	Snippet      string `json:"snippet"`
	Score        int    `json:"score"`
}

const (
	envRoot      = "AIxVuln_RESOURCE_ROOT"
	defaultChunk = 8000
	maxChunk     = 30000
	maxSearchHit = 100
	maxScanBytes = 2 << 20
	snippetRunes = 280
)

// Root returns the resolved resource root directory.
func Root() string {
	if value := strings.TrimSpace(os.Getenv(envRoot)); value != "" {
		if abs, err := filepath.Abs(value); err == nil {
			return abs
		}
		return filepath.Clean(value)
	}
	candidates := []string{"../deploy/resource", "./resource"}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "resource"))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			if abs, err := filepath.Abs(candidate); err == nil {
				return abs
			}
			return filepath.Clean(candidate)
		}
	}
	abs, _ := filepath.Abs(candidates[0])
	return abs
}

func docsRoot() string  { return filepath.Join(Root(), "docs") }
func pocsRoot() string  { return filepath.Join(Root(), "pocs") }
func toolsRoot() string { return filepath.Join(Root(), "tools") }

// ListAgentTools returns the bundled helper tools from tools/manifest.json.
func ListAgentTools() ([]AgentTool, error) {
	data, err := os.ReadFile(filepath.Join(toolsRoot(), "manifest.json"))
	if err != nil {
		return nil, err
	}
	var raw []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		File        string `json:"file"`
		Filename    string `json:"filename"`
		Category    string `json:"category"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse tools manifest: %w", err)
	}
	out := make([]AgentTool, 0, len(raw))
	for _, item := range raw {
		file := strings.TrimSpace(item.File)
		if file == "" {
			file = strings.TrimSpace(item.Filename)
		}
		tool := AgentTool{
			ID:          strings.TrimSpace(item.ID),
			Name:        strings.TrimSpace(item.Name),
			Filename:    filepath.Base(file),
			File:        file,
			Category:    strings.TrimSpace(item.Category),
			Description: strings.TrimSpace(item.Description),
		}
		if abs, err := joinInside(toolsRoot(), file); err == nil {
			tool.Size, tool.SHA256 = fileFingerprint(abs)
		}
		out = append(out, tool)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// ReadAgentTool returns the tool metadata and its bytes.
func ReadAgentTool(id string) (AgentTool, []byte, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return AgentTool{}, nil, fmt.Errorf("tool id is required")
	}
	items, err := ListAgentTools()
	if err != nil {
		return AgentTool{}, nil, err
	}
	for _, item := range items {
		if item.ID != id {
			continue
		}
		abs, err := joinInside(toolsRoot(), item.File)
		if err != nil {
			return AgentTool{}, nil, err
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return AgentTool{}, nil, err
		}
		return item, data, nil
	}
	return AgentTool{}, nil, fmt.Errorf("agent tool %q: %w", id, fs.ErrNotExist)
}

// ListResourceDocs lists files under resource/docs.
func ListResourceDocs() ([]ResourceDoc, error) {
	out := make([]ResourceDoc, 0, 64)
	err := walkTextFiles(docsRoot(), func(rel string, info fs.FileInfo) {
		out = append(out, ResourceDoc{RelativePath: rel, Name: filepath.Base(rel), Size: info.Size()})
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].RelativePath < out[j].RelativePath })
	return out, nil
}

// ReadResourceDoc reads a chunk of a file under resource/docs.
func ReadResourceDoc(relativePath string, offset int, limit int) (*DocChunk, error) {
	return readChunk(docsRoot(), relativePath, offset, limit)
}

// SearchResourceDocs searches text content under resource/docs.
func SearchResourceDocs(query string, pathPrefix string, limit int) ([]ResourceDocSearchHit, error) {
	hits, err := searchTextFiles(docsRoot(), query, pathPrefix, limit)
	if err != nil {
		return nil, err
	}
	out := make([]ResourceDocSearchHit, 0, len(hits))
	for _, hit := range hits {
		out = append(out, ResourceDocSearchHit(hit))
	}
	return out, nil
}

// ListPocKnowledgeDocs lists files under resource/pocs.
func ListPocKnowledgeDocs() ([]PocKnowledgeDoc, error) {
	out := make([]PocKnowledgeDoc, 0, 128)
	err := walkTextFiles(pocsRoot(), func(rel string, info fs.FileInfo) {
		out = append(out, PocKnowledgeDoc{RelativePath: rel, Name: filepath.Base(rel), Size: info.Size()})
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].RelativePath < out[j].RelativePath })
	return out, nil
}

// ReadPocKnowledgeDoc reads a chunk of a file under resource/pocs.
func ReadPocKnowledgeDoc(relativePath string, offset int, limit int) (*DocChunk, error) {
	return readChunk(pocsRoot(), relativePath, offset, limit)
}

// SearchPocKnowledgeDocs searches text content under resource/pocs.
func SearchPocKnowledgeDocs(query string, pathPrefix string, limit int) ([]PocKnowledgeDocSearchHit, error) {
	hits, err := searchTextFiles(pocsRoot(), query, pathPrefix, limit)
	if err != nil {
		return nil, err
	}
	out := make([]PocKnowledgeDocSearchHit, 0, len(hits))
	for _, hit := range hits {
		out = append(out, PocKnowledgeDocSearchHit(hit))
	}
	return out, nil
}

// PocKnowledgeRoots lists the searchable PoC corpus roots.
func PocKnowledgeRoots() []string {
	root := pocsRoot()
	if info, err := os.Stat(root); err == nil && info.IsDir() {
		return []string{root}
	}
	return []string{}
}

func readChunk(root string, relativePath string, offset int, limit int) (*DocChunk, error) {
	rel, err := cleanRelativePath(relativePath)
	if err != nil {
		return nil, err
	}
	if rel == "" {
		return nil, fmt.Errorf("relative path is required")
	}
	abs, err := joinInside(root, rel)
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
	return buildChunk(rel, string(data), offset, limit), nil
}

func buildChunk(rel string, content string, offset int, limit int) *DocChunk {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = defaultChunk
	}
	if limit > maxChunk {
		limit = maxChunk
	}
	runes := []rune(content)
	if offset > len(runes) {
		offset = len(runes)
	}
	end := offset + limit
	if end > len(runes) {
		end = len(runes)
	}
	return &DocChunk{
		RelativePath: rel,
		Name:         filepath.Base(rel),
		Offset:       offset,
		Limit:        limit,
		NextOffset:   end,
		TotalChars:   len(runes),
		HasMore:      end < len(runes),
		Content:      string(runes[offset:end]),
	}
}

func searchTextFiles(root string, query string, pathPrefix string, limit int) ([]struct {
	RelativePath string `json:"relative_path"`
	Name         string `json:"name"`
	Line         int    `json:"line"`
	Snippet      string `json:"snippet"`
	Score        int    `json:"score"`
}, error) {
	type hit struct {
		RelativePath string `json:"relative_path"`
		Name         string `json:"name"`
		Line         int    `json:"line"`
		Snippet      string `json:"snippet"`
		Score        int    `json:"score"`
	}
	terms := splitTerms(query)
	if len(terms) == 0 {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > maxSearchHit {
		limit = maxSearchHit
	}
	prefix, err := cleanRelativePath(pathPrefix)
	if err != nil {
		return nil, err
	}
	var hits []hit
	walkErr := walkTextFiles(root, func(rel string, info fs.FileInfo) {
		if prefix != "" && rel != prefix && !strings.HasPrefix(rel, prefix+"/") {
			return
		}
		if info.Size() > maxScanBytes {
			return
		}
		abs := filepath.Join(root, filepath.FromSlash(rel))
		data, err := os.ReadFile(abs)
		if err != nil || looksBinary(data) {
			return
		}
		text := strings.ReplaceAll(string(data), "\r\n", "\n")
		for lineNo, line := range strings.Split(text, "\n") {
			score := scoreLine(line, terms)
			if score == 0 {
				continue
			}
			snippet := strings.TrimSpace(line)
			if utf8.RuneCountInString(snippet) > snippetRunes {
				snippet = string([]rune(snippet)[:snippetRunes]) + "…"
			}
			hits = append(hits, hit{
				RelativePath: rel,
				Name:         filepath.Base(rel),
				Line:         lineNo + 1,
				Snippet:      snippet,
				Score:        score,
			})
		}
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		if hits[i].RelativePath != hits[j].RelativePath {
			return hits[i].RelativePath < hits[j].RelativePath
		}
		return hits[i].Line < hits[j].Line
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]struct {
		RelativePath string `json:"relative_path"`
		Name         string `json:"name"`
		Line         int    `json:"line"`
		Snippet      string `json:"snippet"`
		Score        int    `json:"score"`
	}, 0, len(hits))
	for _, h := range hits {
		out = append(out, h)
	}
	return out, nil
}

func splitTerms(query string) []string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	seen := make(map[string]struct{}, len(fields))
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.Trim(field, " \t,;:!?\"'()[]{}<>|\\")
		if field == "" {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		out = append(out, field)
	}
	return out
}

func scoreLine(line string, terms []string) int {
	lower := strings.ToLower(line)
	score := 0
	for _, term := range terms {
		if strings.Contains(lower, term) {
			score++
		}
	}
	return score
}

// walkTextFiles walks root, skipping VCS metadata, hidden files and
// AppleDouble artefacts, and reports every regular file with a relative slash
// path.
func walkTextFiles(root string, visit func(rel string, info fs.FileInfo)) error {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return filepath.WalkDir(root, func(abs string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if abs == root {
			return nil
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == ".git" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, "._") || strings.HasPrefix(name, ".") {
			return nil
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		visit(filepath.ToSlash(rel), info)
		return nil
	})
}

func looksBinary(data []byte) bool {
	sample := data
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	for _, b := range sample {
		if b == 0 {
			return true
		}
	}
	return false
}

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
			return "", fmt.Errorf("path escapes the resource directory")
		default:
			out = append(out, part)
		}
	}
	return strings.Join(out, "/"), nil
}

func joinInside(root string, rel string) (string, error) {
	root = filepath.Clean(root)
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if abs != root {
		relToRoot, err := filepath.Rel(root, abs)
		if err != nil {
			return "", err
		}
		if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("path escapes the resource directory")
		}
	}
	return abs, nil
}

var (
	fingerprintMu    sync.Mutex
	fingerprintCache = map[string]fingerprint{}
)

type fingerprint struct {
	size  int64
	mtime int64
	sum   string
}

func fileFingerprint(abs string) (int64, string) {
	info, err := os.Stat(abs)
	if err != nil {
		return 0, ""
	}
	key := abs
	fingerprintMu.Lock()
	cached, ok := fingerprintCache[key]
	fingerprintMu.Unlock()
	if ok && cached.size == info.Size() && cached.mtime == info.ModTime().UnixNano() {
		return cached.size, cached.sum
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return info.Size(), ""
	}
	sum := sha256.Sum256(data)
	value := hex.EncodeToString(sum[:])
	fingerprintMu.Lock()
	fingerprintCache[key] = fingerprint{size: info.Size(), mtime: info.ModTime().UnixNano(), sum: value}
	fingerprintMu.Unlock()
	return info.Size(), value
}
