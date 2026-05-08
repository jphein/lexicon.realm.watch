package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyClaudeMDSweep_RewritesReferences(t *testing.T) {
	dir := t.TempDir()
	src := `# CLAUDE.md — realm-portal

This is the realm-portal homelab portal.

## Tests

Run from realm-portal/:

` + "```" + `
cd ~/Projects/realm-portal && go test ./...
` + "```" + `
`
	mustWrite(t, filepath.Join(dir, "CLAUDE.md"), src)

	edited, err := applyClaudeMDSweep(dir, "realm-portal", "portal.realm.watch")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if !edited {
		t.Fatal("expected sweep to report edited=true")
	}
	got := mustRead(t, filepath.Join(dir, "CLAUDE.md"))
	if strings.Contains(got, "realm-portal") {
		t.Errorf("old name still present after sweep:\n%s", got)
	}
	if !strings.Contains(got, "portal.realm.watch") {
		t.Errorf("new name not present:\n%s", got)
	}
}

func TestApplyClaudeMDSweep_NoFile(t *testing.T) {
	dir := t.TempDir()
	edited, err := applyClaudeMDSweep(dir, "realm-portal", "portal.realm.watch")
	if err != nil {
		t.Fatalf("sweep against missing file should not error: %v", err)
	}
	if edited {
		t.Errorf("missing file should produce edited=false; got true")
	}
}

func TestApplyClaudeMDSweep_Idempotent(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "CLAUDE.md"), "Lives at portal.realm.watch.\n")

	edited, err := applyClaudeMDSweep(dir, "realm-portal", "portal.realm.watch")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if edited {
		t.Errorf("file already at new name should not be edited; got edited=true")
	}
}

// Step 4 reports the changes through the runbook driver.
func TestExecute_Step4_ActuallyEditsClaudeMD(t *testing.T) {
	projects := t.TempDir()
	newDir := filepath.Join(projects, "portal.realm.watch")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mustWrite(t, filepath.Join(newDir, "CLAUDE.md"), "# realm-portal\n\nDocs for realm-portal.\n")

	fake := newFakeFS()
	fake.addDir(newDir)
	env := newTestEnv(t, fake, projects)
	env.oldID = "realm-portal"
	env.newName = "portal.realm.watch"

	steps := buildRenameSteps()
	skip := skipAllExcept(4)
	if rc := executeRenamePlan(env, steps, skip); rc != 0 {
		t.Fatalf("step 4 failed; stderr=%q", env.stderr.(*bytes.Buffer).String())
	}
	got := mustRead(t, filepath.Join(newDir, "CLAUDE.md"))
	if strings.Contains(got, "realm-portal") {
		t.Errorf("CLAUDE.md still references old name: %q", got)
	}
}
