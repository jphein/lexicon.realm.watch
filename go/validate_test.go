package lexicon

import (
	"path/filepath"
	"testing"
)

func TestValidate_LiveDataPasses(t *testing.T) {
	cat, err := LoadCatalog(filepath.Join("..", "catalog", "projects.yaml"))
	if err != nil {
		t.Skip("catalog not yet seeded:", err)
	}
	v, err := loadLiveVocabsCombined()
	if err != nil {
		t.Fatalf("vocab: %v", err)
	}
	rb, err := LoadRecipeBook(filepath.Join("..", "vocabularies", "recipes.yaml"))
	if err != nil {
		t.Fatalf("recipes: %v", err)
	}
	issues := Validate(cat, v, rb)
	for _, i := range issues {
		if i.Severity != SeverityWarning {
			t.Errorf("expected only warnings, got error-level issue: %v", i)
		}
	}
}

func TestValidate_PendingRealmIsWarning(t *testing.T) {
	cat := &Catalog{Projects: []*Project{
		{ID: "needs-classification", CurrentName: "needs-classification", Realm: PendingRealm},
	}}
	v := &Vocabulary{categories: map[string]map[string]vocabGroup{
		"realms": {"fantasy": {Words: []string{"a"}}},
	}}
	issues := Validate(cat, v, &RecipeBook{recipes: map[string]recipeDef{}})
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %v", len(issues), issues)
	}
	got := issues[0]
	if got.Code != "pending_realm" || got.Severity != SeverityWarning {
		t.Errorf("expected pending_realm/warning, got %v", got)
	}
}

func TestValidate_DetectsMissingRealm(t *testing.T) {
	cat := &Catalog{Projects: []*Project{
		{ID: "x", CurrentName: "x", Realm: "no_such_realm"},
	}}
	v := &Vocabulary{categories: map[string]map[string]vocabGroup{
		"realms": {"fantasy": {Words: []string{"a"}}},
	}}
	rb := &RecipeBook{recipes: map[string]recipeDef{}}
	issues := Validate(cat, v, rb)
	found := false
	for _, i := range issues {
		if i.Code == "unknown_realm" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unknown_realm issue, got %v", issues)
	}
}

func TestValidate_DetectsRecipeSourceMissing(t *testing.T) {
	v := &Vocabulary{categories: map[string]map[string]vocabGroup{
		"adjectives": {"any": {Words: []string{"a"}}},
	}}
	rb := &RecipeBook{recipes: map[string]recipeDef{
		"bad": {
			Pattern: "{noun}",
			Sources: map[string]recipeSource{
				"noun": {From: "nouns", Group: "any"},
			},
		},
	}}
	issues := Validate(&Catalog{}, v, rb)
	found := false
	for _, i := range issues {
		if i.Code == "missing_source" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing_source issue, got %v", issues)
	}
}
