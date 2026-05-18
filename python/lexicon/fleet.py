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


def load_fleet_catalog(path: str | Path) -> FleetCatalog:
    p = Path(path)
    yaml = YAML(typ="rt")
    with p.open() as f:
        raw = yaml.load(f) or {}
    nodes_raw = raw.get("nodes", []) or []
    entries = [FleetEntry.from_raw(dict(n)) for n in nodes_raw]
    return FleetCatalog(entries=entries, source_path=p, raw_root=raw)
