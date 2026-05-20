"""lexicon — Python parity library for ``lexicon.realm.watch``.

Public surface mirrors the Go reference implementation. See repo root for the
design spec and CLI docs.
"""

from __future__ import annotations

import random as _random
from pathlib import Path
from typing import Iterable

from .catalog import Catalog, ClaimOpts, PriorName, Project, load_catalog
from .fleet import (
    FleetCatalog,
    FleetEntry,
    FleetPriorName,
    load_fleet_catalog,
)
from .recipe import RecipeBook, RollOptions, load_recipe_book
from .seeded import seeded_index
from .vlans import (
    VLANCatalog,
    VLANEntry,
    VLANPriorName,
    load_vlan_catalog,
)
from .vocabulary import Vocabulary, load_vocabulary_dir, load_vocabulary_file
from .zones import (
    ZoneCatalog,
    ZoneEntry,
    ZonePriorName,
    load_zone_catalog,
)

__all__ = [
    "Catalog",
    "ClaimOpts",
    "FleetCatalog",
    "FleetEntry",
    "FleetPriorName",
    "PriorName",
    "Project",
    "RecipeBook",
    "RollOptions",
    "VLANCatalog",
    "VLANEntry",
    "VLANPriorName",
    "Vocabulary",
    "ZoneCatalog",
    "ZoneEntry",
    "ZonePriorName",
    "load_catalog",
    "load_fleet_catalog",
    "load_recipe_book",
    "load_vlan_catalog",
    "load_vocabulary_dir",
    "load_vocabulary_file",
    "load_zone_catalog",
    "roll",
    "roll_n",
    "roll_seeded",
    "seeded_index",
]

__version__ = "0.1.0"


def _resolve_recipe_path(
    recipes_path: str | Path | None,
    vocabularies_dir: str | Path | None,
) -> Path:
    if recipes_path is not None:
        return Path(recipes_path)
    if vocabularies_dir is not None:
        return Path(vocabularies_dir) / "recipes.yaml"
    raise ValueError("must pass recipes_path or vocabularies_dir")


def roll(
    recipe: str,
    *,
    realm: str | None = None,
    prefix: str | None = None,
    vocabularies_dir: str | Path | None = None,
    recipes_path: str | Path | None = None,
    vocabulary: Vocabulary | None = None,
    book: RecipeBook | None = None,
    rng: _random.Random | None = None,
) -> str:
    """Convenience wrapper: roll one name."""
    book = book or load_recipe_book(_resolve_recipe_path(recipes_path, vocabularies_dir))
    if vocabulary is None:
        if vocabularies_dir is None:
            raise ValueError("must pass vocabularies_dir or vocabulary")
        vocabulary = load_vocabulary_dir(vocabularies_dir)
    return book.roll(recipe, vocabulary, RollOptions(realm=realm, prefix=prefix), rng=rng)


def roll_seeded(
    recipe: str,
    seed: str,
    *,
    realm: str | None = None,
    prefix: str | None = None,
    vocabularies_dir: str | Path | None = None,
    recipes_path: str | Path | None = None,
    vocabulary: Vocabulary | None = None,
    book: RecipeBook | None = None,
) -> str:
    """Convenience wrapper: roll one name deterministically from ``seed``."""
    book = book or load_recipe_book(_resolve_recipe_path(recipes_path, vocabularies_dir))
    if vocabulary is None:
        if vocabularies_dir is None:
            raise ValueError("must pass vocabularies_dir or vocabulary")
        vocabulary = load_vocabulary_dir(vocabularies_dir)
    return book.roll_seeded(recipe, vocabulary, seed, RollOptions(realm=realm, prefix=prefix))


def roll_n(
    recipe: str,
    n: int,
    *,
    realm: str | None = None,
    prefix: str | None = None,
    vocabularies_dir: str | Path | None = None,
    recipes_path: str | Path | None = None,
    vocabulary: Vocabulary | None = None,
    book: RecipeBook | None = None,
    rng: _random.Random | None = None,
) -> list[str]:
    """Convenience wrapper: roll up to ``n`` unique candidates."""
    book = book or load_recipe_book(_resolve_recipe_path(recipes_path, vocabularies_dir))
    if vocabulary is None:
        if vocabularies_dir is None:
            raise ValueError("must pass vocabularies_dir or vocabulary")
        vocabulary = load_vocabulary_dir(vocabularies_dir)
    return book.roll_n(recipe, vocabulary, n, RollOptions(realm=realm, prefix=prefix), rng=rng)


def _candidates_iter(values: Iterable[str]) -> list[str]:
    """Internal helper retained for symmetry with Go's helpers; not exported."""
    return list(values)


def load_catalog_by_kind(path, kind: str = "projects"):
    """Dispatch load by catalog kind. Convenience for callers."""
    if kind == "projects":
        from .catalog import load_catalog
        return load_catalog(path)
    if kind == "fleet":
        return load_fleet_catalog(path)
    if kind == "vlans":
        return load_vlan_catalog(path)
    if kind == "zones":
        return load_zone_catalog(path)
    raise ValueError(f"unknown catalog kind: {kind}")
