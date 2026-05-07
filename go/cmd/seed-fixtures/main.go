// Command seed-fixtures regenerates tests/fixtures/seeded-recipes.json — the
// cross-language parity contract. It loads the canonical vocabularies and
// recipes, calls RollSeeded for a curated set of (seed, recipe, options)
// triples, and prints the result as JSON to stdout.
//
// The contract is intentionally regenerated from the live vocabularies, so
// any vocabulary change requires re-rolling and bumping the parity tests in
// the Python and JS libraries to match.
//
// Usage:
//
//	cd go && /snap/bin/go run ./cmd/seed-fixtures > ../tests/fixtures/seeded-recipes.json
//
// Flags:
//
//	--vocabularies DIR   Override vocabularies directory (default: ../vocabularies)
//	--recipes PATH       Override recipes.yaml path (default: ../vocabularies/recipes.yaml)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	lexicon "github.com/jphein/lexicon.realm.watch/go"
)

type fixtureCase struct {
	Seed         string            `json:"seed"`
	Recipe       string            `json:"recipe"`
	Options      map[string]string `json:"options"`
	ExpectedName string            `json:"expected_name"`
}

type fixtureFile struct {
	Doc       string        `json:"_doc"`
	Algorithm string        `json:"_algorithm"`
	Generator string        `json:"_generator"`
	Cases     []fixtureCase `json:"cases"`
}

// caseSpec describes one parity case before resolution.
type caseSpec struct {
	seed   string
	recipe string
	realm  string
	prefix string
}

func (c caseSpec) options() map[string]string {
	if c.realm == "" && c.prefix == "" {
		return nil
	}
	out := map[string]string{}
	if c.realm != "" {
		out["realm"] = c.realm
	}
	if c.prefix != "" {
		out["prefix"] = c.prefix
	}
	return out
}

// allCases enumerates the parity coverage. Keep this list deterministic and
// well-distributed across recipes, realms, and seed shapes so that any
// language-level divergence in SHA-256, big-endian framing, or modulus is
// caught by at least one case.
func allCases() []caseSpec {
	var cs []caseSpec

	// agent — no options, varied seed shapes (ascii, digits, unicode, long, empty).
	for _, seed := range []string{
		"realmwatch",
		"abc123",
		"the-oracle",
		"jp@techempower.org",
		"中文种子", // Unicode multi-byte — catches non-utf8 framing
		"",                         // empty seed — catches "no input" handling
		"a-very-long-seed-that-should-not-affect-output-determinism-1234567890",
	} {
		cs = append(cs, caseSpec{seed: seed, recipe: "agent"})
	}

	// branch — required prefix, varied prefixes and seeds.
	branchCases := []struct{ seed, prefix string }{
		{"v1.0.0", "feat"},
		{"hotfix-2026", "fix"},
		{"refactor-the-roller", "refactor"},
		{"docs-pass", "docs"},
		{"chore-bump", "chore"},
	}
	for _, b := range branchCases {
		cs = append(cs, caseSpec{seed: b.seed, recipe: "branch", prefix: b.prefix})
	}

	// entity — fantasy-only, but seeds vary.
	for _, seed := range []string{
		"sentinel-zero",
		"oakheart-99",
		"realm.watch",
		"dreamer",
	} {
		cs = append(cs, caseSpec{seed: seed, recipe: "entity"})
	}

	// project — one case per realm, plus a couple of duplicate-realm seeds to
	// catch slot-counter regressions where the same (seed, recipe) but different
	// realm yields the wrong word.
	realms := []string{"fantasy", "tarot", "oracle", "void", "forge", "signal", "stellar"}
	for _, realm := range realms {
		cs = append(cs, caseSpec{seed: "lexicon", recipe: "project", realm: realm})
	}
	// Same realm, different seeds — should yield different names.
	for _, seed := range []string{"alpha", "beta", "gamma"} {
		cs = append(cs, caseSpec{seed: seed, recipe: "project", realm: "fantasy"})
	}
	// Same seed, every realm again with a different seed — extra coverage.
	for _, realm := range realms {
		cs = append(cs, caseSpec{seed: "homelab", recipe: "project", realm: realm})
	}

	return cs
}

func main() {
	fs := flag.NewFlagSet("seed-fixtures", flag.ExitOnError)
	vocabsDir := fs.String("vocabularies", "../vocabularies", "vocabularies directory")
	recipesPath := fs.String("recipes", "", "recipes.yaml path (default: <vocabularies>/recipes.yaml)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if *recipesPath == "" {
		*recipesPath = filepath.Join(*vocabsDir, "recipes.yaml")
	}

	v, err := loadVocabsFromDir(*vocabsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load vocabularies: %v\n", err)
		os.Exit(1)
	}
	rb, err := lexicon.LoadRecipeBook(*recipesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load recipes: %v\n", err)
		os.Exit(1)
	}

	specs := allCases()
	out := fixtureFile{
		Doc:       "Cross-language seeded-recipe parity contract. Each case asserts that RollSeeded(recipe, seed, options) produces expected_name. Python and JS implementations MUST produce identical output for every case.",
		Algorithm: "SeededIndex(seed, slot, modulus) = uint64(BE first 8 bytes of SHA-256(utf8(seed) || BE_uint64_8bytes(slot))) mod modulus. Slot counter increments per slot occurrence within a single roll. Within-roll uniqueness: a slot retries up to len(words) times before falling back to a duplicate.",
		Generator: "go/cmd/seed-fixtures — regenerate via: cd go && /snap/bin/go run ./cmd/seed-fixtures > ../tests/fixtures/seeded-recipes.json",
		Cases:     make([]fixtureCase, 0, len(specs)),
	}
	for _, sp := range specs {
		opts := lexicon.RollOptions{Realm: sp.realm, Prefix: sp.prefix}
		name, err := rb.RollSeeded(sp.recipe, v, sp.seed, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "RollSeeded(%q, seed=%q, %+v): %v\n", sp.recipe, sp.seed, opts, err)
			os.Exit(1)
		}
		out.Cases = append(out.Cases, fixtureCase{
			Seed:         sp.seed,
			Recipe:       sp.recipe,
			Options:      sp.options(),
			ExpectedName: name,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}
}

// loadVocabsFromDir is a private mirror of the same helper in cmd/lexicon —
// duplicating it here keeps cmd/seed-fixtures self-contained and avoids
// promoting an internal helper to the public API just for this generator.
func loadVocabsFromDir(dir string) (*lexicon.Vocabulary, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	var combined *lexicon.Vocabulary
	for _, m := range matches {
		if filepath.Base(m) == "recipes.yaml" {
			continue
		}
		v, err := lexicon.LoadVocabularyFile(m)
		if err != nil {
			return nil, err
		}
		if combined == nil {
			combined = v
			continue
		}
		for _, cat := range v.Categories() {
			for _, group := range v.Groups(cat) {
				words, _ := v.Group(cat, group)
				combined.AddGroup(cat, group, words)
			}
		}
	}
	if combined == nil {
		return nil, fmt.Errorf("no vocabulary files in %s", dir)
	}
	return combined, nil
}
