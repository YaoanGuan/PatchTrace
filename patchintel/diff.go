package patchintel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"AIxVuln/misc"
)

// Hunk is one unified-diff hunk.
type Hunk struct {
	File     string   `json:"file"`
	Header   string   `json:"header"`
	OldStart int      `json:"old_start"`
	OldCount int      `json:"old_count"`
	NewStart int      `json:"new_start"`
	NewCount int      `json:"new_count"`
	Lines    []string `json:"lines"`
}

// PatchProfile is the structured representation of an ingested patch.
type PatchProfile struct {
	ID               string    `json:"id"`
	CVE              string    `json:"cve,omitempty"`
	GHSA             string    `json:"ghsa,omitempty"`
	AffectedVersions string    `json:"affected_versions,omitempty"`
	FixCommit        string    `json:"fix_commit,omitempty"`
	Scene            string    `json:"scene,omitempty"`
	Source           string    `json:"source,omitempty"`
	SourceURL        string    `json:"source_url,omitempty"`
	Files            []string  `json:"files,omitempty"`
	Hunks            []Hunk    `json:"hunks"`
	RawDiff          string    `json:"raw_diff"`
	CreatedAt        time.Time `json:"created_at"`
}

const (
	maxPatchBytes   = 10 << 20
	maxFetchTimeout = 30 * time.Second
)

var (
	cvePattern    = regexp.MustCompile(`(?i)\bCVE-\d{4}-\d{4,7}\b`)
	ghsaPattern   = regexp.MustCompile(`(?i)\bGHSA(-[0-9a-z]{4}){3}\b`)
	commitPattern = regexp.MustCompile(`(?m)^(?:From|commit)[ \t]+([0-9a-f]{7,40})\b`)
	hunkPattern   = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)
)

// ParseDiff parses a unified diff into a PatchProfile.
func ParseDiff(raw string) (*PatchProfile, error) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("patch diff is empty")
	}
	profile := &PatchProfile{
		RawDiff:   raw,
		CreatedAt: time.Now().UTC(),
	}
	currentFile := ""
	var current *Hunk
	flush := func() {
		if current != nil {
			profile.Hunks = append(profile.Hunks, *current)
			current = nil
		}
	}
	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				currentFile = trimDiffPrefix(fields[3])
				addFile(profile, currentFile)
			}
		case strings.HasPrefix(line, "+++ "):
			flush()
			path := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
			if path != "/dev/null" {
				currentFile = trimDiffPrefix(path)
				addFile(profile, currentFile)
			}
		case strings.HasPrefix(line, "@@"):
			flush()
			hunk, ok := parseHunkHeader(line)
			if !ok {
				continue
			}
			if currentFile == "" {
				currentFile = "unknown"
			}
			hunk.File = currentFile
			current = &hunk
		default:
			if current == nil || line == "" {
				continue
			}
			switch line[0] {
			case ' ', '+', '-', '\\':
				current.Lines = append(current.Lines, line)
			}
		}
	}
	flush()
	if len(profile.Hunks) == 0 {
		return nil, fmt.Errorf("no unified diff hunks found")
	}
	profile.ID = profileID(raw)
	extractMetadata(profile)
	return profile, nil
}

func parseHunkHeader(line string) (Hunk, bool) {
	match := hunkPattern.FindStringSubmatch(line)
	if match == nil {
		return Hunk{}, false
	}
	hunk := Hunk{Header: line, OldCount: 1, NewCount: 1}
	hunk.OldStart = parseInt(match[1])
	if match[2] != "" {
		hunk.OldCount = parseInt(match[2])
	}
	hunk.NewStart = parseInt(match[3])
	if match[4] != "" {
		hunk.NewCount = parseInt(match[4])
	}
	return hunk, true
}

func parseInt(value string) int {
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return n
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func trimDiffPrefix(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	path = strings.TrimPrefix(path, "./")
	return path
}

func addFile(profile *PatchProfile, file string) {
	file = strings.TrimSpace(file)
	if file == "" || file == "/dev/null" {
		return
	}
	for _, existing := range profile.Files {
		if existing == file {
			return
		}
	}
	profile.Files = append(profile.Files, file)
}

func extractMetadata(profile *PatchProfile) {
	if profile.CVE == "" {
		profile.CVE = cvePattern.FindString(profile.RawDiff)
	}
	if profile.GHSA == "" {
		profile.GHSA = ghsaPattern.FindString(profile.RawDiff)
	}
	if profile.FixCommit == "" {
		if match := commitPattern.FindStringSubmatch(profile.RawDiff); match != nil {
			profile.FixCommit = match[1]
		}
	}
}

// ApplyMetadata overlays caller-provided metadata and then fills any remaining
// gaps from the diff text.
func ApplyMetadata(profile *PatchProfile, cve string, ghsa string, affectedVersions string, fixCommit string, scene string) {
	if profile == nil {
		return
	}
	if strings.TrimSpace(cve) != "" {
		profile.CVE = strings.TrimSpace(cve)
	}
	if strings.TrimSpace(ghsa) != "" {
		profile.GHSA = strings.TrimSpace(ghsa)
	}
	if strings.TrimSpace(affectedVersions) != "" {
		profile.AffectedVersions = strings.TrimSpace(affectedVersions)
	}
	if strings.TrimSpace(fixCommit) != "" {
		profile.FixCommit = strings.TrimSpace(fixCommit)
	}
	if strings.TrimSpace(scene) != "" {
		profile.Scene = sanitizeScene(scene)
	}
	extractMetadata(profile)
}

func profileID(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])[:12]
}

// IngestLocal reads and parses a local diff file.
func IngestLocal(path string) (*PatchProfile, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("patch file path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%q is not a regular file", path)
	}
	if info.Size() > maxPatchBytes {
		return nil, fmt.Errorf("patch file is too large (%d bytes, limit %d)", info.Size(), maxPatchBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	profile, err := ParseDiff(string(data))
	if err != nil {
		return nil, err
	}
	profile.Source = "file"
	abs, _ := filepath.Abs(path)
	profile.SourceURL = abs
	return profile, nil
}

// IngestURL downloads and parses a diff from a GitHub commit/PR URL.
func IngestURL(ctx context.Context, rawURL string) (*PatchProfile, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("patch URL is required")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid patch URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("only http(s) patch URLs are supported")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, maxFetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("patch download failed: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxPatchBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPatchBytes {
		return nil, fmt.Errorf("patch download exceeds %d bytes", maxPatchBytes)
	}
	profile, err := ParseDiff(string(data))
	if err != nil {
		return nil, err
	}
	profile.Source = "url"
	profile.SourceURL = rawURL
	return profile, nil
}

var (
	registryMu sync.Mutex
	registry   = map[string]*PatchProfile{}
)

// RegisterProfile stores a profile in memory and persists it for auditability.
func RegisterProfile(profile *PatchProfile) (string, error) {
	if profile == nil {
		return "", fmt.Errorf("patch profile is nil")
	}
	if profile.ID == "" {
		profile.ID = profileID(profile.RawDiff)
	}
	if !validProfileID(profile.ID) {
		return "", fmt.Errorf("invalid patch profile id")
	}
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = time.Now().UTC()
	}
	registryMu.Lock()
	registry[profile.ID] = profile
	registryMu.Unlock()
	if err := persistProfile(profile); err != nil {
		misc.Debug("PatchOracle persist profile failed: %v", err)
	}
	return profile.ID, nil
}

// GetProfile returns a registered profile, falling back to disk.
func GetProfile(id string) (*PatchProfile, error) {
	id = strings.TrimSpace(id)
	if !validProfileID(id) {
		return nil, fmt.Errorf("invalid patch id")
	}
	registryMu.Lock()
	profile, ok := registry[id]
	registryMu.Unlock()
	if ok {
		return profile, nil
	}
	path := profileStorePath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("patch profile %q not found: %w", id, err)
	}
	var loaded PatchProfile
	if err := json.Unmarshal(data, &loaded); err != nil {
		return nil, err
	}
	registryMu.Lock()
	registry[id] = &loaded
	registryMu.Unlock()
	return &loaded, nil
}

func validProfileID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return false
	}
	return true
}

func profileStoreDir() string {
	return filepath.Join(misc.GetDataDir(), "patchintel", "patches")
}

func profileStorePath(id string) string {
	return filepath.Join(profileStoreDir(), id+".json")
}

func persistProfile(profile *PatchProfile) error {
	if err := os.MkdirAll(profileStoreDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(profileStorePath(profile.ID), data, 0o644)
}

func sanitizeScene(scene string) string {
	scene = strings.TrimSpace(scene)
	if scene == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range scene {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		}
	}
	cleaned := strings.Trim(b.String(), ".")
	if cleaned == "" || strings.Contains(cleaned, "..") {
		return ""
	}
	return cleaned
}
