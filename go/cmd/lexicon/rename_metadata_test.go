package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyPackageMetadataSweep_GoMod(t *testing.T) {
	dir := t.TempDir()
	src := "module github.com/jphein/realm-portal\n\ngo 1.21\n"
	mustWrite(t, filepath.Join(dir, "go.mod"), src)

	edited, err := applyPackageMetadataSweep(dir, "realm-portal", "portal.realm.watch")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if !contains(edited, "go.mod") {
		t.Errorf("expected go.mod in edited list: %v", edited)
	}
	got := mustRead(t, filepath.Join(dir, "go.mod"))
	want := "module github.com/jphein/portal.realm.watch\n\ngo 1.21\n"
	if got != want {
		t.Errorf("go.mod mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestApplyPackageMetadataSweep_GoModSubpath(t *testing.T) {
	// realm-sigil's go.mod uses `module github.com/jphein/realm-sigil/go`.
	// The replacement must touch only the segment matching the project name.
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module github.com/jphein/realm-sigil/go\n")

	if _, err := applyPackageMetadataSweep(dir, "realm-sigil", "sigil.realm.watch"); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	got := mustRead(t, filepath.Join(dir, "go.mod"))
	want := "module github.com/jphein/sigil.realm.watch/go\n"
	if got != want {
		t.Errorf("subpath go.mod mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestApplyPackageMetadataSweep_GoModBoundarySafe(t *testing.T) {
	// Old name as substring (not segment) must NOT be touched.
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module github.com/jphein/foo-realm-portal-bar\n")

	if _, err := applyPackageMetadataSweep(dir, "realm-portal", "portal.realm.watch"); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	got := mustRead(t, filepath.Join(dir, "go.mod"))
	if !strings.Contains(got, "foo-realm-portal-bar") {
		t.Errorf("substring match should not be replaced; got: %q", got)
	}
}

func TestApplyPackageMetadataSweep_GoImports(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module github.com/jphein/realm-portal\n")
	gosrc := `package main

import (
	"fmt"
	"github.com/jphein/realm-portal/internal/handler"
)

func main() {
	fmt.Println(handler.Greet())
}
`
	mustWrite(t, filepath.Join(dir, "main.go"), gosrc)

	if _, err := applyPackageMetadataSweep(dir, "realm-portal", "portal.realm.watch"); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	got := mustRead(t, filepath.Join(dir, "main.go"))
	if !strings.Contains(got, "github.com/jphein/portal.realm.watch/internal/handler") {
		t.Errorf("import not rewritten: %q", got)
	}
	if strings.Contains(got, "realm-portal") {
		t.Errorf("old import path still present: %q", got)
	}
}

func TestApplyPackageMetadataSweep_PackageJSON(t *testing.T) {
	dir := t.TempDir()
	src := `{
  "name": "realm-portal",
  "version": "1.0.0",
  "scripts": {
    "build": "vite build"
  }
}
`
	mustWrite(t, filepath.Join(dir, "package.json"), src)

	if _, err := applyPackageMetadataSweep(dir, "realm-portal", "portal.realm.watch"); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	got := mustRead(t, filepath.Join(dir, "package.json"))

	var raw map[string]any
	if err := json.Unmarshal([]byte(got), &raw); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, got)
	}
	if raw["name"] != "portal.realm.watch" {
		t.Errorf("name not updated: %+v", raw)
	}
	// Other fields preserved.
	if raw["version"] != "1.0.0" {
		t.Errorf("version field lost: %+v", raw)
	}
}

func TestApplyPackageMetadataSweep_PyprojectTOML(t *testing.T) {
	dir := t.TempDir()
	src := `[project]
name = "realm-portal"
version = "0.1.0"
description = "x"

[tool.something]
name = "should-not-touch"
`
	mustWrite(t, filepath.Join(dir, "pyproject.toml"), src)

	if _, err := applyPackageMetadataSweep(dir, "realm-portal", "portal.realm.watch"); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	got := mustRead(t, filepath.Join(dir, "pyproject.toml"))
	if !strings.Contains(got, `name = "portal.realm.watch"`) {
		t.Errorf("[project] name not updated:\n%s", got)
	}
	if !strings.Contains(got, `name = "should-not-touch"`) {
		t.Errorf("[tool.something] name was incorrectly modified:\n%s", got)
	}
}

func TestApplyPackageMetadataSweep_VersionJSON(t *testing.T) {
	dir := t.TempDir()
	src := `{
  "name": "realm-portal",
  "realm": "forge"
}
`
	mustWrite(t, filepath.Join(dir, "version.json"), src)

	if _, err := applyPackageMetadataSweep(dir, "realm-portal", "portal.realm.watch"); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	got := mustRead(t, filepath.Join(dir, "version.json"))
	var raw map[string]any
	if err := json.Unmarshal([]byte(got), &raw); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if raw["name"] != "portal.realm.watch" {
		t.Errorf("name not updated: %+v", raw)
	}
}

func TestApplyPackageMetadataSweep_Idempotent(t *testing.T) {
	// Running twice yields same result and second call edits nothing.
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module github.com/jphein/realm-portal\n")

	if _, err := applyPackageMetadataSweep(dir, "realm-portal", "portal.realm.watch"); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	edited, err := applyPackageMetadataSweep(dir, "realm-portal", "portal.realm.watch")
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if len(edited) != 0 {
		t.Errorf("second pass should be a no-op; edited=%v", edited)
	}
}

func TestApplyPackageMetadataSweep_NoMetadataFiles(t *testing.T) {
	// A bare project dir with no metadata files should sweep cleanly.
	dir := t.TempDir()
	edited, err := applyPackageMetadataSweep(dir, "realm-portal", "portal.realm.watch")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(edited) != 0 {
		t.Errorf("expected no edits, got %v", edited)
	}
}

// TestApplyPackageMetadataSweep_NestedPerLanguageDirs is the issue #5
// regression. realm-sigil's layout puts go.mod under go/, package.json under
// js/, and pyproject.toml under python/. A top-level-only sweep misses all
// three.
func TestApplyPackageMetadataSweep_NestedPerLanguageDirs(t *testing.T) {
	dir := t.TempDir()
	mustWriteNested(t, filepath.Join(dir, "go", "go.mod"),
		"module github.com/jphein/realm-sigil/go\n")
	mustWriteNested(t, filepath.Join(dir, "js", "package.json"),
		`{"name": "realm-sigil", "version": "1.0.0"}`+"\n")
	mustWriteNested(t, filepath.Join(dir, "python", "pyproject.toml"),
		"[project]\nname = \"realm-sigil\"\nversion = \"0.1.0\"\n")
	mustWrite(t, filepath.Join(dir, "version.json"),
		`{"name": "realm-sigil", "realm": "forge"}`+"\n")

	edited, err := applyPackageMetadataSweep(dir, "realm-sigil", "sigil.realm.watch")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	for _, want := range []string{"go/go.mod", "js/package.json", "python/pyproject.toml", "version.json"} {
		if !contains(edited, want) {
			t.Errorf("expected %q in edited list: %v", want, edited)
		}
	}

	if got := mustRead(t, filepath.Join(dir, "go", "go.mod")); !strings.Contains(got, "github.com/jphein/sigil.realm.watch/go") {
		t.Errorf("nested go.mod not rewritten: %q", got)
	}
	if got := mustRead(t, filepath.Join(dir, "js", "package.json")); !strings.Contains(got, `"name": "sigil.realm.watch"`) {
		t.Errorf("nested package.json not rewritten: %q", got)
	}
	if got := mustRead(t, filepath.Join(dir, "python", "pyproject.toml")); !strings.Contains(got, `name = "sigil.realm.watch"`) {
		t.Errorf("nested pyproject.toml not rewritten: %q", got)
	}
}

// Excluded directory names (vendor, node_modules, venv, hidden dirs) must not
// have their metadata files rewritten. A vendored copy of go.mod with the old
// name is meant to stay frozen.
func TestApplyPackageMetadataSweep_SkipsExcludedDirs(t *testing.T) {
	dir := t.TempDir()
	mustWriteNested(t, filepath.Join(dir, "vendor", "pinned", "go.mod"),
		"module github.com/jphein/realm-portal/vendored\n")
	mustWriteNested(t, filepath.Join(dir, "node_modules", "pkg", "package.json"),
		`{"name": "realm-portal"}`+"\n")
	mustWriteNested(t, filepath.Join(dir, ".cache", "package.json"),
		`{"name": "realm-portal"}`+"\n")

	edited, err := applyPackageMetadataSweep(dir, "realm-portal", "portal.realm.watch")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(edited) != 0 {
		t.Errorf("expected no edits inside excluded dirs; got %v", edited)
	}
	// Files should be byte-identical.
	for _, p := range []string{
		filepath.Join(dir, "vendor", "pinned", "go.mod"),
		filepath.Join(dir, "node_modules", "pkg", "package.json"),
		filepath.Join(dir, ".cache", "package.json"),
	} {
		if !strings.Contains(mustRead(t, p), "realm-portal") {
			t.Errorf("excluded file was modified: %s", p)
		}
	}
}

// Files buried more than 3 levels deep are out of scope. Most metadata files
// live near the top of a project; deeply nested matches are likelier to be
// fixtures or vendored copies than canonical metadata.
func TestApplyPackageMetadataSweep_DepthLimit(t *testing.T) {
	dir := t.TempDir()
	deep := filepath.Join(dir, "a", "b", "c", "d", "package.json")
	mustWriteNested(t, deep, `{"name": "realm-portal"}`+"\n")

	edited, err := applyPackageMetadataSweep(dir, "realm-portal", "portal.realm.watch")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(edited) != 0 {
		t.Errorf("expected depth limit to skip deeply-nested file; got %v", edited)
	}
}

// Step 3 reports the changes through the runbook driver.
func TestExecute_Step3_ActuallyEditsFiles(t *testing.T) {
	projects := t.TempDir()
	newDir := filepath.Join(projects, "portal.realm.watch")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mustWrite(t, filepath.Join(newDir, "go.mod"), "module github.com/jphein/realm-portal\n")

	fake := newFakeFS()
	fake.addDir(newDir)
	env := newTestEnv(t, fake, projects)
	env.oldID = "realm-portal"
	env.newName = "portal.realm.watch"

	steps := buildRenameSteps()
	skip := skipAllExcept(3)
	if rc := executeRenamePlan(env, steps, skip); rc != 0 {
		t.Fatalf("step 3 failed; stderr=%q", env.stderr.(*bytes.Buffer).String())
	}
	got := mustRead(t, filepath.Join(newDir, "go.mod"))
	if !strings.Contains(got, "module github.com/jphein/portal.realm.watch") {
		t.Errorf("go.mod was not edited: %q", got)
	}
}

// mustRead is local to this file; mustWrite is reused from cmd_import_test.go.
func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// mustWriteNested is mustWrite + MkdirAll on the parent — needed for tests
// that exercise the recursive sweep with files in subdirectories.
func mustWriteNested(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
	}
	mustWrite(t, p, body)
}
