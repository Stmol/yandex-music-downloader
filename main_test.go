package main

import (
	"strings"
	"testing"
)

func TestIsKnownProblematicTerm(t *testing.T) {
	tests := []struct {
		name string
		term string
		want bool
	}{
		{name: "xterm", term: "xterm", want: true},
		{name: "xterm 256 color", term: "xterm-256color", want: false},
		{name: "screen 256 color", term: "screen-256color", want: false},
		{name: "tmux 256 color", term: "tmux-256color", want: false},
		{name: "empty", term: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isKnownProblematicTerm(tt.term); got != tt.want {
				t.Fatalf("isKnownProblematicTerm(%q) = %v, want %v", tt.term, got, tt.want)
			}
		})
	}
}

func TestProblematicTermWarning(t *testing.T) {
	warning := problematicTermWarning("xterm")

	for _, want := range []string{
		"TERM=xterm",
		"Colors, selected row highlighting, and focus/navigation",
		"export TERM=xterm-256color",
	} {
		if !strings.Contains(warning, want) {
			t.Fatalf("warning does not contain %q:\n%s", want, warning)
		}
	}
}
