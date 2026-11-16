package misc

import (
	"path/filepath"
	"strings"
)

const (
	ProjectWorkDirName   = "workdir"
	ProjectSourceDirName = "sourceCodeDir"

	SandboxWorkDir         = "/workdir"
	SandboxSourceDir       = "/workdir/sourceCodeDir"
	LegacySandboxSourceDir = "/sourceCodeDir"
)

func ProjectWorkDir(projectDir string) string {
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		return ""
	}
	return filepath.Join(projectDir, ProjectWorkDirName)
}

func ProjectSourceDir(projectDir string) string {
	workDir := ProjectWorkDir(projectDir)
	if workDir == "" {
		return ""
	}
	return filepath.Join(workDir, ProjectSourceDirName)
}

func NormalizeVisibleSourcePath(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "\\", "/")
	for _, prefix := range []string{
		SandboxSourceDir,
		strings.TrimPrefix(SandboxSourceDir, "/"),
		LegacySandboxSourceDir,
		strings.TrimPrefix(LegacySandboxSourceDir, "/"),
	} {
		if value == prefix {
			return ""
		}
		if strings.HasPrefix(value, prefix+"/") {
			return strings.TrimPrefix(value[len(prefix):], "/")
		}
	}
	return strings.TrimPrefix(value, "/")
}

func ContainsVisibleSourceRootReference(raw string) bool {
	value := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	if value == "" {
		return false
	}
	return strings.Contains(value, SandboxSourceDir) || strings.Contains(value, LegacySandboxSourceDir)
}
