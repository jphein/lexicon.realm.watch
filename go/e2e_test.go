package lexicon

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestE2E exercises the full lexicon chain — vocabulary load, recipes load,
// roll (random + seeded), catalog load + queries, validate, and the CLI
// binary's roll/recipes/validate/rename --plan/catalog import commands. If
// this passes from a clean checkout, the v1 surface is wired end to end.
//
// We keep this in package lexicon (rather than a black-box _test package) so
// we can reuse the existing loadLiveVocabsCombined helper.
func TestE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in -short mode")
	}

	// --- 1. vocabulary -----------------------------------------------------
	v, err := loadLiveVocabsCombined()
	if err != nil {
		t.Fatalf("vocab: %v", err)
	}
	if _, ok := v.Group("realms", "fantasy"); !ok {
		t.Fatal("expected fantasy realm to be present in live vocabularies")
	}

	// --- 2. recipes --------------------------------------------------------
	rb, err := LoadRecipeBook(filepath.Join("..", "vocabularies", "recipes.yaml"))
	if err != nil {
		t.Fatalf("LoadRecipeBook: %v", err)
	}
	for _, want := range []string{"project", "agent", "branch", "entity"} {
		if !rb.Has(want) {
			t.Errorf("recipes.yaml missing %q recipe", want)
		}
	}

	// --- 3. RollN unique candidates ---------------------------------------
	candidates, err := rb.RollN("project", v, 5, RollOptions{Realm: "fantasy"})
	if err != nil {
		t.Fatalf("RollN: %v", err)
	}
	if len(candidates) != 5 {
		t.Fatalf("RollN(project, 5) returned %d candidates, want 5", len(candidates))
	}
	uniq := map[string]bool{}
	for _, c := range candidates {
		if c == "" {
			t.Errorf("RollN produced empty candidate in %v", candidates)
		}
		if !strings.Contains(c, "-") {
			t.Errorf("project candidate %q should contain a dash separator", c)
		}
		if uniq[c] {
			t.Errorf("RollN produced duplicate candidate %q in %v", c, candidates)
		}
		uniq[c] = true
	}

	// --- 4. RollSeeded smoke (full fixture parity is in TestRollSeeded_MatchesFixture) ---
	a, err := rb.RollSeeded("agent", v, "lexicon-e2e", RollOptions{})
	if err != nil {
		t.Fatalf("RollSeeded: %v", err)
	}
	b, err := rb.RollSeeded("agent", v, "lexicon-e2e", RollOptions{})
	if err != nil {
		t.Fatalf("RollSeeded second call: %v", err)
	}
	if a != b {
		t.Errorf("RollSeeded non-deterministic: %q vs %q", a, b)
	}
	if !strings.Contains(a, "_") {
		t.Errorf("agent name should contain underscore: %q", a)
	}

	// --- 5. catalog --------------------------------------------------------
	cat, err := LoadCatalog(filepath.Join("..", "catalog", "projects.yaml"))
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	// --- 6. Resolve ('realmwatch' must exist; it's the seed entry) --------
	rw, ok := cat.Resolve("realmwatch")
	if !ok {
		t.Fatal("Resolve(realmwatch) returned !ok — catalog should seed realmwatch")
	}
	if rw.ID != "realmwatch" {
		t.Errorf("Resolve(realmwatch).ID = %q, want %q", rw.ID, "realmwatch")
	}

	// --- 7. ByRealm — oracle realm should have at least lexicon + realm-sigil ---
	oracle := cat.ByRealm("oracle")
	if len(oracle) < 2 {
		t.Fatalf("ByRealm(oracle) returned %d, want >= 2", len(oracle))
	}
	wantOracleIDs := map[string]bool{"lexicon": false, "realm-sigil": false}
	for _, p := range oracle {
		if _, want := wantOracleIDs[p.ID]; want {
			wantOracleIDs[p.ID] = true
		}
	}
	for id, found := range wantOracleIDs {
		if !found {
			t.Errorf("ByRealm(oracle) missing expected project id %q", id)
		}
	}

	// --- 8. Validate — live catalog must have zero error-level issues.
	// Warnings (e.g. pending_realm sentinels for in-flight migrations) are
	// allowed: they're surfaced visibly but don't fail validation.
	issues := Validate(cat, v, rb)
	for _, i := range issues {
		if i.Severity != SeverityWarning {
			t.Errorf("validate error-level issue: %s", i)
		}
	}

	// --- 9. CLI binary smoke ----------------------------------------------
	bin := buildLexiconBinary(t)

	runCLI(t, bin, "validate exits 0",
		[]string{"validate",
			"--catalog", "../catalog/projects.yaml",
			"--vocabularies", "../vocabularies"},
		".", 0, []string{"OK"})

	runCLI(t, bin, "recipes lists known recipes",
		[]string{"recipes", "--vocabularies", "../vocabularies"},
		".", 0, []string{"project", "agent", "branch", "entity"})

	runCLI(t, bin, "roll agent emits non-empty",
		[]string{"roll", "agent", "--vocabularies", "../vocabularies"},
		".", 0, []string{"_"})

	// --- 10. CLI rename --plan smoke --------------------------------------
	runCLI(t, bin, "rename --plan prints the 10-step runbook",
		[]string{
			"rename", "realmwatch", "watch.realm.watch",
			"--plan",
			"--projects-dir", "/tmp/lexicon-e2e-fake-projects",
		},
		".", 0,
		[]string{
			"rename plan: realmwatch → watch.realm.watch",
			"Local directory rename",
			"GitHub repo rename",
			"Manual-verify checklist",
		})

	// --- 11. CLI catalog import --dry-run on a tmpdir copy ---------------
	tmp := t.TempDir()
	fakeProjects := filepath.Join(tmp, "Projects")
	makeFakeProjectTree(t, fakeProjects)
	runCLI(t, bin, "catalog import --dry-run lists fake projects",
		[]string{"catalog", "import", "--from", fakeProjects, "--dry-run"},
		tmp, 0,
		[]string{"would import", "alpha-tool", "beta-lib"})

	// --- 12. Round-trip Claim against a tmpdir catalog copy --------------
	tmpCat := filepath.Join(tmp, "projects.yaml")
	srcCat, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "catalog-test.yaml"))
	if err != nil {
		t.Fatalf("read catalog-test.yaml: %v", err)
	}
	if err := os.WriteFile(tmpCat, srcCat, 0o644); err != nil {
		t.Fatalf("write tmp catalog: %v", err)
	}
	runCLI(t, bin, "claim records a rename in tmpdir catalog",
		[]string{
			"claim", "watch.realm.watch",
			"--renames", "realmwatch",
			"--reason", "e2e",
			"--catalog", tmpCat,
		},
		".", 0, nil)
	reloaded, err := LoadCatalog(tmpCat)
	if err != nil {
		t.Fatalf("reload tmp catalog: %v", err)
	}
	rw2, ok := reloaded.Resolve("realmwatch")
	if !ok {
		t.Fatal("after CLI claim, Resolve(realmwatch) should still find the entry by id")
	}
	if rw2.CurrentName != "watch.realm.watch" {
		t.Errorf("CLI claim didn't persist rename: current_name = %q", rw2.CurrentName)
	}
}

// buildLexiconBinary compiles ./cmd/lexicon into a t.TempDir-scoped binary so
// each test run gets a fresh build (no stale binary surviving a code edit).
func buildLexiconBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "lexicon")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("/snap/bin/go", "build", "-o", bin, "./cmd/lexicon")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Fall back to plain "go" if /snap/bin/go is unavailable (e.g. CI).
		cmd = exec.Command("go", "build", "-o", bin, "./cmd/lexicon")
		cmd.Dir = "."
		out, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("build lexicon binary: %v\n%s", err, out)
		}
	}
	return bin
}

// runCLI runs the freshly-built binary, asserts the exit code, asserts each
// substring is present in stdout, and surfaces stderr on failure.
func runCLI(t *testing.T, bin, label string, args []string, cwd string, wantCode int, wantSubs []string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	gotCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			gotCode = exitErr.ExitCode()
		} else {
			t.Fatalf("%s: run binary: %v", label, err)
		}
	}
	if gotCode != wantCode {
		t.Errorf("%s: exit code = %d, want %d\nstdout: %s\nstderr: %s",
			label, gotCode, wantCode, stdout.String(), stderr.String())
		return
	}
	if wantCode == 0 && strings.TrimSpace(stdout.String()) == "" && len(wantSubs) > 0 {
		t.Errorf("%s: empty stdout, want non-empty containing %v", label, wantSubs)
		return
	}
	for _, sub := range wantSubs {
		if !strings.Contains(stdout.String(), sub) {
			t.Errorf("%s: stdout missing %q\nfull stdout:\n%s\nstderr:\n%s",
				label, sub, stdout.String(), stderr.String())
		}
	}
}

// makeFakeProjectTree builds a couple of project dirs the import walker can
// chew on. Each project gets enough metadata to drive at least one inference
// branch (package.json or go.mod plus a README).
func makeFakeProjectTree(t *testing.T, root string) {
	t.Helper()
	mk := func(rel, contents string) {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}

	// alpha-tool — has package.json, README.
	mk("alpha-tool/package.json", `{"name":"alpha-tool","version":"0.0.1"}`)
	mk("alpha-tool/README.md", "# alpha-tool\n\nA tiny tool for the e2e fake tree.\n")

	// beta-lib — has go.mod, README.
	mk("beta-lib/go.mod", "module example.com/beta-lib\n\ngo 1.22\n")
	mk("beta-lib/README.md", "# beta-lib\n\nA library used by the e2e fake tree.\n")
}
