# Catalogs

`lexicon.realm.watch` is the system of record for three rosters that live in
`catalog/`. They are plain YAML files — hand-editable, version-controlled,
and consumed by the Go/Python/JS libraries.

## The three catalogs

### `catalog/projects.yaml`

The centerpiece. Every project under `~/Projects/` (and every public
`*.realm.watch` site) gets one entry: permanent `id`, `current_name`, realm,
kind, repo, domain, status, and an append-only `prior_names` history.

The library API around this file (`Resolve`, `ByRealm`, `Claim`) is the
day-to-day surface. `lexicon claim` and `lexicon rename` are conveniences
that produce the same output a careful hand-edit would. Schema details live
in `docs/superpowers/specs/2026-04-26-lexicon-design.md` (Section 1).

### `catalog/agents.yaml`

The roster of **durable named agents** — the recurring ones JP spawns by
name like `audit-realmwatch`, `fix-passwords`, `scout`. One-shot dreamer
agents (e.g., a swarm member spawned for a single PR) do **not** belong
here.

Each entry: `id`, `current_name`, `role`, `voice` (referencing
`voices.yaml`), `description`, optional `notes`, and `status`. An agent's
`role` should match one of the voice roles in `voices.yaml` so the right
voice is picked automatically when the agent is spawned.

### `catalog/voices.yaml`

The voice roster — Andrew, Ava, Brian, Emma, Davis. Each entry binds a
`role` to an Azure Dragon HD voice name and a subtitle color. This file
mirrors the voice table in `~/.claude/CLAUDE.md`; lexicon is the canonical
home and CLAUDE.md gets a one-liner pointer.

`davis` carries an explicit `notes: Orchestrator only` — subagents must not
be assigned that voice.

## How they relate

```
projects.yaml
  ├── id ──────────── permanent slug, never changes
  ├── current_name ── live name today (rolled or chosen)
  └── realm ───────── must exist in vocabularies/realms.yaml

agents.yaml
  ├── id ──────────── permanent slug, never changes
  ├── current_name ── usually equals id; renames recorded in prior_names
  ├── role ────────── architect | code-reviewer | debugger | researcher | narrator
  └── voice ───────── must exist as an id in voices.yaml

voices.yaml
  ├── id ──────────── permanent slug (andrew, ava, brian, emma, davis)
  ├── name ────────── Azure voice name (en-US-X:DragonHDLatestNeural)
  ├── role ────────── matches role in agents.yaml
  └── color ───────── subtitle_color used when this voice speaks
```

`projects.yaml` and `agents.yaml` are independent rosters — projects don't
reference agents or vice versa. Both reference `vocabularies/realms.yaml`
or `voices.yaml` respectively for validation.

## Validation

`lexicon validate` (when implemented for the auxiliary catalogs) checks:

- Every `agents.yaml` entry's `voice` refers to a valid `voices.yaml` id.
- Every `projects.yaml` entry's `realm` refers to a valid
  `vocabularies/realms.yaml` group.
- No duplicate `id`s within a catalog file.
- `prior_names` is append-only and well-formed (each entry has `name`,
  `retired`, `reason`).

For now (v0.x) the auxiliary catalogs ship as data files; wiring them into
the Go library is a follow-up.
