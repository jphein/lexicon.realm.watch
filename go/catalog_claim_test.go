package lexicon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// copyFixtureToTemp returns a temp-file path that can be safely mutated.
func copyFixtureToTemp(t *testing.T, fixtureName string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(testdataDir(t), fixtureName))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	dst := filepath.Join(t.TempDir(), fixtureName)
	if err := os.WriteFile(dst, src, 0o644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	return dst
}

func TestCatalog_Claim_RecordsRename(t *testing.T) {
	path := copyFixtureToTemp(t, "catalog-test.yaml")
	c, err := LoadCatalog(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	err = c.Claim("watch.realm.watch", ClaimOpts{
		RenamesOf: "realmwatch",
		Reason:    "moved into realm.watch family",
		Retired:   "2026-04-26",
	})
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	rw, ok := c.Resolve("realmwatch")
	if !ok {
		t.Fatal("realmwatch should still resolve via id")
	}
	if rw.CurrentName != "watch.realm.watch" {
		t.Errorf("current_name not updated: %q", rw.CurrentName)
	}
	if len(rw.PriorNames) != 1 || rw.PriorNames[0].Name != "realmwatch" {
		t.Errorf("prior_names not appended: %+v", rw.PriorNames)
	}
}

func TestCatalog_Claim_RejectsDuplicateName(t *testing.T) {
	path := copyFixtureToTemp(t, "catalog-test.yaml")
	c, _ := LoadCatalog(path)
	err := c.Claim("clock.realm.watch", ClaimOpts{
		RenamesOf: "realmwatch",
		Reason:    "test",
	})
	if err == nil {
		t.Fatal("expected error: clock.realm.watch is already taken")
	}
}

func TestCatalog_Claim_NewProject(t *testing.T) {
	path := copyFixtureToTemp(t, "catalog-test.yaml")
	c, _ := LoadCatalog(path)
	err := c.Claim("ledger.realm.watch", ClaimOpts{
		Kind:        "service",
		Realm:       "forge",
		Description: "ledger service",
		Created:     "2026-04-26",
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("Claim new: %v", err)
	}
	p, ok := c.Resolve("ledger.realm.watch")
	if !ok {
		t.Fatal("new entry should resolve")
	}
	if p.ID != "ledger.realm.watch" {
		t.Errorf("new entry id should equal first name: %q", p.ID)
	}
}

func TestCatalog_Save_RoundTrips(t *testing.T) {
	path := copyFixtureToTemp(t, "catalog-test.yaml")
	c, _ := LoadCatalog(path)
	if err := c.Claim("watch.realm.watch", ClaimOpts{
		RenamesOf: "realmwatch",
		Reason:    "test",
		Retired:   "2026-04-26",
	}); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	c2, err := LoadCatalog(path)
	if err != nil {
		t.Fatalf("re-Load: %v", err)
	}
	rw, ok := c2.Resolve("realmwatch")
	if !ok || rw.CurrentName != "watch.realm.watch" {
		t.Errorf("change did not persist: %+v ok=%v", rw, ok)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "watch.realm.watch") {
		t.Error("file does not mention new name")
	}
}

// TestCatalog_Save_PreservesCommentsAndNullStyle exercises the contract that
// Save must round-trip the on-disk YAML without destroying curated
// formatting: comments survive, blank-line separators between entries
// survive, untouched fields keep their original scalar style (`~` stays `~`,
// not coerced to `""`).
//
// Regression test for the rename byproduct seen on 2026-05-07: every
// `domain: ~` got rewritten to `domain: ""` and every leading comment was
// stripped, producing 320+ lines of unrelated diff noise around a
// single-entry rename.
func TestCatalog_Save_PreservesCommentsAndNullStyle(t *testing.T) {
	src := `# Header comment line one
# Header comment line two
projects:
  - id: alpha
    current_name: alpha
    kind: tool
    realm: forge
    domain: ~
    repo: ~
    description: A
    created: 2026-04-01
    prior_names: []
    status: active

  - id: beta
    current_name: beta
    kind: tool
    realm: forge
    domain: ~
    repo: ~
    description: B
    created: 2026-04-01
    prior_names: []
    status: active
`
	path := filepath.Join(t.TempDir(), "cat.yaml")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	c, err := LoadCatalog(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := c.Claim("alpha2", ClaimOpts{
		RenamesOf: "alpha",
		Reason:    "round-trip test",
		Retired:   "2026-05-08",
	}); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := c.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	out := string(got)

	// 1. Header comments must survive.
	for _, want := range []string{"# Header comment line one", "# Header comment line two"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing comment %q after Save:\n%s", want, out)
		}
	}

	// 2. Untouched `~` (null) fields must stay `~`, not be coerced to "".
	if strings.Contains(out, `domain: ""`) || strings.Contains(out, `repo: ""`) {
		t.Errorf("untouched null was coerced to empty string:\n%s", out)
	}
	if !strings.Contains(out, "domain: ~") {
		t.Errorf("`domain: ~` not preserved:\n%s", out)
	}

	// 3. The rename was actually applied to alpha.
	if !strings.Contains(out, "current_name: alpha2") {
		t.Errorf("rename not applied to file:\n%s", out)
	}

	// 4. Beta is untouched — including its specific shape.
	if !strings.Contains(out, "  - id: beta\n    current_name: beta\n") {
		t.Errorf("beta entry shape changed:\n%s", out)
	}

	// 5. ISO date for unchanged created field must not be quoted.
	if strings.Contains(out, `created: "2026-04-01"`) {
		t.Errorf("ISO date got quoted unnecessarily:\n%s", out)
	}
}

// TestCatalog_Save_AppendNew_KeepsHeaderComments verifies that the new-entry
// path through Claim (no RenamesOf) also preserves curated formatting.
func TestCatalog_Save_AppendNew_KeepsHeaderComments(t *testing.T) {
	src := `# Curated header
projects:
  - id: alpha
    current_name: alpha
    kind: tool
    realm: forge
    domain: ~
    repo: ~
    description: A
    created: 2026-04-01
    prior_names: []
    status: active
`
	path := filepath.Join(t.TempDir(), "cat.yaml")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	c, err := LoadCatalog(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := c.Claim("gamma", ClaimOpts{
		Kind:        "tool",
		Realm:       "signal",
		Description: "gamma project",
		Created:     "2026-05-08",
		Status:      "active",
	}); err != nil {
		t.Fatalf("claim new: %v", err)
	}
	if err := c.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, _ := os.ReadFile(path)
	out := string(got)
	if !strings.Contains(out, "# Curated header") {
		t.Errorf("header lost on append:\n%s", out)
	}
	if !strings.Contains(out, "id: gamma") {
		t.Errorf("new entry not added:\n%s", out)
	}
	// Existing alpha must still show `domain: ~`.
	if !strings.Contains(out, "domain: ~") {
		t.Errorf("existing null not preserved on append:\n%s", out)
	}
}
