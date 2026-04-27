package main

import (
	"flag"
	"fmt"
	"io"
	"sort"
)

func cmdVocabularies(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("vocabularies", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("vocabularies", "", "path to vocabularies dir")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	v, err := loadVocabsFromDir(resolveVocabulariesDir(*dir))
	if err != nil {
		fmt.Fprintf(stderr, "load vocabularies: %v\n", err)
		return 1
	}
	cats := v.Categories()
	sort.Strings(cats)
	for _, cat := range cats {
		fmt.Fprintf(stdout, "%s\n", cat)
		groups := v.Groups(cat)
		sort.Strings(groups)
		for _, g := range groups {
			words, _ := v.Group(cat, g)
			fmt.Fprintf(stdout, "  %-12s (%d words)\n", g, len(words))
		}
	}
	return 0
}
