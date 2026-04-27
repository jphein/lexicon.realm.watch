package lexicon

import "fmt"

// Issue is a single validation finding.
type Issue struct {
	Code    string // machine-readable: unknown_realm, missing_source, ...
	Subject string // what the issue is about (project id, recipe name, etc.)
	Detail  string // human-readable detail
}

func (i Issue) String() string {
	return fmt.Sprintf("[%s] %s: %s", i.Code, i.Subject, i.Detail)
}

// Validate cross-checks catalog references against vocabularies and recipes.
// Returns nil/empty when everything is consistent.
func Validate(cat *Catalog, v *Vocabulary, rb *RecipeBook) []Issue {
	issues := []Issue{}

	// Catalog: every project's realm must exist in vocabularies/realms.yaml.
	if cat != nil && v != nil {
		for _, p := range cat.Projects {
			if p.Realm == "" {
				continue
			}
			if _, ok := v.Group("realms", p.Realm); !ok {
				issues = append(issues, Issue{
					Code:    "unknown_realm",
					Subject: p.ID,
					Detail:  fmt.Sprintf("realm %q not defined in vocabularies/realms.yaml", p.Realm),
				})
			}
		}
	}

	// Recipes: every source must resolve to a non-empty group.
	if rb != nil && v != nil {
		for name, r := range rb.recipes {
			for slot, src := range r.Sources {
				words, ok := v.Group(src.From, src.Group)
				if !ok || len(words) == 0 {
					issues = append(issues, Issue{
						Code:    "missing_source",
						Subject: name,
						Detail: fmt.Sprintf("slot %q references %s.%s which is missing or empty",
							slot, src.From, src.Group),
					})
				}
			}
		}
	}

	return issues
}
