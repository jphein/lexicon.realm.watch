package main

import (
	"flag"
	"fmt"
	"io"

	lexicon "github.com/jphein/lexicon.realm.watch/go"
)

func cmdResolve(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("resolve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	catalog := fs.String("catalog", "", "path to catalog/projects.yaml")

	// Allow flags to appear before or after the name positional.
	var (
		name     string
		flagArgs = make([]string, 0, len(args))
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		if name == "" && a != "" && a[0] != '-' {
			name = a
			continue
		}
		flagArgs = append(flagArgs, a)
	}
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if name == "" {
		fmt.Fprintln(stderr, "Usage: lexicon resolve <name>")
		return 2
	}

	cat, err := lexicon.LoadCatalog(resolveCatalogPath(*catalog))
	if err != nil {
		fmt.Fprintf(stderr, "load catalog: %v\n", err)
		return 1
	}
	p, ok := cat.Resolve(name)
	if !ok {
		fmt.Fprintf(stderr, "no project named %q in catalog\n", name)
		return 1
	}
	fmt.Fprintf(stdout, "id:           %s\n", p.ID)
	fmt.Fprintf(stdout, "current_name: %s\n", p.CurrentName)
	fmt.Fprintf(stdout, "kind:         %s\n", p.Kind)
	fmt.Fprintf(stdout, "realm:        %s\n", p.Realm)
	if p.Domain != "" {
		fmt.Fprintf(stdout, "domain:       %s\n", p.Domain)
	}
	if p.Repo != "" {
		fmt.Fprintf(stdout, "repo:         %s\n", p.Repo)
	}
	fmt.Fprintf(stdout, "status:       %s\n", p.Status)
	if p.Description != "" {
		fmt.Fprintf(stdout, "description:  %s\n", p.Description)
	}
	if len(p.PriorNames) > 0 {
		fmt.Fprintln(stdout, "prior names:")
		for _, pn := range p.PriorNames {
			fmt.Fprintf(stdout, "  - %s (retired %s) %s\n", pn.Name, pn.Retired, pn.Reason)
		}
	}
	return 0
}
