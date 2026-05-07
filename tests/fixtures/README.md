# tests/fixtures — cross-language parity contract

This directory holds the data that pins the **cross-language** behavior of
`lexicon`. Anything in here is a contract: every language implementation
(Go, Python, JS) must produce identical output when given identical input.

## seeded-recipes.json — the core parity fixture

`seeded-recipes.json` is a JSON document with `_doc`, `_algorithm`,
`_generator`, and a `cases` array. Each case asserts:

```json
{
  "seed": "realmwatch",
  "recipe": "agent",
  "options": null,
  "expected_name": "hopeful_faraday"
}
```

> Calling `RollSeeded(recipe, vocabulary, seed, options)` against the
> vocabularies and recipes in this repo MUST return `expected_name`,
> in every language implementation.

`options` is either `null` (no options) or a `{realm, prefix}`-shaped object.
Unused keys are omitted, never set to empty string — the absence of a key is
itself part of the contract.

### What the contract covers

| Recipe  | Cases | Notes                                                        |
|---------|-------|--------------------------------------------------------------|
| agent   | 7     | Unicode seed, empty seed, very long seed, common ASCII       |
| branch  | 5     | Five `prefix` values (feat/fix/refactor/docs/chore)          |
| entity  | 4     | Fantasy-only, varied seeds                                   |
| project | 17    | Each of the 7 realms × 2 seeds, plus 3 fantasy seed variants |

Total: 33 cases. The list is intentionally over-specified for `project`
because realm-fanout is the historically risky path (off-by-one errors,
group-name overrides, slot-counter regressions).

### The algorithm being asserted

The generator and every consumer must implement the same
`SeededIndex(seed, slot, modulus)`:

```
SeededIndex(seed, slot, modulus) =
  uint64( BE_first_8_bytes( SHA-256( utf8(seed) || BE_uint64_8bytes(slot) ) ) )
  mod modulus
```

The slot counter increments per slot occurrence within a single roll, so
`{noun}-{noun}` reads slots 0 and 1 from the same seed. Within-roll
uniqueness: each slot retries up to `len(words)` times before falling back
to a duplicate. See `go/seeded.go` for the reference implementation.

## How to regenerate

The fixture is **generated**, not hand-written. The generator is checked in
at `go/cmd/seed-fixtures/main.go` and reads the live vocabularies + recipes
from this repo, so a change to either will change the fixture.

```bash
cd go
/snap/bin/go run ./cmd/seed-fixtures > ../tests/fixtures/seeded-recipes.json
```

When you regenerate, the Python and JS parity tests must also be re-run —
their failures pinpoint the cross-language drift.

## When NOT to regenerate

Per the project `CLAUDE.md`:

> Don't edit `tests/fixtures/seeded-recipes.json` `expected_name` values
> without re-running the parity test.

If a test fails because Python disagrees with Go, **the bug is in Python**
(or vice versa) — do not "fix" the failing test by overwriting the
`expected_name` field. Regenerate only when a vocabulary change is the
deliberate cause.

## Other fixtures here

- `catalog-test.yaml` — a tiny catalog used by Go unit tests for resolution / claim.
- `vocabularies-test.yaml` — a tiny vocabulary used by Go unit tests for the recipe engine.

These are Go-only fixtures, not part of the cross-language contract.
