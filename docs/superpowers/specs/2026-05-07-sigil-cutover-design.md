# Sigil Cutover Design — lexicon as the canonical word source

**Date:** 2026-05-07
**Author:** wise_faraday (with JP, swarm wave A)
**Status:** Design — implementation deferred until lexicon v1.0 ships
**Related:** `docs/superpowers/specs/2026-04-26-lexicon-design.md` § 3 ("Sigil retirement")

## Premise

`realm-sigil` currently owns `words/realms.json` as a hand-curated source. Per
the v1 design, lexicon becomes the system of record for realm vocabularies and
sigil consumes them as derived artifacts. This doc describes the rollout: what
script, what versions, what CI, what stays the same.

## Out of scope

- Adding new sigil features.
- Modifying lexicon recipes or the catalog schema.
- Touching `~/Projects/realm-sigil/` while lexicon v1.0 is still in flight.
  This doc is intent + sequence; the actual sigil PR happens after lexicon ships.

## Current state (snapshot 2026-05-07)

- **lexicon** (`~/Projects/lexicon.realm.watch`):
  - `vocabularies/realms.yaml` — 7 realms with descriptive `words[]` (fantasy,
    tarot, oracle, void, forge, signal, stellar)
  - `vocabularies/adjectives.yaml`, `vocabularies/nouns.yaml` — same 7 realms
    plus `any`. Per-realm word counts: 10 (fantasy) and 14 (others).
  - `sync-vocabularies.sh` (this delivery) — converts the above to sigil's
    `words/realms.json` format.
- **sigil** (`~/Projects/realm-sigil`):
  - `words/realms.json` — 7 realms × 20 adjectives × 20 nouns, hand-curated.
  - `sync-words.sh` — generates `go/realms.go`, `python/realm_sigil/realms.py`,
    `js/realms.js` from `words/realms.json`. Embeds words into language
    packages so they have zero runtime IO.
- **Drift:** Running `./sync-vocabularies.sh --check` against current sigil
  reports drift in every realm — sigil's words are not lexicon's words. This
  is expected: the cutover *replaces* sigil's hand-picked corpus with
  lexicon's. JP must approve the new corpus (or grow lexicon's lists to
  match the previous breadth) before the sigil PR lands.

## Format mapping

Lexicon vocabularies are split across files (one per word category) with
lowercase entries. Sigil expects a single nested file with capitalized words:

```
lexicon/vocabularies/adjectives.yaml          sigil/words/realms.json
   adjectives:                                   {
     fantasy:                                       "fantasy": {
       words: [primal, runic, ...]   ────►            "adjectives": ["Primal", "Runic", ...],
   nouns:                                              "nouns":      ["Sigil",  "Pulsar", ...]
     fantasy:                                       },
       words: [sigil, pulsar, ...]                ...
```

Rules `sync-vocabularies.sh` enforces:

1. Iterate realm groups present in **both** `adjectives.yaml` AND `nouns.yaml`.
2. Skip the `any` group (lexicon-only; sigil doesn't model it).
3. Skip realms missing from either file (warn to stderr — not fatal).
4. Title-case each word (handles hyphenated words segment-by-segment to mirror
   sigil's existing convention, e.g. `white-hot` → `White-Hot`).
5. Sort realm keys alphabetically for determinism.
6. Emit JSON with 2-space indent + trailing newline (matches sigil's existing
   formatting).

The lexicon `realms.yaml` `words[]` lists (descriptive nouns like "primal,
shadow, ember") are **not** consumed by sigil. They serve lexicon-internal
recipes only. Sigil only needs adjectives and nouns.

## Rollout sequence

The cutover is a coordinated release across two repos. Order matters: lexicon
must ship its v1.0 surface (with the script and the vocabularies) before sigil
can rely on either.

1. **Lexicon v1.0 (this swarm).**
   - All Wave A + Wave B work lands.
   - `sync-vocabularies.sh` is checked in and tested.
   - `vocabularies/{realms,adjectives,nouns}.yaml` are stable for the v1
     contract. Future word additions are minor bumps; word removals/renames
     are major bumps because sigil consumes them.
   - JP tags `v1.0.0`, GitHub release published.

2. **Lexicon ↔ sigil parity audit (manual, JP-driven).**
   - JP runs `./sync-vocabularies.sh --dry-run` and compares against current
     `realm-sigil/words/realms.json`.
   - Two paths from here:
     - **Path A (preferred):** Adopt lexicon's corpus as-is. Sigil's user-
       facing names will change. Acceptable if no live deploys depend on
       specific sigil-derived strings.
     - **Path B:** Grow lexicon's word lists in `vocabularies/*.yaml` to
       cover what sigil currently has. Re-run sync. Repeat until `--check`
       reports no drift. This preserves any name JP has already shown
       publicly. Bump lexicon to v1.0.1 after each merge.
   - Decision lands in a comment in `vocabularies/realms.yaml` so future
     readers know why the lists are sized as they are.

3. **Sigil cutover PR (separate session, separate repo, JP authors).**
   - Branch in `realm-sigil`: `feat/lexicon-as-source`.
   - Replace `sync-words.sh` with a thin wrapper that delegates to lexicon:

     ```bash
     #!/usr/bin/env bash
     # Vocabularies live in lexicon.realm.watch — this is a passthrough.
     set -euo pipefail
     LEXICON_DIR="${LEXICON_DIR:-$HOME/Projects/lexicon.realm.watch}"
     "$LEXICON_DIR/sync-vocabularies.sh" --sigil-dir "$(dirname "$0")"
     # Then regenerate the per-language embeds from words/realms.json:
     # (existing logic that writes go/realms.go, python/realm_sigil/realms.py, js/realms.js)
     ```

   - **Or** delete `sync-words.sh` entirely and let lexicon's script run
     against sigil's checkout. Simpler; preferred unless a sigil-only word
     edit becomes a real workflow.
   - `words/realms.json` is regenerated by the new flow. Commit the diff.
   - `README.md`: add a section "Vocabularies":

     > Realm word lists are sourced from `lexicon.realm.watch`. To update,
     > edit `vocabularies/{realms,adjectives,nouns}.yaml` in lexicon and
     > re-run `./sync-vocabularies.sh` against this repo. The CI check
     > below enforces no drift.

   - Bump sigil's `version.json` to a minor release (e.g. v0.4.0 → v0.5.0).
   - Tag and release.

4. **Sigil CI gains a drift check (same PR).**
   - In `.github/workflows/ci.yml`, add a job that checks out lexicon and
     runs `./sync-vocabularies.sh --check`. Fails if `words/realms.json`
     drifts from lexicon's vocabularies.

     ```yaml
     vocab-drift:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v4
           with: { path: sigil }
         - uses: actions/checkout@v4
           with:
             repository: jphein/lexicon.realm.watch
             path: lexicon
         - name: Drift check
           run: |
             cd lexicon
             ./sync-vocabularies.sh --check --sigil-dir ../sigil
     ```

   - This catches the case where someone hand-edits `words/realms.json` in
     sigil — the canonical path is now lexicon, and CI surfaces violations.

5. **Future word edits.**
   - Edit `vocabularies/*.yaml` in lexicon. Open a PR.
   - Lexicon CI runs `lexicon validate` to check internal consistency.
   - On merge, lexicon publishes a patch release (or minor if adding a realm).
   - JP runs `./sync-vocabularies.sh` against sigil locally, opens a sigil PR
     with the regenerated `words/realms.json` and any embed updates. Sigil's
     drift CI passes; merge; release.
   - `lexicon` tag → `sigil` tag is intentionally **not** automatic. The
     human in the loop is the audit step (JP eyeballs the corpus change).

## Version coordination

- Lexicon SemVer policy for vocabularies (added to `CLAUDE.md` "Don't" rules
  during README rewrite, sleepy_einstein's task):
  - **Adding** a word to an existing realm group → patch.
  - **Adding** a new realm group → minor.
  - **Removing/renaming** a word that any consumer (currently: sigil) uses →
    major. Sigil compatibility is now part of lexicon's contract.
- Sigil pins lexicon's version (or commit SHA) in CI when running the drift
  check, so a lexicon main branch update doesn't silently break sigil's CI
  until JP deliberately bumps the pin.

## CI shape — recap

After the cutover, three CI checks enforce the contract:

| Repo    | Workflow                | What it checks                                          |
|---------|-------------------------|---------------------------------------------------------|
| lexicon | `lexicon validate`      | Internal consistency: catalog ↔ vocabularies            |
| lexicon | `go test ./...`         | Library + CLI behavior                                  |
| sigil   | `vocab-drift` (new)     | `words/realms.json` matches lexicon's current output    |

The cross-language fixture parity test (`tests/fixtures/seeded-recipes.json`)
is internal to lexicon — it lives in lexicon's CI, not sigil's. Sigil keeps
its own determinism tests for hash → name derivation; those are unchanged.

## What we are explicitly *not* doing

- **No automated cross-repo PRs.** Sigil bumps are driven by JP after a vocab
  audit. The script is the automation; the merge is human-decided.
- **No service or HTTP layer.** Sync is a one-shot script. Files in, files out.
- **No backward-compat shim for `sync-words.sh`.** Once cutover lands, sigil's
  old path is replaced or removed. Anyone who scripted against `sync-words.sh`
  (none of JP's known callers) updates the path. This is small enough that a
  shim costs more clarity than it saves.
- **No vocabularies category split per-deployment.** Sigil takes the same
  realms × adjectives × nouns lexicon produces. Per-tenant or per-deploy
  customization is a v2 design and lives in a separate spec.

## Risk and rollback

- **Risk:** Sigil deploys regenerate names from new vocabularies and break a
  user's mental model ("the auth daemon used to be called Arcane-Sigil, now
  it's Primal-Beacon"). Mitigated by Path B in the parity audit (grow lexicon
  to cover sigil's existing corpus).
- **Risk:** Lexicon vocab churn cascades into sigil CI failures. Mitigated by
  pinning a specific lexicon SHA/tag in sigil's drift workflow.
- **Rollback:** Revert sigil's PR. Lexicon stays where it is. The script is
  read-only against sigil unless invoked in `write` mode, so lexicon is never
  the cause of an outage in sigil-consuming systems — only the sigil PR is.

## Decisions (JP, 2026-05-07)

1. **Path A vs Path B per realm.** Confirmed: Path B for `fantasy` and
   `signal` (grow lexicon's lists to cover sigil's existing corpus before
   cutover, so user-visible names don't churn); Path A for `oracle`,
   `forge`, `void`, `stellar`, `tarot` (adopt lexicon as-is — these realms
   have no public history in sigil yet, churn is harmless). Same decision
   recorded as a comment in `vocabularies/realms.yaml`.
2. **`sync-words.sh` disposition.** _Open — to be confirmed at sigil PR
   time._ Default recommendation: delete. One less script to maintain.
3. **Lexicon SHA pin policy in sigil's CI.** _Open — to be confirmed at
   sigil PR time._ Default recommendation: pin to tag (slow but safe).
