"""Recipe engine — mirrors ``go/recipe.go``.

Loads ``vocabularies/recipes.yaml`` and produces names by filling pattern
slots from the :class:`~lexicon.vocabulary.Vocabulary`. Two roll modes:

- :meth:`RecipeBook.roll` — non-deterministic (``random``).
- :meth:`RecipeBook.roll_seeded` — SHA-256 deterministic, byte-identical
  with Go and JS.
"""

from __future__ import annotations

import random
import re
from dataclasses import dataclass, field
from pathlib import Path
from typing import Callable

from ruamel.yaml import YAML

from .seeded import seeded_index
from .vocabulary import Vocabulary

__all__ = [
    "RecipeBook",
    "RollOptions",
    "load_recipe_book",
]


@dataclass
class RollOptions:
    """Optional inputs that some recipes require."""

    realm: str | None = None
    prefix: str | None = None


@dataclass
class _RecipeSource:
    from_: str
    group: str


@dataclass
class _RecipeDef:
    description: str
    pattern: str
    sources: dict[str, _RecipeSource] = field(default_factory=dict)
    required_options: list[str] = field(default_factory=list)


_SLOT_RE = re.compile(r"\{([a-z_]+)(?::([a-z]+))?\}")


@dataclass
class _Token:
    literal: str = ""
    slot: str = ""
    transform: str = "raw"


def _parse_pattern(pat: str) -> list[_Token]:
    tokens: list[_Token] = []
    idx = 0
    for m in _SLOT_RE.finditer(pat):
        if m.start() > idx:
            tokens.append(_Token(literal=pat[idx : m.start()]))
        tokens.append(
            _Token(slot=m.group(1), transform=m.group(2) or "raw")
        )
        idx = m.end()
    if idx < len(pat):
        tokens.append(_Token(literal=pat[idx:]))
    return tokens


def _apply_transform(word: str, transform: str) -> str:
    if transform == "cap":
        if not word:
            return word
        return word[0].upper() + word[1:]
    if transform == "lower":
        return word.lower()
    if transform == "upper":
        return word.upper()
    return word


def _resolve_option(slot: str, opts: RollOptions) -> tuple[str, bool]:
    """Return ``(value, is_option)``. Raises if option required but missing."""
    if slot == "prefix":
        if not opts.prefix:
            raise ValueError("option 'prefix' is required")
        return opts.prefix, True
    if slot == "realm":
        if not opts.realm:
            raise ValueError("option 'realm' is required")
        return opts.realm, True
    return "", False


def _check_required(recipe: _RecipeDef, opts: RollOptions) -> None:
    for req in recipe.required_options:
        if req == "realm" and not opts.realm:
            raise ValueError("recipe requires --realm")
        if req == "prefix" and not opts.prefix:
            raise ValueError("recipe requires --prefix")


class RecipeBook:
    """The loaded ``recipes.yaml``. See module docstring."""

    def __init__(self, recipes: dict[str, _RecipeDef]) -> None:
        self._recipes = recipes

    def has(self, name: str) -> bool:
        return name in self._recipes

    def names(self) -> list[str]:
        return sorted(self._recipes.keys())

    def describe(self, name: str) -> str:
        r = self._recipes.get(name)
        return r.description if r else ""

    def required_options(self, name: str) -> list[str]:
        r = self._recipes.get(name)
        return list(r.required_options) if r else []

    def roll(
        self,
        name: str,
        vocabulary: Vocabulary,
        opts: RollOptions | None = None,
        *,
        rng: random.Random | None = None,
    ) -> str:
        """Roll one name, non-deterministically."""
        opts = opts or RollOptions()
        rng = rng or random.Random()
        recipe = self._recipe(name)
        _check_required(recipe, opts)
        return self._fill(
            recipe,
            vocabulary,
            opts,
            lambda _slot, modulus: rng.randrange(modulus),
        )

    def roll_seeded(
        self,
        name: str,
        vocabulary: Vocabulary,
        seed: str,
        opts: RollOptions | None = None,
    ) -> str:
        """Roll one name deterministically from ``seed``."""
        opts = opts or RollOptions()
        recipe = self._recipe(name)
        _check_required(recipe, opts)
        return self._fill(
            recipe,
            vocabulary,
            opts,
            lambda slot, modulus: seeded_index(seed, slot, modulus),
        )

    def roll_n(
        self,
        name: str,
        vocabulary: Vocabulary,
        n: int,
        opts: RollOptions | None = None,
        *,
        rng: random.Random | None = None,
    ) -> list[str]:
        """Roll up to ``n`` unique candidates. Returns fewer if the recipe's
        combinatorial space is smaller than ``n``."""
        if n <= 0:
            return []
        seen: set[str] = set()
        out: list[str] = []
        # Bound retries proportional to n; if the space is exhausted, return what we have.
        max_attempts = n * 50
        attempts = 0
        while len(out) < n and attempts < max_attempts:
            attempts += 1
            candidate = self.roll(name, vocabulary, opts, rng=rng)
            if candidate not in seen:
                seen.add(candidate)
                out.append(candidate)
        return out

    # -- internals ---------------------------------------------------------

    def _recipe(self, name: str) -> _RecipeDef:
        recipe = self._recipes.get(name)
        if recipe is None:
            raise KeyError(
                f"unknown recipe {name!r} (have {self.names()})"
            )
        return recipe

    def _fill(
        self,
        recipe: _RecipeDef,
        vocabulary: Vocabulary,
        opts: RollOptions,
        pick_index: Callable[[int, int], int],
    ) -> str:
        tokens = _parse_pattern(recipe.pattern)
        out_parts: list[str] = []
        slot_counter: dict[str, int] = {}
        picked: dict[str, set[int]] = {}

        for tk in tokens:
            if tk.literal:
                out_parts.append(tk.literal)
                continue
            value, is_option = _resolve_option(tk.slot, opts)
            if is_option:
                out_parts.append(_apply_transform(value, tk.transform))
                continue
            src = recipe.sources.get(tk.slot)
            if src is None:
                raise ValueError(
                    f"recipe {recipe.description!r}: slot {tk.slot!r} has no source and no option provided"
                )
            group = src.group
            # "fantasy"-group sources get overridden by --realm to keep parity with Go.
            if group == "fantasy" and opts.realm:
                group = opts.realm
            words = vocabulary.group(src.from_, group)
            if not words:
                raise ValueError(
                    f"recipe {recipe.description!r}: source {src.from_}.{group} missing or empty"
                )
            seen = picked.setdefault(tk.slot, set())
            idx = 0
            for _ in range(len(words)):
                slot_no = slot_counter.get(tk.slot, 0)
                candidate = pick_index(slot_no, len(words))
                slot_counter[tk.slot] = slot_no + 1
                if candidate not in seen or len(seen) >= len(words):
                    idx = candidate
                    break
            seen.add(idx)
            out_parts.append(_apply_transform(words[idx], tk.transform))
        return "".join(out_parts)


def load_recipe_book(path: str | Path) -> RecipeBook:
    """Load ``vocabularies/recipes.yaml`` into a :class:`RecipeBook`."""
    yaml = YAML(typ="safe")
    with Path(path).open("r", encoding="utf-8") as fh:
        raw = yaml.load(fh) or {}
    recipes_raw = raw.get("recipes") if isinstance(raw, dict) else None
    if not isinstance(recipes_raw, dict):
        raise ValueError(f"{path}: top-level 'recipes' mapping missing")
    recipes: dict[str, _RecipeDef] = {}
    for name, body in recipes_raw.items():
        if not isinstance(body, dict):
            raise ValueError(f"{path}: recipe {name!r} is not a mapping")
        sources_raw = body.get("sources") or {}
        sources: dict[str, _RecipeSource] = {}
        for slot_name, src in sources_raw.items():
            if not isinstance(src, dict):
                raise ValueError(
                    f"{path}: recipe {name!r} source {slot_name!r} is not a mapping"
                )
            sources[str(slot_name)] = _RecipeSource(
                from_=str(src.get("from", "")),
                group=str(src.get("group", "")),
            )
        recipes[str(name)] = _RecipeDef(
            description=str(body.get("description", "")),
            pattern=str(body.get("pattern", "")),
            sources=sources,
            required_options=[str(o) for o in (body.get("required_options") or [])],
        )
    return RecipeBook(recipes)
