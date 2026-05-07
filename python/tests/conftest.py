"""Shared pytest fixtures.

The Python package lives at ``<repo>/python``; cross-language fixtures live
at ``<repo>/tests/fixtures``. This conftest exposes the repo root and the
fixture/vocabulary/catalog paths so tests don't have to recompute them.
"""

from __future__ import annotations

from pathlib import Path

import pytest


@pytest.fixture(scope="session")
def repo_root() -> Path:
    # python/tests/conftest.py → python/tests → python → repo
    return Path(__file__).resolve().parent.parent.parent


@pytest.fixture(scope="session")
def fixtures_dir(repo_root: Path) -> Path:
    return repo_root / "tests" / "fixtures"


@pytest.fixture(scope="session")
def vocabularies_dir(repo_root: Path) -> Path:
    return repo_root / "vocabularies"


@pytest.fixture(scope="session")
def vocabularies_test_path(fixtures_dir: Path) -> Path:
    return fixtures_dir / "vocabularies-test.yaml"


@pytest.fixture(scope="session")
def catalog_test_path(fixtures_dir: Path) -> Path:
    return fixtures_dir / "catalog-test.yaml"


@pytest.fixture(scope="session")
def seeded_recipes_path(fixtures_dir: Path) -> Path:
    return fixtures_dir / "seeded-recipes.json"


@pytest.fixture(scope="session")
def recipes_path(vocabularies_dir: Path) -> Path:
    return vocabularies_dir / "recipes.yaml"
