package toolCalling

import (
	"AIxVuln/skill"
	"AIxVuln/taskManager"
	"encoding/json"
	"strings"
	"testing"
)

func TestReadSkillToolSummarizesMarkdown(t *testing.T) {
	oldReadChunk := readSkillChunkFunc
	oldReadDoc := readSkillDocumentFunc
	oldSummarize := summarizeSkillMarkdown
	t.Cleanup(func() {
		readSkillChunkFunc = oldReadChunk
		readSkillDocumentFunc = oldReadDoc
		summarizeSkillMarkdown = oldSummarize
	})

	readSkillDocumentFunc = func(skillName string, relativePath string) (*skill.FileDocument, error) {
		return &skill.FileDocument{
			SkillName:    skillName,
			RelativePath: "SKILL.md",
			Content:      "# Raw Skill\n\nThis is very long markdown content.",
			TotalChars:   len([]rune("# Raw Skill\n\nThis is very long markdown content.")),
		}, nil
	}
	summarizeSkillMarkdown = func(_ *taskManager.Task, _ string, _ string, raw string) skillMarkdownSummary {
		if !strings.Contains(raw, "very long markdown content") {
			t.Fatalf("expected raw markdown to be passed into summarizer, got %q", raw)
		}
		return skillMarkdownSummary{
			Content: "steps\n- one\n- two",
			Method:  skillMarkdownSummaryMethodGeneralLLM,
		}
	}

	tool := NewReadSkillTool(nil)
	resp := tool.Execute(map[string]interface{}{
		"skillName":    "frida-gadget",
		"relativePath": "SKILL.md",
		"offset":       0,
		"limit":        5,
	})

	var payload struct {
		Success bool            `json:"success"`
		Error   string          `json:"error"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !payload.Success {
		t.Fatalf("expected success, got error: %s", payload.Error)
	}
	var chunk skill.FileChunk
	if err := json.Unmarshal(payload.Result, &chunk); err != nil {
		t.Fatalf("unmarshal chunk: %v", err)
	}
	if !chunk.Summarized {
		t.Fatalf("expected summarized markdown chunk, got %+v", chunk)
	}
	if chunk.SummaryMethod == "" {
		t.Fatalf("expected summary method, got %+v", chunk)
	}
	if chunk.OriginalTotalChars == 0 {
		t.Fatalf("expected original_total_chars, got %+v", chunk)
	}
	if chunk.Content != "steps" {
		t.Fatalf("expected summary slice, got %+v", chunk)
	}
}

func TestReadSkillToolKeepsNonMarkdownRaw(t *testing.T) {
	oldReadChunk := readSkillChunkFunc
	oldReadDoc := readSkillDocumentFunc
	oldSummarize := summarizeSkillMarkdown
	t.Cleanup(func() {
		readSkillChunkFunc = oldReadChunk
		readSkillDocumentFunc = oldReadDoc
		summarizeSkillMarkdown = oldSummarize
	})

	readSkillChunkFunc = func(skillName string, relativePath string, offset int, limit int) (*skill.FileChunk, error) {
		return &skill.FileChunk{
			SkillName:    skillName,
			RelativePath: relativePath,
			Offset:       offset,
			Limit:        limit,
			NextOffset:   7,
			TotalChars:   7,
			HasMore:      false,
			Content:      "echo hi",
		}, nil
	}
	summarizeSkillMarkdown = func(_ *taskManager.Task, _ string, _ string, _ string) skillMarkdownSummary {
		t.Fatal("markdown summarizer should not be called for non-markdown files")
		return skillMarkdownSummary{}
	}

	tool := NewReadSkillTool(nil)
	resp := tool.Execute(map[string]interface{}{
		"skillName":    "frida-gadget",
		"relativePath": "scripts/run.sh",
	})

	var payload struct {
		Success bool            `json:"success"`
		Error   string          `json:"error"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !payload.Success {
		t.Fatalf("expected success, got error: %s", payload.Error)
	}
	var chunk skill.FileChunk
	if err := json.Unmarshal(payload.Result, &chunk); err != nil {
		t.Fatalf("unmarshal chunk: %v", err)
	}
	if chunk.Summarized {
		t.Fatalf("expected raw chunk for non-markdown file, got %+v", chunk)
	}
	if strings.TrimSpace(chunk.Content) != "echo hi" {
		t.Fatalf("unexpected chunk content: %+v", chunk)
	}
}
