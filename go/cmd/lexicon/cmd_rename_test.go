package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lexicon "github.com/jphein/lexicon.realm.watch/go"
)

// ---------------------------------------------------------------------------
// fakeFS — in-memory stand-in for the renameFS interface so --execute tests
// never touch real ~/Projects or ~/.claude.
// ---------------------------------------------------------------------------

type fakeFS struct {
	entries  map[string]fakeEntry // path -> entry
	commands [][]string           // recorded fs.Run invocations
	cmdDirs  []string             // working dir captured for each Run / RunInDir call
	runErr   error
	runOut   []byte
}

type fakeEntry struct {
	mode    os.FileMode
	target  string // for symlinks
}

func newFakeFS() *fakeFS {
	return &fakeFS{entries: map[string]fakeEntry{}}
}

func (f *fakeFS) addDir(path string)     { f.entries[path] = fakeEntry{mode: os.ModeDir | 0o755} }
func (f *fakeFS) addSymlink(path, target string) {
	f.entries[path] = fakeEntry{mode: os.ModeSymlink | 0o777, target: target}
}

type fakeFileInfo struct {
	name string
	mode os.FileMode
}

func (fi fakeFileInfo) Name() string       { return fi.name }
func (fi fakeFileInfo) Size() int64        { return 0 }
func (fi fakeFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fi fakeFileInfo) IsDir() bool        { return fi.mode&os.ModeDir != 0 }
func (fi fakeFileInfo) Sys() any           { return nil }

func (f *fakeFS) Stat(p string) (os.FileInfo, error) {
	e, ok := f.entries[p]
	if !ok {
		return nil, &fs.PathError{Op: "stat", Path: p, Err: fs.ErrNotExist}
	}
	return fakeFileInfo{name: filepath.Base(p), mode: e.mode}, nil
}

func (f *fakeFS) Lstat(p string) (os.FileInfo, error) {
	return f.Stat(p)
}

func (f *fakeFS) Rename(o, n string) error {
	e, ok := f.entries[o]
	if !ok {
		return &fs.PathError{Op: "rename", Path: o, Err: fs.ErrNotExist}
	}
	delete(f.entries, o)
	f.entries[n] = e
	return nil
}

func (f *fakeFS) Symlink(target, link string) error {
	if _, exists := f.entries[link]; exists {
		return &fs.PathError{Op: "symlink", Path: link, Err: fs.ErrExist}
	}
	f.entries[link] = fakeEntry{mode: os.ModeSymlink | 0o777, target: target}
	return nil
}

func (f *fakeFS) Run(name string, args ...string) ([]byte, error) {
	f.commands = append(f.commands, append([]string{name}, args...))
	f.cmdDirs = append(f.cmdDirs, "")
	return f.runOut, f.runErr
}

func (f *fakeFS) RunInDir(dir, name string, args ...string) ([]byte, error) {
	f.commands = append(f.commands, append([]string{name}, args...))
	f.cmdDirs = append(f.cmdDirs, dir)
	return f.runOut, f.runErr
}

// ---------------------------------------------------------------------------
// --plan tests
// ---------------------------------------------------------------------------

func TestRename_PlanPrintsElevenSteps(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"lexicon", "rename", "realmwatch", "watch.realm.watch",
		"--plan",
		"--projects-dir", "/tmp/fake-projects",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	for n := 1; n <= 11; n++ {
		needle := fmt.Sprintf("%2d.", n)
		if !strings.Contains(out, needle) {
			t.Errorf("plan output missing step %d marker %q\n%s", n, needle, out)
		}
	}
	for _, want := range []string{
		"Local directory rename",
		"Transitional symlink",
		"Package metadata sweep",
		"CLAUDE.md sweeps",
		"GitHub repo rename",
		"DNS / Caddy reminder",
		"Outline wiki path",
		"Claude Code session storage rename",
		"MemPalace wing rename",
		"lexicon claim",
		"Manual-verify checklist",
		"realmwatch → watch.realm.watch",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("plan output missing %q\n--full output--\n%s", want, out)
		}
	}
}

func TestRename_PlanIsDefault(t *testing.T) {
	// No --plan or --execute flag; should default to plan.
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"lexicon", "rename", "old", "new",
		"--projects-dir", "/tmp/fake",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "rename plan:") {
		t.Errorf("expected default to be plan; got: %s", stdout.String())
	}
}

func TestRename_PlanWithSkipMarksSteps(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"lexicon", "rename", "old", "new",
		"--plan", "--skip=5", "--skip=6",
		"--projects-dir", "/tmp/fake",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "[~]  5.") {
		t.Errorf("step 5 should be marked skipped: %q", out)
	}
	if !strings.Contains(out, "[~]  6.") {
		t.Errorf("step 6 should be marked skipped: %q", out)
	}
	if !strings.Contains(out, "[ ]  1.") {
		t.Errorf("step 1 should be unmarked: %q", out)
	}
}

func TestRename_RejectsBothPlanAndExecute(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"lexicon", "rename", "old", "new",
		"--plan", "--execute",
	}, &stdout, &stderr)
	if code == 0 {
		t.Error("expected non-zero when both --plan and --execute given")
	}
	if !strings.Contains(stderr.String(), "mutually exclusive") {
		t.Errorf("stderr should mention mutual exclusion: %q", stderr.String())
	}
}

func TestRename_BadSkipValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"lexicon", "rename", "old", "new",
		"--plan", "--skip=99",
	}, &stdout, &stderr)
	if code == 0 {
		t.Error("expected non-zero exit for skip=99")
	}
}

func TestRename_SkipElevenIsValid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"lexicon", "rename", "old", "new",
		"--plan", "--skip=11",
		"--projects-dir", "/tmp/fake",
	}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("skip=11 should be valid; exit %d; stderr=%q", code, stderr.String())
	}
}

func TestRename_RequiresTwoPositionals(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"lexicon", "rename", "only-one", "--plan"}, &stdout, &stderr)
	if code == 0 {
		t.Error("expected non-zero exit when missing newName")
	}
}

// ---------------------------------------------------------------------------
// --execute tests (use renameEnv directly + fakeFS, so runbook stays in-memory)
// ---------------------------------------------------------------------------

func TestExecute_RenamesDirectoryAndCreatesSymlink(t *testing.T) {
	fake := newFakeFS()
	projects := "/projects"
	fake.addDir(filepath.Join(projects, "realmwatch"))

	env := newTestEnv(t, fake, projects)
	env.oldID = "realmwatch"
	env.newName = "watch.realm.watch"

	steps := buildRenameSteps()
	// Execute only steps 1 and 2 to isolate filesystem behavior.
	skip := skipAllExcept(1, 2)
	rc := executeRenamePlan(env, steps, skip)
	if rc != 0 {
		t.Fatalf("execute returned %d; stderr=%q", rc, env.stderr.(*bytes.Buffer).String())
	}

	if _, ok := fake.entries[filepath.Join(projects, "realmwatch")]; !ok {
		t.Error("symlink at old path should exist after step 2")
	}
	newDir, ok := fake.entries[filepath.Join(projects, "watch.realm.watch")]
	if !ok {
		t.Error("new dir should exist after step 1")
	}
	if newDir.mode&os.ModeDir == 0 {
		t.Errorf("new path should be a directory, mode=%v", newDir.mode)
	}
	if link := fake.entries[filepath.Join(projects, "realmwatch")]; link.mode&os.ModeSymlink == 0 {
		t.Errorf("old path should be a symlink, mode=%v", link.mode)
	}
}

func TestExecute_GHCommandRecorded(t *testing.T) {
	tmpCat := writeTempCatalog(t, `projects:
  - id: realmwatch
    current_name: realmwatch
    kind: service
    realm: void
    domain: ~
    repo: https://github.com/jphein/realmwatch
    description: x
    created: 2025-09-01
    prior_names: []
    status: active
`)
	fake := newFakeFS()
	projects := "/projects"
	fake.addDir(filepath.Join(projects, "watch.realm.watch"))
	fake.runOut = []byte("ok")

	env := newTestEnv(t, fake, projects)
	env.oldID = "realmwatch"
	env.newName = "watch.realm.watch"
	env.catalogPath = tmpCat

	steps := buildRenameSteps()
	skip := skipAllExcept(5)
	if rc := executeRenamePlan(env, steps, skip); rc != 0 {
		t.Fatalf("execute step 5 failed; stderr=%q", env.stderr.(*bytes.Buffer).String())
	}

	if len(fake.commands) != 1 {
		t.Fatalf("expected exactly 1 command, got %d: %v", len(fake.commands), fake.commands)
	}
	cmd := fake.commands[0]
	if cmd[0] != "gh" {
		t.Errorf("expected gh command, got %v", cmd)
	}
	if !contains(cmd, "rename") || !contains(cmd, "watch.realm.watch") {
		t.Errorf("gh command missing rename or new name: %v", cmd)
	}
}

// TestExecute_GHCommandHasNoCFlag is a regression test for issue #1: gh
// has no -C flag. The runbook used to invoke `gh -C <dir> repo rename …`
// which fails on every invocation. It must use cmd.Dir / RunInDir instead.
func TestExecute_GHCommandHasNoCFlag(t *testing.T) {
	tmpCat := writeTempCatalog(t, `projects:
  - id: realmwatch
    current_name: realmwatch
    kind: service
    realm: void
    domain: ~
    repo: https://github.com/jphein/realmwatch
    description: x
    created: 2025-09-01
    prior_names: []
    status: active
`)
	fake := newFakeFS()
	projects := "/projects"
	fake.addDir(filepath.Join(projects, "watch.realm.watch"))
	fake.runOut = []byte("ok")

	env := newTestEnv(t, fake, projects)
	env.oldID = "realmwatch"
	env.newName = "watch.realm.watch"
	env.catalogPath = tmpCat

	steps := buildRenameSteps()
	skip := skipAllExcept(5)
	if rc := executeRenamePlan(env, steps, skip); rc != 0 {
		t.Fatalf("step 5 should succeed; stderr=%q", env.stderr.(*bytes.Buffer).String())
	}
	if len(fake.commands) != 1 {
		t.Fatalf("expected 1 command, got %d: %v", len(fake.commands), fake.commands)
	}
	cmd := fake.commands[0]
	if contains(cmd, "-C") {
		t.Errorf("gh command must not include unsupported -C flag: %v", cmd)
	}
	if want := filepath.Join(projects, "watch.realm.watch"); fake.cmdDirs[0] != want {
		t.Errorf("gh should run with cwd=%q; got %q", want, fake.cmdDirs[0])
	}
}

// TestExecute_GHSkipsWhenNoRemote is a regression test for issue #1: when
// a project has no GitHub remote (catalog `repo:` is empty), step 5 should
// log a skip and return nil rather than attempting `gh repo rename` and
// failing.
func TestExecute_GHSkipsWhenNoRemote(t *testing.T) {
	tmpCat := writeTempCatalog(t, `projects:
  - id: realm-portal
    current_name: realm-portal
    kind: tool
    realm: forge
    domain: ~
    repo: ~
    description: x
    created: 2026-04-01
    prior_names: []
    status: local-only
`)
	fake := newFakeFS()
	projects := "/projects"
	fake.addDir(filepath.Join(projects, "portal.realm.watch"))

	env := newTestEnv(t, fake, projects)
	env.oldID = "realm-portal"
	env.newName = "portal.realm.watch"
	env.catalogPath = tmpCat

	steps := buildRenameSteps()
	skip := skipAllExcept(5)
	rc := executeRenamePlan(env, steps, skip)
	if rc != 0 {
		t.Fatalf("step 5 should not error when no remote; stderr=%q", env.stderr.(*bytes.Buffer).String())
	}
	if len(fake.commands) != 0 {
		t.Errorf("gh must not run when project has no remote; commands=%v", fake.commands)
	}
	stdout := env.stdout.(*bytes.Buffer).String()
	if !strings.Contains(stdout, "skip") && !strings.Contains(stdout, "no remote") {
		t.Errorf("step 5 should log a skip notice; got: %s", stdout)
	}
}

// TestExecute_GHCreateRemote_RunsWhenFlagSet exercises issue #4: when a
// project has no GitHub remote and --create-remote is set, step 5 invokes
// `gh repo create` from the project dir.
func TestExecute_GHCreateRemote_RunsWhenFlagSet(t *testing.T) {
	tmpCat := writeTempCatalog(t, `projects:
  - id: realm-portal
    current_name: realm-portal
    kind: tool
    realm: forge
    domain: ~
    repo: ~
    description: Unified homelab portal
    created: 2026-04-01
    prior_names: []
    status: local-only
`)
	fake := newFakeFS()
	projects := "/projects"
	fake.addDir(filepath.Join(projects, "portal.realm.watch"))
	fake.runOut = []byte("Created jphein/portal.realm.watch")

	env := newTestEnv(t, fake, projects)
	env.oldID = "realm-portal"
	env.newName = "portal.realm.watch"
	env.catalogPath = tmpCat
	env.createRemote = true

	steps := buildRenameSteps()
	skip := skipAllExcept(5)
	if rc := executeRenamePlan(env, steps, skip); rc != 0 {
		t.Fatalf("step 5 should succeed; stderr=%q", env.stderr.(*bytes.Buffer).String())
	}
	if len(fake.commands) != 1 {
		t.Fatalf("expected 1 gh invocation; got %d: %v", len(fake.commands), fake.commands)
	}
	cmd := fake.commands[0]
	if !contains(cmd, "create") || !contains(cmd, "jphein/portal.realm.watch") {
		t.Errorf("expected gh repo create jphein/portal.realm.watch …; got %v", cmd)
	}
	if !contains(cmd, "--private") {
		t.Errorf("expected default visibility --private; got %v", cmd)
	}
	if !contains(cmd, "--source=.") || !contains(cmd, "--push") {
		t.Errorf("expected --source=. --push; got %v", cmd)
	}
	if want := filepath.Join(projects, "portal.realm.watch"); fake.cmdDirs[0] != want {
		t.Errorf("gh should run from %q; got %q", want, fake.cmdDirs[0])
	}
}

func TestExecute_GHCreateRemote_PublicFlag(t *testing.T) {
	tmpCat := writeTempCatalog(t, `projects:
  - id: realm-portal
    current_name: realm-portal
    kind: tool
    realm: forge
    domain: ~
    repo: ~
    description: x
    created: 2026-04-01
    prior_names: []
    status: local-only
`)
	fake := newFakeFS()
	projects := "/projects"
	fake.addDir(filepath.Join(projects, "portal.realm.watch"))
	fake.runOut = []byte("Created")

	env := newTestEnv(t, fake, projects)
	env.oldID = "realm-portal"
	env.newName = "portal.realm.watch"
	env.catalogPath = tmpCat
	env.createRemote = true
	env.publicRemote = true

	steps := buildRenameSteps()
	if rc := executeRenamePlan(env, steps, skipAllExcept(5)); rc != 0 {
		t.Fatalf("step 5 failed; stderr=%q", env.stderr.(*bytes.Buffer).String())
	}
	cmd := fake.commands[0]
	if !contains(cmd, "--public") {
		t.Errorf("expected --public; got %v", cmd)
	}
	if contains(cmd, "--private") {
		t.Errorf("--private should not appear when --public set; got %v", cmd)
	}
}

// TestExecute_GHCreateRemote_UpdatesCatalogRepoField verifies that after gh
// repo create succeeds, the catalog's repo: field is populated so subsequent
// runs (or step 9's record) treats the project as having a remote.
func TestExecute_GHCreateRemote_UpdatesCatalogRepoField(t *testing.T) {
	tmpCat := writeTempCatalog(t, `projects:
  - id: realm-portal
    current_name: realm-portal
    kind: tool
    realm: forge
    domain: ~
    repo: ~
    description: x
    created: 2026-04-01
    prior_names: []
    status: local-only
`)
	fake := newFakeFS()
	projects := "/projects"
	fake.addDir(filepath.Join(projects, "portal.realm.watch"))
	fake.runOut = []byte("Created")

	env := newTestEnv(t, fake, projects)
	env.oldID = "realm-portal"
	env.newName = "portal.realm.watch"
	env.catalogPath = tmpCat
	env.createRemote = true

	steps := buildRenameSteps()
	if rc := executeRenamePlan(env, steps, skipAllExcept(5)); rc != 0 {
		t.Fatalf("step 5 failed; stderr=%q", env.stderr.(*bytes.Buffer).String())
	}
	cat, err := lexicon.LoadCatalog(tmpCat)
	if err != nil {
		t.Fatalf("reload catalog: %v", err)
	}
	proj, ok := cat.Resolve("realm-portal")
	if !ok {
		t.Fatal("project lost from catalog")
	}
	want := "https://github.com/jphein/portal.realm.watch"
	if proj.Repo != want {
		t.Errorf("repo not updated: %q (want %q)", proj.Repo, want)
	}
}

// TestExecute_GHExistingRemote_UpdatesCatalogRepoField is the issue #6
// regression. When a project already has a GitHub remote, step 5 calls
// `gh repo rename`. The catalog's repo: field used to keep the OLD URL —
// gh's HTTP redirect made things still work, but the canonical record was
// stale. After the fix it should reflect the new URL.
func TestExecute_GHExistingRemote_UpdatesCatalogRepoField(t *testing.T) {
	tmpCat := writeTempCatalog(t, `projects:
  - id: realm-sigil
    current_name: realm-sigil
    kind: library
    realm: forge
    domain: ~
    repo: https://github.com/jphein/realm-sigil
    description: x
    created: 2026-04-01
    prior_names: []
    status: active
`)
	fake := newFakeFS()
	projects := "/projects"
	fake.addDir(filepath.Join(projects, "sigil.realm.watch"))
	fake.runOut = []byte("ok")

	env := newTestEnv(t, fake, projects)
	env.oldID = "realm-sigil"
	env.newName = "sigil.realm.watch"
	env.catalogPath = tmpCat

	steps := buildRenameSteps()
	if rc := executeRenamePlan(env, steps, skipAllExcept(5)); rc != 0 {
		t.Fatalf("step 5 should succeed; stderr=%q", env.stderr.(*bytes.Buffer).String())
	}

	cat, err := lexicon.LoadCatalog(tmpCat)
	if err != nil {
		t.Fatalf("reload catalog: %v", err)
	}
	proj, ok := cat.Resolve("realm-sigil")
	if !ok {
		t.Fatal("project lost from catalog")
	}
	want := "https://github.com/jphein/sigil.realm.watch"
	if proj.Repo != want {
		t.Errorf("repo field stale after rename: %q (want %q)", proj.Repo, want)
	}
}

// TestScrubLocalOnlyFromNotes covers the issue #7 patterns observed across
// JP's catalog: whole-content "Local only" qualifiers, leading qualifier
// followed by a real sentence, and a trailing parenthetical.
func TestScrubLocalOnlyFromNotes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"whole_localonly", "Local only", ""},
		{"whole_localonly_period", "Local only.", ""},
		{"whole_no_remote_paren", "Local only (no remote)", ""},
		{"whole_no_remote_paren_period", "Local only (no remote).", ""},
		{"leading_local_only", "Local only. Companion piece to `oracle-mcp` UI", "Companion piece to `oracle-mcp` UI"},
		{"leading_no_remote", "Local only (no remote). Phase 1 shipped, Phase 2 in flight", "Phase 1 shipped, Phase 2 in flight"},
		{"leading_no_remote_with_path", "Local only (no remote). `deploy.sh` pushes to `openclaw` host", "`deploy.sh` pushes to `openclaw` host"},
		{"trailing_paren", "Some note about deployment (no remote)", "Some note about deployment"},
		{"unrelated_remote_mention", "Deploys to remote VM at 10.0.6.137", "Deploys to remote VM at 10.0.6.137"},
		{"local_only_in_middle", "Some preamble. Local only. trailing", "Some preamble. Local only. trailing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := scrubLocalOnlyFromNotes(tc.in)
			if got != tc.want {
				t.Errorf("scrubLocalOnlyFromNotes(%q)\n  got:  %q\n  want: %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestExecute_GHCreateRemote_ScrubsLocalOnlyNotes is the issue #7 regression.
// When --create-remote adds a remote to a project whose notes start with
// "Local only" or end with "(no remote)", that qualifier must be stripped so
// the catalog doesn't self-contradict (repo URL set + notes claiming no remote).
func TestExecute_GHCreateRemote_ScrubsLocalOnlyNotes(t *testing.T) {
	tmpCat := writeTempCatalog(t, `projects:
  - id: bestiary
    current_name: bestiary
    kind: tool
    realm: oracle
    domain: ~
    repo: ~
    description: x
    created: 2026-04-01
    prior_names: []
    status: local-only
    notes: "Local only (no remote). Deploys to Alpine VM ` + "`jp@10.0.6.137:/opt/bestiary`" + `"
`)
	fake := newFakeFS()
	projects := "/projects"
	fake.addDir(filepath.Join(projects, "bestiary.realm.watch"))
	fake.runOut = []byte("Created")

	env := newTestEnv(t, fake, projects)
	env.oldID = "bestiary"
	env.newName = "bestiary.realm.watch"
	env.catalogPath = tmpCat
	env.createRemote = true

	steps := buildRenameSteps()
	if rc := executeRenamePlan(env, steps, skipAllExcept(5)); rc != 0 {
		t.Fatalf("step 5 failed; stderr=%q", env.stderr.(*bytes.Buffer).String())
	}
	cat, err := lexicon.LoadCatalog(tmpCat)
	if err != nil {
		t.Fatalf("reload catalog: %v", err)
	}
	proj, ok := cat.Resolve("bestiary")
	if !ok {
		t.Fatal("project lost from catalog")
	}
	wantRepo := "https://github.com/jphein/bestiary.realm.watch"
	if proj.Repo != wantRepo {
		t.Errorf("repo not set: %q (want %q)", proj.Repo, wantRepo)
	}
	wantNotes := "Deploys to Alpine VM `jp@10.0.6.137:/opt/bestiary`"
	if proj.Notes != wantNotes {
		t.Errorf("notes not scrubbed:\n  got:  %q\n  want: %q", proj.Notes, wantNotes)
	}
}

func TestExecute_MempalaceWingRenamed(t *testing.T) {
	fake := newFakeFS()
	fake.runOut = []byte("Renamed 42 drawers from wing 'realmwatch' to 'watch.realm.watch'")

	env := newTestEnv(t, fake, "/projects")
	env.oldID = "realmwatch"
	env.newName = "watch.realm.watch"

	steps := buildRenameSteps()
	skip := skipAllExcept(9)
	if rc := executeRenamePlan(env, steps, skip); rc != 0 {
		t.Fatalf("step 9 failed; stderr=%q", env.stderr.(*bytes.Buffer).String())
	}
	if len(fake.commands) != 1 {
		t.Fatalf("expected 1 command, got %d: %v", len(fake.commands), fake.commands)
	}
	cmd := fake.commands[0]
	if cmd[0] != "mempalace" || !contains(cmd, "rename-wing") {
		t.Errorf("expected mempalace rename-wing command; got %v", cmd)
	}
	if !contains(cmd, "--from") || !contains(cmd, "realmwatch") {
		t.Errorf("missing --from realmwatch: %v", cmd)
	}
	if !contains(cmd, "--to") || !contains(cmd, "watch.realm.watch") {
		t.Errorf("missing --to watch.realm.watch: %v", cmd)
	}
}

func TestExecute_MempalaceWingSkipsWhenNoWing(t *testing.T) {
	fake := newFakeFS()
	fake.runOut = []byte("0 drawers matched")
	fake.runErr = fmt.Errorf("exit status 1")

	env := newTestEnv(t, fake, "/projects")
	env.oldID = "no-such-project"
	env.newName = "renamed"

	steps := buildRenameSteps()
	skip := skipAllExcept(9)
	rc := executeRenamePlan(env, steps, skip)
	if rc != 0 {
		t.Errorf("missing wing should not fail the runbook; rc=%d", rc)
	}
	stdout := env.stdout.(*bytes.Buffer).String()
	if !strings.Contains(stdout, "skipping") {
		t.Errorf("expected skip notice; got: %s", stdout)
	}
}

func writeTempCatalog(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "projects.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write tmp catalog: %v", err)
	}
	return p
}

func TestExecute_LexiconClaimUpdatesCatalog(t *testing.T) {
	tmpCat := filepath.Join(t.TempDir(), "projects.yaml")
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "fixtures", "catalog-test.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(tmpCat, src, 0o644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}

	fake := newFakeFS()
	env := newTestEnv(t, fake, "/projects")
	env.oldID = "realmwatch"
	env.newName = "watch.realm.watch"
	env.catalogPath = tmpCat
	env.reason = "test rename"

	steps := buildRenameSteps()
	skip := skipAllExcept(10)
	if rc := executeRenamePlan(env, steps, skip); rc != 0 {
		t.Fatalf("execute step 10 failed; stderr=%q", env.stderr.(*bytes.Buffer).String())
	}

	cat, err := lexicon.LoadCatalog(tmpCat)
	if err != nil {
		t.Fatalf("reload catalog: %v", err)
	}
	p, ok := cat.Resolve("realmwatch")
	if !ok {
		t.Fatal("project realmwatch lost from catalog")
	}
	if p.CurrentName != "watch.realm.watch" {
		t.Errorf("rename not persisted: current_name=%q", p.CurrentName)
	}
}

func TestExecute_DeclineSkipsStep(t *testing.T) {
	fake := newFakeFS()
	projects := "/projects"
	fake.addDir(filepath.Join(projects, "realmwatch"))

	env := newTestEnv(t, fake, projects)
	env.oldID = "realmwatch"
	env.newName = "watch.realm.watch"
	env.confirm = func(string) bool { return false } // always decline

	steps := buildRenameSteps()
	skip := skipAllExcept(1)
	rc := executeRenamePlan(env, steps, skip)
	if rc != 0 {
		t.Fatalf("declined steps should not be reported as errors; rc=%d", rc)
	}
	// Old dir should still exist because user declined the rename.
	if _, ok := fake.entries[filepath.Join(projects, "realmwatch")]; !ok {
		t.Error("declined step should leave filesystem untouched")
	}
}

func TestExecute_StepFailureReportedNonZero(t *testing.T) {
	fake := newFakeFS()
	// Don't create the source dir — step 1 will fail with "source not found".
	env := newTestEnv(t, fake, "/projects")
	env.oldID = "missing-project"
	env.newName = "new-name"

	steps := buildRenameSteps()
	skip := skipAllExcept(1)
	rc := executeRenamePlan(env, steps, skip)
	if rc == 0 {
		t.Error("expected non-zero rc when a step errors")
	}
}

func TestExecute_LocalDirIdempotent(t *testing.T) {
	// Destination already present — step 1 should report the idempotent skip
	// rather than blindly clobbering or hanging.
	fake := newFakeFS()
	projects := "/projects"
	fake.addDir(filepath.Join(projects, "realmwatch"))
	fake.addDir(filepath.Join(projects, "watch.realm.watch"))

	env := newTestEnv(t, fake, projects)
	env.oldID = "realmwatch"
	env.newName = "watch.realm.watch"

	steps := buildRenameSteps()
	skip := skipAllExcept(1)
	rc := executeRenamePlan(env, steps, skip)
	if rc == 0 {
		t.Error("expected non-zero when destination already exists")
	}
	if !strings.Contains(env.stderr.(*bytes.Buffer).String(), "already exists") {
		t.Errorf("expected idempotent message, got: %s", env.stderr.(*bytes.Buffer).String())
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestEnv(t *testing.T, fake *fakeFS, projects string) *renameEnv {
	t.Helper()
	return &renameEnv{
		projectsDir: projects,
		sessionsDir: filepath.Join(t.TempDir(), "sessions"),
		fs:          fake,
		stdout:      &bytes.Buffer{},
		stderr:      &bytes.Buffer{},
		confirm:     func(string) bool { return true },
	}
}

func skipAllExcept(steps ...int) map[int]bool {
	keep := map[int]bool{}
	for _, s := range steps {
		keep[s] = true
	}
	skip := map[int]bool{}
	for n := 1; n <= 11; n++ {
		if !keep[n] {
			skip[n] = true
		}
	}
	return skip
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// Sanity: ensure errors.Is on PathError still works in tests (no-op assertion
// to keep the import live should we ever switch to typed checks).
var _ = errors.Is
