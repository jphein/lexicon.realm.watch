# python/tests/test_fleet.py
"""FleetCatalog tests — load, resolve, mutate, round-trip."""

from __future__ import annotations

from pathlib import Path

import pytest

from lexicon import FleetCatalog, FleetEntry, load_fleet_catalog


FIXTURE = Path(__file__).parent / "fixtures" / "fleet-sample.yaml"


@pytest.fixture()
def fleet() -> FleetCatalog:
    return load_fleet_catalog(FIXTURE)


def test_load_fleet_reads_fixture(fleet: FleetCatalog) -> None:
    ids = [e.fleet_id for e in fleet.entries]
    assert "mac:78:48:59:a8:25:97" in ids
    assert "fleet:11111111-2222-3333-4444-555555555555" in ids
    assert len(fleet.entries) == 5


def test_resolve_by_fleet_id(fleet: FleetCatalog) -> None:
    e = fleet.resolve("mac:78:48:59:a8:25:97")
    assert e is not None
    assert e.current_name == "hp-switch"


def test_resolve_by_current_name(fleet: FleetCatalog) -> None:
    e = fleet.resolve("east-tree-trunk")
    assert e is not None
    assert e.fleet_id == "mac:b4:fb:e4:12:34:56"


def test_resolve_by_prior_name(fleet: FleetCatalog) -> None:
    e = fleet.resolve("hp-laserjet-m234")
    assert e is not None
    assert e.current_name == "glasswing-printer"


def test_resolve_walks_replaced_by_chain(fleet: FleetCatalog) -> None:
    # gst308t-office is retired, replaced_by -> east-tree-trunk
    e = fleet.resolve("gst308t-office")
    assert e is not None
    assert e.current_name == "east-tree-trunk"
    assert e.status == "curated"


def test_resolve_returns_none_on_miss(fleet: FleetCatalog) -> None:
    assert fleet.resolve("does-not-exist") is None


import tempfile


def _write_yaml(text: str) -> Path:
    f = tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False)
    f.write(text)
    f.close()
    return Path(f.name)


def test_invalid_fleet_id_rejected() -> None:
    p = _write_yaml("""
version: 1
nodes:
  - fleet_id: "not-a-valid-id"
    current_name: bad
""")
    with pytest.raises(ValueError, match="fleet_id"):
        load_fleet_catalog(p)


def test_duplicate_current_name_rejected() -> None:
    p = _write_yaml("""
version: 1
nodes:
  - fleet_id: "mac:aa:aa:aa:aa:aa:aa"
    current_name: dup
    status: curated
  - fleet_id: "mac:bb:bb:bb:bb:bb:bb"
    current_name: dup
    status: curated
""")
    with pytest.raises(ValueError, match="duplicate"):
        load_fleet_catalog(p)


def test_replaced_by_only_valid_when_retired() -> None:
    p = _write_yaml("""
version: 1
nodes:
  - fleet_id: "mac:aa:aa:aa:aa:aa:aa"
    current_name: live-with-replaced-by
    status: curated
    replaced_by: "mac:bb:bb:bb:bb:bb:bb"
""")
    with pytest.raises(ValueError, match="replaced_by"):
        load_fleet_catalog(p)


def test_status_must_be_known() -> None:
    p = _write_yaml("""
version: 1
nodes:
  - fleet_id: "mac:aa:aa:aa:aa:aa:aa"
    current_name: bad-status
    status: ghost
""")
    with pytest.raises(ValueError, match="status"):
        load_fleet_catalog(p)


def test_rename_appends_to_prior_names(fleet: FleetCatalog) -> None:
    fleet.rename("mac:78:48:59:a8:25:97", "iron-eye", reason="fantasy-renamed")
    e = fleet.resolve("iron-eye")
    assert e is not None
    assert e.current_name == "iron-eye"
    assert any(p.name == "hp-switch" for p in e.prior_names)
    assert fleet.resolve("hp-switch") is e


def test_retire_with_replacement(fleet: FleetCatalog) -> None:
    new_entry = FleetEntry(
        fleet_id="mac:99:99:99:99:99:99",
        current_name="iron-replacement",
        realm="signal",
        kind="switch",
        status="curated",
    )
    fleet.retire(
        "mac:78:48:59:a8:25:97",
        new_entry=new_entry,
        retired_on="2026-05-18",
        reason="swapped",
    )
    assert fleet.resolve("hp-switch").current_name == "iron-replacement"
    old = fleet._by_id["mac:78:48:59:a8:25:97"]
    assert old.status == "retired"
    assert old.replaced_by == "mac:99:99:99:99:99:99"


def test_save_round_trip(fleet: FleetCatalog, tmp_path: Path) -> None:
    fleet.rename("mac:78:48:59:a8:25:97", "iron-eye")
    out = tmp_path / "fleet.yaml"
    fleet.save(out)
    reloaded = load_fleet_catalog(out)
    assert reloaded.resolve("hp-switch").current_name == "iron-eye"
    assert reloaded.resolve("iron-eye") is not None


def test_category_and_ops_ip_fields(fleet: FleetCatalog, tmp_path: Path) -> None:
    """category and ops_ip are first-class fields for ops integration
    (drive scripts/lib/fleet.sh equivalents in downstream consumers)."""
    e = fleet.resolve("hp-switch")
    e.category = "switch_vendor"
    e.ops_ip = "10.0.6.103"
    out = tmp_path / "fleet.yaml"
    fleet.save(out)
    reloaded = load_fleet_catalog(out)
    r = reloaded.resolve("hp-switch")
    assert r.category == "switch_vendor"
    assert r.ops_ip == "10.0.6.103"


def test_category_and_ops_ip_optional(fleet: FleetCatalog) -> None:
    """Existing entries without the new fields still load (backward compat)."""
    e = fleet.resolve("east-tree-trunk")
    assert e.category is None
    assert e.ops_ip is None


def test_stewardship_fields_round_trip(fleet: FleetCatalog, tmp_path: Path) -> None:
    """mgmt_ip / location / os / contacts survive a save+load cycle.

    These absorb what catalog/hosts.yaml modelled before fleet.yaml became
    identity-of-record. This test exists because load() accepts ANY key while
    _to_raw() writes only an explicit tuple — so a field added to FleetEntry but
    forgotten in that tuple loads fine, then vanishes on the next save with no
    error. The discovery callback saves routinely, so "the next save" is minutes
    away, not release-cycle away. Assert the round trip, not just the load.
    """
    e = fleet.resolve("east-tree-trunk")
    e.mgmt_ip = "10.37.5.2"
    e.location = "east — Treelink office"
    e.os = "OpenWrt 25.12.x (realtek/rtl838x)"
    e.contacts = ["jp@jphein.com", "thomas@treelink.us"]

    out = tmp_path / "fleet.yaml"
    fleet.save(out)
    r = load_fleet_catalog(out).resolve("east-tree-trunk")

    assert r.mgmt_ip == "10.37.5.2"
    assert r.location == "east — Treelink office"
    assert r.os == "OpenWrt 25.12.x (realtek/rtl838x)"
    assert r.contacts == ["jp@jphein.com", "thomas@treelink.us"]


def test_stewardship_fields_optional(fleet: FleetCatalog) -> None:
    """Entries predating these fields still load (backward compat)."""
    e = fleet.resolve("hp-switch")
    assert e.mgmt_ip is None
    assert e.location is None
    assert e.os is None
    assert e.contacts is None


def test_unset_stewardship_fields_are_omitted_not_emitted(
    fleet: FleetCatalog, tmp_path: Path
) -> None:
    """An unset field must not appear in the output at all.

    contacts is a list, so defaulting it to field(default_factory=list) would be
    the natural choice — and would stamp `contacts: []` onto every entry in the
    catalog the first time the server saved. Default None + the None-skip in
    _to_raw keeps unrelated entries diff-clean. Guard that.
    """
    out = tmp_path / "fleet.yaml"
    fleet.save(out)
    text = out.read_text()
    for absent in ("contacts", "mgmt_ip", "location", "os:"):
        assert absent not in text, f"{absent!r} emitted for entries that never set it"

    # mgmt_ip distinct from ops_ip: setting one must not imply the other.
    fleet.resolve("hp-switch").mgmt_ip = "10.0.6.103"
    fleet.save(out)
    r = load_fleet_catalog(out).resolve("hp-switch")
    assert r.mgmt_ip == "10.0.6.103"
    assert r.ops_ip is None
