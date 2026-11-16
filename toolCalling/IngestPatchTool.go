package toolCalling

import (
	"AIxVuln/patchintel"
	"AIxVuln/taskManager"
	"context"
	"strings"
	"unicode/utf8"
)

// IngestPatchTool imports a vendor patch diff and produces a structured patch
// profile for semantic inversion.
type IngestPatchTool struct {
	task *taskManager.Task
}

func NewIngestPatchTool(task *taskManager.Task) *IngestPatchTool {
	return &IngestPatchTool{task: task}
}

func (h *IngestPatchTool) Name() string {
	return "IngestPatchTool"
}

func (h *IngestPatchTool) Description() string {
	return "Import a vendor patch diff (local file path or GitHub commit/PR .diff URL) and build a structured patch profile. Returns a patch_id used by InvertPatchTool. Prefer offline local files for demos."
}

func (h *IngestPatchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"source": map[string]interface{}{
				"type":        "string",
				"description": "Local path to a .diff/.patch file, or an http(s) URL to a commit/PR diff.(required)",
			},
			"cveId": map[string]interface{}{
				"type":        "string",
				"description": "Optional CVE id, for example CVE-2025-3248.",
			},
			"ghsaId": map[string]interface{}{
				"type":        "string",
				"description": "Optional GHSA id, for example GHSA-rvqx-wpfh-mfx7.",
			},
			"affectedVersions": map[string]interface{}{
				"type":        "string",
				"description": "Optional affected version range, for example <1.3.0.",
			},
			"fixCommit": map[string]interface{}{
				"type":        "string",
				"description": "Optional fix commit hash.",
			},
			"scene": map[string]interface{}{
				"type":        "string",
				"description": "Optional scene template name under data/templates, for example langflow.",
			},
		},
		"required": []string{"source"},
	}
}

func (h *IngestPatchTool) Execute(parameters map[string]interface{}) string {
	source := stringParam(parameters["source"])
	if source == "" {
		return Fail("Missing 'source' parameter")
	}
	var (
		profile *patchintel.PatchProfile
		err     error
	)
	if isHTTPURL(source) {
		profile, err = patchintel.IngestURL(context.Background(), source)
	} else {
		profile, err = patchintel.IngestLocal(source)
	}
	if err != nil {
		return Fail("ingest patch failed: " + err.Error())
	}
	patchintel.ApplyMetadata(profile,
		stringParam(parameters["cveId"]),
		stringParam(parameters["ghsaId"]),
		stringParam(parameters["affectedVersions"]),
		stringParam(parameters["fixCommit"]),
		stringParam(parameters["scene"]),
	)
	patchID, err := patchintel.RegisterProfile(profile)
	if err != nil {
		return Fail("register patch profile failed: " + err.Error())
	}
	headers := make([]string, 0, 5)
	for _, hunk := range profile.Hunks {
		if len(headers) >= 5 {
			break
		}
		headers = append(headers, hunk.File+" "+hunk.Header)
	}
	return Success(map[string]interface{}{
		"patch_id":          patchID,
		"cve":               profile.CVE,
		"ghsa":              profile.GHSA,
		"affected_versions": profile.AffectedVersions,
		"fix_commit":        profile.FixCommit,
		"scene":             profile.Scene,
		"source":            profile.Source,
		"source_url":        profile.SourceURL,
		"files":             profile.Files,
		"hunk_count":        len(profile.Hunks),
		"diff_chars":        utf8.RuneCountInString(profile.RawDiff),
		"hunk_headers":      headers,
		"next_step":         "调用 InvertPatchTool，传入 patch_id 进行语义反演。",
	})
}

func isHTTPURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}
