package toolCalling

import "testing"

func TestNormalizeAndValidateToolArgumentsTreatsEmptyAsObject(t *testing.T) {
	args, normalized, err := NormalizeAndValidateToolArguments("", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 0 {
		t.Fatalf("expected empty args, got %#v", args)
	}
	if normalized != "{}" {
		t.Fatalf("unexpected normalized json: %q", normalized)
	}
}

func TestNormalizeAndValidateToolArgumentsRejectsNonObjectRoot(t *testing.T) {
	_, _, err := NormalizeAndValidateToolArguments(`["x"]`, map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	})
	if err == nil || err.Error() != "tool arguments root must be a JSON object" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeAndValidateToolArgumentsValidatesRequiredAndTypes(t *testing.T) {
	_, _, err := NormalizeAndValidateToolArguments(`{"session_id":"abc"}`, map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"session_id": map[string]interface{}{"type": "integer"},
		},
		"required": []string{"session_id"},
	})
	if err == nil || err.Error() != "session_id must be an integer" {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _, err = NormalizeAndValidateToolArguments(`{"session_id":1}`, map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"session_id": map[string]interface{}{"type": "integer"},
			"mode":       map[string]interface{}{"type": "string", "enum": []string{"tail", "snapshot"}},
		},
		"required": []string{"session_id", "mode"},
	})
	if err == nil || err.Error() != "mode is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeAndValidateToolArgumentsValidatesNestedObjectsAndArrays(t *testing.T) {
	args, normalized, err := NormalizeAndValidateToolArguments(`{
		"exploitIdeaIdSet": ["E.1", "E.2"],
		"exploit_point": {"title": "x", "confidence": "high"}
	}`, map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"exploitIdeaIdSet": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"exploit_point": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title":      map[string]interface{}{"type": "string"},
					"confidence": map[string]interface{}{"type": "string", "enum": []string{"low", "medium", "high"}},
				},
				"required": []string{"title", "confidence"},
			},
		},
		"required": []string{"exploitIdeaIdSet", "exploit_point"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 2 || normalized == "" {
		t.Fatalf("unexpected normalized result: %#v / %q", args, normalized)
	}
}
