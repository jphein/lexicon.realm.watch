package lexicon

import (
	"fmt"
	"math/rand/v2"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// RollOptions carries optional inputs that some recipes require.
// Add new fields as new recipes need them. Recipes declare which options
// they require via `required_options` in recipes.yaml.
type RollOptions struct {
	Realm  string
	Prefix string
}

// recipeDef mirrors the YAML shape of a single recipe.
type recipeDef struct {
	Description     string                  `yaml:"description"`
	Pattern         string                  `yaml:"pattern"`
	Sources         map[string]recipeSource `yaml:"sources"`
	RequiredOptions []string                `yaml:"required_options"`
}

type recipeSource struct {
	From  string `yaml:"from"`  // category in the vocabulary
	Group string `yaml:"group"` // group within the category
}

// RecipeBook is the loaded recipes.yaml.
type RecipeBook struct {
	recipes map[string]recipeDef
}

// LoadRecipeBook reads vocabularies/recipes.yaml.
func LoadRecipeBook(path string) (*RecipeBook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var raw struct {
		Recipes map[string]recipeDef `yaml:"recipes"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &RecipeBook{recipes: raw.Recipes}, nil
}

// Has reports whether the named recipe is defined.
func (rb *RecipeBook) Has(name string) bool {
	_, ok := rb.recipes[name]
	return ok
}

// Names returns the recipe names in undefined order.
func (rb *RecipeBook) Names() []string {
	out := make([]string, 0, len(rb.recipes))
	for n := range rb.recipes {
		out = append(out, n)
	}
	return out
}

// Describe returns a recipe's description, or "" if unknown.
func (rb *RecipeBook) Describe(name string) string {
	r, ok := rb.recipes[name]
	if !ok {
		return ""
	}
	return r.Description
}

// RequiredOptions returns the option names this recipe requires.
func (rb *RecipeBook) RequiredOptions(name string) []string {
	r, ok := rb.recipes[name]
	if !ok {
		return nil
	}
	return r.RequiredOptions
}

// Roll produces one name from the recipe, using a non-deterministic source.
func (rb *RecipeBook) Roll(name string, v *Vocabulary, opts RollOptions) (string, error) {
	r, ok := rb.recipes[name]
	if !ok {
		return "", fmt.Errorf("unknown recipe %q (have %v)", name, rb.Names())
	}
	if err := checkRequired(r, opts); err != nil {
		return "", err
	}
	return rb.fill(r, v, opts, func(_ uint64, modulus int) int {
		return rand.IntN(modulus)
	})
}

// RollSeeded produces one name deterministically from a seed.
func (rb *RecipeBook) RollSeeded(name string, v *Vocabulary, seed string, opts RollOptions) (string, error) {
	r, ok := rb.recipes[name]
	if !ok {
		return "", fmt.Errorf("unknown recipe %q (have %v)", name, rb.Names())
	}
	if err := checkRequired(r, opts); err != nil {
		return "", err
	}
	return rb.fill(r, v, opts, func(slot uint64, modulus int) int {
		return SeededIndex(seed, slot, modulus)
	})
}

// fill runs the pattern engine. The pickIndex function decides how each slot
// resolves its index — random for Roll, SHA256-derived for RollSeeded.
func (rb *RecipeBook) fill(r recipeDef, v *Vocabulary, opts RollOptions, pickIndex func(slot uint64, modulus int) int) (string, error) {
	tokens, err := parsePattern(r.Pattern)
	if err != nil {
		return "", err
	}
	out := strings.Builder{}
	// Track per-slot occurrence count for independent rolls of repeated slot names.
	slotCounter := map[string]uint64{}
	// Track already-picked indices per slot name for uniqueness within a roll.
	picked := map[string]map[int]bool{}

	for _, tk := range tokens {
		if tk.literal != "" {
			out.WriteString(tk.literal)
			continue
		}
		// Slot — resolve from sources or from opts.
		if val, isOpt, err := resolveOption(tk.slot, opts); err != nil {
			return "", err
		} else if isOpt {
			out.WriteString(applyTransform(val, tk.transform))
			continue
		}
		src, ok := r.Sources[tk.slot]
		if !ok {
			return "", fmt.Errorf("recipe %q: slot %q has no source and no option provided", r.Description, tk.slot)
		}
		group := src.Group
		// "fantasy"-group sources get overridden by --realm if the recipe declares
		// realm as a required option (project recipe). For simplicity in v0.1,
		// we substitute the literal group name "fantasy" with the user's realm.
		if group == "fantasy" && opts.Realm != "" {
			group = opts.Realm
		}
		words, ok := v.Group(src.From, group)
		if !ok || len(words) == 0 {
			return "", fmt.Errorf("recipe %q: source %s.%s missing or empty", r.Description, src.From, group)
		}
		// Slot uniqueness within a roll: try up to len(words) attempts
		// before falling back to a possibly-duplicate pick.
		seen := picked[tk.slot]
		if seen == nil {
			seen = map[int]bool{}
			picked[tk.slot] = seen
		}
		var idx int
		for attempt := 0; attempt < len(words); attempt++ {
			candidate := pickIndex(slotCounter[tk.slot], len(words))
			slotCounter[tk.slot]++
			if !seen[candidate] || len(seen) >= len(words) {
				idx = candidate
				break
			}
		}
		seen[idx] = true
		out.WriteString(applyTransform(words[idx], tk.transform))
	}
	return out.String(), nil
}

// patternToken is either a literal string segment or a slot reference.
type patternToken struct {
	literal   string
	slot      string
	transform string
}

var slotRegex = regexp.MustCompile(`\{([a-z_]+)(?::([a-z]+))?\}`)

func parsePattern(pat string) ([]patternToken, error) {
	tokens := []patternToken{}
	idx := 0
	for _, m := range slotRegex.FindAllStringSubmatchIndex(pat, -1) {
		start, end := m[0], m[1]
		if start > idx {
			tokens = append(tokens, patternToken{literal: pat[idx:start]})
		}
		slot := pat[m[2]:m[3]]
		transform := "raw"
		if m[4] != -1 {
			transform = pat[m[4]:m[5]]
		}
		tokens = append(tokens, patternToken{slot: slot, transform: transform})
		idx = end
	}
	if idx < len(pat) {
		tokens = append(tokens, patternToken{literal: pat[idx:]})
	}
	return tokens, nil
}

func applyTransform(word, transform string) string {
	switch transform {
	case "cap":
		if word == "" {
			return word
		}
		return strings.ToUpper(word[:1]) + word[1:]
	case "lower":
		return strings.ToLower(word)
	case "upper":
		return strings.ToUpper(word)
	default:
		return word
	}
}

func resolveOption(slot string, opts RollOptions) (string, bool, error) {
	switch slot {
	case "prefix":
		if opts.Prefix == "" {
			return "", true, fmt.Errorf("option 'prefix' is required")
		}
		return opts.Prefix, true, nil
	case "realm":
		if opts.Realm == "" {
			return "", true, fmt.Errorf("option 'realm' is required")
		}
		return opts.Realm, true, nil
	}
	return "", false, nil
}

func checkRequired(r recipeDef, opts RollOptions) error {
	for _, req := range r.RequiredOptions {
		switch req {
		case "realm":
			if opts.Realm == "" {
				return fmt.Errorf("recipe requires --realm")
			}
		case "prefix":
			if opts.Prefix == "" {
				return fmt.Errorf("recipe requires --prefix")
			}
		}
	}
	return nil
}
