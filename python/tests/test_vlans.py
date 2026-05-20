# python/tests/test_vlans.py
"""VLANCatalog tests — load, resolve, rename, validate, round-trip."""

from __future__ import annotations

import tempfile
from pathlib import Path

import pytest

from lexicon import VLANCatalog, VLANEntry, VLANPriorName, load_vlan_catalog


FIXTURE = Path(__file__).parent / "fixtures" / "vlans-sample.yaml"


@pytest.fixture()
def vlans() -> VLANCatalog:
    return load_vlan_catalog(FIXTURE)


def _write_yaml(text: str) -> Path:
    f = tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False)
    f.write(text)
    f.close()
    return Path(f.name)


# ── load + resolve ──────────────────────────────────────────────────────────

def test_load_reads_fixture(vlans: VLANCatalog) -> None:
    ids = [e.vlan_id for e in vlans.entries]
    assert ids == [3, 6, 10, 11, 38]
    assert vlans.wan_zones == ["wan", "wanguard"]


def test_resolve_by_int_id(vlans: VLANCatalog) -> None:
    e = vlans.resolve(10)
    assert e is not None
    assert e.label == "Cameras"


def test_resolve_by_str_id(vlans: VLANCatalog) -> None:
    e = vlans.resolve("10")
    assert e is not None
    assert e.label == "Cameras"


def test_resolve_by_current_label(vlans: VLANCatalog) -> None:
    e = vlans.resolve("Admin")
    assert e is not None
    assert e.vlan_id == 6


def test_resolve_by_prior_label(vlans: VLANCatalog) -> None:
    e = vlans.resolve("IoT")
    assert e is not None
    assert e.vlan_id == 10
    assert e.label == "Cameras"


def test_resolve_miss_returns_none(vlans: VLANCatalog) -> None:
    assert vlans.resolve("does-not-exist") is None
    assert vlans.resolve(9999) is None
    assert vlans.resolve("") is None


def test_zone_to_vlan_includes_wan_nulls(vlans: VLANCatalog) -> None:
    zmap = vlans.zone_to_vlan()
    assert zmap["admin"] == 6
    assert zmap["lan"] == 10
    assert zmap["family"] == 11
    assert zmap["wan"] is None
    assert zmap["wanguard"] is None


# ── rename ──────────────────────────────────────────────────────────────────

def test_rename_records_prior_name(vlans: VLANCatalog) -> None:
    vlans.rename(6, "Servers", reason="more accurate label", today="2026-06-01")
    e = vlans.resolve(6)
    assert e.label == "Servers"
    assert len(e.prior_names) == 1
    assert e.prior_names[0].name == "Admin"
    assert e.prior_names[0].retired_on == "2026-06-01"
    assert e.prior_names[0].reason == "more accurate label"


def test_rename_to_same_label_is_noop(vlans: VLANCatalog) -> None:
    vlans.rename(6, "Admin")
    e = vlans.resolve(6)
    assert e.prior_names == []


def test_rename_old_label_still_resolves(vlans: VLANCatalog) -> None:
    vlans.rename(6, "Servers")
    # The old label "Admin" should still resolve to VLAN 6 via prior_names.
    e = vlans.resolve("Admin")
    assert e is not None
    assert e.vlan_id == 6


def test_rename_unknown_id_raises(vlans: VLANCatalog) -> None:
    with pytest.raises(KeyError, match="unknown VLAN id"):
        vlans.rename(9999, "Whatever")


def test_rename_empty_label_raises(vlans: VLANCatalog) -> None:
    with pytest.raises(ValueError, match="non-empty"):
        vlans.rename(6, "")


def test_rename_to_existing_live_label_raises(vlans: VLANCatalog) -> None:
    with pytest.raises(ValueError, match="duplicate label"):
        vlans.rename(6, "Cameras")  # already used by VLAN 10


def test_rename_rolls_back_on_validation_failure(vlans: VLANCatalog) -> None:
    """A failed rename must leave the catalog identical to its pre-call state."""
    before_label = vlans.resolve(6).label
    before_priors = list(vlans.resolve(6).prior_names)
    with pytest.raises(ValueError):
        vlans.rename(6, "Cameras")  # collides with VLAN 10
    after = vlans.resolve(6)
    assert after.label == before_label, "label leaked despite validation failure"
    assert after.prior_names == before_priors, "prior_names leaked despite validation failure"
    # The "Cameras" label must still resolve to VLAN 10, not 6.
    assert vlans.resolve("Cameras").vlan_id == 10


def test_add_rolls_back_on_validation_failure(vlans: VLANCatalog) -> None:
    """A failed add must not leave the entry in self.entries."""
    before_ids = {e.vlan_id for e in vlans.entries}
    bad = VLANEntry(vlan_id=99, label="Admin", type="lan", status="active")  # label collides
    with pytest.raises(ValueError, match="duplicate label"):
        vlans.add(bad)
    assert {e.vlan_id for e in vlans.entries} == before_ids, "bad entry was retained"
    assert vlans.resolve(99) is None


# ── validation ──────────────────────────────────────────────────────────────

def test_invalid_vlan_id_rejected() -> None:
    p = _write_yaml("""
version: 1
vlans:
  9999:
    label: Bad
""")
    with pytest.raises(ValueError, match="invalid vlan_id"):
        load_vlan_catalog(p)


def test_duplicate_label_among_active_rejected() -> None:
    p = _write_yaml("""
version: 1
vlans:
  10:
    label: Same
    status: active
  20:
    label: Same
    status: active
""")
    with pytest.raises(ValueError, match="duplicate label"):
        load_vlan_catalog(p)


def test_duplicate_zone_rejected() -> None:
    p = _write_yaml("""
version: 1
vlans:
  10:
    label: First
    zone: admin
  20:
    label: Second
    zone: admin
""")
    with pytest.raises(ValueError, match="duplicate zone"):
        load_vlan_catalog(p)


def test_unknown_type_rejected() -> None:
    p = _write_yaml("""
version: 1
vlans:
  10:
    label: Bad
    type: bogus
""")
    with pytest.raises(ValueError, match="unknown type"):
        load_vlan_catalog(p)


def test_unknown_status_rejected() -> None:
    p = _write_yaml("""
version: 1
vlans:
  10:
    label: Bad
    status: maybe
""")
    with pytest.raises(ValueError, match="unknown status"):
        load_vlan_catalog(p)


def test_wan_zone_overlap_with_entry_zone_rejected() -> None:
    """wan_zones must not name a zone that's also declared by a live VLAN entry."""
    p = _write_yaml("""
version: 1
wan_zones:
  - admin
vlans:
  6:
    label: Admin
    status: active
    zone: admin
""")
    with pytest.raises(ValueError, match="wan_zone .* collides"):
        load_vlan_catalog(p)


def test_zone_to_vlan_forces_wan_zones_to_none(vlans: VLANCatalog) -> None:
    """wan_zones map to None unconditionally, even if a key would otherwise resolve."""
    zmap = vlans.zone_to_vlan()
    for z in vlans.wan_zones:
        assert zmap[z] is None, f"wan_zone {z!r} should map to None"


def test_inactive_label_collisions_allowed() -> None:
    """Deprecated/inactive VLANs can share labels with active ones (they don't resolve)."""
    p = _write_yaml("""
version: 1
vlans:
  10:
    label: Cameras
    status: active
  11:
    label: Cameras
    status: deprecated
""")
    cat = load_vlan_catalog(p)
    assert cat.resolve("Cameras").vlan_id == 10


# ── save round-trip ─────────────────────────────────────────────────────────

def test_save_preserves_top_level_comments() -> None:
    """Header / section comments in the source file survive a rename + save."""
    p = _write_yaml("""# Top-of-file operator comment — should survive a save().
# Multi-line context worth keeping.
version: 1

# WAN zones section marker — survives too.
wan_zones:
  - wan

vlans:
  10:
    label: Cameras
    status: active
""")
    cat = load_vlan_catalog(p)
    cat.rename(10, "Watchers")
    cat.save()
    text = p.read_text()
    assert "Top-of-file operator comment" in text
    assert "WAN zones section marker" in text
    assert "Watchers" in text
    assert "label: Cameras" not in text  # the rename took effect


def test_save_round_trip_preserves_data(vlans: VLANCatalog) -> None:
    vlans.rename(10, "Watchers", reason="fantasy flavor")
    out = _write_yaml("")
    vlans.save(out)
    reloaded = load_vlan_catalog(out)
    e = reloaded.resolve(10)
    assert e.label == "Watchers"
    # Both prior names should survive — the fixture's "IoT" and the new "Cameras".
    names = [p.name for p in e.prior_names]
    assert "IoT" in names
    assert "Cameras" in names


def test_save_preserves_wan_zones(vlans: VLANCatalog) -> None:
    out = _write_yaml("")
    vlans.save(out)
    reloaded = load_vlan_catalog(out)
    assert reloaded.wan_zones == ["wan", "wanguard"]


# ── add / remove ────────────────────────────────────────────────────────────

def test_add_new_entry(vlans: VLANCatalog) -> None:
    vlans.add(VLANEntry(vlan_id=42, label="Quarantine", type="lan", status="active"))
    assert vlans.resolve(42).label == "Quarantine"


def test_add_duplicate_id_raises(vlans: VLANCatalog) -> None:
    with pytest.raises(ValueError, match="already exists"):
        vlans.add(VLANEntry(vlan_id=6, label="Conflict"))


def test_remove_entry(vlans: VLANCatalog) -> None:
    vlans.remove(38)
    assert vlans.resolve(38) is None


def test_remove_unknown_raises(vlans: VLANCatalog) -> None:
    with pytest.raises(KeyError):
        vlans.remove(9999)
