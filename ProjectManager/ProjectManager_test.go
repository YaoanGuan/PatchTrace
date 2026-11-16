package ProjectManager

import (
	"AIxVuln/dockerManager"
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"os"
	"path/filepath"
	"testing"
)

func TestSourceWorkspaceHasEntries(t *testing.T) {
	root := t.TempDir()
	if sourceWorkspaceHasEntries(root) {
		t.Fatal("empty workspace should not be treated as having source entries")
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("create .git dir: %v", err)
	}
	if sourceWorkspaceHasEntries(root) {
		t.Fatal("workspace with only .git metadata should still be treated as empty")
	}
	if err := os.WriteFile(filepath.Join(root, "target.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write workspace file: %v", err)
	}
	if !sourceWorkspaceHasEntries(root) {
		t.Fatal("workspace with files should be treated as having source entries")
	}
}

func TestShouldSkipProjectOverview(t *testing.T) {
	root := t.TempDir()
	if shouldSkipProjectOverview(false, root) {
		t.Fatal("empty workspace should not skip project overview by default")
	}
	if !shouldSkipProjectOverview(true, root) {
		t.Fatal("explicit skip flag should skip project overview only for empty workspaces")
	}
	if err := os.WriteFile(filepath.Join(root, "artifact.bin"), []byte("x"), 0644); err != nil {
		t.Fatalf("write workspace file: %v", err)
	}
	if shouldSkipProjectOverview(false, root) {
		t.Fatal("non-empty workspace should not skip project overview by default")
	}
	if shouldSkipProjectOverview(true, root) {
		t.Fatal("source-backed workspace should not skip project overview even when explicit skip is requested")
	}
}

func TestNormalizeProjectOverviewSkipKeepsBlackBoxTargetOverviewEnabled(t *testing.T) {
	root := t.TempDir()
	if got := normalizeProjectOverviewSkip(true, root, "渗透目标 http://154.36.152.176:7860/ 并做黑盒侦查"); got {
		t.Fatalf("empty-source black-box task with explicit target should not skip overview, got %v", got)
	}
	if got := normalizeProjectOverviewSkip(true, root, "没有源码，也没有任何目标信息"); !got {
		t.Fatalf("empty-source task without target hint should still allow explicit skip, got %v", got)
	}
}

func TestSetProjectDirRecomputesWorkAndSourceLayout(t *testing.T) {
	pm := &ProjectManager{}
	root := filepath.Join(t.TempDir(), "demo-project")
	pm.SetProjectDir(root)
	if pm.GetProjectDir() != root {
		t.Fatalf("project dir mismatch: got %q want %q", pm.GetProjectDir(), root)
	}
	if pm.GetWorkDir() != filepath.Join(root, misc.ProjectWorkDirName) {
		t.Fatalf("work dir mismatch: got %q", pm.GetWorkDir())
	}
	if pm.GetSourceCodeDir() != filepath.Join(root, misc.ProjectWorkDirName, misc.ProjectSourceDirName) {
		t.Fatalf("source dir mismatch: got %q", pm.GetSourceCodeDir())
	}
}

func TestEnsureProjectOverviewSandboxReusesExistingSandbox(t *testing.T) {
	projectName := "project_overview_existing_sandbox"
	taskManager.SetSandbox(projectName, &dockerManager.Sandbox{ContainerId: "sandbox-overview"})
	defer taskManager.RemoveSandbox(projectName)

	pm := &ProjectManager{projectName: projectName}
	if err := pm.ensureProjectOverviewSandbox(); err != nil {
		t.Fatalf("ensureProjectOverviewSandbox returned error: %v", err)
	}
	if pm.sandboxID != "sandbox-overview" {
		t.Fatalf("unexpected sandbox id: got %q", pm.sandboxID)
	}
}
