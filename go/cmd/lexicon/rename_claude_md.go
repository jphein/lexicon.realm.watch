package main

import (
	"bytes"
	"os"
	"path/filepath"
)

// applyClaudeMDSweep rewrites bare references to oldName as newName in the
// project's own CLAUDE.md (<projDir>/CLAUDE.md). Returns true when an edit
// was made. Returns false (no error) when the file is absent.
//
// Cross-project CLAUDE.md sweeps (other projects referencing this one) are
// out of scope per issue #3; that's a separate "step 4b" still to be
// designed. The bigger ~/.claude/CLAUDE.md project table is also out of
// scope — after the catalog → project-catalog skill migration, that table
// no longer exists at the user level.
func applyClaudeMDSweep(projDir, oldName, newName string) (bool, error) {
	path := filepath.Join(projDir, "CLAUDE.md")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	out := bytes.ReplaceAll(data, []byte(oldName), []byte(newName))
	if bytes.Equal(out, data) {
		return false, nil
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return false, err
	}
	return true, nil
}
