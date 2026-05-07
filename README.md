# lexicon.realm.watch

Names and the changing of names. The third realm.watch tool — works alongside [clock](https://github.com/jphein/clock.realm.watch) (cache invalidation at the prompt seam) and [realm-sigil](https://github.com/jphein/realm-sigil) (deterministic build-time naming) to handle the runtime naming surface and the project-rename workflow.

> *There are only two hard things in computer science. Clock handles one, sigil handles the other, lexicon is what you reach for when they collide.*

Live at **[jphein.github.io/lexicon.realm.watch](https://jphein.github.io/lexicon.realm.watch/)**.

## What it does

- **Roll names** for the things you create every day: agents, branches, projects, entities. Themed vocabularies, deterministic when you want it, random when you don't.
- **Be the system of record** for every project in `~/Projects/` — current name, prior names, realm, kind, repo, domain. Catalog lives in `catalog/projects.yaml`; git is your audit log.
- **Drive renames** of existing projects through a 10-step guided runbook — directory mv, transitional symlink, package metadata sweep, GitHub repo rename, Claude Code session storage, catalog mutation, manual-verify checklist.
- **Own the canonical vocabularies** — realms, adjectives, nouns, scientists, creatures. As of v1.0 these are the source of truth that realm-sigil consumes.

## Three implementations, one contract

Lexicon ships as a Go binary plus parity libraries in Python and JavaScript. Same recipe names, same option keys, same results. Cross-language consistency is enforced by a shared fixture (`tests/fixtures/seeded-recipes.json`): given a seed and a recipe, every implementation must produce the same name byte-for-byte.

```
seed: "lexicon" + recipe: project + realm: signal
  → "Resonant-Pulsar"   in Go
  → "Resonant-Pulsar"   in Python
  → "Resonant-Pulsar"   in JS (Node and browser)
```

The algorithm is the same shape as realm-sigil's: SHA-256 of `(utf8(seed) || big-endian uint64(slot))`, take first 8 bytes big-endian as a uint64, mod the word-list length.

## Install

### Go binary (the CLI)

```bash
git clone https://github.com/jphein/lexicon.realm.watch.git
cd lexicon.realm.watch/go
go build -o lexicon ./cmd/lexicon
# Optionally:
sudo mv lexicon /usr/local/bin/
```

The binary resolves `--catalog` and `--vocabularies` paths relative to the current directory; defaults are `./catalog/projects.yaml` and `./vocabularies`. Override with the flags above or the `LEXICON_CATALOG` / `LEXICON_VOCABULARIES` environment variables.

### Python library

```bash
pip install -e python/    # from the repo root
```

```python
from lexicon import roll, roll_seeded, load_catalog

agent  = roll("agent",   vocabularies_dir="vocabularies")
project = roll("project", realm="signal", vocabularies_dir="vocabularies")
branch  = roll_seeded("branch", "v1.0.0", prefix="feat", vocabularies_dir="vocabularies")

cat = load_catalog("catalog/projects.yaml")
proj = cat.resolve("realmwatch")    # follows prior_names → current_name
```

Depends on `ruamel.yaml` for round-trippable catalog edits. Tested on Python ≥3.10.

### JavaScript library (Node and browser)

```bash
cd js && npm install
# then in your project:
npm install @jphein/lexicon
```

```js
import { roll, rollSeeded, loadCatalog } from '@jphein/lexicon';

const agent   = await roll('agent',   { vocabulariesDir: 'vocabularies' });
const project = await roll('project', { realm: 'signal', vocabulariesDir: 'vocabularies' });

const cat  = await loadCatalog('catalog/projects.yaml');
const proj = cat.resolve('realmwatch');
```

`seededIndex` returns a `Promise` so the same code runs in Node (`node:crypto`) and the browser (`crypto.subtle`) without branching on environment. Requires Node ≥20. Depends on `js-yaml`.

## Quick start

```bash
# Roll names
lexicon roll agent                           # → "gallant_curie"
lexicon roll project --realm fantasy         # → "Lunar-Pulsar"
lexicon roll project --realm signal --n=5    # → five candidates to browse-and-pick
lexicon roll branch --prefix feat            # → "feat/lunar-pulsar-forge"
lexicon roll entity                          # → "Brimwarden Echo"

# Catalog queries
lexicon resolve dreamspace                   # → follows prior_names → current_name
lexicon list --realm signal
lexicon list --kind realm-tool --status active

# Record a new project / rename an existing one
lexicon claim watch.realm.watch \
  --renames realmwatch \
  --reason "moved into realm.watch family"

# Validate before committing
lexicon validate                             # checks catalog ↔ vocabularies ↔ recipes

# Discover surfaces
lexicon recipes                              # which recipes exist, what options they need
lexicon vocabularies                         # word-count per group per file
```

## Renaming a project — the 10-step runbook

`lexicon rename` codifies what JP used to do by hand every time a project moved, missed a step, then ran into stale references for weeks. The runbook is the same shape as the rename procedure, just enforced.

```bash
# Print the plan first, decide what to skip
lexicon rename realmwatch watch.realm.watch --plan

# Execute with per-step confirmation; --yes auto-confirms
lexicon rename realmwatch watch.realm.watch --execute
lexicon rename realmwatch watch.realm.watch --execute --yes --skip 6
```

Steps:

1. **Local directory rename** — `mv ~/Projects/<old> ~/Projects/<new>` *(auto)*
2. **Transitional symlink** — `ln -sf ~/Projects/<new> ~/Projects/<old>` so stale references keep working *(auto)*
3. **Package metadata sweep** — `package.json`, `pyproject.toml`, `go.mod`, `version.json` *(manual reminder)*
4. **CLAUDE.md sweeps** — `~/.claude/CLAUDE.md` project table + project-local CLAUDE.md *(manual reminder)*
5. **GitHub** — `gh repo rename <new-name>` *(auto)*
6. **DNS / Caddy** — listed but **not automated** (too risky to touch the homelab firewall layer without supervision) *(manual reminder)*
7. **Outline wiki** — page path *(manual reminder)*
8. **Claude Code session storage** — `mv ~/.claude/projects/-home-jp-Projects-<old> ~/.claude/projects/-home-jp-Projects-<new>` *(auto)*
9. **`lexicon claim`** — append the rename to `catalog/projects.yaml` *(auto)*
10. **Manual-verify checklist** — cron, systemd, browser bookmarks, shell aliases *(manual reminder)*

Tests run against an in-memory filesystem; `--projects-dir` / `--sessions-dir` flags let you exercise the runbook against a tmpdir without touching `~/Projects`.

## Bootstrapping the catalog

`lexicon catalog import` walks a directory (default `~/Projects`), reads each subproject's `README.md`/`package.json`/`pyproject.toml`/`go.mod` and `git remote`, and emits a draft YAML for hand review:

```bash
lexicon catalog import                       # → stdout, draft for review
lexicon catalog import --dry-run             # → list what would be emitted
lexicon catalog import --out my-draft.yaml   # → file (refuses to overwrite catalog/projects.yaml)
```

Realm is left as `?` for human review — auto-guessing the realm defeats the purpose of having a roller. Status defaults to `local-only` for projects with no git remote.

## Projecting the catalog into a Claude Code skill

`lexicon catalog render` turns `projects.yaml` into LLM-readable artifacts so the catalog can live as a *lazy-loaded* skill instead of bloating eager `~/.claude/CLAUDE.md` context. Two formats:

```bash
lexicon catalog render --format=skill        # → SKILL.md (frontmatter + body, grouped by status)
lexicon catalog render --format=md-table     # → GitHub-flavored markdown table

# Wire the skill into Claude Code:
mkdir -p ~/.claude/skills/project-catalog
lexicon catalog render --catalog catalog/projects.yaml --format=skill \
  > ~/.claude/skills/project-catalog/SKILL.md
```

The skill auto-loads when Claude needs to identify a project, and stays out of the prompt otherwise. `projects.yaml` remains the source of truth — re-render whenever it changes. See `docs/superpowers/specs/2026-05-07-catalog-render-skill.md` for the design rationale.

## Catalogs

Lexicon's data lives in three checked-in YAML files:

| Catalog | What's in it |
|---|---|
| `catalog/projects.yaml` | The system of record for every project in `~/Projects/` — id, current name, kind, realm, repo, domain, prior names, status. |
| `catalog/agents.yaml` | Durable named agents that get spawned across sessions (audit, fix, scout, retro, etc.). |
| `catalog/voices.yaml` | Voice roster — Andrew, Ava, Brian, Emma, Davis. Mirrors the table in `~/.claude/CLAUDE.md`; lexicon is the canonical home. |

`docs/catalogs.md` explains how the three relate. The catalog is just a YAML file — there is no database, no service, no migration framework. Hand-editing is fully supported; `lexicon claim` and `lexicon rename` produce the same output a careful hand-edit would.

## Vocabularies and recipes

Vocabularies are themed word lists, one file per category:

```
vocabularies/
├── realms.yaml         # fantasy, tarot, oracle, void, forge, signal, stellar
├── adjectives.yaml     # one group per realm + an `any` group
├── nouns.yaml          # same shape as adjectives
├── scientists.yaml     # for agent rolls (Docker-corpus + extensions)
├── creatures.yaml      # for entity rolls (sphinx, basilisk, kelpie, …)
└── recipes.yaml        # declarative recipe definitions
```

Recipes are **data, not code.** Adding a new naming surface means adding a recipe entry — no language change required:

```yaml
# vocabularies/recipes.yaml
recipes:
  project:
    pattern: "{adjective:cap}-{noun:cap}"
    sources:
      adjective: { from: adjectives, group: fantasy }
      noun:      { from: nouns,      group: fantasy }
    required_options: [realm]
  agent:
    pattern: "{adjective:lower}_{scientist:lower}"
    sources:
      adjective: { from: adjectives, group: any }
      scientist: { from: scientists, group: any }
```

When a recipe declares `group: fantasy` and the caller passes `--realm signal`, the realm overrides the group lookup. This is how a single `project` recipe handles all seven realms.

## Sigil cutover

In v1.0 lexicon takes ownership of the canonical word lists. `realm-sigil/words/*.json` becomes a derived artifact:

```bash
./sync-vocabularies.sh                       # write sigil's words/ from lexicon's vocabularies/
./sync-vocabularies.sh --check               # CI drift detector — exits non-zero on diff
./sync-vocabularies.sh --dry-run             # print what would be written
```

The cutover plan lives in `docs/superpowers/specs/2026-05-07-sigil-cutover-design.md`. After lexicon v1.0 ships, sigil consumes vocabularies via this script and publishes a point release; sigil's own `sync-words.sh` retires.

## Hooks

Sample Claude Code hooks live in `hooks/`:

- `hooks/catalog-status.py` — SessionStart banner: `lexicon — N projects in M realms; K prior names recorded`.
- `hooks/lexicon-validate-precommit.sh` — git pre-commit hook that aborts on `lexicon validate` failure.

`hooks/README.md` has install instructions; `.claude/settings.json` shows example wiring. These are samples, not auto-installed.

## Versioning

This project uses [realm-sigil](https://github.com/jphein/realm-sigil) for unified versioning across the realm. Each release bumps `version.json` and tags `vX.Y.Z`. See `CHANGELOG.md` for what shipped in each release.

## Layout

```
lexicon.realm.watch/
├── README.md / CLAUDE.md / LICENSE / version.json
├── catalog/
│   ├── projects.yaml          # canonical project registry
│   ├── agents.yaml            # named agent roster
│   └── voices.yaml            # voice roster
├── vocabularies/
│   ├── realms.yaml
│   ├── adjectives.yaml
│   ├── nouns.yaml
│   ├── scientists.yaml
│   ├── creatures.yaml
│   └── recipes.yaml
├── go/                        # Go library + `lexicon` CLI binary
├── python/                    # Python parity library
├── js/                        # JS parity library (Node + browser)
├── static/                    # GitHub Pages landing page (live roller + catalog viewer)
├── hooks/                     # Claude Code hook samples
├── tests/fixtures/            # cross-language fixture data
├── sync-vocabularies.sh       # lexicon → realm-sigil/words/*.json
└── docs/superpowers/          # specs and plans
```

## Testing

```bash
# Go
cd go && go test ./...

# Python
cd python && python -m pytest

# JS
cd js && node --test test/
```

The cross-language parity contract is `tests/fixtures/seeded-recipes.json` — 33 cases covering all four recipes across all seven realms plus edge seeds (Unicode, empty, long). **Don't edit `expected_name` values without re-running the parity test in every language.** That file is the contract.

## Design

See `docs/superpowers/specs/2026-04-26-lexicon-design.md` for the full design and `docs/superpowers/plans/2026-04-26-lexicon-v0.1.md` for the v0.1 implementation plan. The sigil cutover plan lives in `docs/superpowers/specs/2026-05-07-sigil-cutover-design.md`.

## See also

- [clock.realm.watch](https://github.com/jphein/clock.realm.watch) — cache invalidation at the prompt seam
- [realm-sigil](https://github.com/jphein/realm-sigil) — deterministic build-time naming

## License

MIT — see `LICENSE`.
