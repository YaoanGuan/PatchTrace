package DecisionBrain

import (
	"AIxVuln/taskManager"
	"testing"
)

func TestAddReportCallsReportObserver(t *testing.T) {
	db := NewDecisionBrain("report-observer-test", "task", nil)

	var observed taskManager.ReportMeta
	called := false
	db.SetReportObserver(func(report taskManager.ReportMeta) {
		called = true
		observed = report
	})

	db.AddReport(taskManager.ReportMeta{
		Path:    "/tmp/report-observer-test.md",
		Summary: "测试报告",
	})

	if !called {
		t.Fatal("expected report observer to be called")
	}
	if observed.ReportID != "R.1" {
		t.Fatalf("unexpected report id: %s", observed.ReportID)
	}
	if observed.FileName != "report-observer-test.md" {
		t.Fatalf("unexpected file name: %s", observed.FileName)
	}
	if observed.Summary != "测试报告" {
		t.Fatalf("unexpected summary: %s", observed.Summary)
	}
}
