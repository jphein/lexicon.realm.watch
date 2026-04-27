package main

import (
	"bytes"
	"path/filepath"
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

// Many CLI tests need vocabularies on disk. Tests run with cwd=go/cmd/lexicon,
// so the relative paths walk up two levels.
func vocabsArg() string  { return filepath.Join("..", "..", "..", "vocabularies") }
func recipesArg() string { return filepath.Join("..", "..", "..", "vocabularies", "recipes.yaml") }

func TestRoll_AgentEmitsNonEmpty(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"lexicon", "roll", "agent", "--vocabularies", vocabsArg()}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%q", code, stderr.String())
	}
	out := strings.TrimSpace(stdout.String())
	if out == "" {
		t.Error("empty output")
	}
}

func TestRoll_NRequestsMultipleCandidates(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"lexicon", "roll", "agent", "--n=3", "--vocabularies", vocabsArg()}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%q", code, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3", len(lines))
	}
}

func TestRoll_ProjectRequiresRealm(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"lexicon", "roll", "project", "--vocabularies", vocabsArg()}, &stdout, &stderr)
	if code == 0 {
		t.Error("expected non-zero exit when realm is missing")
	}
	if !strings.Contains(stderr.String(), "realm") {
		t.Errorf("stderr should mention realm: %q", stderr.String())
	}
}

func TestResolve_FoundByID(t *testing.T) {
	var stdout, stderr bytes.Buffer
	catalogPath := filepath.Join("..", "..", "..", "tests", "fixtures", "catalog-test.yaml")
	code := run([]string{"lexicon", "resolve", "clock", "--catalog", catalogPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "clock.realm.watch") {
		t.Errorf("output missing current name: %q", stdout.String())
	}
}

func TestResolve_FoundByPriorName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	catalogPath := filepath.Join("..", "..", "..", "tests", "fixtures", "catalog-test.yaml")
	code := run([]string{"lexicon", "resolve", "dreamspace", "--catalog", catalogPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "dreamscape.realm.watch") {
		t.Errorf("output missing current name: %q", stdout.String())
	}
}

func TestResolve_Unknown(t *testing.T) {
	var stdout, stderr bytes.Buffer
	catalogPath := filepath.Join("..", "..", "..", "tests", "fixtures", "catalog-test.yaml")
	code := run([]string{"lexicon", "resolve", "no-such-thing", "--catalog", catalogPath}, &stdout, &stderr)
	if code == 0 {
		t.Error("expected non-zero for unknown name")
	}
}
