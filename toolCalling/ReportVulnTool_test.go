package toolCalling

import "testing"

func TestValidateReportContentAcceptsClosedFence(t *testing.T) {
	report := "# 标题\n\n```java\nSystem.out.println(\"ok\");\n```\n"
	if err := validateReportContent(report); err != nil {
		t.Fatalf("expected valid report, got error: %v", err)
	}
}

func TestValidateReportContentRejectsUnclosedFence(t *testing.T) {
	report := "# 标题\n\n```java\nSystem.out.println(\"oops\");\n"
	if err := validateReportContent(report); err == nil {
		t.Fatal("expected truncated report to be rejected")
	}
}
