"""Project catalog — mirrors ``go/catalog.go`` + ``catalog_query.go`` +
``catalog_claim.go``.

The catalog is a checked-in YAML file (``catalog/projects.yaml``). This module
loads it, exposes resolve/by-realm/by-kind/by-status queries, supports
:meth:`Catalog.claim` for renames or new entries, and round-trips the file via
``ruamel.yaml`` so hand-written comments and key order survive a save.
"""

from __future__ import annotations

import datetime as _dt
import io
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from ruamel.yaml import YAML
from ruamel.yaml.comments import CommentedMap, CommentedSeq

__all__ = [
    "Project",
    "PriorName",
    "Catalog",
    "ClaimOpts",
    "load_catalog",
]


@dataclass
class PriorName:
    name: str
    retired: str
    reason: str = ""

    @classmethod
    def _from_raw(cls, raw: Any) -> "PriorName":
        if not isinstance(raw, dict):
            raise ValueError(f"prior_name entry must be a mapping, got {type(raw).__name__}")
        return cls(
            name=str(raw.get("name", "")),
            retired=str(raw.get("retired", "")),
            reason=str(raw.get("reason", "") or ""),
        )


@dataclass
class Project:
    id: str
    current_name: str = ""
    kind: str = ""
    realm: str = ""
    domain: str | None = None
    repo: str | None = None
    description: str = ""
    created: str = ""
    prior_names: list[PriorName] = field(default_factory=list)
    status: str = ""
    notes: str = ""

    @classmethod
    def _from_raw(cls, raw: Any) -> "Project":
        if not isinstance(raw, dict):
            raise ValueError(f"project entry must be a mapping, got {type(raw).__name__}")
        prior_raw = raw.get("prior_names") or []
        return cls(
            id=str(raw.get("id", "")),
            current_name=str(raw.get("current_name", "") or ""),
            kind=str(raw.get("kind", "") or ""),
            realm=str(raw.get("realm", "") or ""),
            domain=_optional_str(raw.get("domain")),
            repo=_optional_str(raw.get("repo")),
            description=str(raw.get("description", "") or ""),
            created=str(raw.get("created", "") or ""),
            prior_names=[PriorName._from_raw(p) for p in prior_raw],
            status=str(raw.get("status", "") or ""),
            notes=str(raw.get("notes", "") or ""),
        )


def _optional_str(value: Any) -> str | None:
    if value is None:
        return None
    s = str(value)
    return s if s else None


@dataclass
class ClaimOpts:
    """Inputs for :meth:`Catalog.claim`. See ``go/catalog_claim.go`` for parity."""

    renames_of: str = ""  # existing project id; empty means "new entry"
    reason: str = ""
    retired: str = ""
    kind: str = ""
    realm: str = ""
    domain: str = ""
    repo: str = ""
    description: str = ""
    created: str = ""
    status: str = ""


class Catalog:
    """Loaded ``catalog/projects.yaml``."""

    def __init__(
        self,
        path: str | Path | None,
        projects: list[Project],
        root: Any | None = None,
    ) -> None:
        self.path = Path(path) if path else None
        self.projects: list[Project] = projects
        self._root = root  # ruamel CommentedMap (or None) for round-tripping

    # -- queries -----------------------------------------------------------

    def resolve(self, name: str) -> Project | None:
        """Look up by id, current_name, or any prior name. First match wins."""
        for p in self.projects:
            if p.id == name:
                return p
        for p in self.projects:
            if p.current_name == name:
                return p
        for p in self.projects:
            for pn in p.prior_names:
                if pn.name == name:
                    return p
        return None

    def by_realm(self, realm: str) -> list[Project]:
        return [p for p in self.projects if p.realm == realm]

    def by_kind(self, kind: str) -> list[Project]:
        return [p for p in self.projects if p.kind == kind]

    def by_status(self, status: str) -> list[Project]:
        return [p for p in self.projects if p.status == status]

    # -- mutation ----------------------------------------------------------

    def claim(self, new_name: str, opts: ClaimOpts | None = None) -> None:
        """Rename an existing project or append a new one. Mirrors Go's Claim."""
        opts = opts or ClaimOpts()
        if not new_name:
            raise ValueError("claim: new_name is empty")
        existing = self.resolve(new_name)
        if existing is not None:
            if not opts.renames_of or existing.id != opts.renames_of:
                raise ValueError(
                    f"name {new_name!r} is already taken by project "
                    f"{existing.id!r} (current={existing.current_name!r})"
                )
        if opts.renames_of:
            self._rename_in_place(new_name, opts)
        else:
            self._append_new(new_name, opts)

    def save(self, path: str | Path | None = None) -> None:
        """Write the in-memory catalog back to ``path`` (defaults to load path)."""
        target = Path(path) if path else self.path
        if target is None:
            raise ValueError("save: catalog has no source path")
        data = self.dumps()
        target.write_text(data, encoding="utf-8")

    def dumps(self) -> str:
        """Serialize the in-memory catalog to a YAML string."""
        yaml = _round_trip_yaml()
        if self._root is not None:
            doc = self._sync_to_root()
        else:
            doc = self._build_root_from_projects()
        buf = io.StringIO()
        yaml.dump(doc, buf)
        return buf.getvalue()

    # -- internals ---------------------------------------------------------

    def _rename_in_place(self, new_name: str, opts: ClaimOpts) -> None:
        target = self.resolve(opts.renames_of)
        if target is None:
            raise ValueError(f"renames_of={opts.renames_of!r} not found in catalog")
        retired = opts.retired or _today()
        target.prior_names.append(
            PriorName(
                name=target.current_name,
                retired=retired,
                reason=opts.reason or "",
            )
        )
        target.current_name = new_name

    def _append_new(self, new_name: str, opts: ClaimOpts) -> None:
        created = opts.created or _today()
        status = opts.status or "active"
        self.projects.append(
            Project(
                id=new_name,
                current_name=new_name,
                kind=opts.kind,
                realm=opts.realm,
                domain=_optional_str(opts.domain),
                repo=_optional_str(opts.repo),
                description=opts.description,
                created=created,
                prior_names=[],
                status=status,
            )
        )

    def _sync_to_root(self) -> Any:
        """Apply in-memory ``self.projects`` onto the parsed root tree.

        Best-effort round-trip: existing entries keep their key order and
        comments; renames mutate ``current_name`` and append to ``prior_names``;
        new projects are added as fresh CommentedMap entries.
        """
        root = self._root
        if "projects" not in root or not isinstance(root["projects"], CommentedSeq):
            return self._build_root_from_projects()
        seq: CommentedSeq = root["projects"]
        existing_by_id: dict[str, CommentedMap] = {}
        for entry in seq:
            if isinstance(entry, dict) and "id" in entry:
                existing_by_id[str(entry["id"])] = entry  # type: ignore[index]
        new_seq = CommentedSeq()
        for proj in self.projects:
            cm = existing_by_id.get(proj.id)
            if cm is None:
                new_seq.append(_project_to_commented_map(proj))
            else:
                _update_commented_map(cm, proj)
                new_seq.append(cm)
        # Preserve top-level comments by mutating in place.
        seq.clear()
        for item in new_seq:
            seq.append(item)
        return root

    def _build_root_from_projects(self) -> CommentedMap:
        root = CommentedMap()
        seq = CommentedSeq()
        for proj in self.projects:
            seq.append(_project_to_commented_map(proj))
        root["projects"] = seq
        return root


def _today() -> str:
    return _dt.datetime.utcnow().date().isoformat()


def _round_trip_yaml() -> YAML:
    yaml = YAML()
    yaml.indent(mapping=2, sequence=4, offset=2)
    yaml.preserve_quotes = True
    return yaml


def _project_to_commented_map(p: Project) -> CommentedMap:
    cm = CommentedMap()
    cm["id"] = p.id
    cm["current_name"] = p.current_name
    cm["kind"] = p.kind
    cm["realm"] = p.realm
    cm["domain"] = p.domain if p.domain is not None else None
    cm["repo"] = p.repo if p.repo is not None else None
    cm["description"] = p.description
    cm["created"] = p.created
    cm["prior_names"] = _prior_names_to_seq(p.prior_names)
    cm["status"] = p.status
    if p.notes:
        cm["notes"] = p.notes
    return cm


def _update_commented_map(cm: CommentedMap, p: Project) -> None:
    cm["current_name"] = p.current_name
    cm["kind"] = p.kind
    cm["realm"] = p.realm
    cm["domain"] = p.domain if p.domain is not None else None
    cm["repo"] = p.repo if p.repo is not None else None
    cm["description"] = p.description
    cm["created"] = p.created
    cm["prior_names"] = _prior_names_to_seq(p.prior_names)
    cm["status"] = p.status
    if p.notes:
        cm["notes"] = p.notes


def _prior_names_to_seq(prior_names: list[PriorName]) -> CommentedSeq:
    seq = CommentedSeq()
    for pn in prior_names:
        entry = CommentedMap()
        entry["name"] = pn.name
        entry["retired"] = pn.retired
        entry["reason"] = pn.reason
        entry.fa.set_flow_style()
        seq.append(entry)
    return seq


def load_catalog(path: str | Path) -> Catalog:
    """Read ``catalog/projects.yaml`` into a :class:`Catalog`."""
    yaml = _round_trip_yaml()
    p = Path(path)
    with p.open("r", encoding="utf-8") as fh:
        root = yaml.load(fh) or CommentedMap()
    if not isinstance(root, dict):
        raise ValueError(f"{path}: expected top-level mapping, got {type(root).__name__}")
    raw_projects = root.get("projects") or []
    projects = [Project._from_raw(entry) for entry in raw_projects]
    return Catalog(path=p, projects=projects, root=root)
