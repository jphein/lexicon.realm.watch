package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"sort"

	lexicon "github.com/jphein/lexicon.realm.watch/go"
)

func cmdRecipes(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("recipes", flag.ContinueOnError)
	fs.SetOutput(stderr)
	vocabs := fs.String("vocabularies", "", "path to vocabularies dir")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rb, err := lexicon.LoadRecipeBook(filepath.Join(resolveVocabulariesDir(*vocabs), "recipes.yaml"))
	if err != nil {
		fmt.Fprintf(stderr, "load recipes: %v\n", err)
		return 1
	}
	names := rb.Names()
	sort.Strings(names)
	for _, n := range names {
		req := rb.RequiredOptions(n)
		fmt.Fprintf(stdout, "%-10s  %s", n, rb.Describe(n))
		if len(req) > 0 {
			fmt.Fprintf(stdout, "  (requires: %v)", req)
		}
		fmt.Fprintln(stdout)
	}
	return 0
}
