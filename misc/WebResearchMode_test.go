package misc

import "testing"

func TestNormalizeWebResearchMode(t *testing.T) {
	cases := map[string]string{
		"":              WebResearchModeAudit,
		"audit":         WebResearchModeAudit,
		"whitebox":      WebResearchModeAudit,
		"审计":            WebResearchModeAudit,
		"comprehensive": WebResearchModeComprehensive,
		"combined":      WebResearchModeComprehensive,
		"综合":            WebResearchModeComprehensive,
	}
	for input, want := range cases {
		got, err := NormalizeWebResearchMode(input)
		if err != nil {
			t.Fatalf("NormalizeWebResearchMode(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("NormalizeWebResearchMode(%q) = %q, want %q", input, got, want)
		}
	}
	if _, err := NormalizeWebResearchMode("unknown"); err == nil {
		t.Fatal("expected invalid mode to return error")
	}
}
