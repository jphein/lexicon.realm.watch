package main

import (
	"flag"
	"fmt"
	"io"

	lexicon "github.com/jphein/lexicon.realm.watch/go"
)

func cmdClaim(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("claim", flag.ContinueOnError)
	fs.SetOutput(stderr)
	catalog := fs.String("catalog", "", "path to catalog/projects.yaml")
	renames := fs.String("renames", "", "id of existing project being renamed (omit for new entry)")
	reason := fs.String("reason", "", "reason text attached to the prior_name record")
	retired := fs.String("retired", "", "ISO date for the prior_name record (defaults to today)")
	kind := fs.String("kind", "", "kind: realm-tool|service|library|site|infra|tool (new entries)")
	realm := fs.String("realm", "", "realm (new entries)")
	domain := fs.String("domain", "", "domain (new entries; optional)")
	repo := fs.String("repo", "", "repo URL (new entries; optional)")
	desc := fs.String("description", "", "description (new entries)")
	created := fs.String("created", "", "creation date (new entries; defaults to today)")
	status := fs.String("status", "", "status: active|deprecated|retired|local-only (new entries)")

	// Allow flags to appear before or after the new-name positional.
	var (
		newName  string
		flagArgs = make([]string, 0, len(args))
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		if newName == "" && a != "" && a[0] != '-' {
			newName = a
			continue
		}
		flagArgs = append(flagArgs, a)
	}
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if newName == "" {
		fmt.Fprintln(stderr, "Usage: lexicon claim <new-name> [--renames=<old-id> --reason TEXT | --kind ... --realm ...]")
		return 2
	}

	catPath := resolveCatalogPath(*catalog)
	cat, err := lexicon.LoadCatalog(catPath)
	if err != nil {
		fmt.Fprintf(stderr, "load catalog: %v\n", err)
		return 1
	}
	opts := lexicon.ClaimOpts{
		RenamesOf:   *renames,
		Reason:      *reason,
		Retired:     *retired,
		Kind:        *kind,
		Realm:       *realm,
		Domain:      *domain,
		Repo:        *repo,
		Description: *desc,
		Created:     *created,
		Status:      *status,
	}
	if err := cat.Claim(newName, opts); err != nil {
		fmt.Fprintf(stderr, "lexicon claim: %v\n", err)
		return 1
	}
	if err := cat.Save(); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}
	if *renames != "" {
		fmt.Fprintf(stdout, "renamed %s → %s\n", *renames, newName)
	} else {
		fmt.Fprintf(stdout, "added new entry: %s\n", newName)
	}
	return 0
}
