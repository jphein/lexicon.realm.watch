# Catalog → Skill Projection — Design Spec

**Date:** 2026-05-07
**Author:** JP (handed off via Claude Code orchestrator)
**Status:** Implemented (lexicon v1.2.0, commit forthcoming)
**Implementer:** Lexicon agent (fresh-context Lexicon work)
**Cross-references:**
- `docs/superpowers/specs/2026-04-26-lexicon-design.md` (parent design)
- `~/.claude/projects/-home-jp/scratch/claude-code-research/SYNTHESIS.md` (motivation)
- `~/.claude/projects/-home-jp/scratch/claude-code-research/github-managers.md` (ecosystem context)

---

## Premise

A research sweep on 2026-05-07 (two converging sources: official Anthropic docs + community blogs/Reddit/HN/academic) produced three findings that bear directly on Lexicon's catalog:

1. **The 200-line CLAUDE.md context budget is real.** Academic work (Jaroslawicz et al., 2025) shows linear instruction-compliance decay as content length rises. HumanLayer recommends <60 lines; Anthropic's own guidance is <200 lines. JP's `~/CLAUDE.md` is currently **506 lines** post-catalog-sync. Roughly 80 of those are a 122-row project table that:
   - Eats ~5000 tokens of *eager-loaded* context every session
   - Is mostly unused — most sessions touch 1–3 projects, not 122
   - Degrades adherence to every other rule in the file by virtue of length

2. **Progressive disclosure is the dominant architecture.** Tiny root CLAUDE.md, with details offloaded to *lazy-loaded* skills. The catalog is a textbook fit for this pattern: useful when contextually relevant, dead weight otherwise.

3. **Anthropic ships zero lifecycle-automation tooling.** The community gap for "manage your CLAUDE.md / skills / hooks / settings" is real. Closest existing tools: `davila7/claude-code-templates` (27k ⭐), `affaan-m/everything-claude-code` (175k ⭐, includes AgentShield audit), `cclint` (carlrannaberg / felixgeelhaar variants). None of them do **CLAUDE.md → skill extraction**.

The decision: keep Lexicon's mission tight (naming + rename + structured registry), but add a small projection capability so `projects.yaml` can produce the LLM-facing catalog as a **skill** rather than as eager CLAUDE.md content. This:
- Recovers ~80 lines of CLAUDE.md budget
- Validates Lexicon's "data with multiple projections" pattern with a real consumer
- Stays inside Lexicon's existing scope: `projects.yaml` is the structured source, the skill is one of its outputs (similar to how realm-sigil renders Go/Python/JS bindings from one vocabulary file)

---

## Goals

1. **Add `lexicon catalog render` subcommand** that produces an LLM-readable artifact from `catalog/projects.yaml`.
2. **Support at least two output formats:**
   - `--format=skill` — emits a complete `SKILL.md` (frontmatter + body) suitable for dropping into `~/.claude/skills/project-catalog/SKILL.md`.
   - `--format=md-table` — emits the GitHub-flavored markdown table form (the format CLAUDE.md uses today). Useful for review and for one-off injection.
3. **Tests** — unit-test the formatter against fixture catalogs; integration-test against the real `catalog/projects.yaml`.
4. **Help text** — wire the subcommand into `printCatalogHelp` and update top-level `printHelp` if needed.
5. **No I/O surprises** — output to stdout by default; optionally accept `--out PATH` (consistent with `cmd_catalog_import.go`).
6. **Document the rendered skill's purpose** in the body so a future reader (Claude or human) understands why the file exists and where the truth lives.

## Non-goals

- **Auto-installing the skill into `~/.claude/skills/`.** That's an environment-specific deployment step; let JP run a shell pipe (`lexicon catalog render --format=skill > ~/.claude/skills/project-catalog/SKILL.md`).
- **Watching `projects.yaml` for changes and regenerating.** Out of scope for v1; could be a hook later.
- **Sub-component cataloging.** The 33 realmwatch plugin rows and the os.realm.watch sub-component rows currently in `~/CLAUDE.md` should *not* be added to `projects.yaml`. Each project should describe its own internals via its own CLAUDE.md or skill. Lexicon's `projects.yaml` covers top-level projects only. (See "Sub-components" below for the migration plan.)
- **Generating CLAUDE.md edits in-place.** No `--edit-claude-md` flag. Keep the tool side-effect-free; let humans wire output → file.
- **A `realm-tool` taxonomy expansion.** Don't add new `kind` or `realm` values to support the migration; reuse existing ones, or set `realm: ?` and let the operator clean up.

---

## Output specification

### `--format=skill`

Emit a single Markdown file with YAML frontmatter, suitable for `~/.claude/skills/project-catalog/SKILL.md`. Anthropic's skill convention (verified against `~/.claude/skills/*.md` and bundled plugin skills):

```markdown
---
name: project-catalog
description: Use to look up JP's projects under ~/Projects/. Auto-loads when identifying what a project does, where it lives, what tech it uses, or which project handles a given concern.
---

# Project Catalog

Source of truth: `~/Projects/lexicon.realm.watch/catalog/projects.yaml`.
This file is generated by `lexicon catalog render --format=skill` — do not hand-edit; edit the YAML and regenerate.

## How to read

Each project below lists: directory path, kind/realm/status badge, one-line description, and notes when present. Use the path to `cd`; use the description to confirm relevance; consult the project's own `CLAUDE.md` for sub-component / plugin / internal detail.

## Projects (active)

### lexicon (`lexicon.realm.watch`)
- **Path:** `~/Projects/lexicon.realm.watch/`
- **Kind:** realm-tool · **Realm:** oracle · **Status:** active
- Names and the changing of names — the runtime naming workhorse.
- Repo: <https://github.com/jphein/lexicon.realm.watch>

### realm-sigil
- **Path:** `~/Projects/realm-sigil/`
- **Kind:** library · **Realm:** oracle · **Status:** active
- Deterministic magical version name generation across Go/Python/JS.
- Notes: Will become a downstream consumer of lexicon's vocabularies (v1.0 cutover).
- Repo: <https://github.com/jphein/realm-sigil>

[... one entry per project, grouped by status ...]

## Projects (local-only)

[... ...]

## Projects (archived)

[... ...]
```

**Rules:**
- Group projects by `status` (active first, then `local-only`, then `archived`, then any others).
- Within a group, sort by `current_name` ascending (stable across runs).
- Use the `id` for the heading anchor and `current_name` for the parenthesized display when they differ; just the id when they match.
- Skip empty/optional fields cleanly (don't emit `Repo: ~`).
- The `description` field becomes the third bullet line (omit the bullet if description is empty).
- The `notes` field becomes a bullet line prefixed with "Notes:" when present.

**Description**: write the frontmatter `description` as a *use-when* statement, since that's what triggers Claude's skill auto-discovery. The text above is the recommended phrasing.

**Path inference**: derive each project's path from its `current_name`:
- Default: `~/Projects/<current_name>/`
- If `current_name` ends in a domain suffix (e.g., `clock.realm.watch`), use the full string as the directory name (`~/Projects/clock.realm.watch/`).

### `--format=md-table`

Emit a GitHub-flavored markdown table identical in shape to the table currently in `~/CLAUDE.md`:

```markdown
| Project | What | Key Tech | Notes |
|---------|------|----------|-------|
| **lexicon** | Names and the changing of names — the runtime naming workhorse. | Go | [GitHub](https://github.com/jphein/lexicon.realm.watch). Realm: oracle. |
| **realm-sigil** | Deterministic magical version name generation across Go/Python/JS. | Go, Python, JS | [GitHub](https://github.com/jphein/realm-sigil). Realm: oracle. |
[...]
```

**Rules:**
- One row per project, sorted by `current_name`.
- "Key Tech" column: not currently in `projects.yaml` schema. For v1, leave it empty (`—`) or skip the column entirely; consider adding a `tech: [...]` schema field in a follow-up.
- "Notes" column: combine `repo` (as `[GitHub](url)` link if present), `realm`, and `notes` into a single cell, semicolon-separated.

This format is mainly for human review (paste into a doc, diff against current CLAUDE.md). It is *not* intended to replace the skill as the LLM-facing artifact.

---

## Implementation plan

### File structure

```
go/cmd/lexicon/
├── cmd_catalog_render.go     ← NEW
├── cmd_catalog_render_test.go ← NEW
├── cmd_import.go              ← edit cmdCatalog dispatcher + printCatalogHelp
└── (rest unchanged)
```

### Wiring

In `cmd_import.go`'s `cmdCatalog` switch (currently only `"import"`), add:

```go
case "render":
    return cmdCatalogRender(rest, stdout, stderr)
```

In `printCatalogHelp`, append:

```
  render   render the catalog as a skill or markdown table
```

### `cmdCatalogRender` shape

```go
func cmdCatalogRender(args []string, stdout, stderr io.Writer) int {
    fs := flag.NewFlagSet("catalog render", flag.ContinueOnError)
    fs.SetOutput(stderr)
    catalog := fs.String("catalog", "", "path to catalog/projects.yaml")
    format  := fs.String("format", "skill", "output format: skill | md-table")
    out     := fs.String("out", "", "output path (default stdout)")
    if err := fs.Parse(args); err != nil { return 2 }

    cat, err := lexicon.LoadCatalog(resolveCatalogPath(*catalog))
    if err != nil { fmt.Fprintf(stderr, "load catalog: %v\n", err); return 1 }

    var rendered []byte
    switch *format {
    case "skill":
        rendered = renderSkill(cat)
    case "md-table":
        rendered = renderMDTable(cat)
    default:
        fmt.Fprintf(stderr, "unknown --format %q (valid: skill, md-table)\n", *format); return 2
    }

    if *out == "" { stdout.Write(rendered); return 0 }
    if err := os.WriteFile(*out, rendered, 0o644); err != nil {
        fmt.Fprintf(stderr, "write %s: %v\n", *out, err); return 1
    }
    return 0
}
```

The `renderSkill` and `renderMDTable` functions take `*lexicon.Catalog` and return `[]byte`. Pure formatting — no I/O. That keeps tests cheap.

### Tests

Create `cmd_catalog_render_test.go` with at least:

1. **Fixture-driven skill output** — build a small `Catalog` in memory (3-5 projects across statuses), call `renderSkill`, assert the output contains expected headings, status groups, and ordering. Use `golden file` comparison via `testdata/render-skill.golden.md` for stability.
2. **Fixture-driven md-table output** — same shape, golden file `testdata/render-mdtable.golden.md`.
3. **Empty catalog** — render produces a non-empty document with frontmatter and a "no projects" body line.
4. **Unknown --format** — exit code 2, stderr message includes valid formats.
5. **Status grouping** — projects sort within their group by current_name; groups order: active, local-only, archived, then others alphabetical.

Run: `cd go && go test ./cmd/lexicon -run TestCatalogRender`.

### Smoke test

After build:
```bash
cd /home/jp/Projects/lexicon.realm.watch/go
go run ./cmd/lexicon --catalog ../catalog/projects.yaml catalog render --format=skill | head -60
go run ./cmd/lexicon --catalog ../catalog/projects.yaml catalog render --format=md-table
```

Note the `--catalog` flag positioning — it's a top-level flag in this codebase per `cmd_list.go`, so it needs to come before `catalog render`. If that's clunky, a follow-up could let `catalog` subcommands accept their own `--catalog` flag. Match whatever the existing import command's convention is.

---

## Migration plan (post-implementation)

Once the render command lands and tests green, JP (or a follow-up agent) will:

1. **Populate `projects.yaml`.** Today it has 7 entries; the active comprehensive surface is ~89 top-level projects. Two paths:
   - **Cheap path:** run `lexicon catalog import --from ~/Projects --out /tmp/draft.yaml --dry-run` to see what auto-discovery turns up; review and merge by hand into `catalog/projects.yaml`. The existing 7 curated entries take precedence over auto-inferred fields.
   - **Authoring path:** hand-author each entry from the current `~/CLAUDE.md` table content. Slower but yields cleaner descriptions.

2. **Generate the skill:**
   ```bash
   mkdir -p ~/.claude/skills/project-catalog
   cd ~/Projects/lexicon.realm.watch/go && go run ./cmd/lexicon \
     --catalog ../catalog/projects.yaml catalog render --format=skill \
     > ~/.claude/skills/project-catalog/SKILL.md
   ```

3. **Slim `~/CLAUDE.md`.** Replace the 122-row table (currently lines ~46–168) with a single paragraph:

   > **Projects** under `~/Projects/` are catalogued in the `project-catalog` skill, which auto-loads when relevant. Source of truth: `~/Projects/lexicon.realm.watch/catalog/projects.yaml`. Each project also has its own `CLAUDE.md` — read it before working in that project.

4. **Verify:** restart a fresh Claude Code session in `~/Projects/`, ask "what's in `forageforall`?", confirm the skill loads and answers correctly. Check `~/CLAUDE.md` line count is well under 200.

### Sub-components — separate concern

`~/CLAUDE.md` currently has 33 `realmwatch/<plugin>` rows and 2 sub-component rows for `gnome-shell-monitor` / `realm-optimizer` (both inside `os.realm.watch`). These should *not* go into `projects.yaml`. Two options for handling them:

- **Option A (preferred):** each project documents its own internals. `realmwatch/CLAUDE.md` already exists; it could grow a "Plugins" section or ship its own skill at `realmwatch/.claude/skills/plugins/SKILL.md`. The slimmed `~/CLAUDE.md` doesn't need to know about plugins at all.
- **Option B:** keep the 33 plugin rows in `~/CLAUDE.md` even after migrating top-level projects out. Less ideal — the rows still bloat eager context — but easy.

Decide as part of the migration step, not as part of this implementation.

---

## Schema follow-ups (out of scope, noted for future)

The current `Project` struct doesn't have:
- `tech` (Key Tech column equivalent — string or []string)
- `path` (currently inferred; could be explicit for non-default paths)
- `sensitive` (flag for personal/secrets-bearing dirs)
- `auto_archive_candidate` (drift between catalog and on-disk state)

If `--format=md-table`'s "Key Tech" column matters, schema needs a `tech` field. Defer to a separate spec.

---

## Open questions for the implementer

1. **Where should the `--catalog` flag live for `catalog render`?** Top-level (matches `cmd_list.go`) or per-subcommand (matches some other CLIs)? Pick the one that's already conventional in this codebase; the import command is the precedent.

2. **`description` field length cap?** Some current `projects.yaml` descriptions run long. Should `renderSkill` truncate to a sentence at first period, or emit verbatim? *Recommendation:* emit verbatim — sentences are already short, and Claude reads markdown fine. The author has already curated.

3. **Should the skill emit `realm:` for projects whose realm is `~` (none) or `?` (unknown)?** Skip those rows? Render with `Realm: —`? *Recommendation:* render `Realm: —` so the operator sees gaps and fixes the YAML.

4. **What about projects in `prior_names`?** The skill is for *current* state. Don't surface prior names in the body; the rename runbook handles that elsewhere.

5. **`status` values not currently in the schema** — the existing 7 entries use `active` and `local-only`. Migration may surface `archived`, `forked`, `external`. Should `renderSkill` enforce a known set, or pass through? *Recommendation:* pass through (group by whatever is there); validate uniqueness only in `lexicon validate`.

---

## Done criteria

- `lexicon catalog render --format=skill` works against `catalog/projects.yaml` and produces valid SKILL.md output.
- `lexicon catalog render --format=md-table` works and produces the table form.
- Tests pass (`cd go && go test ./...`).
- `printCatalogHelp` lists `render`.
- Top-level `printHelp` mentions catalog has multiple subcommands (already does).
- A README example or CHANGELOG entry mentions the new command.
- Spec marked Status: Implemented (this file).

Migration to `~/.claude/skills/` and slimming `~/CLAUDE.md` is **out of scope for this spec** — those are operational steps once the tool exists.

---

## Why this spec exists

JP asked for "all the implementation" of the SKILL.md / Lexicon-catalog plan. The orchestrator (Claude Code main thread) judged that fresh-context Lexicon work is better done by an agent already grounded in this repo's conventions, rather than from outside. This spec captures the design decisions (research-driven), the format (concrete enough to implement without further design), and the open questions (small, judgment calls), so an implementing agent can land the work in one focused session.
