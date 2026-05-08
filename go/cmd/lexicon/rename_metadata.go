package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// applyPackageMetadataSweep walks projDir and updates name/module references
// in known package files when they match oldName. Returns the list of files
// it edited (for the runbook to report). Idempotent: files already containing
// the new name are left alone.
//
// Supported files:
//   - go.mod                 — module directive (any github.com/<owner>/<old> path)
//   - package.json           — top-level "name" field
//   - pyproject.toml         — [project] name field (or [tool.poetry] name)
//   - version.json           — top-level "name" field
//   - **/*.go                — import strings referencing the old module path
func applyPackageMetadataSweep(projDir, oldName, newName string) ([]string, error) {
	var edited []string

	if e, err := updateGoMod(filepath.Join(projDir, "go.mod"), oldName, newName); err != nil {
		return edited, err
	} else if e {
		edited = append(edited, "go.mod")
	}

	if e, err := updatePackageJSON(filepath.Join(projDir, "package.json"), oldName, newName); err != nil {
		return edited, err
	} else if e {
		edited = append(edited, "package.json")
	}

	if e, err := updatePyprojectTOML(filepath.Join(projDir, "pyproject.toml"), oldName, newName); err != nil {
		return edited, err
	} else if e {
		edited = append(edited, "pyproject.toml")
	}

	if e, err := updateVersionJSON(filepath.Join(projDir, "version.json"), oldName, newName); err != nil {
		return edited, err
	} else if e {
		edited = append(edited, "version.json")
	}

	imports, err := updateGoImports(projDir, oldName, newName)
	if err != nil {
		return edited, err
	}
	edited = append(edited, imports...)

	return edited, nil
}

func updateGoMod(path, oldName, newName string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	out, changed := rewriteGoModuleDirective(data, oldName, newName)
	if !changed {
		return false, nil
	}
	return true, os.WriteFile(path, out, 0o644)
}

// rewriteGoModuleDirective replaces /<oldName>(/...) with /<newName>(/...) in
// the `module` line. Other lines are left as-is.
func rewriteGoModuleDirective(data []byte, oldName, newName string) ([]byte, bool) {
	moduleRe := regexp.MustCompile(`(?m)^module\s+(\S+)`)
	loc := moduleRe.FindSubmatchIndex(data)
	if loc == nil {
		return data, false
	}
	pathStart, pathEnd := loc[2], loc[3]
	oldPath := string(data[pathStart:pathEnd])
	newPath := replacePathSegment(oldPath, oldName, newName)
	if newPath == oldPath {
		return data, false
	}
	out := make([]byte, 0, len(data)+len(newPath)-len(oldPath))
	out = append(out, data[:pathStart]...)
	out = append(out, newPath...)
	out = append(out, data[pathEnd:]...)
	return out, true
}

// replacePathSegment swaps oldSegment for newSegment in a `/`-delimited path
// string, only when oldSegment appears as a complete path segment. So
// `github.com/jphein/realm-portal/go` with old=realm-portal becomes
// `github.com/jphein/portal.realm.watch/go`, but `xrealm-portaly` would not
// match.
func replacePathSegment(path, oldSegment, newSegment string) string {
	parts := strings.Split(path, "/")
	changed := false
	for i, p := range parts {
		if p == oldSegment {
			parts[i] = newSegment
			changed = true
		}
	}
	if !changed {
		return path
	}
	return strings.Join(parts, "/")
}

func updatePackageJSON(path, oldName, newName string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return false, fmt.Errorf("parse package.json: %w", err)
	}
	curr, _ := raw["name"].(string)
	if curr != oldName {
		return false, nil
	}
	raw["name"] = newName
	indent := detectJSONIndent(data)
	out, err := json.MarshalIndent(raw, "", indent)
	if err != nil {
		return false, err
	}
	if !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, '\n')
	}
	return true, os.WriteFile(path, out, 0o644)
}

func updateVersionJSON(path, oldName, newName string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return false, fmt.Errorf("parse version.json: %w", err)
	}
	curr, _ := raw["name"].(string)
	if curr != oldName {
		return false, nil
	}
	raw["name"] = newName
	indent := detectJSONIndent(data)
	out, err := json.MarshalIndent(raw, "", indent)
	if err != nil {
		return false, err
	}
	if !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, '\n')
	}
	return true, os.WriteFile(path, out, 0o644)
}

// detectJSONIndent inspects the first indented line and returns the indent
// string (spaces or a tab). Defaults to "  " when nothing detectable.
func detectJSONIndent(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "{") {
			continue
		}
		i := 0
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i > 0 {
			return line[:i]
		}
	}
	return "  "
}

// updatePyprojectTOML rewrites the name field within either [project] or
// [tool.poetry] sections, leaving everything else (comments, ordering, other
// sections) untouched.
func updatePyprojectTOML(path, oldName, newName string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(data), "\n")
	currentSection := ""
	nameRe := regexp.MustCompile(`^(\s*name\s*=\s*)(['"])([^'"]*)(['"])(\s*.*)$`)
	sectionRe := regexp.MustCompile(`^\[([^\]]+)\]\s*$`)
	changed := false
	for i, line := range lines {
		if m := sectionRe.FindStringSubmatch(line); m != nil {
			currentSection = m[1]
			continue
		}
		if currentSection != "project" && currentSection != "tool.poetry" {
			continue
		}
		if m := nameRe.FindStringSubmatch(line); m != nil {
			if m[3] != oldName {
				continue
			}
			lines[i] = m[1] + m[2] + newName + m[4] + m[5]
			changed = true
		}
	}
	if !changed {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

// updateGoImports walks projDir for *.go files and rewrites any import path
// that contains /<oldName>(/...) to /<newName>(/...). Skips vendor/ and any
// directory named "testdata".
func updateGoImports(projDir, oldName, newName string) ([]string, error) {
	var edited []string
	importRe := regexp.MustCompile(`"([^"]+)"`)
	err := filepath.WalkDir(projDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == "testdata" || name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out := importRe.ReplaceAllFunc(data, func(b []byte) []byte {
			s := string(b[1 : len(b)-1])
			ns := replacePathSegment(s, oldName, newName)
			if ns == s {
				return b
			}
			return []byte(`"` + ns + `"`)
		})
		if bytes.Equal(out, data) {
			return nil
		}
		if err := os.WriteFile(path, out, 0o644); err != nil {
			return err
		}
		rel, _ := filepath.Rel(projDir, path)
		edited = append(edited, rel)
		return nil
	})
	return edited, err
}

