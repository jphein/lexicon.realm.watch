package lexicon

import "fmt"

// Severity classifies a validation finding. "error" means the catalog is
// broken; "warning" means it's intentional-pending and should be visible
// without blocking CI / commit hooks.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// PendingRealm is the sentinel value an operator writes in catalog/projects.yaml
// when a project is recorded but its realm hasn't been chosen yet. It validates
// as a warning, not an error, so bulk migrations can land without forcing a
// premature classification call.
const PendingRealm = "?"

// Issue is a single validation finding.
type Issue struct {
	Severity string // "error" or "warning"
	Code     string // machine-readable: unknown_realm, missing_source, pending_realm, ...
	Subject  string // what the issue is about (project id, recipe name, etc.)
	Detail   string // human-readable detail
}

func (i Issue) String() string {
	sev := i.Severity
	if sev == "" {
		sev = SeverityError
	}
	return fmt.Sprintf("[%s/%s] %s: %s", sev, i.Code, i.Subject, i.Detail)
}

// Validate cross-checks catalog references against vocabularies and recipes.
// Returns nil/empty when everything is consistent.
func Validate(cat *Catalog, v *Vocabulary, rb *RecipeBook) []Issue {
	issues := []Issue{}

	// Catalog: every project's realm must exist in vocabularies/realms.yaml.
	// The literal "?" is a sentinel — it means the realm is intentionally
	// unclassified. We surface that as a warning (review needed) rather than
	// an error so bulk imports can land.
	if cat != nil && v != nil {
		for _, p := range cat.Projects {
			if p.Realm == "" {
				continue
			}
			if p.Realm == PendingRealm {
				issues = append(issues, Issue{
					Severity: SeverityWarning,
					Code:     "pending_realm",
					Subject:  p.ID,
					Detail:   "realm pending classification — set to a real realm or remove the entry",
				})
				continue
			}
			if _, ok := v.Group("realms", p.Realm); !ok {
				issues = append(issues, Issue{
					Severity: SeverityError,
					Code:     "unknown_realm",
					Subject:  p.ID,
					Detail:   fmt.Sprintf("realm %q not defined in vocabularies/realms.yaml", p.Realm),
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
						Severity: SeverityError,
						Code:     "missing_source",
						Subject:  name,
						Detail: fmt.Sprintf("slot %q references %s.%s which is missing or empty",
							slot, src.From, src.Group),
					})
				}
			}
		}
	}

	return issues
}
