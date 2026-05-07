#!/usr/bin/env python3
"""SessionStart hook for lexicon.realm.watch.

Prints a one-line banner summarising the catalog so a Claude Code session
opens with current state in context. No-ops cleanly if the catalog is
missing, unparseable, or the YAML library is unavailable.

Discovery order for catalog/projects.yaml:
  1. $LEXICON_CATALOG (matches the Go CLI's env override)
  2. $CLAUDE_PROJECT_DIR/catalog/projects.yaml (Claude Code sets this)
  3. catalog/projects.yaml relative to this script's repo root
"""

from __future__ import annotations

import os
import sys
from pathlib import Path


def _candidate_paths() -> list[Path]:
    paths: list[Path] = []
    if env := os.environ.get("LEXICON_CATALOG"):
        paths.append(Path(env))
    if proj := os.environ.get("CLAUDE_PROJECT_DIR"):
        paths.append(Path(proj) / "catalog" / "projects.yaml")
    paths.append(Path(__file__).resolve().parent.parent / "catalog" / "projects.yaml")
    return paths


def _load_projects(path: Path) -> list[dict] | None:
    try:
        import yaml  # type: ignore[import-untyped]
    except ImportError:
        return None
    try:
        with path.open("r", encoding="utf-8") as fh:
            data = yaml.safe_load(fh)
    except (OSError, yaml.YAMLError):
        return None
    if not isinstance(data, dict):
        return None
    projects = data.get("projects")
    return projects if isinstance(projects, list) else None


def _format_banner(projects: list[dict]) -> str:
    realms = {p["realm"] for p in projects if isinstance(p, dict) and p.get("realm")}
    prior = sum(
        len(p["prior_names"])
        for p in projects
        if isinstance(p, dict) and isinstance(p.get("prior_names"), list)
    )
    n_proj = len(projects)
    n_realms = len(realms)
    return (
        f"lexicon — {n_proj} project{'s' if n_proj != 1 else ''} "
        f"in {n_realms} realm{'s' if n_realms != 1 else ''}; "
        f"{prior} prior name{'s' if prior != 1 else ''} recorded"
    )


def main() -> int:
    for path in _candidate_paths():
        if not path.is_file():
            continue
        projects = _load_projects(path)
        if projects is None:
            return 0
        print(_format_banner(projects))
        return 0
    return 0


if __name__ == "__main__":
    sys.exit(main())
