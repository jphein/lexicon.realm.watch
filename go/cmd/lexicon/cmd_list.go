package main

import (
	"flag"
	"fmt"
	"io"

	lexicon "github.com/jphein/lexicon.realm.watch/go"
)

func cmdList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	catalog := fs.String("catalog", "", "path to catalog/projects.yaml")
	realm := fs.String("realm", "", "filter by realm")
	kind := fs.String("kind", "", "filter by kind")
	status := fs.String("status", "", "filter by status")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cat, err := lexicon.LoadCatalog(resolveCatalogPath(*catalog))
	if err != nil {
		fmt.Fprintf(stderr, "load catalog: %v\n", err)
		return 1
	}
	for _, p := range cat.Projects {
		if *realm != "" && p.Realm != *realm {
			continue
		}
		if *kind != "" && p.Kind != *kind {
			continue
		}
		if *status != "" && p.Status != *status {
			continue
		}
		desc := p.Description
		if desc == "" {
			desc = "(no description)"
		}
		fmt.Fprintf(stdout, "%-15s %-30s [%s/%s/%s]  %s\n",
			p.ID, p.CurrentName, p.Kind, p.Realm, p.Status, desc)
	}
	return 0
}
