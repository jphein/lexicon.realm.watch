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
