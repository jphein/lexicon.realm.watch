# lexicon — Python

Python parity library for [lexicon.realm.watch](https://github.com/jphein/lexicon.realm.watch).

The Go binary ships the CLI; this package gives Python callers the same roller
and catalog surface so realmwatch (and any other Python tool in the family) can
roll names and read the catalog without shelling out.

See the repo root for the full design spec and CLI docs.

## Install

From a checkout:

```bash
pip install -e python/[dev]
```

## Quickstart

```python
import lexicon

agent = lexicon.roll(
    "agent",
    vocabularies_dir="vocabularies",
    recipes_path="vocabularies/recipes.yaml",
)
# → "gallant_curie"

cat = lexicon.load_catalog("catalog/projects.yaml")
proj = cat.resolve("realmwatch")
print(proj.current_name)
```

## Cross-language parity

`lexicon.seeded_index(seed, slot, modulus)` matches Go's `lexicon.SeededIndex`
byte-for-byte and is verified against `tests/fixtures/seeded-recipes.json` at
the repo root.

## Tests

```bash
cd python
python -m pytest
```
