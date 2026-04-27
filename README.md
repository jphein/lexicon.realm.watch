# lexicon.realm.watch

Names and the changing of names. The third realm.watch tool — works alongside [clock](https://github.com/jphein/clock.realm.watch) (cache invalidation) and [realm-sigil](https://github.com/jphein/realm-sigil) (deterministic build-time naming) to handle the runtime naming surface and the project-rename workflow.

> *There are only two hard things in computer science. Clock handles one, sigil handles the other, lexicon is what you reach for when they collide.*

## What it does

- **Roll names** for the things you create every day: agents, branches, projects, entities. Themed vocabularies, deterministic when you want it, random when you don't.
- **Be the system of record** for every project in `~/Projects/` — current name, prior names, realm, kind, repo, domain. Catalog lives in `catalog/projects.yaml`; git is your audit log.
- **Drive renames** of existing projects (v0.2 — coming soon).
- **Own the canonical vocabularies** — realms, adjectives, nouns, scientists, creatures. In v1.0 these become the source of truth that realm-sigil consumes.

## Quick start

```bash
# Build
cd go && go build -o lexicon ./cmd/lexicon

# Roll names (run from the project root, with the binary at go/lexicon)
go/lexicon roll agent                       # → "gallant_curie"
go/lexicon roll project --realm fantasy     # → "Lunar-Pulsar"
go/lexicon roll branch --prefix feat        # → "feat/lunar-pulsar-forge"
go/lexicon roll project --realm fantasy --n=5

# Catalog queries
go/lexicon resolve dreamspace               # follows prior-name to current
go/lexicon list --realm signal
go/lexicon validate

# Record a rename
go/lexicon claim watch.realm.watch --renames realmwatch --reason "moved into realm.watch family"
```

The CLI resolves `--catalog` and `--vocabularies` paths relative to the current directory; defaults are `./catalog/projects.yaml` and `./vocabularies`. Override with the flags above or the `LEXICON_CATALOG` / `LEXICON_VOCABULARIES` environment variables.

## Design

See `docs/superpowers/specs/2026-04-26-lexicon-design.md` for the full design and `docs/superpowers/plans/2026-04-26-lexicon-v0.1.md` for the v0.1 implementation plan.

## License

MIT — see `LICENSE`.
