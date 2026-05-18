# python/tests/test_fleet.py
"""FleetCatalog tests — load, resolve, mutate, round-trip."""

from __future__ import annotations

from pathlib import Path

import pytest

from lexicon import FleetCatalog, load_fleet_catalog


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
