package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"

	lexicon "github.com/jphein/lexicon.realm.watch/go"
)

func cmdRoll(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("roll", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		realm   = fs.String("realm", "", "realm (required for some recipes)")
		prefix  = fs.String("prefix", "", "prefix (required for branch recipe)")
		n       = fs.Int("n", 1, "number of candidates to roll")
		vocabs  = fs.String("vocabularies", "", "path to vocabularies directory")
		recipes = fs.String("recipes", "", "path to recipes.yaml")
	)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: lexicon roll <recipe> [--realm X] [--prefix Y] [--n N]")
		fs.PrintDefaults()
	}
	// Allow flags to appear before or after the recipe positional. Extract the
	// first non-flag arg as the recipe, leaving the rest for flag.Parse.
	var (
		recipe   string
		flagArgs = make([]string, 0, len(args))
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		if recipe == "" && a != "" && a[0] != '-' {
			recipe = a
			continue
		}
		flagArgs = append(flagArgs, a)
	}
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if recipe == "" {
		fmt.Fprintln(stderr, "lexicon roll: recipe name required")
		fs.Usage()
		return 2
	}

	vocabsDir := resolveVocabulariesDir(*vocabs)
	recipesPath := *recipes
	if recipesPath == "" {
		recipesPath = filepath.Join(vocabsDir, "recipes.yaml")
	}

	v, err := loadVocabsFromDir(vocabsDir)
	if err != nil {
		fmt.Fprintf(stderr, "load vocabularies: %v\n", err)
		return 1
	}
	rb, err := lexicon.LoadRecipeBook(recipesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load recipes: %v\n", err)
		return 1
	}

	opts := lexicon.RollOptions{Realm: *realm, Prefix: *prefix}
	if *n <= 1 {
		name, err := rb.Roll(recipe, v, opts)
		if err != nil {
			fmt.Fprintf(stderr, "lexicon roll: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, name)
		return 0
	}
	names, err := rb.RollN(recipe, v, *n, opts)
	if err != nil {
		fmt.Fprintf(stderr, "lexicon roll: %v\n", err)
		return 1
	}
	for _, name := range names {
		fmt.Fprintln(stdout, name)
	}
	return 0
}

// loadVocabsFromDir merges every *.yaml under dir into one Vocabulary,
// excluding recipes.yaml (handled separately).
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
