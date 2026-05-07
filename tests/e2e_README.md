# End-to-end test — `go/e2e_test.go`

A single Go test (`TestE2E`) that exercises the full lexicon chain in one
sitting. If this passes from a clean checkout, the v1 surface is wired
end-to-end: vocabularies load, recipes roll, catalogs resolve, the binary
runs, and the rename + catalog import CLI surfaces don't drift.

## What it covers

The test runs from `go/` (cwd of the test binary) and walks the chain in
this order:

1. **Vocabulary load** — `loadLiveVocabsCombined()` merges every
   `vocabularies/*.yaml`. Asserts `realms.fantasy` is present.
2. **Recipes load** — `LoadRecipeBook(../vocabularies/recipes.yaml)`.
   Asserts `project`, `agent`, `branch`, `entity` recipes exist.
3. **`RollN`** — pulls 5 unique `project` candidates with
   `--realm fantasy`. Asserts non-empty, contains `-`, and all five
   are distinct.
4. **`RollSeeded` smoke** — same seed twice → identical result, name
   contains `_` (agent recipe shape). Full cross-language fixture
   parity is in `seeded_test.go::TestRollSeeded_MatchesFixture`; we
   intentionally don't duplicate that here.
5. **Catalog load** — `LoadCatalog(../catalog/projects.yaml)`.
6. **`Resolve("realmwatch")`** — must return the seeded entry.
7. **`ByRealm("oracle")`** — must include both `lexicon` and
   `realm-sigil`.
8. **`Validate(...)`** — live catalog must be issue-free.
9. **Build the binary** — `go build -o $TMP/lexicon ./cmd/lexicon`.
10. **CLI smoke** — exec the binary with `validate`, `recipes`, and
    `roll agent`. Each must exit 0 with non-empty stdout matching the
    expected substrings.
11. **`rename --plan`** — exec against a fake `--projects-dir` so no
    real filesystem state is touched. Asserts the 10-step runbook
    header and key step titles render.
12. **`catalog import --dry-run`** — exec against a tmpdir tree
    containing `alpha-tool/package.json` and `beta-lib/go.mod`.
    Asserts the dry-run output names both projects.
13. **`claim` round-trip** — copy `tests/fixtures/catalog-test.yaml`
    into a tmpdir, exec `lexicon claim watch.realm.watch
    --renames realmwatch`, then reload via `LoadCatalog` and assert
    `current_name` is the new value.

The build step uses `/snap/bin/go` first (per project CLAUDE.md) and
falls back to plain `go` if that path is unavailable.

## What it intentionally does NOT cover

- **Python or JS libraries.** `python/` and `js/` have their own test
  suites. The cross-language seeded contract is in
  `tests/fixtures/seeded-recipes.json`; each language verifies parity
  against that fixture in its own runtime. Re-running those checks
  from Go would require booting two extra interpreters and add more
  flake than coverage.
- **`rename --execute`.** The auto steps mutate the filesystem and
  shell out to `gh`; the existing unit tests in
  `cmd_rename_test.go` cover that path with a `fakeFS` substitute.
  E2E only smokes `--plan` so it stays hermetic.
- **DNS / Caddy / Outline / GitHub** side-effects. Manual steps in
  the runbook are reminders only; there's nothing to assert here.
- **Static landing page.** `static/build.sh` has its own surface; the
  e2e test does not render or fetch HTML.
- **CI workflows.** The test asserts the *code* is wired, not the
  CI YAML.

## Running it

```bash
cd go
/snap/bin/go test -run TestE2E ./... -v
```

Use `-short` to skip — the test calls `t.Skip("skipping e2e in -short
mode")` when `testing.Short()` is true. This is useful for fast
inner-loop iteration; the full e2e takes ~250ms because it shells out
to `go build`.

## Maintenance

If a CLI command's stdout text changes (e.g. `lexicon validate` stops
printing `OK`), this test fails fast and points at the assertion. Update
the substring list in `runCLI(...)` calls to match. The fake project
tree in `makeFakeProjectTree` only needs touching when the catalog
import surface gains new field inferences worth asserting on.
