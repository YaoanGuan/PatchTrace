package misc

import "testing"

func TestGetEmptyProjectSkipProjectOverview(t *testing.T) {
	oldValue := GetConfigValueDefault("misc", "EMPTY_PROJECT_SKIP_OVERVIEW", "")
	t.Cleanup(func() {
		if err := SetConfigValue("misc", "EMPTY_PROJECT_SKIP_OVERVIEW", oldValue); err != nil {
			t.Fatalf("restore EMPTY_PROJECT_SKIP_OVERVIEW: %v", err)
		}
	})

	testCases := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "true", value: "true", want: true},
		{name: "false", value: "false", want: false},
		{name: "one", value: "1", want: true},
		{name: "invalid", value: "not-a-bool", want: false},
	}

	for _, tc := range testCases {
		if err := SetConfigValue("misc", "EMPTY_PROJECT_SKIP_OVERVIEW", tc.value); err != nil {
			t.Fatalf("%s: SetConfigValue failed: %v", tc.name, err)
		}
		if got := GetEmptyProjectSkipProjectOverview(); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}
