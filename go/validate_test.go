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
		// Audience set so we isolate the realm warning under test.
		{ID: "needs-classification", CurrentName: "needs-classification", Realm: PendingRealm, Audience: "personal"},
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

func TestValidate_PendingAudienceIsWarning(t *testing.T) {
	cat := &Catalog{Projects: []*Project{
		// No audience, no realm.watch suffix, not a fork → should warn.
		{ID: "speech-to-cli", CurrentName: "speech-to-cli", Realm: "signal", Kind: "tool"},
		// Realm.watch suffix → inferred → no warning.
		{ID: "clock", CurrentName: "clock.realm.watch", Realm: "signal", Kind: "realm-tool"},
		// fork kind → inferred → no warning.
		{ID: "gstack", CurrentName: "gstack", Realm: "signal", Kind: "fork"},
		// Explicit audience → no warning.
		{ID: "donkeyco", CurrentName: "donkeyco", Realm: "signal", Audience: "external"},
	}}
	v := &Vocabulary{categories: map[string]map[string]vocabGroup{
		"realms": {"signal": {Words: []string{"a"}}},
	}}
	issues := Validate(cat, v, &RecipeBook{recipes: map[string]recipeDef{}})
	pending := 0
	for _, i := range issues {
		if i.Code == "pending_audience" {
			pending++
			if i.Subject != "speech-to-cli" {
				t.Errorf("pending_audience on wrong subject: %q", i.Subject)
			}
			if i.Severity != SeverityWarning {
				t.Errorf("pending_audience should be warning, got %q", i.Severity)
			}
		}
	}
	if pending != 1 {
		t.Errorf("expected exactly 1 pending_audience warning, got %d (issues: %v)", pending, issues)
	}
}

func TestInferAudience(t *testing.T) {
	cases := []struct {
		name string
		p    *Project
		want string
	}{
		{"explicit wins", &Project{Audience: "personal", CurrentName: "x.realm.watch", Kind: "fork"}, "personal"},
		{"fork kind", &Project{Kind: "fork", CurrentName: "gstack"}, "fork"},
		{"realm suffix", &Project{Kind: "realm-tool", CurrentName: "clock.realm.watch"}, "realm"},
		{"unclassified", &Project{Kind: "tool", CurrentName: "speech-to-cli"}, ""},
		{"nil project", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := InferAudience(c.p); got != c.want {
				t.Errorf("InferAudience = %q, want %q", got, c.want)
			}
		})
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
