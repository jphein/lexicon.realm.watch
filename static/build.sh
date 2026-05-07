#!/usr/bin/env bash
# build.sh — render lexicon's static landing page into dist/.
#
# Reads:
#   ../catalog/projects.yaml
#   ../vocabularies/{adjectives,nouns,scientists,creatures,realms,recipes}.yaml
#
# Writes:
#   dist/index.html       (placeholders filled)
#   dist/style.css        (copied)
#   dist/app.js           (copied)
#   dist/favicon.svg      (copied)
#
# The page must work fully static — no fetches, no CORS. We inline all data
# as JSON inside index.html and the JS reads it from window globals.

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
DIST="$HERE/dist"
CATALOG="${CATALOG:-$ROOT/catalog/projects.yaml}"
VOCAB_DIR="${VOCAB_DIR:-$ROOT/vocabularies}"

mkdir -p "$DIST"

# Pick a python that has yaml. The system python3 on Ubuntu has it via python3-yaml.
PY="${PYTHON:-python3}"
if ! "$PY" -c "import yaml" 2>/dev/null; then
  echo "build.sh: need python3 with PyYAML — try: sudo apt install python3-yaml" >&2
  exit 1
fi

# Render via a single python pass so all the YAML/JSON stays in one place.
"$PY" - "$ROOT" "$DIST" "$HERE" "$CATALOG" "$VOCAB_DIR" <<'PYEOF'
import json
import os
import sys
import html
import yaml

_, root, dist, here, catalog_path, vocab_dir = sys.argv

# ── load YAML ──────────────────────────────────────────────────

def load(path):
    with open(path, "r", encoding="utf-8") as f:
        return yaml.safe_load(f)

def load_words(name):
    """vocabularies/<name>.yaml → {group: [words]} (top key is <name>)."""
    path = os.path.join(vocab_dir, f"{name}.yaml")
    if not os.path.exists(path):
        return {}
    data = load(path) or {}
    block = data.get(name) or {}
    out = {}
    for group, info in block.items():
        if isinstance(info, dict) and "words" in info:
            out[group] = list(info["words"])
        elif isinstance(info, list):
            out[group] = list(info)
    return out

vocab = {
    "adjectives": load_words("adjectives"),
    "nouns":      load_words("nouns"),
    "scientists": load_words("scientists"),
    "creatures":  load_words("creatures"),
}

realms_doc = load(os.path.join(vocab_dir, "realms.yaml")) or {}
realms_block = realms_doc.get("realms") or {}
realms = {}
for name, info in realms_block.items():
    if isinstance(info, dict):
        realms[name] = {
            "description": info.get("description", ""),
            "words":       list(info.get("words", [])),
        }
# also expose realms as a vocabulary group so recipes referencing realms work
vocab["realms"] = {name: r["words"] for name, r in realms.items()}

recipes_doc = load(os.path.join(vocab_dir, "recipes.yaml")) or {}
recipes_block = recipes_doc.get("recipes") or {}
recipes = {}
for name, info in recipes_block.items():
    if not isinstance(info, dict):
        continue
    recipes[name] = {
        "description":      info.get("description", ""),
        "pattern":          info.get("pattern", ""),
        "sources":          info.get("sources", {}) or {},
        "required_options": info.get("required_options", []) or [],
    }
# recipes that reference a catalog (e.g. voice) aren't rollable here — drop them
recipes = {n: r for n, r in recipes.items() if r.get("pattern")}

cat_doc = load(catalog_path) or {}
projects = cat_doc.get("projects") or []

# ── build catalog table rows ───────────────────────────────────
# Spec: only entries with domain != null go on the public landing page.

def fmt_priors(priors):
    if not priors:
        return '<span class="none">—</span>'
    parts = []
    for p in priors:
        if isinstance(p, dict):
            nm = p.get("name", "")
            retired = p.get("retired", "")
            tag = html.escape(str(nm))
            if retired:
                tag = f"{tag} <small>({html.escape(str(retired))})</small>"
            parts.append(tag)
        else:
            parts.append(html.escape(str(p)))
    return ", ".join(parts)

rows = []
public = [p for p in projects if p.get("domain")]
public.sort(key=lambda p: (p.get("realm") or "zz", p.get("current_name") or ""))

for p in public:
    name   = p.get("current_name") or p.get("id") or "?"
    domain = p.get("domain") or ""
    repo   = p.get("repo") or ""
    realm  = p.get("realm") or ""
    kind   = p.get("kind") or ""
    desc   = p.get("description") or ""
    priors = p.get("prior_names") or []

    # link priority: domain (if set) → repo → no link
    href = None
    if domain:
        href = f"https://{html.escape(domain)}"
    elif repo:
        href = html.escape(repo)

    if href:
        name_cell = f'<a href="{href}" rel="noopener">{html.escape(name)}</a>'
    else:
        name_cell = html.escape(name)

    rows.append(
        f'    <tr data-realm="{html.escape(realm)}">\n'
        f'      <td class="name">{name_cell}</td>\n'
        f'      <td class="realm">{html.escape(realm)}</td>\n'
        f'      <td class="kind">{html.escape(kind)}</td>\n'
        f'      <td class="desc">{html.escape(desc)}</td>\n'
        f'      <td class="priors">{fmt_priors(priors)}</td>\n'
        f'    </tr>'
    )

catalog_html = "\n".join(rows) if rows else (
    '    <tr><td colspan="5" style="text-align:center; color:var(--text-dim);">'
    'no public projects yet</td></tr>'
)

# ── inject into index.html ─────────────────────────────────────

with open(os.path.join(here, "index.html"), "r", encoding="utf-8") as f:
    template = f.read()

vocab_block = (
    "<script>\n"
    "  window.__LEXICON_VOCAB__   = " + json.dumps(vocab,   ensure_ascii=False) + ";\n"
    "  window.__LEXICON_RECIPES__ = " + json.dumps(recipes, ensure_ascii=False) + ";\n"
    "  window.__LEXICON_REALMS__  = " + json.dumps(realms,  ensure_ascii=False) + ";\n"
    "</script>"
)

out = template
out = out.replace("<!-- __VOCAB_BLOCK__ -->", vocab_block)
out = out.replace("            <!-- __CATALOG_ROWS__ -->", catalog_html)
# Tolerate flexible indentation on the placeholder
out = out.replace("<!-- __CATALOG_ROWS__ -->", catalog_html)

with open(os.path.join(dist, "index.html"), "w", encoding="utf-8") as f:
    f.write(out)

print(f"  rendered {dist}/index.html — {len(public)} public project(s), "
      f"{len(recipes)} recipe(s), {sum(len(v) for v in vocab.values())} word groups")
PYEOF

# Copy static assets verbatim
cp "$HERE/style.css"   "$DIST/style.css"
cp "$HERE/app.js"      "$DIST/app.js"
cp "$HERE/favicon.svg" "$DIST/favicon.svg"
echo "  copied style.css, app.js, favicon.svg"

# Quick sanity: assert the rendered HTML has data injected
if ! grep -q "__LEXICON_VOCAB__" "$DIST/index.html"; then
  echo "build.sh: render failed — vocab block missing from dist/index.html" >&2
  exit 1
fi

echo "build.sh: dist/ ready at $DIST"
