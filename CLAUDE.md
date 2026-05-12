# CLAUDE.md — lexicon.realm.watch

This file gives Claude Code project-specific instructions when working in this repo.

## Project shape

- Go library + `lexicon` CLI in `go/`
- Vocabularies (YAML) in `vocabularies/`
- Catalog (canonical project registry) in `catalog/projects.yaml`; agents in `catalog/agents.yaml`, voices in `catalog/voices.yaml`, network hosts in `catalog/hosts.yaml` (provisional schema — Go side does not yet validate or render hosts.yaml)
- Spec: `docs/superpowers/specs/2026-04-26-lexicon-design.md`
- v0.1 plan: `docs/superpowers/plans/2026-04-26-lexicon-v0.1.md`

## Don't

- Don't edit `tests/fixtures/seeded-recipes.json` `expected_index` values without re-running the parity test. They are the cross-language contract.
- Don't edit `vocabularies/*.yaml` without running `lexicon validate` afterwards.
- Don't introduce a database, HTTP service, or persistence layer. The catalog is a YAML file. If a v2 design eventually wraps it in a service, that decision lives in a new spec — not in this codebase.

## Testing

Run all tests: `cd go && go test ./...`
Smoke-test the CLI: `cd go && go run ./cmd/lexicon roll agent`

Note: `go -C go run ./cmd/lexicon ...` from the project root looks tempting but breaks because `-C` changes the cwd of the spawned binary too, which then can't find `./vocabularies`. Use `cd go && go run ./cmd/lexicon ...` and pass `--vocabularies ../vocabularies --catalog ../catalog/projects.yaml`, or build the binary first.

## Versioning

This project follows realm-sigil's version contract via `version.json`. Each release bumps `version.json` and tags `vX.Y.Z`.
