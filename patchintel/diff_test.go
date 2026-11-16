package patchintel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleDiff = `From faac4db133de8b0c9e4c2e2e4f7f0c1a2b3c4d5e Mon Sep 17 00:00:00 2001
From: Langflow <security@example.com>
Subject: fix: forbid arbitrary code execution in validate endpoint (CVE-2025-3248)

diff --git a/src/backend/base/langflow/api/v1/validate.py b/src/backend/base/langflow/api/v1/validate.py
index 1111111..2222222 100644
--- a/src/backend/base/langflow/api/v1/validate.py
+++ b/src/backend/base/langflow/api/v1/validate.py
@@ -10,8 +10,12 @@ async def validate_code(code: str):
     try:
-        exec(code)
+        safe = build_safe_execution_environment()
+        exec(code, safe)
     except Exception:
         return {"valid": False}
+    finally:
+        cleanup(safe)
     return {"valid": True}
`

func TestParseDiffExtractsHunksAndMetadata(t *testing.T) {
	profile, err := ParseDiff(sampleDiff)
	if err != nil {
		t.Fatalf("ParseDiff: %v", err)
	}
	if len(profile.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(profile.Hunks))
	}
	hunk := profile.Hunks[0]
	if hunk.File != "src/backend/base/langflow/api/v1/validate.py" {
		t.Fatalf("unexpected file: %q", hunk.File)
	}
	if hunk.OldStart != 10 || hunk.OldCount != 8 || hunk.NewStart != 10 || hunk.NewCount != 12 {
		t.Fatalf("unexpected hunk range: %+v", hunk)
	}
	if !strings.Contains(strings.Join(hunk.Lines, "\n"), "-        exec(code)") {
		t.Fatalf("expected removed exec line, got %v", hunk.Lines)
	}
	if profile.CVE != "CVE-2025-3248" {
		t.Fatalf("expected CVE extracted from subject, got %q", profile.CVE)
	}
	if profile.FixCommit != "faac4db133de8b0c9e4c2e2e4f7f0c1a2b3c4d5e" {
		t.Fatalf("expected fix commit, got %q", profile.FixCommit)
	}
	if len(profile.Files) != 1 || profile.Files[0] != hunk.File {
		t.Fatalf("unexpected files: %v", profile.Files)
	}
	if profile.ID == "" {
		t.Fatal("expected deterministic profile id")
	}
}

func TestParseDiffRejectsNonDiff(t *testing.T) {
	if _, err := ParseDiff("just some prose\nwith no hunks"); err == nil {
		t.Fatal("expected error for non-diff input")
	}
}

func TestApplyMetadataOverridesAndFillsGaps(t *testing.T) {
	profile, err := ParseDiff(sampleDiff)
	if err != nil {
		t.Fatalf("ParseDiff: %v", err)
	}
	ApplyMetadata(profile, "CVE-2025-9999", "GHSA-rvqx-wpfh-mfx7", "<1.3.0", "faac4db", "langflow")
	if profile.CVE != "CVE-2025-9999" {
		t.Fatalf("override CVE failed: %q", profile.CVE)
	}
	if profile.GHSA != "GHSA-rvqx-wpfh-mfx7" {
		t.Fatalf("override GHSA failed: %q", profile.GHSA)
	}
	if profile.AffectedVersions != "<1.3.0" || profile.FixCommit != "faac4db" || profile.Scene != "langflow" {
		t.Fatalf("unexpected metadata: %+v", profile)
	}
}

func TestIngestLocalReadsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fix.patch")
	if err := os.WriteFile(path, []byte(sampleDiff), 0o644); err != nil {
		t.Fatalf("write patch: %v", err)
	}
	profile, err := IngestLocal(path)
	if err != nil {
		t.Fatalf("IngestLocal: %v", err)
	}
	if profile.Source != "file" || profile.SourceURL != path {
		t.Fatalf("unexpected source metadata: %+v", profile)
	}
	if len(profile.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(profile.Hunks))
	}
}

func TestRegisterAndLoadProfile(t *testing.T) {
	t.Setenv("AIXVULN_DATA_DIR", t.TempDir())
	profile, err := ParseDiff(sampleDiff)
	if err != nil {
		t.Fatalf("ParseDiff: %v", err)
	}
	id, err := RegisterProfile(profile)
	if err != nil {
		t.Fatalf("RegisterProfile: %v", err)
	}
	if id != profile.ID {
		t.Fatalf("unexpected id: %q vs %q", id, profile.ID)
	}
	loaded, err := GetProfile(id)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if loaded.CVE != profile.CVE || len(loaded.Hunks) != len(profile.Hunks) {
		t.Fatalf("loaded profile mismatch: %+v", loaded)
	}
}
