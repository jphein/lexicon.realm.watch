package lexicon

import "testing"

func TestCatalog_ResolveByID(t *testing.T) {
	c := loadTestCatalog(t)
	p, ok := c.Resolve("clock")
	if !ok {
		t.Fatal("expected clock to resolve")
	}
	if p.CurrentName != "clock.realm.watch" {
		t.Errorf("got %q", p.CurrentName)
	}
}

func TestCatalog_ResolveByCurrentName(t *testing.T) {
	c := loadTestCatalog(t)
	p, ok := c.Resolve("dreamscape.realm.watch")
	if !ok || p.ID != "dreamspace" {
		t.Errorf("expected dreamspace via current_name, got %+v ok=%v", p, ok)
	}
}

func TestCatalog_ResolveByPriorName(t *testing.T) {
	c := loadTestCatalog(t)
	p, ok := c.Resolve("dreamspace")
	if !ok || p.ID != "dreamspace" {
		t.Errorf("expected dreamspace by id; got %+v ok=%v", p, ok)
	}
	p, ok = c.Resolve("dreamscape")
	if !ok || p.ID != "dreamspace" {
		t.Errorf("expected dreamspace by prior name 'dreamscape'; got %+v", p)
	}
}

func TestCatalog_Unknown(t *testing.T) {
	c := loadTestCatalog(t)
	_, ok := c.Resolve("nonsense")
	if ok {
		t.Fatal("expected miss")
	}
}

func TestCatalog_ByRealm(t *testing.T) {
	c := loadTestCatalog(t)
	got := c.ByRealm("signal")
	if len(got) != 1 || got[0].ID != "clock" {
		t.Errorf("ByRealm(signal) = %v", got)
	}
}
