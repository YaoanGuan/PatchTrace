package llm

import (
	"errors"
	"testing"
)

func TestIsContextLimitError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "maximum context length", err: errors.New("This model's maximum context length is 128000 tokens"), want: true},
		{name: "context_length_exceeded", err: errors.New("invalid_request_error: context_length_exceeded"), want: true},
		{name: "reduce messages", err: errors.New("Please reduce the length of the messages"), want: true},
		{name: "other error", err: errors.New("connection reset by peer"), want: false},
		{name: "nil", err: nil, want: false},
	}
	for _, tc := range cases {
		if got := IsContextLimitError(tc.err); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}
