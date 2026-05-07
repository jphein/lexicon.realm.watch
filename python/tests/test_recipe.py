"""Recipe engine tests.

Uses the frozen test corpus at ``tests/fixtures/vocabularies-test.yaml`` plus
the live ``vocabularies/recipes.yaml`` for shape checks. Cross-language parity
on seeded rolls is exercised in :mod:`test_seeded`.
"""

from __future__ import annotations

import random
from pathlib import Path

import pytest

from lexicon import (
    RecipeBook,
    RollOptions,
    Vocabulary,
    load_recipe_book,
    load_vocabulary_dir,
    load_vocabulary_file,
    roll,
    roll_n,
    roll_seeded,
    seeded_index,
)


@pytest.fixture()
def test_vocabulary(vocabularies_test_path: Path) -> Vocabulary:
    return load_vocabulary_file(vocabularies_test_path)


@pytest.fixture()
def live_vocabulary(vocabularies_dir: Path) -> Vocabulary:
    return load_vocabulary_dir(vocabularies_dir)


@pytest.fixture()
def book(recipes_path: Path) -> RecipeBook:
    return load_recipe_book(recipes_path)


def test_recipe_book_lists_known_recipes(book: RecipeBook) -> None:
    names = book.names()
    assert "project" in names
    assert "agent" in names
    assert "branch" in names


def test_required_options(book: RecipeBook) -> None:
    assert "realm" in book.required_options("project")
    assert "prefix" in book.required_options("branch")
    assert book.required_options("agent") == []


def test_unknown_recipe_raises(book: RecipeBook, test_vocabulary: Vocabulary) -> None:
    with pytest.raises(KeyError):
        book.roll("nonsense", test_vocabulary, RollOptions())


def test_project_requires_realm(book: RecipeBook, live_vocabulary: Vocabulary) -> None:
    with pytest.raises(ValueError, match="realm"):
        book.roll("project", live_vocabulary, RollOptions())


def test_branch_requires_prefix(book: RecipeBook, live_vocabulary: Vocabulary) -> None:
    with pytest.raises(ValueError, match="prefix"):
        book.roll("branch", live_vocabulary, RollOptions())


def test_branch_pattern(book: RecipeBook, live_vocabulary: Vocabulary) -> None:
    out = book.roll("branch", live_vocabulary, RollOptions(prefix="feat"), rng=random.Random(1))
    assert out.startswith("feat/")
    parts = out.split("/", 1)[1].split("-")
    assert len(parts) == 3
    assert all(parts), f"branch parts have empties: {out!r}"


def test_agent_pattern_lowercase(book: RecipeBook, live_vocabulary: Vocabulary) -> None:
    out = book.roll("agent", live_vocabulary, RollOptions(), rng=random.Random(7))
    assert "_" in out
    assert out == out.lower()


def test_project_uses_realm_words(
    book: RecipeBook, live_vocabulary: Vocabulary
) -> None:
    out = book.roll("project", live_vocabulary, RollOptions(realm="signal"), rng=random.Random(0))
    # cap-cap pattern
    parts = out.split("-")
    assert len(parts) == 2
    assert parts[0][0].isupper()
    assert parts[1][0].isupper()


def test_roll_seeded_is_deterministic(
    book: RecipeBook, live_vocabulary: Vocabulary
) -> None:
    a = book.roll_seeded("agent", live_vocabulary, "fixed-seed")
    b = book.roll_seeded("agent", live_vocabulary, "fixed-seed")
    assert a == b


def test_roll_seeded_matches_seeded_index_directly(
    book: RecipeBook, live_vocabulary: Vocabulary
) -> None:
    """The seeded engine consults seeded_index per slot-name occurrence.

    The slot counter is keyed by slot *name*, not a global counter. For the
    agent recipe (``{adjective:lower}_{scientist:lower}``), adjective and
    scientist each get their own counter starting at 0. So the produced name
    is ``adjectives[seeded_index(seed, 0, len(adj))]`` +
    ``scientists[seeded_index(seed, 0, len(sci))]``.
    """
    seed = "parity-seed"
    out = book.roll_seeded("agent", live_vocabulary, seed)
    adjectives = live_vocabulary.group("adjectives", "any")
    scientists = live_vocabulary.group("scientists", "any")
    assert adjectives is not None and scientists is not None
    expected_adj = adjectives[seeded_index(seed, 0, len(adjectives))].lower()
    expected_sci = scientists[seeded_index(seed, 0, len(scientists))].lower()
    assert out == f"{expected_adj}_{expected_sci}"


def test_roll_n_returns_unique(book: RecipeBook, live_vocabulary: Vocabulary) -> None:
    out = book.roll_n(
        "project",
        live_vocabulary,
        5,
        RollOptions(realm="fantasy"),
        rng=random.Random(123),
    )
    assert len(out) <= 5
    assert len(set(out)) == len(out)


def test_roll_n_zero_returns_empty(book: RecipeBook, live_vocabulary: Vocabulary) -> None:
    assert book.roll_n("agent", live_vocabulary, 0) == []


def test_top_level_helpers(vocabularies_dir: Path, recipes_path: Path) -> None:
    name = roll(
        "agent",
        vocabularies_dir=vocabularies_dir,
        recipes_path=recipes_path,
        rng=random.Random(11),
    )
    assert "_" in name
    deterministic = roll_seeded(
        "agent",
        seed="check",
        vocabularies_dir=vocabularies_dir,
        recipes_path=recipes_path,
    )
    again = roll_seeded(
        "agent",
        seed="check",
        vocabularies_dir=vocabularies_dir,
        recipes_path=recipes_path,
    )
    assert deterministic == again
    multi = roll_n(
        "project",
        3,
        realm="fantasy",
        vocabularies_dir=vocabularies_dir,
        recipes_path=recipes_path,
        rng=random.Random(2),
    )
    assert len(set(multi)) == len(multi)
