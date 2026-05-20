# python/tests/test_zones.py
"""ZoneCatalog tests — load, resolve, rename, validate, round-trip."""

from __future__ import annotations

import tempfile
from pathlib import Path

import pytest

from lexicon import ZoneCatalog, ZoneEntry, ZonePriorName, load_zone_catalog


FIXTURE = Path(__file__).parent / "fixtures" / "zones-sample.yaml"


@pytest.fixture()
def zones() -> ZoneCatalog:
    return load_zone_catalog(FIXTURE)


def _write_yaml(text: str) -> Path:
    f = tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False)
    f.write(text)
    f.close()
    return Path(f.name)


# ── load + resolve ──────────────────────────────────────────────────────────

def test_load_reads_fixture(zones: ZoneCatalog) -> None:
    names = [e.current_name for e in zones.entries]
    assert names == ["admin", "lan", "iot", "wan"]
    assert zones.host == "example-gateway"


def test_resolve_by_current_name(zones: ZoneCatalog) -> None:
    e = zones.resolve("admin")
    assert e is not None
    assert e.type == "lan"
    assert e.notes == "Servers and management"


def test_resolve_by_prior_name(zones: ZoneCatalog) -> None:
    e = zones.resolve("home")
    assert e is not None
    assert e.current_name == "lan"


def test_resolve_miss_returns_none(zones: ZoneCatalog) -> None:
    assert zones.resolve("does-not-exist") is None
    assert zones.resolve("") is None


def test_names_lists_current(zones: ZoneCatalog) -> None:
    assert zones.names() == ["admin", "lan", "iot", "wan"]


# ── rename ──────────────────────────────────────────────────────────────────

def test_rename_records_prior_name(zones: ZoneCatalog) -> None:
    zones.rename("lan", "cameras", reason="VLAN renamed", today="2026-05-20")
    e = zones.resolve("cameras")
    assert e is not None
    # The original prior_name "home" plus the new "lan".
    names = [p.name for p in e.prior_names]
    assert names == ["home", "lan"]
    assert e.prior_names[-1].retired_on == "2026-05-20"
    assert e.prior_names[-1].reason == "VLAN renamed"


def test_rename_old_name_still_resolves(zones: ZoneCatalog) -> None:
    zones.rename("lan", "cameras")
    # Both prior names should still resolve to the same entry.
    assert zones.resolve("lan").current_name == "cameras"
    assert zones.resolve("home").current_name == "cameras"


def test_rename_to_same_name_is_noop(zones: ZoneCatalog) -> None:
    before = list(zones.resolve("lan").prior_names)
    zones.rename("lan", "lan")
    assert zones.resolve("lan").prior_names == before


def test_rename_unknown_raises(zones: ZoneCatalog) -> None:
    with pytest.raises(KeyError, match="unknown zone"):
        zones.rename("nope", "whatever")


def test_rename_empty_raises(zones: ZoneCatalog) -> None:
    with pytest.raises(ValueError, match="non-empty"):
        zones.rename("lan", "")


def test_rename_to_existing_live_name_raises(zones: ZoneCatalog) -> None:
    with pytest.raises(ValueError, match="duplicate current_name"):
        zones.rename("lan", "iot")  # iot is live


def test_rename_rolls_back_on_validation_failure(zones: ZoneCatalog) -> None:
    """Failed rename must leave the entry exactly as it was."""
    e_before = zones.resolve("lan")
    before_name = e_before.current_name
    before_priors = list(e_before.prior_names)
    with pytest.raises(ValueError):
        zones.rename("lan", "iot")  # collides
    after = zones.resolve("lan")  # should still be reachable by old name
    assert after.current_name == before_name
    assert after.prior_names == before_priors


# ── validation ──────────────────────────────────────────────────────────────

def test_duplicate_current_name_rejected() -> None:
    p = _write_yaml("""
version: 1
zones:
  - current_name: foo
  - current_name: foo
""")
    with pytest.raises(ValueError, match="duplicate current_name"):
        load_zone_catalog(p)


def test_prior_name_collision_with_live_rejected() -> None:
    p = _write_yaml("""
version: 1
zones:
  - current_name: foo
  - current_name: bar
    prior_names:
      - name: foo
""")
    with pytest.raises(ValueError, match="collides"):
        load_zone_catalog(p)


def test_empty_current_name_rejected() -> None:
    p = _write_yaml("""
version: 1
zones:
  - current_name: ""
""")
    with pytest.raises(ValueError, match="non-empty"):
        load_zone_catalog(p)


def test_unknown_type_rejected() -> None:
    p = _write_yaml("""
version: 1
zones:
  - current_name: foo
    type: bogus
""")
    with pytest.raises(ValueError, match="unknown zone type"):
        load_zone_catalog(p)


# ── save round-trip ─────────────────────────────────────────────────────────

def test_save_round_trip_preserves_data(zones: ZoneCatalog) -> None:
    zones.rename("lan", "cameras", reason="test")
    out = _write_yaml("")
    zones.save(out)
    reloaded = load_zone_catalog(out)
    e = reloaded.resolve("cameras")
    assert e is not None
    names = [p.name for p in e.prior_names]
    assert "home" in names  # original prior
    assert "lan" in names   # new prior from this rename
    assert reloaded.host == "example-gateway"


def test_save_preserves_top_level_comments() -> None:
    p = _write_yaml("""# Header comment that should survive a rename.
# Operator notes about the firewall setup.
version: 1
host: example-gw

zones:
  - current_name: lan
""")
    cat = load_zone_catalog(p)
    cat.rename("lan", "cameras")
    cat.save()
    text = p.read_text()
    assert "Header comment" in text
    assert "Operator notes" in text
    assert "cameras" in text
    assert "current_name: lan" not in text  # rename took effect


# ── add / remove ────────────────────────────────────────────────────────────

def test_add_new_entry(zones: ZoneCatalog) -> None:
    zones.add(ZoneEntry(current_name="quarantine", type="lan"))
    assert zones.resolve("quarantine").type == "lan"


def test_add_duplicate_name_raises(zones: ZoneCatalog) -> None:
    with pytest.raises(ValueError, match="duplicate current_name"):
        zones.add(ZoneEntry(current_name="admin"))


def test_add_rolls_back_on_validation_failure(zones: ZoneCatalog) -> None:
    before = zones.names()
    with pytest.raises(ValueError):
        zones.add(ZoneEntry(current_name="admin"))  # collides
    assert zones.names() == before


def test_remove_entry(zones: ZoneCatalog) -> None:
    zones.remove("wan")
    assert zones.resolve("wan") is None
    assert "wan" not in zones.names()


def test_remove_unknown_raises(zones: ZoneCatalog) -> None:
    with pytest.raises(KeyError):
        zones.remove("nope")
