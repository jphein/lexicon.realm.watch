package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lexicon "github.com/jphein/lexicon.realm.watch/go"
)

// fixtureCatalog returns a small in-memory catalog spanning the well-known
// status groups so the formatter exercises grouping, ordering, and optional
// fields without depending on the live catalog/projects.yaml.
func fixtureCatalog() *lexicon.Catalog {
	return &lexicon.Catalog{
		Path: "fixture",
		Projects: []*lexicon.Project{
			{
				ID: "alpha", CurrentName: "alpha.realm.watch",
				Kind: "realm-tool", Realm: "oracle", Status: "active",
				Description: "First fixture project.",
				Repo:        "https://github.com/jphein/alpha",
			},
			{
				ID: "zeta", CurrentName: "zeta",
				Kind: "library", Realm: "signal", Status: "active",
				Description: "Library that sorts after alpha.",
			},
			{
				ID: "homelab", CurrentName: "homelab",
				Kind: "service", Realm: "void", Status: "local-only",
				Description: "LAN-only service.",
				Notes:       "Lives on disks.",
			},
			{
				ID: "old-thing", CurrentName: "old-thing",
				Kind: "tool", Realm: "forge", Status: "archived",
				Description: "",
			},
			{
				ID: "mystery", CurrentName: "mystery",
				Kind: "tool", Realm: "?", Status: "experimental",
				Description: "Unknown realm — exercises pass-through statuses.",
			},
		},
	}
}

func TestCatalogRender_SkillOutput(t *testing.T) {
	got := string(renderSkill(fixtureCatalog()))

	wantContains := []string{
		"---\nname: project-catalog\n",
		"description: Use to look up JP's projects",
		"# Project Catalog\n",
		"## How to read\n",
		"## Projects (active)\n",
		"## Projects (local-only)\n",
		"## Projects (archived)\n",
		"## Projects (experimental)\n",
		"### alpha (`alpha.realm.watch`)",
		"- **Path:** `~/Projects/alpha.realm.watch/`",
		"- **Kind:** realm-tool · **Realm:** oracle · **Status:** active",
		"- First fixture project.",
		"- Repo: <https://github.com/jphein/alpha>",
		"### zeta\n",
		"### homelab\n",
		"- Notes: Lives on disks.",
		"**Realm:** —", // realm "?" passes through valueOrDash
	}
	for _, want := range wantContains {
		if !strings.Contains(got, want) {
			t.Errorf("renderSkill missing %q\n--- output ---\n%s", want, got)
		}
	}

	alphaIdx := strings.Index(got, "### alpha")
	zetaIdx := strings.Index(got, "### zeta")
	homelabIdx := strings.Index(got, "### homelab")
	if !(alphaIdx < zetaIdx && zetaIdx < homelabIdx) {
		t.Errorf("expected order alpha → zeta → homelab; got positions %d/%d/%d", alphaIdx, zetaIdx, homelabIdx)
	}

	if strings.Contains(got, "Repo: <~>") {
		t.Errorf("renderSkill should suppress placeholder repo `~`, got it in output")
	}
}

func TestCatalogRender_SkillEmptyCatalog(t *testing.T) {
	cat := &lexicon.Catalog{Path: "empty", Projects: nil}
	got := string(renderSkill(cat))
	if !strings.HasPrefix(got, "---\nname: project-catalog\n") {
		t.Errorf("empty render missing frontmatter; got:\n%s", got)
	}
	if !strings.Contains(got, "_No projects in the catalog yet._") {
		t.Errorf("empty render missing no-projects body; got:\n%s", got)
	}
}

func TestCatalogRender_SkillStatusGroupOrder(t *testing.T) {
	cat := fixtureCatalog()
	got := string(renderSkill(cat))
	groups := []string{
		"## Projects (active)",
		"## Projects (local-only)",
		"## Projects (archived)",
		"## Projects (experimental)",
	}
	last := -1
	for _, g := range groups {
		idx := strings.Index(got, g)
		if idx == -1 {
			t.Errorf("missing group header %q", g)
			continue
		}
		if idx <= last {
			t.Errorf("group %q out of order (idx=%d, prev=%d)", g, idx, last)
		}
		last = idx
	}
}

func TestCatalogRender_MDTable(t *testing.T) {
	got := string(renderMDTable(fixtureCatalog()))

	wantPrefix := "| Project | What | Key Tech | Notes |\n|---------|------|----------|-------|\n"
	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("md-table missing header; got:\n%s", got)
	}

	wantLines := []string{
		"| **alpha.realm.watch** | First fixture project. | — | [GitHub](https://github.com/jphein/alpha); Realm: oracle |",
		"| **homelab** | LAN-only service. | — | Realm: void; Lives on disks. |",
		"| **mystery** | Unknown realm — exercises pass-through statuses. | — | — |",
		"| **old-thing** | — | — | Realm: forge |",
		"| **zeta** | Library that sorts after alpha. | — | Realm: signal |",
	}
	for _, want := range wantLines {
		if !strings.Contains(got, want) {
			t.Errorf("md-table missing line:\n  %s\n--- output ---\n%s", want, got)
		}
	}

	alphaIdx := strings.Index(got, "| **alpha.realm.watch** |")
	homelabIdx := strings.Index(got, "| **homelab** |")
	zetaIdx := strings.Index(got, "| **zeta** |")
	if !(alphaIdx < homelabIdx && homelabIdx < zetaIdx) {
		t.Errorf("md-table not sorted by current_name (alpha→homelab→zeta); got %d/%d/%d", alphaIdx, homelabIdx, zetaIdx)
	}
}

func TestCatalogRender_MDTableEmpty(t *testing.T) {
	got := string(renderMDTable(&lexicon.Catalog{Path: "empty"}))
	if !strings.HasPrefix(got, "| Project |") {
		t.Errorf("empty md-table should still emit the header; got:\n%s", got)
	}
	rows := strings.Count(got, "\n| **")
	if rows != 0 {
		t.Errorf("empty md-table has %d data rows, want 0", rows)
	}
}

func TestCatalogRender_UnknownFormat(t *testing.T) {
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "projects.yaml")
	if err := os.WriteFile(catalogPath, []byte("projects: []\n"), 0o644); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
	var stdout, stderr bytes.Buffer
	rc := cmdCatalogRender([]string{"--catalog", catalogPath, "--format=garbage"}, &stdout, &stderr)
	if rc != 2 {
		t.Errorf("unknown format exit code = %d, want 2", rc)
	}
	if !strings.Contains(stderr.String(), `unknown --format "garbage"`) {
		t.Errorf("unknown format stderr missing message; got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "valid: skill, md-table") {
		t.Errorf("unknown format stderr missing valid list; got %q", stderr.String())
	}
}

func TestCatalogRender_WritesToOutPath(t *testing.T) {
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "projects.yaml")
	if err := os.WriteFile(catalogPath, []byte(`projects:
  - id: only
    current_name: only
    kind: tool
    realm: signal
    status: active
    description: Single fixture entry.
    prior_names: []
`), 0o644); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
	outPath := filepath.Join(dir, "skill.md")
	var stdout, stderr bytes.Buffer
	rc := cmdCatalogRender([]string{"--catalog", catalogPath, "--out", outPath}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("exit code = %d, stderr=%q", rc, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("--out should suppress stdout; got %d bytes", stdout.Len())
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read out: %v", err)
	}
	if !strings.Contains(string(body), "### only") {
		t.Errorf("out file missing project entry:\n%s", body)
	}
}
