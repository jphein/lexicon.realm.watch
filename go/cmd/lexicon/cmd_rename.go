package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	lexicon "github.com/jphein/lexicon.realm.watch/go"
)

// renameFS abstracts the side-effecting operations the runbook performs so
// that --execute can be exercised in os.TempDir() without touching ~/Projects.
type renameFS interface {
	Stat(path string) (os.FileInfo, error)
	Rename(oldPath, newPath string) error
	Symlink(oldname, newname string) error
	Lstat(path string) (os.FileInfo, error)
	Run(name string, args ...string) ([]byte, error)
	RunInDir(dir, name string, args ...string) ([]byte, error)
}

type osRenameFS struct{}

func (osRenameFS) Stat(p string) (os.FileInfo, error)  { return os.Stat(p) }
func (osRenameFS) Rename(o, n string) error            { return os.Rename(o, n) }
func (osRenameFS) Symlink(o, n string) error           { return os.Symlink(o, n) }
func (osRenameFS) Lstat(p string) (os.FileInfo, error) { return os.Lstat(p) }
func (osRenameFS) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}
func (osRenameFS) RunInDir(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

// renameStep is one item in the 10-step runbook.
type renameStep struct {
	num         int
	title       string
	detail      string
	auto        bool // true if --execute can do this for you; false = manual reminder
	skipKey     string
	doFunc      func(env *renameEnv) error
}

// renameEnv carries the context shared across steps in --execute mode.
type renameEnv struct {
	oldID         string
	newName       string
	projectsDir   string // default: ~/Projects
	sessionsDir   string // default: ~/.claude/projects
	catalogPath   string
	reason        string
	fs            renameFS
	stdout        io.Writer
	stderr        io.Writer
	confirm       func(prompt string) bool
	now           func() string
}

func cmdRename(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rename", flag.ContinueOnError)
	fs.SetOutput(stderr)
	plan := fs.Bool("plan", false, "print the runbook checklist (default if neither --plan nor --execute given)")
	execute := fs.Bool("execute", false, "run the runbook with per-step confirmation")
	yes := fs.Bool("yes", false, "auto-confirm every prompt (with --execute)")
	skipMulti := newStringSliceFlag()
	fs.Var(skipMulti, "skip", "step number to skip (repeatable; e.g. --skip=5 --skip=6)")
	catalog := fs.String("catalog", "", "path to catalog/projects.yaml")
	_ = fs.String("vocabularies", "", "ignored — accepted for consistency with other commands")
	projectsDir := fs.String("projects-dir", "", "override ~/Projects (mainly for tests)")
	sessionsDir := fs.String("sessions-dir", "", "override ~/.claude/projects (mainly for tests)")
	reason := fs.String("reason", "", "reason text attached to the prior_name record (step 9)")

	// Allow positionals before/after flags.
	var positional []string
	flagArgs := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a != "" && a[0] != '-' && len(positional) < 2 {
			positional = append(positional, a)
			continue
		}
		flagArgs = append(flagArgs, a)
	}
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if len(positional) != 2 {
		fmt.Fprintln(stderr, "Usage: lexicon rename <old-id> <new-name> [--plan|--execute] [--skip N] [--yes]")
		return 2
	}
	if !*plan && !*execute {
		*plan = true // default
	}
	if *plan && *execute {
		fmt.Fprintln(stderr, "rename: --plan and --execute are mutually exclusive")
		return 2
	}

	skipSet := map[int]bool{}
	for _, s := range skipMulti.values {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || n < 1 || n > 10 {
			fmt.Fprintf(stderr, "rename: --skip expects step number 1-10, got %q\n", s)
			return 2
		}
		skipSet[n] = true
	}

	env := &renameEnv{
		oldID:       positional[0],
		newName:     positional[1],
		projectsDir: *projectsDir,
		sessionsDir: *sessionsDir,
		catalogPath: resolveCatalogPath(*catalog),
		reason:      *reason,
		fs:          osRenameFS{},
		stdout:      stdout,
		stderr:      stderr,
		confirm:     stdinConfirm(stdout, *yes),
	}
	if env.projectsDir == "" {
		home, _ := os.UserHomeDir()
		env.projectsDir = filepath.Join(home, "Projects")
	}
	if env.sessionsDir == "" {
		home, _ := os.UserHomeDir()
		env.sessionsDir = filepath.Join(home, ".claude", "projects")
	}

	steps := buildRenameSteps()

	if *plan {
		printRenamePlan(stdout, env, steps, skipSet)
		return 0
	}
	return executeRenamePlan(env, steps, skipSet)
}

// buildRenameSteps returns the 10-step runbook from spec section 3.
// The order mirrors the spec exactly.
func buildRenameSteps() []renameStep {
	return []renameStep{
		{
			num: 1, title: "Local directory rename", auto: true, skipKey: "local-dir",
			detail: "mv {projects}/{old} {projects}/{new}",
			doFunc: stepLocalDirRename,
		},
		{
			num: 2, title: "Transitional symlink", auto: true, skipKey: "symlink",
			detail: "ln -sf {projects}/{new} {projects}/{old}",
			doFunc: stepTransitionalSymlink,
		},
		{
			num: 3, title: "Package metadata sweep", auto: false, skipKey: "metadata",
			detail: "Update name field in package.json, pyproject.toml, go.mod, version.json",
			doFunc: stepPackageMetadataReminder,
		},
		{
			num: 4, title: "CLAUDE.md sweeps", auto: false, skipKey: "claude-md",
			detail: "Update ~/.claude/CLAUDE.md project table and project-local CLAUDE.md",
			doFunc: stepClaudeMDReminder,
		},
		{
			num: 5, title: "GitHub repo rename", auto: true, skipKey: "gh",
			detail: "gh repo rename {new}  (run from {projects}/{new})",
			doFunc: stepGHRepoRename,
		},
		{
			num: 6, title: "DNS / Caddy reminder", auto: false, skipKey: "dns",
			detail: "Edit Caddyfile entries; OpenWrt unbound configs; reload services. NOT AUTOMATED.",
			doFunc: stepDNSReminder,
		},
		{
			num: 7, title: "Outline wiki path", auto: false, skipKey: "outline",
			detail: "Update outline.jphe.in page slugs that reference {old}",
			doFunc: stepOutlineReminder,
		},
		{
			num: 8, title: "Claude Code session storage rename", auto: true, skipKey: "sessions",
			detail: "mv ~/.claude/projects/-home-jp-Projects-{old} ~/.claude/projects/-home-jp-Projects-{new}",
			doFunc: stepSessionsRename,
		},
		{
			num: 9, title: "lexicon claim — append rename to catalog", auto: true, skipKey: "claim",
			detail: "lexicon claim {new} --renames={old} [--reason ...]",
			doFunc: stepLexiconClaim,
		},
		{
			num: 10, title: "Manual-verify checklist", auto: false, skipKey: "verify",
			detail: "Eyeball: cron jobs, systemd units, browser bookmarks, ~/.bashrc aliases, etc.",
			doFunc: stepManualVerify,
		},
	}
}

func printRenamePlan(w io.Writer, env *renameEnv, steps []renameStep, skip map[int]bool) {
	fmt.Fprintf(w, "rename plan: %s → %s\n", env.oldID, env.newName)
	fmt.Fprintln(w, strings.Repeat("-", 60))
	for _, s := range steps {
		marker := "[ ]"
		if skip[s.num] {
			marker = "[~]"
		}
		mode := "auto"
		if !s.auto {
			mode = "manual"
		}
		fmt.Fprintf(w, "%s %2d. %s (%s)\n", marker, s.num, s.title, mode)
		fmt.Fprintf(w, "       %s\n", expandPlaceholders(s.detail, env))
	}
	fmt.Fprintln(w, strings.Repeat("-", 60))
	fmt.Fprintln(w, "Run with --execute to apply auto steps; manual steps print reminders.")
}

func executeRenamePlan(env *renameEnv, steps []renameStep, skip map[int]bool) int {
	fmt.Fprintf(env.stdout, "Executing rename: %s → %s\n", env.oldID, env.newName)
	fmt.Fprintln(env.stdout, strings.Repeat("-", 60))

	failed := 0
	for _, s := range steps {
		fmt.Fprintf(env.stdout, "Step %d: %s\n", s.num, s.title)
		fmt.Fprintf(env.stdout, "  %s\n", expandPlaceholders(s.detail, env))
		if skip[s.num] {
			fmt.Fprintln(env.stdout, "  -> skipped (--skip)")
			continue
		}
		prompt := fmt.Sprintf("  proceed with step %d?", s.num)
		if !env.confirm(prompt) {
			fmt.Fprintln(env.stdout, "  -> declined")
			continue
		}
		if err := s.doFunc(env); err != nil {
			fmt.Fprintf(env.stderr, "  ! step %d failed: %v\n", s.num, err)
			failed++
			// Per spec: runbook is idempotent per step; we report and continue
			// rather than abort, so later steps can still be considered.
			continue
		}
		fmt.Fprintln(env.stdout, "  -> done")
	}
	fmt.Fprintln(env.stdout, strings.Repeat("-", 60))
	if failed > 0 {
		fmt.Fprintf(env.stderr, "rename complete with %d step error(s)\n", failed)
		return 1
	}
	fmt.Fprintln(env.stdout, "rename complete")
	return 0
}

// ---------------------------------------------------------------------------
// Step implementations
// ---------------------------------------------------------------------------

func stepLocalDirRename(env *renameEnv) error {
	oldPath := filepath.Join(env.projectsDir, env.oldID)
	newPath := filepath.Join(env.projectsDir, env.newName)
	if _, err := env.fs.Stat(newPath); err == nil {
		return fmt.Errorf("destination already exists: %s (idempotent skip)", newPath)
	}
	if _, err := env.fs.Stat(oldPath); err != nil {
		return fmt.Errorf("source not found: %s", oldPath)
	}
	return env.fs.Rename(oldPath, newPath)
}

func stepTransitionalSymlink(env *renameEnv) error {
	target := filepath.Join(env.projectsDir, env.newName)
	link := filepath.Join(env.projectsDir, env.oldID)
	if info, err := env.fs.Lstat(link); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			// Idempotent — already a symlink, leave it.
			return nil
		}
		return fmt.Errorf("transitional path exists and is not a symlink: %s", link)
	}
	return env.fs.Symlink(target, link)
}

func stepPackageMetadataReminder(env *renameEnv) error {
	fmt.Fprintln(env.stdout, "  package.json    : update \"name\" field if present")
	fmt.Fprintln(env.stdout, "  pyproject.toml  : update [project] name if present")
	fmt.Fprintln(env.stdout, "  go.mod          : update module directive + internal imports")
	fmt.Fprintln(env.stdout, "  version.json    : update name to match new project name")
	fmt.Fprintf(env.stdout, "  (search root: %s)\n", filepath.Join(env.projectsDir, env.newName))
	return nil
}

func stepClaudeMDReminder(env *renameEnv) error {
	fmt.Fprintln(env.stdout, "  ~/.claude/CLAUDE.md       : project table — replace old row")
	fmt.Fprintf(env.stdout,  "  %s/CLAUDE.md  : project-local references\n", filepath.Join(env.projectsDir, env.newName))
	return nil
}

func stepGHRepoRename(env *renameEnv) error {
	// Skip if catalog says this project has no GitHub remote — `gh repo
	// rename` would otherwise fail. The catalog is the source of truth here;
	// we look up by the immutable id (env.oldID) regardless of whether step
	// 9 has already updated current_name.
	if env.catalogPath != "" {
		cat, err := lexicon.LoadCatalog(env.catalogPath)
		if err == nil {
			if proj, ok := cat.Resolve(env.oldID); ok && proj.Repo == "" {
				fmt.Fprintf(env.stdout, "  skipped: %s has no GitHub remote (catalog repo: ~)\n", env.oldID)
				return nil
			}
		}
	}
	cwd := filepath.Join(env.projectsDir, env.newName)
	// gh has no -C flag — must be invoked with the project as cwd.
	out, err := env.fs.RunInDir(cwd, "gh", "repo", "rename", env.newName, "--yes")
	if err != nil {
		// Echo gh's output so the operator can see why (missing auth, already renamed).
		fmt.Fprintf(env.stderr, "  gh output: %s\n", strings.TrimSpace(string(out)))
		return err
	}
	fmt.Fprintf(env.stdout, "  gh: %s\n", strings.TrimSpace(string(out)))
	return nil
}

func stepDNSReminder(env *renameEnv) error {
	fmt.Fprintln(env.stdout, "  Caddyfile entries (e.g. ~/Projects/disks/Caddyfile)")
	fmt.Fprintln(env.stdout, "  OpenWrt unbound config (LAN DNS overrides on gatekeeper)")
	fmt.Fprintln(env.stdout, "  Service reloads after edits — do NOT skip verification.")
	return nil
}

func stepOutlineReminder(env *renameEnv) error {
	fmt.Fprintf(env.stdout, "  Update outline.jphe.in pages referencing %q\n", env.oldID)
	return nil
}

func stepSessionsRename(env *renameEnv) error {
	oldDir := filepath.Join(env.sessionsDir, "-home-jp-Projects-"+env.oldID)
	newDir := filepath.Join(env.sessionsDir, "-home-jp-Projects-"+env.newName)
	if _, err := env.fs.Stat(newDir); err == nil {
		return nil // idempotent
	}
	if _, err := env.fs.Stat(oldDir); err != nil {
		// Not an error — JP may never have used Claude Code in this project.
		fmt.Fprintf(env.stdout, "  (no session dir at %s — skipping)\n", oldDir)
		return nil
	}
	return env.fs.Rename(oldDir, newDir)
}

func stepLexiconClaim(env *renameEnv) error {
	cat, err := lexicon.LoadCatalog(env.catalogPath)
	if err != nil {
		return fmt.Errorf("load catalog: %w", err)
	}
	opts := lexicon.ClaimOpts{
		RenamesOf: env.oldID,
		Reason:    env.reason,
	}
	if err := cat.Claim(env.newName, opts); err != nil {
		return fmt.Errorf("claim: %w", err)
	}
	if err := cat.Save(); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	fmt.Fprintf(env.stdout, "  catalog updated: %s → %s\n", env.oldID, env.newName)
	return nil
}

func stepManualVerify(env *renameEnv) error {
	fmt.Fprintln(env.stdout, "  cron jobs        : crontab -l | grep -i "+env.oldID)
	fmt.Fprintln(env.stdout, "  systemd units    : systemctl list-units | grep -i "+env.oldID)
	fmt.Fprintln(env.stdout, "  shell aliases    : grep -i "+env.oldID+" ~/.bashrc ~/.bash_aliases")
	fmt.Fprintln(env.stdout, "  browser bookmarks: visual scan")
	fmt.Fprintln(env.stdout, "  scripts/         : grep -rli "+env.oldID+" ~/Projects/scripts")
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func expandPlaceholders(s string, env *renameEnv) string {
	r := strings.NewReplacer(
		"{old}", env.oldID,
		"{new}", env.newName,
		"{projects}", env.projectsDir,
	)
	return r.Replace(s)
}

// stringSliceFlag collects repeatable flag values (--skip=1 --skip=2).
type stringSliceFlag struct{ values []string }

func newStringSliceFlag() *stringSliceFlag { return &stringSliceFlag{} }
func (s *stringSliceFlag) String() string  { return strings.Join(s.values, ",") }
func (s *stringSliceFlag) Set(v string) error {
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			s.values = append(s.values, part)
		}
	}
	return nil
}

func stdinConfirm(stdout io.Writer, autoYes bool) func(string) bool {
	if autoYes {
		return func(prompt string) bool {
			fmt.Fprintf(stdout, "%s [y/N]: y (auto)\n", prompt)
			return true
		}
	}
	r := bufio.NewReader(os.Stdin)
	return func(prompt string) bool {
		fmt.Fprintf(stdout, "%s [y/N]: ", prompt)
		line, err := r.ReadString('\n')
		if err != nil {
			return false
		}
		line = strings.TrimSpace(strings.ToLower(line))
		return line == "y" || line == "yes"
	}
}
