package lexicon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// seededCase mirrors one entry in tests/fixtures/seeded-recipes.json — the
// cross-language parity contract. Python and JS readers consume the same file.
type seededCase struct {
	Seed         string            `json:"seed"`
	Recipe       string            `json:"recipe"`
	Options      map[string]string `json:"options"`
	ExpectedName string            `json:"expected_name"`
}

type seededFixture struct {
	Cases []seededCase `json:"cases"`
}

func loadSeededFixture(t *testing.T) seededFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdataDir(t), "seeded-recipes.json"))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	var f seededFixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(f.Cases) == 0 {
		t.Fatalf("fixture has zero cases — cross-language contract is empty")
	}
	return f
}

func TestSeededIndex_Determinism(t *testing.T) {
	idxA := SeededIndex("abc123", 0, 4)
	idxB := SeededIndex("abc123", 0, 4)
	if idxA != idxB {
		t.Fatalf("SeededIndex non-deterministic: %d vs %d", idxA, idxB)
	}
	if idxA < 0 || idxA >= 4 {
		t.Fatalf("SeededIndex out of range: %d", idxA)
	}
}

func TestSeededIndex_DifferentSlots(t *testing.T) {
	a := SeededIndex("abc123", 0, 100)
	b := SeededIndex("abc123", 1, 100)
	if a == b {
		t.Logf("WARNING: slots 0 and 1 collided for seed abc123 (size 100). Acceptable but rare.")
	}
}

// TestRollSeeded_MatchesFixture is the Go side of the cross-language parity
// contract. Python (python/tests/test_seeded.py) and JS (js/test/seeded.test.js)
// run the same assertions against the same fixture; if all three pass, the
// SHA-256-derived seeded RNG is byte-equivalent across runtimes.
func TestRollSeeded_MatchesFixture(t *testing.T) {
	f := loadSeededFixture(t)
	rb := loadTestRecipes(t)
	v, err := loadLiveVocabsCombined()
	if err != nil {
		t.Fatalf("vocab: %v", err)
	}
	for i, c := range f.Cases {
		opts := RollOptions{}
		if r, ok := c.Options["realm"]; ok {
			opts.Realm = r
		}
		if p, ok := c.Options["prefix"]; ok {
			opts.Prefix = p
		}
		got, err := rb.RollSeeded(c.Recipe, v, c.Seed, opts)
		if err != nil {
			t.Errorf("case %d (%s seed=%q): RollSeeded error: %v", i, c.Recipe, c.Seed, err)
			continue
		}
		if got != c.ExpectedName {
			t.Errorf("case %d (%s seed=%q): got %q, want %q", i, c.Recipe, c.Seed, got, c.ExpectedName)
		}
	}
}
