# lexicon.realm.watch — Design Spec

**Date:** 2026-04-26
**Author:** JP (with Claude Code, brainstormed)
**Status:** Approved for implementation planning

## Premise

There are two hard things in computer science: cache invalidation and naming things. The realm.watch family already has tools for both:

- `clock.realm.watch` — fixes the AI cache-invalidation problem at the prompt seam (current time injection via Claude Code hooks).
- `realm-sigil` — names things at build time, deterministically, from a git hash.

`lexicon.realm.watch` is the third move: the **runtime naming workhorse** that handles the case where naming and cache invalidation collide — renaming an existing project and propagating the new name across every place the old name was cached. It is also the daily roller for ad-hoc names (agents, branches, entities) and the canonical word library that other realm.watch tools (starting with sigil) consume.

## Goals

1. **Roll names on demand** for the recurring naming surfaces in JP's workflow: agents, branches, projects, entities, voices.
2. **Be the system of record** for every project in `~/Projects/` — current name, prior names, realm, kind, repo, domain.
3. **Drive renames** of existing projects through a guided runbook that updates local directories, package metadata, GitHub repo names, CLAUDE.md references, Outline wiki, and Claude Code session storage paths.
4. **Own the canonical vocabularies** (realms, adjectives, nouns, scientists, creatures) — replacing `realm-sigil/sync-words.sh` and serving as the single source of truth for any future realm.watch tool that needs themed words.
5. **Stay in the realm.watch family aesthetic** — small focused product, fantasy metaphor, GitHub Pages landing page, MIT license.

## Non-goals (v1)

- Full rename automation (DNS, Caddy reload, cron job rewrites, bookmark sweeps). v1 is a *guided runbook* — automation lands incrementally.
- A running HTTP service. v1's catalog is a checked-in YAML file; if cross-machine queries become a real need later, a service layer can wrap the same file.
- Cross-conversation memory persistence. The catalog itself is the memory.
- Named-entity uniqueness across the realm. Rolls can collide; the catalog only enforces uniqueness at *claim* time.

## Section 1 — Repo Layout & Catalog Schema

### Repo layout

```
lexicon.realm.watch/
├── README.md / CLAUDE.md / LICENSE / version.json   # realm.watch family standard
├── catalog/
│   ├── projects.yaml              # canonical project registry (centerpiece)
│   ├── agents.yaml                # v1.x: named agent roster
│   └── voices.yaml                # v1.x: voice roster (mirrors CLAUDE.md voice table)
├── vocabularies/
│   ├── realms.yaml                # fantasy/tarot/oracle/void/forge/signal/stellar
│   ├── adjectives.yaml            # gallant/furious/mystic/lunar/...
│   ├── nouns.yaml                 # sigil/pulsar/oracle/forge/echo/...
│   ├── scientists.yaml            # curie/einstein/... (Docker-corpus, for agent names)
│   ├── creatures.yaml             # v1.x for realmwatch RPG entities
│   └── recipes.yaml               # declarative recipe definitions (see Section 2)
├── go/                            # Go library + `lexicon` CLI binary
│   ├── go.mod
│   ├── lexicon.go
│   ├── catalog.go
│   ├── cmd/lexicon/main.go
│   └── *_test.go
├── python/                        # Python library (consumed by realmwatch + others)
│   ├── pyproject.toml
│   ├── lexicon/__init__.py
│   ├── lexicon/roller.py
│   ├── lexicon/catalog.py
│   └── tests/
├── js/                            # JS library (Node + browser, drives the landing page)
│   ├── package.json
│   ├── src/index.js
│   ├── src/roller.js
│   ├── src/catalog.js
│   └── test/
├── static/                        # GitHub Pages landing page
│   ├── index.html
│   ├── favicon.svg
│   ├── style.css
│   ├── app.js
│   └── build.sh                   # renders catalog → static HTML at deploy time
├── hooks/                         # Claude Code hooks (e.g., catalog-status banner)
├── tests/fixtures/                # cross-language fixture data (see Section 4)
├── sync-vocabularies.sh           # embed vocabularies into Vercel/JS-only deploys
└── docs/superpowers/specs/        # this file lives here
```

The Go/Python/JS triplet, `vocabularies/`, `static/`, and `sync-vocabularies.sh` are deliberate echoes of `realm-sigil`'s layout. Anyone who has navigated sigil's repo can navigate this one.

### `catalog/projects.yaml` schema

```yaml
projects:
  - id: clock                                    # permanent slug — never changes
    current_name: clock.realm.watch              # live name
    kind: realm-tool                             # realm-tool | service | library | site | infra | tool
    realm: signal                                # must exist in vocabularies/realms.yaml
    domain: clock.realm.watch                    # null if local-only
    repo: https://github.com/jphein/clock.realm.watch
    description: A clock for an AI that doesn't have one.
    created: 2026-04-23
    prior_names: []                              # append-only history
    status: active                               # active | deprecated | retired | local-only
    notes: ~

  # After a rename:
  - id: dreamspace
    current_name: dreamscape.realm.watch
    realm: oracle
    prior_names:
      - { name: dreamspace,  retired: 2026-05-01, reason: "moved into realm.watch family" }
      - { name: dreamscape,  retired: 2026-05-01, reason: "intermediate symlink alias" }
    # ...remaining fields
```

**Schema rules:**

- `id` is the *earliest known name*, lowercased and slugified. **Permanent.** It is the dictionary key. After a rename, `id` keeps recording the project's origin; `current_name` shows where it lives now.
- `current_name` is the live name today. `lexicon resolve <any-name-this-project-ever-had>` returns `current_name`.
- `prior_names[]` is append-only. Each entry: `{ name, retired (ISO date), reason }`. Earliest first.
- `realm` validates against `vocabularies/realms.yaml`. Catalog load fails fast if invalid.
- `status: local-only` flags projects without a public face — included in the catalog so JP has *one* canonical roster of his realm, but excluded from the public landing page render.

**What the catalog gets you:**

- `lexicon resolve <name>` → current name + full rename trail. Works for the original name, current name, or any in-between.
- `lexicon list --realm fantasy` → every project in that realm.
- `lexicon claim <new-name> --renames=<old-id>` → appends to `prior_names`, updates `current_name`, writes YAML back deterministically.
- The catalog *replaces* the project table in `~/.claude/CLAUDE.md`, which goes stale immediately after any rename.

### Other catalogs (v1.x)

- `agents.yaml` — durable named agents (the recurring ones JP spawns by name like `fix-passwords`, `audit-realmwatch`). One-shot agents don't get entered.
- `voices.yaml` — mirrors the voice roster in `~/.claude/CLAUDE.md` (Andrew/Ava/Brian/Emma/Davis with role + color metadata). Lexicon becomes the canonical home; CLAUDE.md gets a one-liner pointer.

These ship empty in v1 and get populated as the surfaces stabilize.

## Section 2 — Roller (vocabularies, recipes, library API, CLI)

### Vocabulary file format (`vocabularies/realms.yaml` example)

```yaml
realms:
  fantasy:
    description: High-fantasy Tolkienesque vocabulary
    words: [primal, shadow, ember, runic, moonlit, ravenforge, brimwarden, ...]
  signal:
    description: Networking, electromagnetic, data-flow imagery
    words: [pulsar, beacon, qubit, relay, packet, lighthouse, ...]
  # ...one entry per realm
```

Same outer shape for `adjectives.yaml`, `nouns.yaml`, `scientists.yaml`, `creatures.yaml`. One file per word category. Each top-level group has `description` + `words[]`. Each language's library reads YAML via its standard library (Go `gopkg.in/yaml.v3`, Python `pyyaml`, JS `js-yaml`).

### Recipes (`vocabularies/recipes.yaml`)

Recipes are **data, not code.** Adding a new naming surface = adding a recipe entry, no language code change.

```yaml
recipes:
  project:
    description: Project / service name in the realm.watch family
    pattern: "{adjective:cap}-{noun:cap}"           # → "Lunar-Pulsar"
    sources:
      adjective: { from: adjectives, group: fantasy }
      noun:      { from: nouns,      group: fantasy }
    # produces "Lunar-Pulsar"; user appends ".realm.watch" if hosting

  agent:
    description: Docker-style snake_case agent name
    pattern: "{adjective:lower}_{scientist:lower}"   # → "gallant_curie"
    sources:
      adjective: { from: adjectives, group: any }
      scientist: { from: scientists, group: any }

  branch:
    description: Conventional-commit branch name
    pattern: "{prefix}/{adjective:lower}-{noun:lower}-{noun:lower}"
    sources:
      adjective: { from: adjectives, group: any }
      noun:      { from: nouns,      group: any }
    required_options: [prefix]

  entity:
    description: Realmwatch RPG creature/persona
    pattern: "{adjective:cap} {creature:cap}"        # → "Brimwarden Echo"
    sources:
      adjective: { from: adjectives, group: fantasy }
      creature:  { from: creatures,  group: any }

  voice:
    description: Pull a named voice from the roster (deterministic)
    source_catalog: voices                           # not a roll — a catalog lookup
```

Pattern tokens use `{name:transform}`. Transforms: `cap` (capitalize), `lower`, `upper`, raw. Multiple slots of the same source name are independent rolls.

### Library API (Go reference)

```go
import "github.com/jphein/lexicon.realm.watch/go"

// Roll candidates
name := lexicon.Roll("project", lexicon.WithRealm("fantasy"))
// → "Lunar-Pulsar"

agent := lexicon.Roll("agent")
// → "gallant_curie"

// Branch needs a prefix option
branch := lexicon.Roll("branch", lexicon.WithPrefix("feat"))
// → "feat/lunar-pulsar-forge"

// Roll N unique candidates at once
candidates := lexicon.RollN("project", 5, lexicon.WithRealm("fantasy"))
// → ["Lunar-Pulsar", "Ravenforge-Echo", ...]
// RollN guarantees uniqueness within the returned set. If the recipe's
// combinatorial space is smaller than N, returns the full space (≤ N entries).

// Catalog access
cat, err := lexicon.LoadCatalog("/path/to/catalog/projects.yaml")
proj, ok := cat.Resolve("realmwatch")              // looks in id, current_name, prior_names
list := cat.ByRealm("fantasy")
err = cat.Claim("oracle.realm.watch",
                lexicon.WithRenamesOf("realmwatch"),
                lexicon.WithReason("moved into realm.watch family"))
err = cat.Save()                                   // round-trips YAML deterministically
```

Identical surface in Python and JS — same recipe names, same option keys, same return shapes. Cross-language consistency enforced by fixture tests (Section 4).

### CLI surface

```
lexicon roll <recipe> [--realm X] [--prefix Y] [--n N]
lexicon resolve <name>                              # any name → current + history
lexicon list [--realm X] [--kind Y] [--status Z]
lexicon claim <new-name> --renames=<old-id> [--reason TEXT]
lexicon rename <old-id> <new-name> [--plan|--execute] [--skip STEP]
lexicon validate                                    # check catalog ↔ vocabularies consistency
lexicon recipes                                     # list available recipes + required options
lexicon vocabularies                                # list groups + word counts per file
```

**Defaults:** `lexicon roll project --n=5` rolls five candidates so JP can browse-and-pick. Single-roll mode (`--n=1`) for scripts.

The CLI is implemented in Go (single static binary, distributable). Python and JS ship libraries only — the Go binary is what `~/Projects/scripts/` and shell aliases call.

## Section 3 — Renaming (migration runbook + first targets + sigil retirement)

### `lexicon rename` runbook

`lexicon rename <old-id> <new-name> --plan` emits a numbered checklist of steps. `--execute` runs them with per-step confirm. Steps:

1. **Local directory rename** — `mv ~/Projects/<old> ~/Projects/<new>`
2. **Transitional symlink** — `ln -sf ~/Projects/<new> ~/Projects/<old>` (so stale references keep working during the transition; can be removed later via `lexicon retire-symlink <old>`).
3. **Package metadata sweep:**
   - `package.json` `name` field
   - `pyproject.toml` `[project] name`
   - `go.mod` `module` directive (and any internal imports)
   - `version.json` (per realm-sigil convention)
4. **CLAUDE.md sweeps:**
   - `~/.claude/CLAUDE.md` project table — replace `<old>` row with new entry
   - Project-local `CLAUDE.md` — references to project name in headings/links
5. **GitHub:** `gh repo rename <new-name>` if remote exists. Print the old-URL → new-URL redirect note (GitHub does the redirect automatically; runbook just documents it).
6. **DNS / Caddy:** print a reminder block listing the old domain, the new domain, and the file paths likely to need editing (`disks/Caddyfile`, OpenWrt unbound config, etc.). **Not automated** — too risky to touch the homelab firewall layer without supervision.
7. **Outline wiki:** print the path that needs updating (`outline.jphe.in/<collection>/<page>`).
8. **Claude Code session storage:** `mv ~/.claude/projects/-home-jp-Projects-<old> ~/.claude/projects/-home-jp-Projects-<new>` (the project's session history dir).
9. **`lexicon claim`** — append the rename to the catalog. This is the last automated step; everything before it is filesystem/git work that needs to land first.
10. **Manual-verify checklist** — print a final list of places JP should eyeball: cron jobs (`crontab -l`), systemd units (`systemctl list-units`), browser bookmarks, `~/.bashrc` aliases, etc.

Each step has `--dry-run` (default for `--plan` mode) and `--skip <step>` for non-applicable steps (e.g., a local-only project skips step 5).

### Rollout — first projects to rename

1. **Self-bootstrap** — populate `catalog/projects.yaml` with every existing project. No renames yet, just enumerate. This is the hardest step because the catalog starts empty; needs a one-time `lexicon catalog import` script that walks `~/Projects/`, reads each project's `package.json`/`pyproject.toml`/`go.mod`/`README.md` for description, and emits a draft YAML for JP to review/edit.
2. **realmwatch → ?.realm.watch** — the keystone rename. Pick a realm word that fits the role; lexicon's roller produces the candidates at the moment of decision (no pre-committed list — that would defeat the purpose of having a roller). This rename exercises every step in the runbook; if it works for realmwatch it works for everything.
3. **dreamspace + dreamscape consolidation** — clean up the symlink confusion, pick one canonical name, retire the other.
4. **Realm-curious projects** — `oracle`, `realm-portal`, `realmcoin`, `the-oracle`, `oracle-mcp`, `oracle-chat` — bring into `*.realm.watch` family where it makes sense. Each gets a roll → JP picks → `lexicon rename`.

### Sigil retirement of `sync-words.sh`

`realm-sigil/words/realms.json` becomes a derived artifact built from `lexicon/vocabularies/realms.yaml`:

- `lexicon/sync-vocabularies.sh` is the new authoritative sync. It converts `vocabularies/*.yaml` → `realm-sigil/words/*.json` in the format sigil expects.
- `realm-sigil/sync-words.sh` becomes a thin wrapper that just calls lexicon's sync, or is removed entirely.
- Sigil's README updates: "Vocabularies sourced from `lexicon.realm.watch`. To update word lists, edit lexicon and re-run sync."
- Sigil's CI gains a check: `words/*.json` must match what sync produces from the current lexicon checkout.

This cutover happens *after* lexicon v1 ships and the catalog includes sigil. Coordinated bump: lexicon publishes v1.0; sigil publishes a v1.x point release that swaps to lexicon-derived words.

## Section 4 — Landing page, error handling, testing

### Landing page (`static/`)

`https://jphein.github.io/lexicon.realm.watch/` — same hosting pattern as clock and sigil.

**Content:**

- Header: `lexicon.realm.watch — names and the changing of names`
- One-paragraph framing: "There are two hard things in computer science. Clock handles one, sigil handles the other, lexicon is what you reach for when they collide."
- **Live roller widget** — pick recipe, pick realm/options, hit "Roll" → see candidates. Powered by the JS library running in-browser. No backend.
- **Catalog viewer** — table of *public* projects, defined as `domain != null` (not just `status != local-only` — a project might be `active` but intentionally never published, e.g., the homelab-internal `realm-portal`). Columns: name, realm, kind, description, prior names. Filterable by realm. Built at deploy time by `static/build.sh` reading `catalog/projects.yaml`.
- **Joke footer** — short note linking to clock and sigil; one-line wink at Phil Karlton.

**Aesthetics:**

- Dark/light theme via `prefers-color-scheme` (per global CLAUDE.md).
- SVG favicon — a small open-book glyph or rune (matching the lexicon metaphor).
- Same restraint as clock's landing page — minimal, fantasy-tinged, not overwrought.

### Error handling

- **Vocabulary file missing** — library construction fails at startup, not at first roll. Clear error pointing at the missing path.
- **Recipe references unknown vocabulary group** — `lexicon validate` catches this; library load fails fast in dev. Production callers see a typed error with the offending recipe + group name.
- **`lexicon claim <name>` collides with existing entry** — error showing the conflicting `id` and what its `current_name` is. `--force` flag for true rename overwrites; default refuses.
- **Catalog references invalid realm** — `lexicon validate` flags it; `LoadCatalog` returns a typed error listing every invalid `realm` reference.
- **Rename runbook step fails mid-execution** — runbook is idempotent per step (re-running `mv` on an already-renamed dir is a no-op + warning, not a failure). State is the filesystem and the catalog; no separate state file to corrupt.

### Testing

- **Per-language unit tests** — each library tests against committed fixture vocabularies in `tests/fixtures/vocabularies-test.yaml` (small, stable corpus). Live `vocabularies/` is *not* used in tests (so vocabulary edits don't break tests).
- **Cross-language fixture parity test** — `tests/fixtures/seeded-recipes.json` lists `(seed, recipe, options) → expected_name` triplets. Each language's roller has a `RollSeeded(seed, recipe, options)` deterministic mode that must match the expected name byte-for-byte. Run in CI for all three languages. This catches drift between Go/Python/JS implementations the same way realm-sigil's hash-determinism tests do.
- **Catalog round-trip test** — load `catalog/projects.yaml`, mutate via `Claim()`, write back, diff source against output. Should preserve YAML key order, comments, indentation. Use `gopkg.in/yaml.v3` (Go), `ruamel.yaml` (Python), `js-yaml` with custom dumper config (JS) — all configured to round-trip cleanly.
- **`lexicon validate` runs in CI** — catalog ↔ vocabularies consistency check on every PR.
- **Runbook tests** — `lexicon rename --plan` is tested for output stability (golden-file snapshot of the printed checklist). `--execute` tested in a tmpdir scratch project, not against `~/Projects/`.

## Architecture summary

```
                          ┌────────────────────────┐
                          │   vocabularies/*.yaml  │
                          │  (the actual product)  │
                          └────────────┬───────────┘
                                       │ read by
                  ┌────────────────────┼────────────────────┐
                  ▼                    ▼                    ▼
            ┌─────────┐          ┌──────────┐         ┌─────────┐
            │  Go lib │          │  Py lib  │         │  JS lib │
            │  + CLI  │          │          │         │         │
            └────┬────┘          └─────┬────┘         └────┬────┘
                 │ read/write          │ read              │ read
                 ▼                     ▼                   ▼
          ┌────────────────────────────────────────────────────┐
          │           catalog/projects.yaml                    │
          │  (system of record — projects, names, history)     │
          └────────────────────────────────────────────────────┘
                                │
                                ▼
                  ┌─────────────────────────┐
                  │  realm-sigil consumes   │
                  │  vocabularies via       │
                  │  sync-vocabularies.sh   │
                  └─────────────────────────┘
```

The roller is stateless. The catalog is the only stateful piece, and it is just a YAML file in the repo — no database, no service, no migration framework.

## Build sequence

A rough ordering for `writing-plans` to expand on. Not the implementation plan itself.

1. Repo skeleton — directories, `version.json`, `LICENSE`, README stub, CLAUDE.md
2. Vocabularies — port `realm-sigil/words/realms.json` → `vocabularies/realms.yaml`; extract adjectives/nouns from a starter corpus; small `creatures.yaml` and `scientists.yaml` (Docker corpus)
3. Recipes file with the five v1 recipes (project, agent, branch, entity, voice)
4. Go library: vocabulary loader, recipe engine, catalog loader, `lexicon` CLI commands `roll`, `resolve`, `list`, `validate`, `recipes`, `vocabularies`
5. Python library — same surface
6. JS library — same surface, plus a browser bundle for the landing page
7. Cross-language fixture parity test
8. Static landing page + `static/build.sh`
9. Catalog bootstrap — `lexicon catalog import` script; populate `catalog/projects.yaml` from `~/Projects/`
10. `lexicon claim` (the catalog mutation)
11. `lexicon rename --plan` (runbook printer)
12. `lexicon rename --execute` (per-step runner)
13. First real rename: `realmwatch` → rolled name
14. Sigil cutover — `sync-vocabularies.sh` → sigil's `words/`
15. Subsequent project renames as JP triages them

### Decisions made (formerly open)

- **Cross-language seeded RNG** — SHA-256 of `(seed || slot_index)`, take first 8 bytes as big-endian uint64, mod `len(words)`. All three languages must implement this byte-for-byte identically. Verified by the `tests/fixtures/seeded-recipes.json` parity test. Mirrors how `realm-sigil` derives names from git hashes.
- **Realm assignment when caller doesn't specify** — refuse. `lexicon roll project` with no `--realm` exits non-zero and prints the list of available realms with one-line descriptions. No silent default; the realm is a meaningful choice and the tool should surface it.
- **Catalog hand-editing** — fully supported. The YAML file is the source of truth; `lexicon claim` and `lexicon rename` are conveniences that produce the same output a careful hand-edit would. Self-test: after any catalog mutation, re-running `lexicon validate` must pass.

## Open questions for `writing-plans` to resolve

- **Module / package names** — `github.com/jphein/lexicon.realm.watch/go` vs `github.com/jphein/lexicon` vs other; Python distribution name; JS npm name. Prefer parity with how sigil handles it.
- **Catalog YAML formatter** — which library in each language preserves comments + key order on round-trip. Go's `yaml.v3` does; Python `ruamel.yaml` does; JS less standard. Decide before implementing `Claim`.
- **Decomposition into release milestones** — the v1 surface is large (Go + Python + JS + CLI + landing page + catalog bootstrap + rename runbook + sigil cutover). The plan should break this into shippable stages — likely Go-first (lib + CLI + roller + catalog + claim) as v0.1, rename runbook as v0.2, Python/JS parity as v0.3, sigil cutover as v1.0.
