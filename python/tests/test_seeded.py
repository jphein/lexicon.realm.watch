"""Cross-language parity tests.

Two layers:

1. :func:`seeded_index` raw byte-for-byte parity sanity (the math primitive).
2. ``RollSeeded(recipe, seed, options)`` parity against the cross-language
   fixture at ``tests/fixtures/seeded-recipes.json``. Go, Python, and JS all
   run this same JSON through their own implementations and must agree.
"""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from lexicon import (
    RollOptions,
    load_recipe_book,
    load_vocabulary_dir,
    seeded_index,
)


# ---------------------------------------------------------------------------
# Layer 1 — raw seeded_index sanity
# ---------------------------------------------------------------------------


def test_seeded_index_zero_modulus_returns_zero() -> None:
    assert seeded_index("anything", 0, 0) == 0
    assert seeded_index("anything", 5, -1) == 0


def test_seeded_index_independence_across_slots() -> None:
    """Different slots over the same seed should generally pick differently."""
    seed = "stable-seed"
    indices = {seeded_index(seed, slot, 1000) for slot in range(20)}
    assert len(indices) > 1


def test_seeded_index_deterministic() -> None:
    assert seeded_index("repeatable", 7, 256) == seeded_index("repeatable", 7, 256)


def test_seeded_index_known_vectors() -> None:
    """Hard-coded vectors verified against Go's SeededIndex."""
    assert seeded_index("abc123", 0, 4) == 1
    assert seeded_index("abc123", 1, 4) == 2
    assert seeded_index("realmwatch", 0, 5) == 0
    assert seeded_index("test", 0, 100) == 90


# ---------------------------------------------------------------------------
# Layer 2 — recipe parity against the cross-language fixture
# ---------------------------------------------------------------------------


def _load_cases(path: Path) -> list[dict]:
    raw = json.loads(path.read_text(encoding="utf-8"))
    if isinstance(raw, dict) and "cases" in raw:
        return list(raw["cases"])
    if isinstance(raw, list):
        return list(raw)
    raise ValueError(f"unrecognized fixture shape in {path}")


def test_fixture_file_exists(seeded_recipes_path: Path) -> None:
    if not seeded_recipes_path.exists():
        pytest.skip(
            f"cross-language fixture {seeded_recipes_path} not present yet — "
            "gallant_hopper is producing it; rerun once that lands."
        )
    assert seeded_recipes_path.is_file()


def test_seeded_recipes_match_fixture(
    seeded_recipes_path: Path,
    vocabularies_dir: Path,
    recipes_path: Path,
) -> None:
    """Every (seed, recipe, options) → expected_name in the fixture must
    match Python's roll_seeded byte-for-byte."""
    if not seeded_recipes_path.exists():
        pytest.skip("cross-language fixture missing")

    cases = _load_cases(seeded_recipes_path)
    assert cases, "fixture file has no cases"

    book = load_recipe_book(recipes_path)
    vocabulary = load_vocabulary_dir(vocabularies_dir)

    failures: list[str] = []
    for i, case in enumerate(cases):
        # Tolerate the legacy raw-modulus shape (seed/words/slot/expected_index)
        # in case a fixture revision keeps a few of those.
        if "expected_name" not in case:
            if "words" in case and "expected_index" in case and "slot" in case:
                modulus = len(case["words"])
                got = seeded_index(case["seed"], case["slot"], modulus)
                if got != case["expected_index"]:
                    failures.append(
                        f"case[{i}] (legacy) seed={case['seed']!r} "
                        f"slot={case['slot']} -> got {got}, want {case['expected_index']}"
                    )
                continue
            failures.append(f"case[{i}] missing expected_name")
            continue

        seed = case["seed"]
        recipe_name = case["recipe"]
        opts_raw = case.get("options") or {}
        opts = RollOptions(
            realm=opts_raw.get("realm"),
            prefix=opts_raw.get("prefix"),
        )
        try:
            got = book.roll_seeded(recipe_name, vocabulary, seed, opts)
        except Exception as exc:  # noqa: BLE001 — surface in failures list
            failures.append(
                f"case[{i}] recipe={recipe_name!r} seed={seed!r}: raised {exc}"
            )
            continue
        if got != case["expected_name"]:
            failures.append(
                f"case[{i}] recipe={recipe_name!r} seed={seed!r} options={opts_raw}: "
                f"got {got!r}, want {case['expected_name']!r}"
            )
    if failures:
        joined = "\n  ".join(failures)
        pytest.fail(f"{len(failures)} parity failures:\n  {joined}")
