"""Vocabulary loader — mirrors ``go/vocabulary.go``.

A :class:`Vocabulary` is a flat dictionary of category → group → words. The
YAML's top-level keys (``realms``, ``adjectives``, ``nouns``, ``scientists``,
``creatures``) each map to a map of group name → ``{description, words}``.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
from typing import Iterable

from ruamel.yaml import YAML

__all__ = ["Vocabulary", "load_vocabulary_file", "load_vocabulary_dir"]


@dataclass
class Vocabulary:
    """Category → group → list of words."""

    categories: dict[str, dict[str, list[str]]] = field(default_factory=dict)

    def group(self, category: str, group: str) -> list[str] | None:
        """Return words for ``(category, group)``, or ``None`` if missing."""
        cat = self.categories.get(category)
        if cat is None:
            return None
        return cat.get(group)

    def has_group(self, category: str, group: str) -> bool:
        return self.group(category, group) is not None

    def categories_list(self) -> list[str]:
        return sorted(self.categories.keys())

    def groups(self, category: str) -> list[str]:
        cat = self.categories.get(category)
        if cat is None:
            return []
        return sorted(cat.keys())

    def add_group(self, category: str, group: str, words: Iterable[str]) -> None:
        self.categories.setdefault(category, {})[group] = list(words)

    def merge(self, other: "Vocabulary") -> None:
        for cat, groups in other.categories.items():
            target = self.categories.setdefault(cat, {})
            for group, words in groups.items():
                target[group] = list(words)


def _normalize_groups(raw: dict | None) -> dict[str, list[str]]:
    """Convert raw YAML group blocks into ``{group: [words]}``."""
    if not raw:
        return {}
    out: dict[str, list[str]] = {}
    for group_name, body in raw.items():
        if body is None:
            out[group_name] = []
            continue
        if isinstance(body, list):
            # Permissive: allow ``group: [words...]`` shorthand.
            out[group_name] = [str(w) for w in body]
            continue
        words = body.get("words") if isinstance(body, dict) else None
        out[group_name] = [str(w) for w in (words or [])]
    return out


def load_vocabulary_file(path: str | Path) -> Vocabulary:
    """Load a single ``vocabularies/*.yaml`` file."""
    yaml = YAML(typ="safe")
    with Path(path).open("r", encoding="utf-8") as fh:
        raw = yaml.load(fh) or {}
    if not isinstance(raw, dict):
        raise ValueError(f"{path}: expected top-level mapping, got {type(raw).__name__}")
    cats: dict[str, dict[str, list[str]]] = {}
    for category, groups in raw.items():
        cats[str(category)] = _normalize_groups(groups)
    return Vocabulary(categories=cats)


def load_vocabulary_dir(directory: str | Path) -> Vocabulary:
    """Load every ``*.yaml`` under ``directory`` (except ``recipes.yaml``)
    and merge into one :class:`Vocabulary`."""
    d = Path(directory)
    if not d.is_dir():
        raise FileNotFoundError(f"vocabulary directory not found: {d}")
    merged = Vocabulary()
    for path in sorted(d.glob("*.yaml")):
        if path.name == "recipes.yaml":
            continue
        merged.merge(load_vocabulary_file(path))
    return merged
