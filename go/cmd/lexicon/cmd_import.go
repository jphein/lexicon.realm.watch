package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// cmdCatalog dispatches the `lexicon catalog <subcommand>` group. Subcommands
// today: import (bootstrap from a project tree) and render (project the
// catalog into a Claude Code skill or markdown table). Future subcommands
// (export, diff, merge) plug in without further main.go changes.
func cmdCatalog(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printCatalogHelp(stdout)
		return 0
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "import":
		return cmdCatalogImport(rest, stdout, stderr)
	case "render":
		return cmdCatalogRender(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown catalog subcommand %q\n\n", sub)
		printCatalogHelp(stderr)
		return 2
	}
}

func printCatalogHelp(w io.Writer) {
	fmt.Fprintln(w, `lexicon catalog — operations on the project catalog

Usage: lexicon catalog <subcommand> [options]

Subcommands:
  import   walk a directory of projects and emit a draft catalog YAML
  render   render the catalog as a skill or markdown table

Run 'lexicon catalog <subcommand> --help' for command-specific options.`)
}

// cmdCatalogImport walks a directory of projects (default ~/Projects) and
// emits a draft catalog/projects.yaml fragment by inferring each project's
// name (from package metadata), description (from README), and repo (from
// `git remote get-url origin`). Output goes to stdout unless --out is set.
//
// This is a one-time bootstrap aid. The output is deliberately marked with
// `realm: ?` so a human reviews realm assignment before the file ships.
func cmdCatalogImport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("catalog import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fromDir := fs.String("from", "", "directory to walk (default ~/Projects)")
	outPath := fs.String("out", "", "output path (default stdout)")
	dryRun := fs.Bool("dry-run", false, "list projects that would be imported, do not emit YAML")
	include := fs.String("include-hidden", "", "comma-separated list of dotted-name dirs to include (default skips dotfiles)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	root, err := resolveImportRoot(*fromDir)
	if err != nil {
		fmt.Fprintf(stderr, "resolve --from: %v\n", err)
		return 1
	}

	// Refuse to clobber the project's own catalog unless the caller is being
	// explicit. We compare absolute paths because callers often pass a
	// relative `--out` when running from go/.
	if *outPath != "" && !*dryRun {
		if abs, _ := filepath.Abs(*outPath); abs != "" {
			base := filepath.Base(abs)
			parent := filepath.Base(filepath.Dir(abs))
			if base == "projects.yaml" && parent == "catalog" {
				if _, err := os.Stat(abs); err == nil {
					fmt.Fprintf(stderr,
						"refusing to overwrite existing %s — pass a different --out, "+
							"or delete the file first if you really mean to bootstrap from scratch\n", abs)
					return 1
				}
			}
		}
	}

	includeHidden := map[string]bool{}
	for _, name := range strings.Split(*include, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			includeHidden[name] = true
		}
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		fmt.Fprintf(stderr, "read %s: %v\n", root, err)
		return 1
	}

	var projects []*importedProject
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") && !includeHidden[name] {
			continue
		}
		dir := filepath.Join(root, name)
		// Skip symlinks pointing outside the root — they're often transitional
		// rename aliases (lexicon rename creates them) and would double-count.
		info, err := os.Lstat(dir)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		p := inferProject(dir, name)
		projects = append(projects, p)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })

	if *dryRun {
		fmt.Fprintf(stdout, "would import %d project(s) from %s:\n", len(projects), root)
		for _, p := range projects {
			fmt.Fprintf(stdout, "  - %s  (%s)  status=%s\n", p.ID, oneLine(p.Description, 60), p.Status)
		}
		return 0
	}

	out, err := emitImportYAML(projects)
	if err != nil {
		fmt.Fprintf(stderr, "emit yaml: %v\n", err)
		return 1
	}

	if *outPath == "" {
		stdout.Write(out)
		return 0
	}
	if err := os.WriteFile(*outPath, out, 0o644); err != nil {
		fmt.Fprintf(stderr, "write %s: %v\n", *outPath, err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %d project(s) to %s\n", len(projects), *outPath)
	return 0
}

// resolveImportRoot expands ~ and falls back to ~/Projects when no --from is
// supplied. We resolve to an absolute path so error messages are readable.
func resolveImportRoot(fromFlag string) (string, error) {
	if fromFlag == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Projects"), nil
	}
	if strings.HasPrefix(fromFlag, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, fromFlag[1:]), nil
	}
	return filepath.Abs(fromFlag)
}

// importedProject is the in-memory shape we collect before emitting YAML.
// We don't reuse lexicon.Project because we want to write a *partial* entry
// (no `created`, blank `realm: ?`) and have YAML round-trip cleanly without
// emitting empty fields the human reviewer would have to delete.
type importedProject struct {
	ID          string `yaml:"id"`
	CurrentName string `yaml:"current_name"`
	Kind        string `yaml:"kind"`
	Realm       string `yaml:"realm"`
	Domain      string `yaml:"domain,omitempty"`
	Repo        string `yaml:"repo,omitempty"`
	Description string `yaml:"description,omitempty"`
	Created     string `yaml:"created,omitempty"`
	PriorNames  []any  `yaml:"prior_names"`
	Status      string `yaml:"status"`
	Notes       string `yaml:"notes,omitempty"`
}

// inferProject reads the dir and best-effort extracts metadata. Anything we
// can't determine becomes a sentinel ("?", "") so the human reviewer's eye
// is drawn to it.
func inferProject(dir, dirName string) *importedProject {
	p := &importedProject{
		ID:         slugify(dirName),
		Kind:       "tool",
		Realm:      "?",
		PriorNames: []any{},
		Status:     "local-only",
	}

	// Name preference: package.json > pyproject.toml > go.mod > directory name.
	if name := readPackageJSONName(dir); name != "" {
		p.CurrentName = name
	} else if name := readPyprojectName(dir); name != "" {
		p.CurrentName = name
	} else if name := readGoModName(dir); name != "" {
		p.CurrentName = name
	} else {
		p.CurrentName = dirName
	}

	if desc := readReadmeDescription(dir); desc != "" {
		p.Description = desc
	}

	if remote := readGitRemote(dir); remote != "" {
		p.Repo = remote
		p.Status = "active" // has a public remote → not just local-only
	}

	// Best-effort created date from the directory mtime. Reviewers can correct.
	if info, err := os.Stat(dir); err == nil {
		p.Created = info.ModTime().Format("2006-01-02")
	} else {
		p.Created = time.Now().Format("2006-01-02")
	}

	return p
}

// readPackageJSONName pulls "name" from package.json. We tolerate scoped
// names like "@jphein/foo" — the catalog cares about identity, not registry.
func readPackageJSONName(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return strings.TrimSpace(pkg.Name)
}

// readPyprojectName reads the [project] name from pyproject.toml without a
// real TOML parser. The format is stable enough for a heuristic: scan for a
// `[project]` header and take the first `name = "..."` after it. If anything
// is funky we just return "" and let the caller fall through.
func readPyprojectName(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "pyproject.toml"))
	if err != nil {
		return ""
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	inProject := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inProject = line == "[project]"
			continue
		}
		if !inProject {
			continue
		}
		if eq := strings.Index(line, "="); eq > 0 {
			key := strings.TrimSpace(line[:eq])
			val := strings.TrimSpace(line[eq+1:])
			if key == "name" {
				return strings.Trim(val, `"' `)
			}
		}
	}
	return ""
}

// readGoModName extracts the last path segment of the module directive — what
// a Go user would call this project. `module github.com/jphein/foo` → `foo`.
func readGoModName(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			mod := strings.TrimSpace(strings.TrimPrefix(line, "module"))
			mod = strings.Trim(mod, `"`)
			if i := strings.LastIndex(mod, "/"); i >= 0 {
				return mod[i+1:]
			}
			return mod
		}
	}
	return ""
}

// readReadmeDescription tries README.md first, then README. We take the first
// non-empty paragraph that isn't a heading or badge line. Sentence-trim to
// keep things to one line; the human reviewer can always edit.
func readReadmeDescription(dir string) string {
	for _, name := range []string{"README.md", "README", "readme.md"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		return firstParagraph(string(data))
	}
	return ""
}

func firstParagraph(s string) string {
	scanner := bufio.NewScanner(strings.NewReader(s))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var para []string
	flush := func() string {
		if len(para) == 0 {
			return ""
		}
		joined := strings.TrimSpace(strings.Join(para, " "))
		// Cut at first sentence end so descriptions stay catalog-sized.
		if idx := sentenceEnd(joined); idx > 0 {
			joined = strings.TrimSpace(joined[:idx+1])
		}
		return joined
	}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if len(para) > 0 {
				return flush()
			}
			continue
		}
		if isSkippableReadmeLine(line) {
			continue
		}
		para = append(para, line)
	}
	return flush()
}

// isSkippableReadmeLine filters lines we don't want as descriptions: ATX
// headings, setext underlines, badge / shield image-link lines, blockquotes,
// HR rules. The point is to find the *prose* paragraph, not the chrome.
func isSkippableReadmeLine(line string) bool {
	if strings.HasPrefix(line, "#") {
		return true
	}
	if strings.HasPrefix(line, ">") {
		return true
	}
	if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "===") {
		return true
	}
	// Badge lines: leading `[![...]` image-link or bare image.
	if strings.HasPrefix(line, "[![") || strings.HasPrefix(line, "![") {
		return true
	}
	// HTML chrome (logos, alignment wrappers).
	if strings.HasPrefix(line, "<") {
		return true
	}
	return false
}

func sentenceEnd(s string) int {
	for i, r := range s {
		if (r == '.' || r == '!' || r == '?') && i+1 < len(s) {
			next := s[i+1]
			if next == ' ' || next == '\n' {
				return i
			}
		}
	}
	if len(s) > 0 {
		last := s[len(s)-1]
		if last == '.' || last == '!' || last == '?' {
			return len(s) - 1
		}
	}
	return -1
}

// readGitRemote shells out to `git -C <dir> remote get-url origin`. We
// deliberately use the git binary instead of go-git: it's already installed,
// the output format is stable, and it handles every auth/url case the user's
// own setup does.
func readGitRemote(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return ""
	}
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))
	// Normalize SSH-style remotes (`git@github.com:owner/repo.git`) to https
	// so the catalog stores something a reader can paste into a browser.
	if strings.HasPrefix(url, "git@") {
		if i := strings.Index(url, ":"); i > 0 {
			host := strings.TrimPrefix(url[:i], "git@")
			path := strings.TrimSuffix(url[i+1:], ".git")
			url = "https://" + host + "/" + path
		}
	}
	url = strings.TrimSuffix(url, ".git")
	return url
}

// slugify lowercases and squashes anything that isn't alnum or `-` to `-`.
// Catalog `id` values must be filesystem-safe permanent slugs.
func slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == '-' || r == '_' || r == '.':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

func oneLine(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len([]rune(s)) > max {
		r := []rune(s)
		return string(r[:max-1]) + "…"
	}
	return s
}

// emitImportYAML writes the projects under a `projects:` key with a banner
// comment at the top so the human reviewer immediately sees what to fix.
func emitImportYAML(projects []*importedProject) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("# Draft catalog generated by `lexicon catalog import`.\n")
	buf.WriteString("# Review every entry before merging into catalog/projects.yaml:\n")
	buf.WriteString("#   - replace `realm: ?` with the correct realm name\n")
	buf.WriteString("#   - tighten `kind:` (default `tool`) where you know better\n")
	buf.WriteString("#   - add `domain:` for projects with a public face\n")
	buf.WriteString("#   - confirm `created:` (we used directory mtime as a guess)\n\n")

	wrap := struct {
		Projects []*importedProject `yaml:"projects"`
	}{Projects: projects}
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(wrap); err != nil {
		return nil, err
	}
	enc.Close()
	return buf.Bytes(), nil
}
