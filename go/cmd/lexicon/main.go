// Command lexicon is the CLI entrypoint for lexicon.realm.watch.
//
//	lexicon roll <recipe> [--realm X] [--prefix Y] [--n N]
//	lexicon resolve <name>
//	lexicon list [--realm X] [--kind Y] [--status Z]
//	lexicon claim <new-name> --renames=<old-id> [--reason TEXT]
//	lexicon validate
//	lexicon recipes
//	lexicon vocabularies
package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

// run is the testable entrypoint. argv[0] is the program name.
func run(argv []string, stdout, stderr io.Writer) int {
	if len(argv) < 2 || argv[1] == "--help" || argv[1] == "-h" || argv[1] == "help" {
		printHelp(stdout)
		return 0
	}

	cmd, rest := argv[1], argv[2:]
	switch cmd {
	case "roll":
		return cmdRoll(rest, stdout, stderr)
	case "resolve":
		return cmdResolve(rest, stdout, stderr)
	case "list":
		return cmdList(rest, stdout, stderr)
	case "validate":
		return cmdValidate(rest, stdout, stderr)
	case "recipes":
		return cmdRecipes(rest, stdout, stderr)
	case "vocabularies":
		return cmdVocabularies(rest, stdout, stderr)
	case "claim":
		return cmdClaim(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", cmd)
		printHelp(stderr)
		return 2
	}
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, `lexicon — names and the changing of names

Usage: lexicon <command> [options]

Commands:
  roll <recipe>           roll candidate name(s) from a recipe
  resolve <name>          look up a project by any name it has had
  list                    list catalog entries (filterable)
  claim <new-name>        record a new entry or rename an existing one
  validate                check catalog/vocabularies/recipes for consistency
  recipes                 list available recipes
  vocabularies            list vocabulary groups

Common flags:
  --catalog PATH          override the catalog path (default: ./catalog/projects.yaml)
  --vocabularies PATH     override the vocabularies dir (default: ./vocabularies)

Run 'lexicon <command> --help' for command-specific options.`)
}

// Default paths — relative to the current working directory. Most commands
// resolve these via the env-var / flag overrides set up below.
const (
	defaultCatalogPath     = "catalog/projects.yaml"
	defaultVocabulariesDir = "vocabularies"
	defaultRecipesYAMLPath = "vocabularies/recipes.yaml"
)

func resolveCatalogPath(override string) string {
	if override != "" {
		return override
	}
	if env := os.Getenv("LEXICON_CATALOG"); env != "" {
		return env
	}
	return defaultCatalogPath
}

func resolveVocabulariesDir(override string) string {
	if override != "" {
		return override
	}
	if env := os.Getenv("LEXICON_VOCABULARIES"); env != "" {
		return env
	}
	return defaultVocabulariesDir
}

// Stub command handlers — overwritten by Tasks 14–19 each implementing one.
// Until those tasks land, every command returns a "not yet implemented" error.

func cmdValidate(args []string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "lexicon validate: not yet implemented (Task 17)")
	return 1
}
func cmdRecipes(args []string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "lexicon recipes: not yet implemented (Task 18)")
	return 1
}
func cmdVocabularies(args []string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "lexicon vocabularies: not yet implemented (Task 18)")
	return 1
}
func cmdClaim(args []string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "lexicon claim: not yet implemented (Task 19)")
	return 1
}
