#!/usr/bin/env python3
"""One-shot: insert `audience:` after `realm:` for each project in catalog/projects.yaml.

Classification table is hand-built per
docs/superpowers/specs/2026-05-08-audience-classification.md § Allowed values.

Run from repo root:  python3 scripts/backfill-audience.py
Idempotent: skips projects that already have an `audience:` line.
"""
from __future__ import annotations

import sys
from pathlib import Path

AUDIENCE = {
    # realm — themed public *.realm.watch family
    "clock": "realm",
    "dark": "realm",
    "realm-sigil": "realm",
    "lexicon": "realm",
    "status": "realm",
    "oracle": "realm",
    "realm-portal": "realm",
    "realmcoin": "realm",
    "os.realm.watch": "realm",
    "mirror.realm.watch": "realm",
    "familiar.realm.watch": "realm",
    "bestiary": "realm",
    "oracle-chat": "realm",
    "portfolio": "realm",  # spec Open Q: "Likely realm once renamed"

    # personal — homelab + jphe.in + JP-identity-bound
    "realmwatch": "personal",
    "speech-to-cli": "personal",
    "claude-code-switcher": "personal",
    "cloud-chat-assistant": "personal",
    "artcardsv5": "personal",
    "dreamspace": "personal",
    "hostname-badge": "personal",
    "techempower": "personal",
    "unshuffled": "personal",
    "opus": "personal",
    "gnome-speaks": "personal",
    "gnome-shell-monitor": "personal",
    "kiyo-xhci-fix": "personal",
    "disks": "personal",
    "scripts": "personal",
    "optimize": "personal",
    "openwrt": "personal",
    "esp-wifi-repeater": "personal",
    "realm-optimizer": "personal",
    "gaming-tuning": "personal",
    "minecraft-bedrock-linux": "personal",
    "obs-plugins": "personal",
    "lettertomom": "personal",
    "outline": "personal",
    "update": "personal",
    "palace-daemon": "personal",
    "multipass-structural-memory-eval": "personal",
    "vault-gate": "personal",
    "tonemask": "personal",
    "tablet-tune": "personal",
    "jp": "personal",
    "ventoy": "personal",
    "usb-issues": "personal",
    "streamcam-fixes": "personal",
    "streaming": "personal",
    "veadotube-avatars": "personal",
    "umbra": "personal",
    "notebook": "personal",
    "printing": "personal",
    "stack": "personal",
    "ipv6": "personal",
    "current-sensor": "personal",
    "openclaw": "personal",
    "vault": "personal",
    "ha": "personal",
    "mempalace-data": "personal",

    # external — standalone brands / client / family
    "claudedoublehours": "external",
    "donkeyco": "external",
    "imaginalvision.com": "external",
    "jpheinnet": "external",
    "poppasblog": "external",
    "jphein-wordpress-server": "external",
    "jphein-wordpress-site": "external",
    "forageforall": "external",
    "starcharts": "external",
    "roblox": "external",
    "shawnahein.com": "external",
    "solar": "external",
    "sdp": "external",
    "sdp+disability-appeal": "external",
    "jewelrycycle": "external",
    "oldsites": "external",
    "clawwatch": "external",
    "storyvox": "external",
    "voxsherpa-tts": "external",

    # fork — upstream-identity-bound
    "memorypalace": "fork",
    "mempalace-triage": "fork",
    "oracle-mcp": "fork",
    "rlm": "fork",
    "karta": "fork",
    "gstack": "fork",
    "claude-code-python": "fork",
    "claude-code-source-leaked": "fork",
    "claude-code-source": "fork",
}


def main() -> int:
    repo = Path(__file__).resolve().parent.parent
    yaml_path = repo / "catalog" / "projects.yaml"
    text = yaml_path.read_text()
    lines = text.split("\n")
    out: list[str] = []
    seen_ids: set[str] = set()
    classified: list[str] = []
    skipped_existing: list[str] = []
    unknown: list[str] = []

    current_id: str | None = None
    audience_emitted_for: set[str] = set()

    i = 0
    while i < len(lines):
        line = lines[i]
        stripped = line.lstrip()
        if line.startswith("  - id:"):
            current_id = line.split(":", 1)[1].strip()
            seen_ids.add(current_id)
            out.append(line)
            i += 1
            continue

        if current_id and stripped.startswith("audience:"):
            audience_emitted_for.add(current_id)
            skipped_existing.append(current_id)
            out.append(line)
            i += 1
            continue

        if (
            current_id
            and current_id not in audience_emitted_for
            and stripped.startswith("realm:")
        ):
            out.append(line)
            audience = AUDIENCE.get(current_id)
            if audience is None:
                unknown.append(current_id)
            else:
                indent = line[: len(line) - len(line.lstrip())]
                out.append(f"{indent}audience: {audience}")
                classified.append(current_id)
                audience_emitted_for.add(current_id)
            i += 1
            continue

        out.append(line)
        i += 1

    yaml_path.write_text("\n".join(out))

    print(f"Catalog: {len(seen_ids)} projects total")
    print(f"  classified now: {len(classified)}")
    print(f"  already had audience: {len(skipped_existing)}")
    if unknown:
        print(f"  UNKNOWN (no entry in script's table): {len(unknown)}")
        for u in unknown:
            print(f"    - {u}")
        return 1
    extras = set(AUDIENCE) - seen_ids
    if extras:
        print(f"  WARNING: script has entries not in catalog: {sorted(extras)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
