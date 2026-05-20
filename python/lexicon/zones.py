# python/lexicon/zones.py
"""Firewall-zone catalog — stable identity for fw4/nftables zone names.

A zone is what fw4 (OpenWrt) or any nftables wrapper calls a named slice of
the firewall — `admin`, `lan`, `family`, `wan`, etc. The name is referenced
by every forwarding rule, redirect, accept_to_*/reject_from_* chain. When
operators want to rename a zone (e.g. to match a renamed VLAN), all those
references must move with it, and the prior name should remain resolvable
in tooling so audit trails and dashboards don't suddenly forget which
zone an old log line referred to.

This catalog is the identity layer for that workflow. The actual fw4 state
lives on the firewall device — realmwatch (or a downstream tool) is
responsible for the SSH + uci mutation + fw4 reload side of the rename. The
catalog just tracks current_name and prior_names per zone, scoped to a host
(one firewall device per yaml file).

Schema mirrors FleetCatalog/VLANCatalog: yaml top-level has `version`,
`host`, `zones`. Each zone has current_name, prior_names list, optional
type (lan/wan/forwarding), optional notes.

YAML round-trips via ruamel.yaml typ=rt; .save() mutates self._raw_root in
place so top-level operator comments survive. Per-entry sub-maps are
rebuilt, so per-zone inline comments are not preserved across a rename.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import date
from pathlib import Path
from typing import Any

from ruamel.yaml import YAML
from ruamel.yaml.comments import CommentedMap, CommentedSeq


VALID_TYPES = ("lan", "wan", "forwarding", None)  # type is optional


@dataclass
class ZonePriorName:
    name: str
    retired_on: str | None = None
    reason: str | None = None


@dataclass
class ZoneEntry:
    current_name: str
    prior_names: list[ZonePriorName] = field(default_factory=list)
    type: str | None = None
    notes: str | None = None

    @classmethod
    def from_raw(cls, raw: dict) -> "ZoneEntry":
        priors = [
            ZonePriorName(
                name=p["name"],
                retired_on=p.get("retired_on"),
                reason=p.get("reason"),
            )
            for p in (raw.get("prior_names") or [])
        ]
        return cls(
            current_name=raw["current_name"],
            prior_names=priors,
            type=raw.get("type"),
            notes=raw.get("notes"),
        )


class ZoneCatalog:
    def __init__(
        self,
        entries: list[ZoneEntry],
        host: str | None = None,
        source_path: Path | None = None,
        raw_root: Any = None,
    ):
        self.entries: list[ZoneEntry] = entries
        self.host: str | None = host
        self.source_path: Path | None = source_path
        self._raw_root = raw_root
        self._by_name: dict[str, ZoneEntry] = {}
        self._reindex()

    def _reindex(self) -> None:
        self._by_name.clear()
        # Two passes: first live current_names (they win), then prior_names.
        for e in self.entries:
            self._by_name[e.current_name] = e
        for e in self.entries:
            for p in e.prior_names:
                # Don't shadow a live current_name with a prior name from
                # another entry — current names always win.
                self._by_name.setdefault(p.name, e)

    def resolve(self, name: str) -> ZoneEntry | None:
        """Look up by current name or by any prior name. None if not found."""
        if not name:
            return None
        return self._by_name.get(name)

    def names(self) -> list[str]:
        """Return current names of all entries, in insertion order."""
        return [e.current_name for e in self.entries]

    def rename(
        self,
        old_name: str,
        new_name: str,
        reason: str | None = None,
        today: str | None = None,
    ) -> None:
        """Rename a zone. old_name must match an entry's current_name (not a
        prior name — operators should always be addressing the live entry).

        Transactional: validates the resulting state before committing the
        change; on failure the entry is restored exactly.
        """
        # O(1) via _by_name; verify the hit is the LIVE entry, not just a
        # prior-name shadow (which would mean addressing the wrong VLAN).
        e = self._by_name.get(old_name)
        if e is None or e.current_name != old_name:
            raise KeyError(f"unknown zone: {old_name!r}")
        if not new_name:
            raise ValueError("new_name must be non-empty")
        if new_name == old_name:
            return
        old_priors = list(e.prior_names)
        e.prior_names.append(
            ZonePriorName(name=old_name, retired_on=today or _today(), reason=reason)
        )
        e.current_name = new_name
        try:
            _validate_entries(self.entries)
        except Exception:
            e.current_name = old_name
            e.prior_names = old_priors
            raise
        self._reindex()

    def add(self, entry: ZoneEntry) -> None:
        """Add a new zone. Validates resulting state before committing."""
        _validate_entries(self.entries + [entry])
        self.entries.append(entry)
        self._reindex()

    def remove(self, name: str) -> None:
        """Remove a zone by current_name. Raises KeyError if unknown."""
        e = next((x for x in self.entries if x.current_name == name), None)
        if e is None:
            raise KeyError(f"unknown zone: {name!r}")
        self.entries = [x for x in self.entries if x.current_name != name]
        self._reindex()

    def save(self, path: str | Path | None = None) -> None:
        """Write catalog back to yaml, preserving top-level comments via
        in-place mutation of self._raw_root when available."""
        target = Path(path) if path else self.source_path
        if target is None:
            raise ValueError("no path provided and no source_path on catalog")
        yaml = YAML(typ="rt")
        yaml.default_flow_style = False

        out = self._raw_root if self._raw_root is not None else CommentedMap()

        if "version" not in out:
            out["version"] = 1
        if self.host:
            out["host"] = self.host

        # Rebuild the zones sequence — preserves order from self.entries.
        new_seq = CommentedSeq()
        for e in self.entries:
            new_seq.append(self._to_raw(e))
        out["zones"] = new_seq

        with target.open("w") as f:
            yaml.dump(out, f)

    @staticmethod
    def _to_raw(e: ZoneEntry) -> CommentedMap:
        m = CommentedMap()
        m["current_name"] = e.current_name
        if e.type:
            m["type"] = e.type
        if e.notes:
            m["notes"] = e.notes
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


def _validate_entries(entries: list[ZoneEntry]) -> None:
    # Two-pass validation so collision checks are order-independent.
    # A single-pass would miss the case where entry A's prior_name matches
    # entry B's current_name when B appears later in the list.

    # Pass 1: per-entry shape checks + collect all live current_names.
    live_names: dict[str, int] = {}
    for idx, e in enumerate(entries):
        if not e.current_name:
            raise ValueError("zone entries must have a non-empty current_name")
        if e.type not in VALID_TYPES:
            raise ValueError(
                f"unknown zone type {e.type!r} for {e.current_name!r} "
                f"(must be one of {[t for t in VALID_TYPES if t]} or None)"
            )
        if e.current_name in live_names:
            raise ValueError(
                f"duplicate current_name {e.current_name!r}: "
                f"already used by entry #{live_names[e.current_name]}"
            )
        live_names[e.current_name] = idx

    # Pass 2: collision checks now that we know the full live-name set.
    # A prior_name on entry idx_A that matches the current_name of any OTHER
    # live entry idx_B (whether B appears before or after A) is a collision —
    # operators couldn't reliably address either zone. Matching this same
    # entry's current_name is fine after a rename round-trip.
    for idx, e in enumerate(entries):
        for p in e.prior_names:
            owner_idx = live_names.get(p.name)
            if owner_idx is not None and owner_idx != idx:
                raise ValueError(
                    f"prior_name {p.name!r} on zone {e.current_name!r} "
                    f"collides with live zone (entry #{owner_idx})"
                )


def load_zone_catalog(path: str | Path) -> ZoneCatalog:
    """Load a ZoneCatalog from yaml. Validates before returning."""
    p = Path(path)
    yaml = YAML(typ="rt")
    with p.open() as f:
        raw = yaml.load(f) or {}
    zones_raw = raw.get("zones") or []
    entries = [ZoneEntry.from_raw(dict(z)) for z in zones_raw]
    _validate_entries(entries)
    host = raw.get("host")
    return ZoneCatalog(entries=entries, host=host, source_path=p, raw_root=raw)
