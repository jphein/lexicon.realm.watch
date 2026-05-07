"""Catalog tests — load, query, mutate, round-trip."""

from __future__ import annotations

import shutil
from pathlib import Path

import pytest

from lexicon import Catalog, ClaimOpts, Project, load_catalog


@pytest.fixture()
def catalog(catalog_test_path: Path) -> Catalog:
    return load_catalog(catalog_test_path)


def test_load_catalog_reads_test_fixture(catalog: Catalog) -> None:
    ids = [p.id for p in catalog.projects]
    assert "clock" in ids
    assert "dreamspace" in ids
    assert "realmwatch" in ids


def test_resolve_by_id(catalog: Catalog) -> None:
    proj = catalog.resolve("clock")
    assert proj is not None
    assert proj.current_name == "clock.realm.watch"


def test_resolve_by_current_name(catalog: Catalog) -> None:
    proj = catalog.resolve("dreamscape.realm.watch")
    assert proj is not None
    assert proj.id == "dreamspace"


def test_resolve_by_prior_name(catalog: Catalog) -> None:
    proj = catalog.resolve("dreamscape")
    assert proj is not None
    assert proj.id == "dreamspace"
    assert proj.current_name == "dreamscape.realm.watch"


def test_resolve_unknown(catalog: Catalog) -> None:
    assert catalog.resolve("never-existed") is None


def test_by_realm(catalog: Catalog) -> None:
    void = catalog.by_realm("void")
    assert any(p.id == "realmwatch" for p in void)
    oracle = catalog.by_realm("oracle")
    assert any(p.id == "dreamspace" for p in oracle)


def test_by_kind(catalog: Catalog) -> None:
    services = catalog.by_kind("service")
    assert any(p.id == "realmwatch" for p in services)


def test_by_status(catalog: Catalog) -> None:
    local_only = catalog.by_status("local-only")
    assert all(p.status == "local-only" for p in local_only)
    assert any(p.id == "realmwatch" for p in local_only)


def test_claim_new_project(catalog: Catalog) -> None:
    catalog.claim(
        "freshly-claimed",
        ClaimOpts(
            kind="tool",
            realm="signal",
            description="Fresh entry from claim()",
            created="2026-05-07",
        ),
    )
    proj = catalog.resolve("freshly-claimed")
    assert proj is not None
    assert proj.id == "freshly-claimed"
    assert proj.current_name == "freshly-claimed"
    assert proj.kind == "tool"
    assert proj.realm == "signal"
    assert proj.status == "active"  # default
    assert proj.prior_names == []


def test_claim_rename_existing(catalog: Catalog) -> None:
    catalog.claim(
        "realm-watch",
        ClaimOpts(
            renames_of="realmwatch",
            reason="moved into realm.watch family",
            retired="2026-05-07",
        ),
    )
    proj = catalog.resolve("realmwatch")
    assert proj is not None
    assert proj.current_name == "realm-watch"
    assert any(pn.name == "realmwatch" for pn in proj.prior_names)
    # Old name still resolves to same project via prior_names.
    again = catalog.resolve("realmwatch")
    assert again is proj


def test_claim_rejects_collision(catalog: Catalog) -> None:
    with pytest.raises(ValueError, match="already taken"):
        catalog.claim(
            "clock.realm.watch",
            ClaimOpts(
                kind="tool",
                realm="signal",
                description="Should fail — name in use.",
            ),
        )


def test_claim_rejects_collision_with_prior_name(catalog: Catalog) -> None:
    with pytest.raises(ValueError, match="already taken"):
        catalog.claim(
            "dreamscape",
            ClaimOpts(kind="tool", realm="oracle", description="Hits prior name"),
        )


def test_claim_empty_name_raises(catalog: Catalog) -> None:
    with pytest.raises(ValueError, match="empty"):
        catalog.claim("", ClaimOpts(kind="tool", realm="oracle"))


def test_claim_renames_of_unknown(catalog: Catalog) -> None:
    with pytest.raises(ValueError, match="not found"):
        catalog.claim(
            "totally-new",
            ClaimOpts(renames_of="never-existed", reason="oops"),
        )


def test_save_and_round_trip(
    catalog_test_path: Path, tmp_path: Path
) -> None:
    """Save → reload should produce a structurally equivalent catalog."""
    work = tmp_path / "projects.yaml"
    shutil.copyfile(catalog_test_path, work)

    cat = load_catalog(work)
    cat.claim(
        "ravenforge",
        ClaimOpts(
            kind="tool",
            realm="fantasy",
            description="Round-trip check.",
            created="2026-05-07",
        ),
    )
    cat.save()

    reloaded = load_catalog(work)
    assert reloaded.resolve("ravenforge") is not None
    # Existing entries must still be there with the same data.
    clock = reloaded.resolve("clock")
    assert clock is not None
    assert clock.current_name == "clock.realm.watch"
    assert clock.realm == "signal"
    dreamspace = reloaded.resolve("dreamspace")
    assert dreamspace is not None
    assert len(dreamspace.prior_names) == 2


def test_save_preserves_top_level_comments(catalog_test_path: Path, tmp_path: Path) -> None:
    """Comments at the top of the file (e.g., the registry header) survive a save.

    The live ``catalog/projects.yaml`` opens with comment lines; if save() ever
    drops those, the realm registry loses its provenance header.
    """
    src = Path(__file__).resolve().parent.parent.parent / "catalog" / "projects.yaml"
    if not src.exists():
        pytest.skip("live catalog not present in this checkout")
    work = tmp_path / "projects.yaml"
    shutil.copyfile(src, work)

    original = work.read_text(encoding="utf-8")
    # Mutate something benign and save.
    cat = load_catalog(work)
    cat.claim(
        "preserves-comments-test",
        ClaimOpts(
            kind="tool",
            realm="signal",
            description="Comment-preservation probe.",
            created="2026-05-07",
        ),
    )
    cat.save()
    saved = work.read_text(encoding="utf-8")
    # Find a leading comment line in the original and assert it survives.
    leading_comment = None
    for line in original.splitlines():
        if line.startswith("#"):
            leading_comment = line
            break
    if leading_comment is not None:
        assert leading_comment in saved
