package lexicon

import (
	"path/filepath"
	"strings"
	"testing"
)

func loadTestRecipes(t *testing.T) *RecipeBook {
	t.Helper()
	rb, err := LoadRecipeBook(filepath.Join("..", "vocabularies", "recipes.yaml"))
	if err != nil {
		t.Fatalf("LoadRecipeBook: %v", err)
	}
	return rb
}

func loadAllTestVocabs(t *testing.T) *Vocabulary {
	t.Helper()
	v, err := LoadVocabularyFile(filepath.Join(testdataDir(t), "vocabularies-test.yaml"))
	if err != nil {
		t.Fatalf("LoadVocabularyFile: %v", err)
	}
	return v
}

func TestRecipeBook_Has(t *testing.T) {
	rb := loadTestRecipes(t)
	if !rb.Has("project") || !rb.Has("agent") || !rb.Has("branch") || !rb.Has("entity") {
		t.Fatalf("expected all v1 recipes to be present, got %v", rb.Names())
	}
}

func TestRoll_AgentShape(t *testing.T) {
	rb := loadTestRecipes(t)
	v, err := loadLiveVocabsCombined()
	if err != nil {
		t.Fatalf("vocab: %v", err)
	}
	name, err := rb.Roll("agent", v, RollOptions{})
	if err != nil {
		t.Fatalf("Roll: %v", err)
	}
	if !strings.Contains(name, "_") {
		t.Errorf("agent name should contain underscore: %q", name)
	}
	if name != strings.ToLower(name) {
		t.Errorf("agent name should be lowercase: %q", name)
	}
}

func TestRollSeeded_Determinism(t *testing.T) {
	rb := loadTestRecipes(t)
	v, err := loadLiveVocabsCombined()
	if err != nil {
		t.Fatalf("vocab: %v", err)
	}
	a, errA := rb.RollSeeded("agent", v, "fixed-seed-1", RollOptions{})
	b, errB := rb.RollSeeded("agent", v, "fixed-seed-1", RollOptions{})
	if errA != nil || errB != nil {
		t.Fatalf("RollSeeded: %v / %v", errA, errB)
	}
	if a != b {
		t.Errorf("RollSeeded non-deterministic: %q vs %q", a, b)
	}
}

func TestRoll_BranchRequiresPrefix(t *testing.T) {
	rb := loadTestRecipes(t)
	v, err := loadLiveVocabsCombined()
	if err != nil {
		t.Fatalf("vocab: %v", err)
	}
	_, err = rb.Roll("branch", v, RollOptions{})
	if err == nil {
		t.Fatal("expected error when prefix option is missing")
	}
	name, err := rb.Roll("branch", v, RollOptions{Prefix: "feat"})
	if err != nil {
		t.Fatalf("Roll with prefix: %v", err)
	}
	if !strings.HasPrefix(name, "feat/") {
		t.Errorf("branch name should start with feat/: %q", name)
	}
}

func TestRoll_ProjectRequiresRealm(t *testing.T) {
	rb := loadTestRecipes(t)
	v, err := loadLiveVocabsCombined()
	if err != nil {
		t.Fatalf("vocab: %v", err)
	}
	_, err = rb.Roll("project", v, RollOptions{})
	if err == nil {
		t.Fatal("expected error when realm option is missing")
	}
	_, err = rb.Roll("project", v, RollOptions{Realm: "fantasy"})
	if err != nil {
		t.Fatalf("Roll with realm: %v", err)
	}
}

func TestRoll_UnknownRecipe(t *testing.T) {
	rb := loadTestRecipes(t)
	v, err := loadLiveVocabsCombined()
	if err != nil {
		t.Fatalf("vocab: %v", err)
	}
	_, err = rb.Roll("nonsense", v, RollOptions{})
	if err == nil {
		t.Fatal("expected error for unknown recipe")
	}
}

// loadLiveVocabsCombined loads all vocabularies/*.yaml into one Vocabulary.
// In v0.1 we just merge each file's top-level categories.
func loadLiveVocabsCombined() (*Vocabulary, error) {
	combined := &Vocabulary{categories: map[string]map[string]vocabGroup{}}
	files := []string{
		"../vocabularies/realms.yaml",
		"../vocabularies/adjectives.yaml",
		"../vocabularies/nouns.yaml",
		"../vocabularies/scientists.yaml",
		"../vocabularies/creatures.yaml",
	}
	for _, f := range files {
		v, err := LoadVocabularyFile(f)
		if err != nil {
			return nil, err
		}
		for cat, groups := range v.categories {
			if combined.categories[cat] == nil {
				combined.categories[cat] = map[string]vocabGroup{}
			}
			for g, gd := range groups {
				combined.categories[cat][g] = gd
			}
		}
	}
	return combined, nil
}
