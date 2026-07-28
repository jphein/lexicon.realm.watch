#!/usr/bin/env bash
# sync-vocabularies.sh — convert lexicon vocabularies to realm-sigil's words/*.json.
#
# Lexicon is the source of truth for realm-themed vocabularies. Sigil consumes
# them as derived artifacts. This script reads vocabularies/{realms,adjectives,
# nouns}.yaml and emits realm-sigil/words/realms.json in the shape sigil expects.
#
# Modes:
#   (default)  Write the JSON file(s) into the sigil tree.
#   --check    Compare what would be written against what's there. Exit 1 on drift.
#   --dry-run  Print what would be written to stdout. Don't touch the filesystem.
#
# Args:
#   --sigil-dir <path>   Path to realm-sigil checkout (default: ~/Projects/realm-sigil)
#   --lexicon-dir <path> Path to lexicon checkout (default: script's directory)
#
# Idempotent: running twice produces byte-identical output. Suitable for CI.
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"

LEXICON_DIR="$SCRIPT_DIR"
SIGIL_DIR="${HOME}/Projects/realm-sigil"
MODE="write"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check)       MODE="check"; shift ;;
    --dry-run)     MODE="dry-run"; shift ;;
    --sigil-dir)   SIGIL_DIR="$2"; shift 2 ;;
    --lexicon-dir) LEXICON_DIR="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,/^set -euo/p' "$0" | sed -n '/^#/p' | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

ADJ_YAML="$LEXICON_DIR/vocabularies/adjectives.yaml"
NOUN_YAML="$LEXICON_DIR/vocabularies/nouns.yaml"
REALMS_YAML="$LEXICON_DIR/vocabularies/realms.yaml"

for f in "$ADJ_YAML" "$NOUN_YAML" "$REALMS_YAML"; do
  if [[ ! -f "$f" ]]; then
    echo "missing: $f" >&2
    exit 1
  fi
done

if [[ "$MODE" != "dry-run" && ! -d "$SIGIL_DIR" ]]; then
  echo "sigil dir not found: $SIGIL_DIR" >&2
  exit 1
fi

# Render realms.json from lexicon's adjectives + nouns YAML.
# Algorithm:
#   - For each realm group present in BOTH adjectives.yaml AND nouns.yaml,
#     emit { adjectives: [...], nouns: [...] } with each word title-cased.
#   - Skip the "any" group (lexicon-only; sigil doesn't model it).
#   - Skip realms missing from either file (warn to stderr).
#   - Sort realm keys to match a stable, deterministic output order.
RENDERED="$(python3 - "$ADJ_YAML" "$NOUN_YAML" "$REALMS_YAML" <<'PYEOF'
import json
import sys

import yaml

adj_path, noun_path, realms_path = sys.argv[1], sys.argv[2], sys.argv[3]

with open(adj_path) as f:
    adjectives = yaml.safe_load(f).get("adjectives") or {}
with open(noun_path) as f:
    nouns = yaml.safe_load(f).get("nouns") or {}
with open(realms_path) as f:
    realms = yaml.safe_load(f).get("realms") or {}


def titleize(word: str) -> str:
    # Sigil's existing format uses simple Capitalization (first letter upper,
    # rest preserved). Hyphenated words like "white-hot" capitalize each
    # segment to mirror sigil's own corpus style ("Carbonized", "White-Hot").
    return "-".join(seg[:1].upper() + seg[1:] for seg in word.split("-"))


out = {}
skipped = []
for realm in sorted(realms.keys()):
    adj_group = adjectives.get(realm)
    noun_group = nouns.get(realm)
    if not adj_group or not noun_group:
        skipped.append(realm)
        continue
    out[realm] = {
        "adjectives": [titleize(w) for w in adj_group.get("words", [])],
        "nouns":      [titleize(w) for w in noun_group.get("words", [])],
    }

if skipped:
    print(f"warn: skipped realms missing adjectives or nouns: {skipped}", file=sys.stderr)

# Match sigil's existing 2-space indent + trailing newline.
print(json.dumps(out, indent=2, ensure_ascii=False))
PYEOF
)"

TARGET="$SIGIL_DIR/words/realms.json"

# The reserved set travels the SAME path as the vocabularies, because it is only meaningful
# alongside them: a consumer that generates a node-identity table must assert
# `vocabulary ∩ reserved == ∅` BEFORE asserting uniqueness, and it can only do that if the
# exclusion list arrives with the words. Shipping the words without the exclusions is what lets a
# downstream project quietly name a board after a role (`crown` IS the gateway role) — which is
# the ambiguity this file exists to prevent. Flat sorted list; the grouped-by-reason YAML keeps
# the rationale, consumers only need membership.
RESERVED_YAML="$LEXICON_DIR/vocabularies/reserved.yaml"
RENDERED_RESERVED="$(
  python3 - "$RESERVED_YAML" <<'PYEOF'
import json, sys
try:
    import yaml
except ImportError:
    print("error: PyYAML required to render reserved.json", file=sys.stderr); sys.exit(1)
with open(sys.argv[1]) as f:
    doc = yaml.safe_load(f) or {}
groups = doc.get("reserved", {}) or {}
words = sorted({w.strip().lower() for g in groups.values() for w in (g.get("words") or [])})
print(json.dumps({"reserved": words}, indent=2, ensure_ascii=False))
PYEOF
)"
TARGET_RESERVED="$SIGIL_DIR/words/reserved.json"

emit() { # emit <mode> <target> <rendered> <label>
  local mode="$1" target="$2" rendered="$3" label="$4"
  case "$mode" in
    dry-run)
      echo "# would write: $target"
      printf '%s\n' "$rendered"
      ;;
    check)
      if [[ ! -f "$target" ]]; then
        echo "drift: $target does not exist" >&2
        return 1
      fi
      if ! diff -u "$target" <(printf '%s\n' "$rendered") > "/tmp/sync-vocab-diff.$$" 2>&1; then
        echo "drift detected in $target" >&2
        cat "/tmp/sync-vocab-diff.$$" >&2
        rm -f "/tmp/sync-vocab-diff.$$"
        return 1
      fi
      rm -f "/tmp/sync-vocab-diff.$$"
      echo "ok: $target matches lexicon $label"
      ;;
    write)
      mkdir -p "$(dirname "$target")"
      printf '%s\n' "$rendered" > "$target"
      echo "wrote: $target"
      ;;
  esac
}

# Both targets are checked even when the first drifts, so `--check` reports every divergence in one
# run rather than making the caller re-run to discover the second.
rc=0
emit "$MODE" "$TARGET" "$RENDERED" "vocabularies" || rc=1
emit "$MODE" "$TARGET_RESERVED" "$RENDERED_RESERVED" "reserved set" || rc=1
exit "$rc"
