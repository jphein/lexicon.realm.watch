package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// makeImportFixture builds a small directory tree of fake "projects" that the
// import command can scan, exercising every metadata source (package.json,
// pyproject.toml, go.mod, README, git remote) plus the empty-dir fallback.
//
// Returns the root path. Tests treat this as ~/Projects.
func makeImportFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Project A: package.json + README + (later) git remote.
	a := filepath.Join(root, "alpha")
	mustMkdir(t, a)
	mustWrite(t, filepath.Join(a, "package.json"),
		`{"name":"@jphein/alpha","version":"0.1.0"}`)
	mustWrite(t, filepath.Join(a, "README.md"),
		"# Alpha\n\n[![badge](x.svg)](y)\n\nA tiny alpha service. With two sentences.\n\nSecond paragraph ignored.\n")
	initGitRepo(t, a, "https://github.com/jphein/alpha")

	// Project B: pyproject.toml + README, no git.
	b := filepath.Join(root, "beta")
	mustMkdir(t, b)
	mustWrite(t, filepath.Join(b, "pyproject.toml"),
		"[build-system]\nrequires = [\"hatchling\"]\n\n[project]\nname = \"beta-thing\"\nversion = \"0.0.1\"\n")
	mustWrite(t, filepath.Join(b, "README.md"),
		"Just a paragraph, no heading.\n")

	// Project C: go.mod only, no README, no remote.
	c := filepath.Join(root, "gamma")
	mustMkdir(t, c)
	mustWrite(t, filepath.Join(c, "go.mod"),
		"module github.com/jphein/gamma-tool\n\ngo 1.22\n")

	// Project D: empty directory (should still produce a stub).
	d := filepath.Join(root, "delta")
	mustMkdir(t, d)

	// Hidden directory — should be skipped by default.
	hidden := filepath.Join(root, ".cache")
	mustMkdir(t, hidden)
	mustWrite(t, filepath.Join(hidden, "package.json"), `{"name":"hidden"}`)

	// A loose file at root — must not appear as a project.
	mustWrite(t, filepath.Join(root, "stray.txt"), "loose")

	return root
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
}

func mustWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

// initGitRepo creates a minimal git repo with a fake `origin` remote, so the
// import command can pick up the URL via `git remote get-url origin`. We
// don't commit anything — `remote get-url` works on an empty repo.
func initGitRepo(t *testing.T, dir, remote string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed in PATH")
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"remote", "add", "origin", remote},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestCatalogImport_StdoutEmitsValidYAML(t *testing.T) {
	root := makeImportFixture(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"lexicon", "catalog", "import", "--from", root,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%q", code, stderr.String())
	}

	var parsed struct {
		Projects []map[string]any `yaml:"projects"`
	}
	if err := yaml.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid YAML: %v\n---\n%s", err, stdout.String())
	}

	got := map[string]map[string]any{}
	for _, p := range parsed.Projects {
		id, _ := p["id"].(string)
		got[id] = p
	}

	// alpha: should pull name from package.json, repo from git, status active.
	a, ok := got["alpha"]
	if !ok {
		t.Fatalf("expected 'alpha' in output: %s", stdout.String())
	}
	if a["current_name"] != "@jphein/alpha" {
		t.Errorf("alpha current_name = %q, want @jphein/alpha", a["current_name"])
	}
	if a["repo"] != "https://github.com/jphein/alpha" {
		t.Errorf("alpha repo = %q, want https://github.com/jphein/alpha", a["repo"])
	}
	if a["status"] != "active" {
		t.Errorf("alpha status = %q, want active (has remote)", a["status"])
	}
	desc, _ := a["description"].(string)
	if !strings.HasPrefix(desc, "A tiny alpha service.") {
		t.Errorf("alpha description = %q, want first sentence of README", desc)
	}

	// beta: pyproject name, no remote.
	b, ok := got["beta"]
	if !ok {
		t.Fatalf("expected 'beta' in output")
	}
	if b["current_name"] != "beta-thing" {
		t.Errorf("beta current_name = %q, want beta-thing", b["current_name"])
	}
	if b["status"] != "local-only" {
		t.Errorf("beta status = %q, want local-only", b["status"])
	}
	if _, has := b["repo"]; has {
		t.Errorf("beta should have no repo field, got %v", b["repo"])
	}

	// gamma: go.mod last segment.
	c, ok := got["gamma"]
	if !ok {
		t.Fatalf("expected 'gamma' in output")
	}
	if c["current_name"] != "gamma-tool" {
		t.Errorf("gamma current_name = %q, want gamma-tool", c["current_name"])
	}

	// delta: empty dir → falls back to dirname.
	d, ok := got["delta"]
	if !ok {
		t.Fatalf("expected 'delta' in output")
	}
	if d["current_name"] != "delta" {
		t.Errorf("delta current_name = %q, want delta", d["current_name"])
	}

	// realm sentinel — every entry should have realm: ?
	for id, p := range got {
		if p["realm"] != "?" {
			t.Errorf("%s realm = %q, want ?", id, p["realm"])
		}
	}

	// Hidden dir must not appear.
	if _, has := got[".cache"]; has {
		t.Error("hidden .cache directory should not be imported by default")
	}
	if _, has := got["cache"]; has {
		t.Error("hidden .cache directory should not be imported by default")
	}
}

func TestCatalogImport_DryRunListsButDoesNotEmitYAML(t *testing.T) {
	root := makeImportFixture(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"lexicon", "catalog", "import", "--from", root, "--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "would import") {
		t.Errorf("dry-run output should announce 'would import': %q", out)
	}
	for _, want := range []string{"alpha", "beta", "gamma", "delta"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run missing %q in: %q", want, out)
		}
	}
	// Dry-run must not produce parseable YAML output.
	if strings.Contains(out, "projects:") {
		t.Errorf("dry-run should not emit YAML body: %q", out)
	}
}

func TestCatalogImport_OutWritesFile(t *testing.T) {
	root := makeImportFixture(t)
	outPath := filepath.Join(t.TempDir(), "draft.yaml")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"lexicon", "catalog", "import", "--from", root, "--out", outPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), outPath) {
		t.Errorf("expected stdout to mention output path: %q", stdout.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read out: %v", err)
	}
	var parsed struct {
		Projects []map[string]any `yaml:"projects"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output yaml invalid: %v", err)
	}
	if len(parsed.Projects) < 4 {
		t.Errorf("expected ≥4 projects in output, got %d", len(parsed.Projects))
	}
}

func TestCatalogImport_RefusesToOverwriteCatalogProjectsYAML(t *testing.T) {
	// Stage a fake repo: <tmp>/catalog/projects.yaml already exists.
	root := makeImportFixture(t)
	catalogDir := filepath.Join(t.TempDir(), "catalog")
	mustMkdir(t, catalogDir)
	target := filepath.Join(catalogDir, "projects.yaml")
	mustWrite(t, target, "projects: []\n")

	before, _ := os.ReadFile(target)

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"lexicon", "catalog", "import", "--from", root, "--out", target,
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit when --out points at existing catalog/projects.yaml")
	}
	if !strings.Contains(stderr.String(), "refusing to overwrite") {
		t.Errorf("expected refusal message; got stderr=%q", stderr.String())
	}
	after, _ := os.ReadFile(target)
	if !bytes.Equal(before, after) {
		t.Errorf("target file was modified despite refusal")
	}
}

func TestCatalogImport_OverwriteAllowedIfFileMissing(t *testing.T) {
	// catalog/projects.yaml does not exist yet — `--out catalog/projects.yaml`
	// is a legitimate first-time bootstrap and must succeed.
	root := makeImportFixture(t)
	catalogDir := filepath.Join(t.TempDir(), "catalog")
	mustMkdir(t, catalogDir)
	target := filepath.Join(catalogDir, "projects.yaml")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"lexicon", "catalog", "import", "--from", root, "--out", target,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("first-time bootstrap should succeed; exit %d stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("expected file at %s: %v", target, err)
	}
}

func TestCatalogImport_HelpExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"lexicon", "catalog"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("`catalog` with no subcommand should print help and exit 0; got %d stderr=%q",
			code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "import") {
		t.Errorf("expected catalog help to mention import: %q", stdout.String())
	}
}

func TestCatalogImport_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"lexicon", "catalog", "wat"}, &stdout, &stderr)
	if code == 0 {
		t.Error("expected non-zero exit for unknown catalog subcommand")
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"clock.realm.watch", "clock-realm-watch"},
		{"Foo Bar", "foo-bar"},
		{"already-good", "already-good"},
		{"My_Project", "my-project"},
		{"--leading", "leading"},
		{"trailing--", "trailing"},
	}
	for _, c := range cases {
		if got := slugify(c.in); got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFirstParagraph_SkipsHeadingsAndBadges(t *testing.T) {
	in := `# Title

![badge](x)
[![ci](y)](z)

The actual prose. Second sentence here.

Second paragraph.`
	got := firstParagraph(in)
	if got != "The actual prose." {
		t.Errorf("firstParagraph = %q, want %q", got, "The actual prose.")
	}
}
