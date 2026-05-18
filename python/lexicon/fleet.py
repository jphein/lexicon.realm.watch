# python/lexicon/fleet.py
"""Fleet catalog — stable per-node identity for realmwatch.

Parallels Catalog (projects) but with retired/replaced_by lifecycle.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from datetime import date
from pathlib import Path
from typing import Any

from ruamel.yaml import YAML
from ruamel.yaml.comments import CommentedMap, CommentedSeq


FLEET_ID_RE = re.compile(r"^(mac:[0-9a-f]{2}(:[0-9a-f]{2}){5}|fleet:[0-9a-f-]{36})$")
LIVE_STATUSES = ("tentative", "curated")
VALID_STATUSES = ("tentative", "curated", "retired")
MAX_RESOLVE_HOPS = 10


@dataclass
class FleetPriorName:
    name: str
    retired_on: str | None = None
    reason: str | None = None


@dataclass
class FleetEntry:
    fleet_id: str
    current_name: str
    prior_names: list[FleetPriorName] = field(default_factory=list)
    realm: str | None = None
    kind: str | None = None
    role: str | None = None
    vendor: str | None = None
    status: str = "curated"
    notes: str | None = None
    first_seen: str | None = None
    last_seen: str | None = None
    replaced_by: str | None = None
    retired_on: str | None = None
    retire_reason: str | None = None
    discovery_evidence: dict | None = None

    @classmethod
    def from_raw(cls, raw: dict) -> "FleetEntry":
        priors = [
            FleetPriorName(
                name=p["name"],
                retired_on=p.get("retired_on"),
                reason=p.get("reason"),
            )
            for p in raw.get("prior_names", []) or []
        ]
        return cls(
            fleet_id=raw["fleet_id"],
            current_name=raw["current_name"],
            prior_names=priors,
            realm=raw.get("realm"),
            kind=raw.get("kind"),
            role=raw.get("role"),
            vendor=raw.get("vendor"),
            status=raw.get("status", "curated"),
            notes=raw.get("notes"),
            first_seen=raw.get("first_seen"),
            last_seen=raw.get("last_seen"),
            replaced_by=raw.get("replaced_by"),
            retired_on=raw.get("retired_on"),
            retire_reason=raw.get("retire_reason"),
            discovery_evidence=raw.get("discovery_evidence"),
        )


class FleetCatalog:
    def __init__(self, entries: list[FleetEntry], source_path: Path | None = None, raw_root: Any = None):
        self.entries: list[FleetEntry] = entries
        self.source_path: Path | None = source_path
        self._raw_root = raw_root
        self._by_id: dict[str, FleetEntry] = {}
        self._by_name: dict[str, str] = {}
        self._reindex()

    def _reindex(self) -> None:
        self._by_id.clear()
        self._by_name.clear()
        for e in self.entries:
            self._by_id[e.fleet_id] = e
        for e in self.entries:
            if e.status in LIVE_STATUSES:
                self._by_name[e.current_name] = e.fleet_id
                for p in e.prior_names:
                    self._by_name.setdefault(p.name, e.fleet_id)
            elif e.status == "retired":
                self._by_name.setdefault(e.current_name, e.fleet_id)

    def resolve(self, name_or_id: str) -> FleetEntry | None:
        if not name_or_id:
            return None
        e = self._by_id.get(name_or_id)
        if e is None:
            fid = self._by_name.get(name_or_id)
            e = self._by_id.get(fid) if fid else None
        if e is None:
            return None
        hops = 0
        seen: set[str] = set()
        while e and e.status == "retired" and e.replaced_by:
            if e.fleet_id in seen or hops >= MAX_RESOLVE_HOPS:
                return None
            seen.add(e.fleet_id)
            hops += 1
            e = self._by_id.get(e.replaced_by)
        return e

    def rename(
        self,
        fleet_id: str,
        new_name: str,
        reason: str | None = None,
        today: str | None = None,
    ) -> None:
        e = self._by_id.get(fleet_id)
        if e is None:
            raise KeyError(f"unknown fleet_id: {fleet_id}")
        if e.status not in LIVE_STATUSES:
            raise ValueError(f"cannot rename {e.status} entry {fleet_id}")
        old = e.current_name
        if new_name == old:
            return
        e.prior_names.append(
            FleetPriorName(name=old, retired_on=today or _today(), reason=reason)
        )
        e.current_name = new_name
        _validate_entries(self.entries)
        self._reindex()

    def retire(
        self,
        old_fleet_id: str,
        new_entry: FleetEntry | None = None,
        retired_on: str | None = None,
        reason: str | None = None,
    ) -> None:
        e = self._by_id.get(old_fleet_id)
        if e is None:
            raise KeyError(f"unknown fleet_id: {old_fleet_id}")
        if new_entry is not None:
            if new_entry.fleet_id in self._by_id:
                raise ValueError(f"fleet_id already exists: {new_entry.fleet_id}")
            self.entries.append(new_entry)
            e.replaced_by = new_entry.fleet_id
        e.status = "retired"
        e.retired_on = retired_on or _today()
        if reason:
            e.retire_reason = reason
        _validate_entries(self.entries)
        self._reindex()

    def save(self, path: str | Path | None = None) -> None:
        target = Path(path) if path else self.source_path
        if target is None:
            raise ValueError("no path provided and no source_path on catalog")
        yaml = YAML(typ="rt")
        yaml.default_flow_style = False
        out = CommentedMap()
        out["version"] = 1
        out["nodes"] = CommentedSeq(self._to_raw(e) for e in self.entries)
        with target.open("w") as f:
            yaml.dump(out, f)

    @staticmethod
    def _to_raw(e: FleetEntry) -> CommentedMap:
        m = CommentedMap()
        m["fleet_id"] = e.fleet_id
        m["current_name"] = e.current_name
        m["prior_names"] = CommentedSeq(
            CommentedMap({"name": p.name, "retired_on": p.retired_on, "reason": p.reason})
            for p in e.prior_names
        )
        for k in ("realm", "kind", "role", "vendor", "notes",
                  "first_seen", "last_seen", "replaced_by",
                  "retired_on", "retire_reason"):
            v = getattr(e, k)
            if v is not None:
                m[k] = v
        m["status"] = e.status
        if e.discovery_evidence is not None:
            m["discovery_evidence"] = e.discovery_evidence
        return m


def _today() -> str:
    return date.today().isoformat()


def _validate_entries(entries: list[FleetEntry]) -> None:
    seen_ids: set[str] = set()
    live_names: dict[str, str] = {}
    for e in entries:
        if not FLEET_ID_RE.match(e.fleet_id):
            raise ValueError(f"invalid fleet_id: {e.fleet_id!r}")
        if e.fleet_id in seen_ids:
            raise ValueError(f"duplicate fleet_id: {e.fleet_id}")
        seen_ids.add(e.fleet_id)

        if e.status not in VALID_STATUSES:
            raise ValueError(f"unknown status {e.status!r} for {e.fleet_id}")

        if e.replaced_by and e.status != "retired":
            raise ValueError(
                f"replaced_by only valid for retired entries; "
                f"{e.fleet_id} has status={e.status}"
            )

        if e.status in LIVE_STATUSES:
            if e.current_name in live_names:
                raise ValueError(
                    f"duplicate current_name {e.current_name!r}: "
                    f"{live_names[e.current_name]} and {e.fleet_id}"
                )
            live_names[e.current_name] = e.fleet_id
            for p in e.prior_names:
                if p.name in live_names:
                    raise ValueError(
                        f"prior_name {p.name!r} on {e.fleet_id} "
                        f"collides with live entry {live_names[p.name]}"
                    )


def load_fleet_catalog(path: str | Path) -> FleetCatalog:
    p = Path(path)
    yaml = YAML(typ="rt")
    with p.open() as f:
        raw = yaml.load(f) or {}
    nodes_raw = raw.get("nodes", []) or []
    entries = [FleetEntry.from_raw(dict(n)) for n in nodes_raw]
    _validate_entries(entries)
    return FleetCatalog(entries=entries, source_path=p, raw_root=raw)
