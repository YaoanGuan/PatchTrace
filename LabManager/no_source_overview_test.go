package LabManager

import (
	"AIxVuln/ProjectManager"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildNoSourceProjectOverview(t *testing.T) {
	overview := buildNoSourceProjectOverview("对目标执行黑盒渗透测试")
	for _, required := range []string{
		"未提供源码",
		"空源码目录",
		"黑盒任务",
		"对目标执行黑盒渗透测试",
	} {
		if !strings.Contains(overview, required) {
			t.Fatalf("overview should contain %q, got %q", required, overview)
		}
	}
}

func TestBuildProjectOverviewFailureFallback(t *testing.T) {
	overview := buildProjectOverviewFailureFallback("分析目标 http://127.0.0.1:8080 并自动做题")
	for _, required := range []string{
		"项目速览未能成功生成",
		"保守调度",
		"直接中断任务",
		"分析目标 http://127.0.0.1:8080 并自动做题",
	} {
		if !strings.Contains(overview, required) {
			t.Fatalf("failure fallback overview should contain %q, got %q", required, overview)
		}
	}
	if !isProjectOverviewFailureFallback(overview) {
		t.Fatalf("expected failure fallback detector to match %q", overview)
	}
	if isProjectOverviewFailureFallback(buildNoSourceProjectOverview("test")) {
		t.Fatal("no-source overview should not be treated as failure fallback")
	}
}

func TestIsNoSourceProjectOverview(t *testing.T) {
	if !isNoSourceProjectOverview(buildNoSourceProjectOverview("test")) {
		t.Fatal("expected no-source overview detector to match fallback overview")
	}
	if isNoSourceProjectOverview(buildProjectOverviewFailureFallback("test")) {
		t.Fatal("failure fallback should not be treated as no-source overview")
	}
}

func TestGetProjectOverviewSuppressesStaleNoSourceOverviewWhenSourceExists(t *testing.T) {
	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	projectName := "overview-source-backed-" + strings.ReplaceAll(strings.ToLower(t.Name()), "/", "-")
	pm, err := ProjectManager.NewProjectManager(ProjectManager.ProjectConfig{
		ProjectName:         projectName,
		SourceCodeDir:       sourceDir,
		SkipProjectOverview: true,
		TaskContent:         "审计上传源码",
	})
	if err != nil {
		t.Fatalf("NewProjectManager returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(pm.GetProjectDir())
	})
	if pm.ShouldSkipProjectOverview() {
		t.Fatal("source-backed project should not skip overview after normalization")
	}

	lm := &LabManager{
		webGroup:        &WebSecurityGroup{pm: pm},
		projectOverview: buildNoSourceProjectOverview("审计上传源码"),
	}
	if got := lm.getProjectOverview(); got != "" {
		t.Fatalf("expected stale no-source overview to be suppressed, got %q", got)
	}
}
