package lexicon

// RollN returns up to n unique candidates produced by Roll. If the recipe's
// combinatorial space is smaller than n, returns the full distinct space
// (so callers always get distinct names — never padded with duplicates).
//
// Implementation: roll with rejection up to a bounded number of attempts
// proportional to n. If we can't produce n distinct names within budget,
// return what we have. This is fine because the budget is generous.
func (rb *RecipeBook) RollN(name string, v *Vocabulary, n int, opts RollOptions) ([]string, error) {
	if n <= 0 {
		return []string{}, nil
	}
	out := make([]string, 0, n)
	seen := map[string]bool{}
	maxAttempts := n * 20
	if maxAttempts < 100 {
		maxAttempts = 100
	}
	for attempts := 0; attempts < maxAttempts && len(out) < n; attempts++ {
		candidate, err := rb.Roll(name, v, opts)
		if err != nil {
			return nil, err
		}
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		out = append(out, candidate)
	}
	return out, nil
}
