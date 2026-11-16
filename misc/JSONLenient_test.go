package misc

import "testing"

func TestRepairJSONStringLiteralsEscapesRawNewlines(t *testing.T) {
	raw := "{\n  \"task_content\": \"first line\nsecond line\",\n  \"ok\": true\n}"
	repaired, changed := RepairJSONStringLiterals(raw)
	if !changed {
		t.Fatal("expected repair to report changed=true")
	}
	want := "{\n  \"task_content\": \"first line\\nsecond line\",\n  \"ok\": true\n}"
	if repaired != want {
		t.Fatalf("unexpected repaired json:\nwant: %q\ngot:  %q", want, repaired)
	}
}

func TestUnmarshalJSONLenientAcceptsRawNewlineInString(t *testing.T) {
	raw := "{\"task_content\":\"first line\nsecond line\",\"count\":1}"
	var args map[string]interface{}
	if err := UnmarshalJSONLenient(raw, &args); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if got := args["task_content"]; got != "first line\nsecond line" {
		t.Fatalf("unexpected task_content: %#v", got)
	}
}

func TestUnmarshalJSONLenientLeavesNormalJSONUntouched(t *testing.T) {
	raw := "{\"task_content\":\"first line\\nsecond line\",\"count\":1}"
	var args map[string]interface{}
	if err := UnmarshalJSONLenient(raw, &args); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if got := args["task_content"]; got != "first line\nsecond line" {
		t.Fatalf("unexpected task_content: %#v", got)
	}
}

func TestUnmarshalJSONLenientTreatsEmptyInputAsEmptyObject(t *testing.T) {
	var args map[string]interface{}
	if err := UnmarshalJSONLenient("   ", &args); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if args == nil {
		t.Fatal("expected empty object map, got nil")
	}
	if len(args) != 0 {
		t.Fatalf("expected empty object, got: %#v", args)
	}
}
