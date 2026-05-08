package lexicon

import (
	"path/filepath"
	"testing"
)

func loadTestCatalog(t *testing.T) *Catalog {
	t.Helper()
	c, err := LoadCatalog(filepath.Join(testdataDir(t), "catalog-test.yaml"))
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	return c
}

func TestLoadCatalog_BasicShape(t *testing.T) {
	c := loadTestCatalog(t)
	if got, want := len(c.Projects), 3; got != want {
		t.Errorf("len(Projects) = %d, want %d", got, want)
	}
	clock := c.Projects[0]
	if clock.ID != "clock" || clock.CurrentName != "clock.realm.watch" {
		t.Errorf("clock entry malformed: %+v", clock)
	}
}

func TestLoadCatalog_PriorNamesParsed(t *testing.T) {
	c := loadTestCatalog(t)
	dream := c.Projects[1]
	if got, want := len(dream.PriorNames), 2; got != want {
		t.Errorf("len(PriorNames) = %d, want %d", got, want)
	}
	if dream.PriorNames[0].Name != "dreamspace" {
		t.Errorf("first prior name = %q, want dreamspace", dream.PriorNames[0].Name)
	}
}

func TestLoadCatalog_NullableFields(t *testing.T) {
	c := loadTestCatalog(t)
	rw := c.Projects[2]
	if rw.Domain != "" {
		t.Errorf("realmwatch.domain should be empty, got %q", rw.Domain)
	}
	if rw.Repo != "" {
		t.Errorf("realmwatch.repo should be empty, got %q", rw.Repo)
	}
}

func TestLoadCatalog_AudienceParsed(t *testing.T) {
	c := loadTestCatalog(t)
	if got := c.Projects[0].Audience; got != "realm" {
		t.Errorf("clock.audience = %q, want %q", got, "realm")
	}
	// dreamspace and realmwatch fixtures don't set audience → empty.
	if got := c.Projects[1].Audience; got != "" {
		t.Errorf("dreamspace.audience = %q, want empty (not set in fixture)", got)
	}
}

func TestLoadCatalog_Missing(t *testing.T) {
	_, err := LoadCatalog("/no/such.yaml")
	if err == nil {
		t.Fatal("expected error")
	}
}
