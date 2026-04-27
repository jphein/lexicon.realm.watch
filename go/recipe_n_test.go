package lexicon

import "testing"

func TestRollN_UniqueWithinSet(t *testing.T) {
	rb := loadTestRecipes(t)
	v, err := loadLiveVocabsCombined()
	if err != nil {
		t.Fatalf("vocab: %v", err)
	}
	got, err := rb.RollN("agent", v, 5, RollOptions{})
	if err != nil {
		t.Fatalf("RollN: %v", err)
	}
	if len(got) != 5 {
		t.Errorf("got %d, want 5", len(got))
	}
	seen := map[string]bool{}
	for _, n := range got {
		if seen[n] {
			t.Errorf("duplicate name: %q", n)
		}
		seen[n] = true
	}
}

func TestRollN_CapsAtCombinatorialSpace(t *testing.T) {
	rb := loadTestRecipes(t)
	v := &Vocabulary{categories: map[string]map[string]vocabGroup{
		"adjectives": {"any": {Words: []string{"a", "b"}}},
		"scientists": {"any": {Words: []string{"x", "y"}}},
	}}
	got, err := rb.RollN("agent", v, 100, RollOptions{})
	if err != nil {
		t.Fatalf("RollN: %v", err)
	}
	if len(got) > 4 {
		t.Errorf("got %d candidates, expected ≤4", len(got))
	}
}
