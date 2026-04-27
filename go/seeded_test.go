package lexicon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type seededCase struct {
	Seed          string   `json:"seed"`
	Words         []string `json:"words"`
	Slot          uint64   `json:"slot"`
	ExpectedIndex *int     `json:"expected_index"`
	ExpectedWord  *string  `json:"expected_word"`
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

func TestSeededIndex_MatchesFixture(t *testing.T) {
	f := loadSeededFixture(t)
	for i, c := range f.Cases {
		got := SeededIndex(c.Seed, c.Slot, len(c.Words))
		if c.ExpectedIndex == nil {
			t.Logf("case %d: seed=%q slot=%d → index=%d word=%q (record this in fixture)",
				i, c.Seed, c.Slot, got, c.Words[got])
			continue
		}
		if got != *c.ExpectedIndex {
			t.Errorf("case %d: got index %d, want %d", i, got, *c.ExpectedIndex)
		}
	}
}
