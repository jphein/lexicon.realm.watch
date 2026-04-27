package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_HelpListsCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"lexicon", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("--help exit code = %d, want 0", code)
	}
	out := stdout.String()
	for _, cmd := range []string{"roll", "resolve", "list", "validate", "recipes", "vocabularies", "claim"} {
		if !strings.Contains(out, cmd) {
			t.Errorf("--help output missing %q", cmd)
		}
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"lexicon", "wat"}, &stdout, &stderr)
	if code == 0 {
		t.Error("expected non-zero exit for unknown command")
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Errorf("expected 'unknown command' in stderr, got: %q", stderr.String())
	}
}
