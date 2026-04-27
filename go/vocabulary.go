// Package lexicon provides themed name rolling and a project catalog for the
// realm.watch family of tools. See docs/superpowers/specs/ for the design.
package lexicon

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Vocabulary is a flat dictionary of category → group → words, loaded from a
// single YAML file. The YAML's top-level keys (e.g. "realms", "adjectives")
// each map to a map of group name → group definition. The group definition
// has a description and a list of words.
type Vocabulary struct {
	categories map[string]map[string]vocabGroup
}

type vocabGroup struct {
	Description string   `yaml:"description"`
	Words       []string `yaml:"words"`
}

// LoadVocabularyFile reads one of the vocabularies/*.yaml files and merges
// its categories into a Vocabulary. Multiple files may be loaded by calling
// LoadVocabularyFile once per file and merging via NewVocabulary; v0.1 only
// needs single-file loads.
func LoadVocabularyFile(path string) (*Vocabulary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var raw map[string]map[string]vocabGroup
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &Vocabulary{categories: raw}, nil
}

// Group returns the words for category/group, or (nil, false) if the pair is
// unknown. The returned slice should be treated as read-only by callers.
func (v *Vocabulary) Group(category, group string) ([]string, bool) {
	cat, ok := v.categories[category]
	if !ok {
		return nil, false
	}
	g, ok := cat[group]
	if !ok {
		return nil, false
	}
	return g.Words, true
}

// Categories lists the top-level categories present in the loaded data.
func (v *Vocabulary) Categories() []string {
	out := make([]string, 0, len(v.categories))
	for k := range v.categories {
		out = append(out, k)
	}
	return out
}

// Groups lists the groups present in a category.
func (v *Vocabulary) Groups(category string) []string {
	cat, ok := v.categories[category]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(cat))
	for k := range cat {
		out = append(out, k)
	}
	return out
}
