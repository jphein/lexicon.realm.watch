# python/lexicon/vlans.py
"""VLAN catalog — stable per-VLAN identity for realmwatch.

Parallels FleetCatalog but keyed on a numeric VLAN ID (3..4094) rather than a
MAC- or UUID-based fleet_id. Unlike fleet entries, VLAN IDs don't get retired
and replaced — a VLAN's *label* changes, the ID stays. So there's no
retire/replaced_by lifecycle here, just rename + add + remove.

Per-entry shape mirrors the operator-curated yaml that realmwatch already
ships (firewall_parser.VLANS). The `zone:` key encodes fw4 zone-name
mismatches (some realms have a zone literally named "lan" that carries IoT
traffic); the parser consumes it via ZONE_VLAN.

YAML round-trips via ruamel.yaml-typ=rt so operator comments and ordering
survive `.save()`.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import date
from pathlib import Path
from typing import Any

from ruamel.yaml import YAML
from ruamel.yaml.comments import CommentedMap, CommentedSeq


VALID_TYPES = ("wan", "lan", "reserved")
VALID_STATUSES = ("active", "standby", "planned", "reserved", "deprecated", "inactive")
LIVE_STATUSES = ("active", "standby", "planned")
MIN_VLAN_ID = 1
MAX_VLAN_ID = 4094


@dataclass
class VLANPriorName:
    name: str
    retired_on: str | None = None
    reason: str | None = None


@dataclass
class VLANEntry:
    vlan_id: int
    label: str
    type: str = "lan"
    status: str = "active"
    desc: str = ""
    icon: str = ""
    zone: str | None = None
    prior_names: list[VLANPriorName] = field(default_factory=list)

    @classmethod
    def from_raw(cls, vlan_id: int, raw: dict) -> "VLANEntry":
        priors = [
            VLANPriorName(
                name=p["name"],
                retired_on=p.get("retired_on"),
                reason=p.get("reason"),
            )
            for p in (raw.get("prior_names") or [])
        ]
        return cls(
            vlan_id=vlan_id,
            label=raw.get("label", f"VLAN {vlan_id}"),
            type=raw.get("type", "lan"),
            status=raw.get("status", "active"),
            desc=raw.get("desc", ""),
            icon=raw.get("icon", ""),
            zone=raw.get("zone"),
            prior_names=priors,
        )


class VLANCatalog:
    def __init__(
        self,
        entries: list[VLANEntry],
        wan_zones: list[str] | None = None,
        source_path: Path | None = None,
        raw_root: Any = None,
    ):
        self.entries: list[VLANEntry] = entries
        self.wan_zones: list[str] = list(wan_zones or [])
        self.source_path: Path | None = source_path
        self._raw_root = raw_root
        self._by_id: dict[int, VLANEntry] = {}
        self._by_label: dict[str, int] = {}
        self._by_zone: dict[str, int] = {}
        self._reindex()

    def _reindex(self) -> None:
        self._by_id.clear()
        self._by_label.clear()
        self._by_zone.clear()
        for e in self.entries:
            self._by_id[e.vlan_id] = e
            if e.zone:
                self._by_zone[e.zone] = e.vlan_id
            if e.status in LIVE_STATUSES:
                self._by_label[e.label] = e.vlan_id
                for p in e.prior_names:
                    # Don't shadow a live label with a prior name.
                    self._by_label.setdefault(p.name, e.vlan_id)

    def resolve(self, name_or_id: int | str) -> VLANEntry | None:
        """Look up by VLAN ID (int) or by current/prior label (str)."""
        if isinstance(name_or_id, int):
            return self._by_id.get(name_or_id)
        if name_or_id is None or name_or_id == "":
            return None
        # Try numeric string first.
        try:
            return self._by_id.get(int(name_or_id))
        except (TypeError, ValueError):
            pass
        vid = self._by_label.get(name_or_id)
        return self._by_id.get(vid) if vid is not None else None

    def zone_to_vlan(self) -> dict[str, int | None]:
        """fw4 zone-name → VLAN ID. WAN zones map to None."""
        out: dict[str, int | None] = dict(self._by_zone)
        for z in self.wan_zones:
            out.setdefault(z, None)
        return out

    def rename(
        self,
        vlan_id: int,
        new_label: str,
        reason: str | None = None,
        today: str | None = None,
    ) -> None:
        """Rename a VLAN's label; preserves old label in prior_names."""
        e = self._by_id.get(vlan_id)
        if e is None:
            raise KeyError(f"unknown VLAN id: {vlan_id}")
        if e.status not in LIVE_STATUSES:
            raise ValueError(f"cannot rename {e.status} VLAN {vlan_id}")
        if not new_label:
            raise ValueError("new_label must be non-empty")
        if new_label == e.label:
            return
        e.prior_names.append(
            VLANPriorName(name=e.label, retired_on=today or _today(), reason=reason)
        )
        e.label = new_label
        _validate_entries(self.entries)
        self._reindex()

    def add(self, entry: VLANEntry) -> None:
        if entry.vlan_id in self._by_id:
            raise ValueError(f"VLAN {entry.vlan_id} already exists")
        self.entries.append(entry)
        _validate_entries(self.entries)
        self._reindex()

    def remove(self, vlan_id: int) -> None:
        if vlan_id not in self._by_id:
            raise KeyError(f"unknown VLAN id: {vlan_id}")
        self.entries = [e for e in self.entries if e.vlan_id != vlan_id]
        self._reindex()

    def save(self, path: str | Path | None = None) -> None:
        target = Path(path) if path else self.source_path
        if target is None:
            raise ValueError("no path provided and no source_path on catalog")
        yaml = YAML(typ="rt")
        yaml.default_flow_style = False
        out = CommentedMap()
        out["version"] = 1
        if self.wan_zones:
            out["wan_zones"] = CommentedSeq(self.wan_zones)
        vlans_map = CommentedMap()
        for e in sorted(self.entries, key=lambda x: x.vlan_id):
            vlans_map[e.vlan_id] = self._to_raw(e)
        out["vlans"] = vlans_map
        with target.open("w") as f:
            yaml.dump(out, f)

    @staticmethod
    def _to_raw(e: VLANEntry) -> CommentedMap:
        m = CommentedMap()
        m["label"] = e.label
        m["type"] = e.type
        m["status"] = e.status
        if e.desc:
            m["desc"] = e.desc
        if e.icon:
            m["icon"] = e.icon
        if e.zone:
            m["zone"] = e.zone
        if e.prior_names:
            m["prior_names"] = CommentedSeq(
                CommentedMap(
                    {"name": p.name, "retired_on": p.retired_on, "reason": p.reason}
                )
                for p in e.prior_names
            )
        return m


def _today() -> str:
    return date.today().isoformat()


def _validate_entries(entries: list[VLANEntry]) -> None:
    seen_ids: set[int] = set()
    live_labels: dict[str, int] = {}
    live_zones: dict[str, int] = {}
    for e in entries:
        if not isinstance(e.vlan_id, int) or not (MIN_VLAN_ID <= e.vlan_id <= MAX_VLAN_ID):
            raise ValueError(f"invalid vlan_id: {e.vlan_id!r} (must be int in [{MIN_VLAN_ID}, {MAX_VLAN_ID}])")
        if e.vlan_id in seen_ids:
            raise ValueError(f"duplicate vlan_id: {e.vlan_id}")
        seen_ids.add(e.vlan_id)

        if e.type not in VALID_TYPES:
            raise ValueError(f"unknown type {e.type!r} for VLAN {e.vlan_id}")
        if e.status not in VALID_STATUSES:
            raise ValueError(f"unknown status {e.status!r} for VLAN {e.vlan_id}")
        if not e.label:
            raise ValueError(f"VLAN {e.vlan_id} must have a non-empty label")

        if e.status in LIVE_STATUSES:
            if e.label in live_labels:
                raise ValueError(
                    f"duplicate label {e.label!r}: "
                    f"VLAN {live_labels[e.label]} and VLAN {e.vlan_id}"
                )
            live_labels[e.label] = e.vlan_id
            for p in e.prior_names:
                # A prior_name on a different live entry is a real collision.
                # Matching this same entry's current label is fine — natural
                # after a rename round-trip.
                owner = live_labels.get(p.name)
                if owner is not None and owner != e.vlan_id:
                    raise ValueError(
                        f"prior_name {p.name!r} on VLAN {e.vlan_id} "
                        f"collides with live VLAN {owner}"
                    )
            if e.zone:
                if e.zone in live_zones:
                    raise ValueError(
                        f"duplicate zone {e.zone!r}: "
                        f"VLAN {live_zones[e.zone]} and VLAN {e.vlan_id}"
                    )
                live_zones[e.zone] = e.vlan_id


def load_vlan_catalog(path: str | Path) -> VLANCatalog:
    """Load a VLANCatalog from a yaml file. Validates before returning."""
    p = Path(path)
    yaml = YAML(typ="rt")
    with p.open() as f:
        raw = yaml.load(f) or {}
    vlans_raw = raw.get("vlans") or {}
    entries: list[VLANEntry] = []
    for vid, entry_raw in vlans_raw.items():
        try:
            vid_int = int(vid)
        except (TypeError, ValueError):
            raise ValueError(f"non-integer VLAN key in {p}: {vid!r}")
        entries.append(VLANEntry.from_raw(vid_int, dict(entry_raw or {})))
    wan_zones = list(raw.get("wan_zones") or [])
    _validate_entries(entries)
    return VLANCatalog(entries=entries, wan_zones=wan_zones, source_path=p, raw_root=raw)
