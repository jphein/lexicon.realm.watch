# Catalog Audience Classification — Design Spec

**Date:** 2026-05-07 → 2026-05-08
**Author:** JP (with orchestrator brainstorm)
**Status:** Implemented
**Cross-references:**
- `docs/superpowers/specs/2026-04-26-lexicon-design.md` (parent design)
- `docs/superpowers/specs/2026-05-07-catalog-render-skill.md` (render projection — will need updating)

---

## Premise

JP runs three project domains, not one:

1. **`*.realm.watch`** — themed public tools (clock, dark, lexicon, sigil, status, mirror, os, plus today's renames: portal, coin, oracle, sigil, bestiary, chat). The realm.watch family is curated, fantasy-aesthetic, intended for sharing or contribution. Lexicon's rename runbook was built around this convention.

2. **`*.jphe.in`** — personal services + JP's name brand. Vault, Authelia, Home Assistant, Immich, Outline, Jellyfin, Navidrome, Syncthing, Disks, the personal `jphein.net`/`jp.jphein.github.io` retrospective. These are *internal* or *identity-bound* — they signal "this is JP's", not "this is a tool you can use."

3. **Standalone brands** — `donkeyco`, `imaginalvision.com`, `jpheinnet`, `poppasblog`, `jphein-wordpress-*`, `forageforall`, `jewelrycycle`, `shawnahein.com`, `solar`, `sdp`, `sdp+disability-appeal`. These have their own identities (client work, family/legacy projects, sensitive personal records) and don't belong on either of the other two TLDs.

Today the catalog flattens all three into one list. Lexicon's renames default to `jphein/<new-name>` GitHub repos and the rename runbook implicitly assumes realm.watch convention. The catalog skill renders by `status:` (active / local-only / archived) which doesn't match how JP thinks about projects.

A small schema addition makes the existing distinctions explicit and unlocks several quality-of-life wins.

## Goals

1. **Add an `audience:` field** to `Project` recording which bucket the project belongs to.
2. **Classify all 88 entries** in `catalog/projects.yaml` (manual, one-time pass — same shape as Lexicon agent's realm classification in v1.4.0).
3. **Surface `audience` in `lexicon list`** — filter via `--audience=<value>`.
4. **Group the rendered skill by audience first**, then by status — Claude (and JP) sees a navigable catalog matching the actual mental model.
5. **No breaking schema changes** — `audience` is optional; absent defaults to a sensible fallback (likely `realm` for back-compat with v1.4 catalog state, or empty meaning "unclassified").

## Non-goals

- **Renaming convention enforcement.** `lexicon rename` doesn't *force* realm-audience projects to end in `.realm.watch` or personal-audience projects to end in `.jphe.in`. The schema is descriptive (records what is), not prescriptive (forcing what could be).
- **Multi-domain support inside the rename runbook.** Renames work the same regardless of audience — `lexicon rename vault-gate gate.jphe.in --audience=personal` is a one-shot rename to a `*.jphe.in` name; nothing about the runbook needs to know `jphe.in` specifically.
- **Cross-audience moves as first-class operations.** If JP ever wants to move something between audiences (e.g., promote a personal tool into the realm.watch family), that's two ops: a regular rename + an `--audience=` update. Don't add a separate verb.
- **Deprecation tracking.** `archived` already lives in `status:`; don't duplicate.

## Schema

```yaml
- id: lexicon
  current_name: lexicon.realm.watch
  kind: realm-tool
  realm: oracle
  audience: realm     # <-- new field
  domain: lexicon.realm.watch
  repo: https://github.com/jphein/lexicon.realm.watch
  description: Names and the changing of names — the runtime naming workhorse.
  ...
```

### Allowed values

| Value | Meaning | Examples |
|---|---|---|
| `realm` | The realm.watch family — themed, public-shareable tools and libraries. | clock, dark, familiar, lexicon, mirror, os, status, plus all 2026-05-08 renames. |
| `personal` | JP's personal/internal homelab + name brand. Implicitly tied to `jphe.in` or `jphein.github.io`. | speech-to-cli, cloud-chat-assistant, claude-code-switcher, gnome-speaks, claudedoublehours, kiyo-xhci-fix, disks, vault-gate, gnome-shell-monitor (sub-comp), realm-optimizer (sub-comp), realmwatch (until renamed), tonemask, tablet-tune, vault, ventoy, usb-issues, streamcam-fixes, streaming, veadotube-avatars, jp, ipv6, current-sensor, openclaw, gaming-tuning, minecraft-bedrock-linux, obs-plugins, lettertomom, outline, hostname-badge, opus, dreamspace, unshuffled, techempower, artcardsv5, scripts, optimize, openwrt, esp_wifi_repeater, multipass-structural-memory-eval, mempalace-triage, mempalace-data, ha. |
| `external` | Standalone brand or client/family work with its own identity. Lives on its own TLD (or no TLD). | donkeyco, imaginalvision.com, jpheinnet, poppasblog, jphein-wordpress-server, jphein-wordpress-site, forageforall, jewelrycycle, shawnahein.com, solar, sdp, sdp+disability-appeal, starcharts, roblox, claudedoublehours (extension on store), VoxSherpa-TTS, candela, clawwatch. |
| `fork` | Upstream fork — identity belongs to upstream. Don't rename, don't move. | rlm, karta, gstack, claude-code-python, claude-code-source-leaked, mempalace-triage, esp_wifi_repeater (already covered above), oracle-mcp's repo (`the-oracle.git`). |
| `archived` (optional, redundant with status: archived) | Decommissioned. | umbra, oldsites, claude-code-source. Or just rely on `status: archived`. |

**Recommendation:** four values — `realm`, `personal`, `external`, `fork`. Skip `archived` (use `status:`).

### Default

For projects with no `audience:` set, default to:

- If `current_name` ends in `.realm.watch` → `realm`
- Else if `current_name` matches a known fork pattern (the `fork` kind already in the catalog) → `fork`
- Else → no default. Print a warning when `lexicon validate` runs.

This avoids silently mis-classifying anything.

## Rendering changes

Update the catalog skill projection (`lexicon catalog render --format=skill`) to group by audience first, then status within. Heading order:

```
## Realm-watch family (active)
### clock (`clock.realm.watch`)
...

## Realm-watch family (local-only)
...

## Personal services & homelab (active)
### speech-to-cli
...

## Personal services & homelab (local-only)
...

## Standalone brands
### donkeyco
...

## Forks
### gstack (upstream: garrytan/gstack)
...

## Archived
### umbra
...
```

This matches JP's mental model and makes the skill genuinely useful for navigation. The current "alphabetical by name within status" rendering misses the conceptual grouping.

## CLI changes

### `lexicon list`
Add `--audience=<value>` filter. Same UX as existing `--realm`, `--kind`, `--status` flags.

### `lexicon rename`
No required changes. Optional: accept `--audience=<value>` to set the audience on the renamed project (useful when promoting/demoting). Skip if not passed.

### `lexicon validate`
Warn when a project has no `audience:` and the default-inference rules don't fire.

## Migration plan

1. **Schema patch** — add `audience` field to `Project` struct, with `omitempty` so existing entries without it stay clean.
2. **Catalog backfill** — one-shot pass classifying all 88 entries. Mostly mechanical (looking at `current_name` and `kind`); a few judgment calls for the borderline cases. Same effort as the realm classification.
3. **Render update** — extend `cmd_catalog_render.go` to group by audience.
4. **List filter** — small addition to `cmd_list.go`.
5. **Validate update** — warn-not-fail behavior.
6. **Re-render the skill** — `~/.claude/skills/project-catalog/SKILL.md` now grouped by audience.
7. **Trim CLAUDE.md** — once skill rendering is by audience, the CLAUDE.md project pointer can mention "filtered by audience" so users (and Claude) know how to query.

Each step is independently shippable; no big-bang rollout needed.

## Open questions

1. **Naming**: `audience`, `tier`, `bucket`, `surface`? `audience` reads as "who is this for"; `tier` reads as "ranking"; `bucket` reads as "I gave up naming." I lean `audience` but it's a small bikeshed.
2. **Forks under `audience: fork`** vs **forks distributed across the other audiences**? Some forks (rlm, karta) are realm-aligned conceptually but identity-bound to upstream. Putting them in `fork` keeps the registry clean.
3. **`mempalace-triage`** — it's a fork-ish (lives at `MemPalace/mempalace-triage` on GitHub) but JP authored it. Probably `fork` or `personal`? Edge case.
4. **`portfolio`** — was just discussed for rename to `portfolio.realm.watch`. Audience flips from "external" (brand-y portfolio of work) to "realm" (themed showcase). Conceptually it's about JP's work but stylistically realm-aligned. Likely `realm` once renamed.

## Severity / priority

Low-medium. The catalog works fine without this; it's a quality-of-life upgrade. But it's the missing piece that makes Lexicon's "canonical project registry" claim land — right now the registry knows current name, prior names, repo, realm, status, but not the *bucket* the project lives in. That's the field that informs every catalog-level decision (skill grouping, rename target convention, deploy target, audience for documentation, etc.).

## Done criteria

- [x] Schema: `Project.Audience` field exists (`audience,omitempty` in `go/catalog.go`).
- [x] All 88 catalog entries classified (14 realm / 46 personal / 19 external / 9 fork).
- [x] `lexicon list --audience=...` filters.
- [x] `lexicon catalog render --format=skill` groups by audience, then status.
- [x] Regression test for the new render grouping (`TestCatalogRender_SkillAudienceGroupOrder`).
- [x] `~/.claude/skills/project-catalog/SKILL.md` regenerated.
- [x] Spec marked `Status: Implemented` (this file).

## Implementation notes

- `validate.go` warns with code `pending_audience` when audience is unset and no inference rule fires (kind=fork → fork; current_name suffix `.realm.watch` → realm).
- `InferAudience(p)` is the single source of truth shared between renderer, list filter, and validator.
- The catalog YAML round-trip (`updateProjectMapping`/`newProjectMapping` in `catalog.go`) preserves the field on save; backfill itself was a one-shot Python pass (`scripts/backfill-audience.py`) inserting `audience:` directly after `realm:` so the field sits next to its conceptual neighbors rather than at end-of-mapping.
