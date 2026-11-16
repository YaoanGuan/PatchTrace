package toolCalling

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRewriteToolArgsWithAIXFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "payload.txt"), []byte("alpha\nbeta"), 0600); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "note.md"), []byte("# title"), 0600); err != nil {
		t.Fatalf("write note: %v", err)
	}

	args := map[string]interface{}{
		"content": "<aixfile>payload.txt</aixfile>",
		"nested": map[string]interface{}{
			"prompt": "before\n<aixfile>sub/note.md</aixfile>\nafter",
		},
	}
	rewritten, err := rewriteToolArgsWithAIXFiles(args, root)
	if err != nil {
		t.Fatalf("rewriteToolArgsWithAIXFiles: %v", err)
	}
	if got := rewritten["content"]; got != "alpha\nbeta" {
		t.Fatalf("unexpected content rewrite: %#v", got)
	}
	nested, _ := rewritten["nested"].(map[string]interface{})
	if got := nested["prompt"]; got != "before\n# title\nafter" {
		t.Fatalf("unexpected nested prompt rewrite: %#v", got)
	}
}

func TestRewriteToolArgsWithAIXFilesRejectsEscape(t *testing.T) {
	root := t.TempDir()
	_, err := rewriteToolArgsWithAIXFiles(map[string]interface{}{
		"content": "<aixfile>../secret.txt</aixfile>",
	}, root)
	if err == nil {
		t.Fatal("expected escape error")
	}
}

func TestRewriteToolArgsWithAIXFilesRejectsUnclosedTag(t *testing.T) {
	root := t.TempDir()
	_, err := rewriteToolArgsWithAIXFiles(map[string]interface{}{
		"content": "<aixfile>payload.txt",
	}, root)
	if err == nil {
		t.Fatal("expected unclosed tag error")
	}
}

func TestResolveAIXFileReferenceAcceptsNewAndLegacyVisiblePrefixes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "payload.txt"), []byte("ok"), 0600); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	cases := []string{
		"/workdir/sourceCodeDir/payload.txt",
		"workdir/sourceCodeDir/payload.txt",
		"/sourceCodeDir/payload.txt",
		"sourceCodeDir/payload.txt",
	}
	for _, raw := range cases {
		got, err := resolveAIXFileReference(raw, root)
		if err != nil {
			t.Fatalf("resolveAIXFileReference(%q): %v", raw, err)
		}
		want := filepath.Join(root, "payload.txt")
		if got != want {
			t.Fatalf("resolveAIXFileReference(%q) = %q, want %q", raw, got, want)
		}
	}
}
