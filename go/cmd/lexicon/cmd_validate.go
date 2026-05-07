package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"

	lexicon "github.com/jphein/lexicon.realm.watch/go"
)

func cmdValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	catalog := fs.String("catalog", "", "path to catalog/projects.yaml")
	vocabs := fs.String("vocabularies", "", "path to vocabularies dir")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cat, err := lexicon.LoadCatalog(resolveCatalogPath(*catalog))
	if err != nil {
		fmt.Fprintf(stderr, "load catalog: %v\n", err)
		return 1
	}
	v, err := loadVocabsFromDir(resolveVocabulariesDir(*vocabs))
	if err != nil {
		fmt.Fprintf(stderr, "load vocabularies: %v\n", err)
		return 1
	}
	rb, err := lexicon.LoadRecipeBook(filepath.Join(resolveVocabulariesDir(*vocabs), "recipes.yaml"))
	if err != nil {
		fmt.Fprintf(stderr, "load recipes: %v\n", err)
		return 1
	}
	issues := lexicon.Validate(cat, v, rb)
	if len(issues) == 0 {
		fmt.Fprintln(stdout, "OK")
		return 0
	}
	errors, warnings := 0, 0
	for _, i := range issues {
		fmt.Fprintln(stderr, i)
		if i.Severity == lexicon.SeverityWarning {
			warnings++
		} else {
			errors++
		}
	}
	if errors == 0 {
		fmt.Fprintf(stdout, "OK (with %d warning(s))\n", warnings)
		return 0
	}
	return 1
}
