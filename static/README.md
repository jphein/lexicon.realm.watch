# static/ — lexicon.realm.watch landing page

The public face of lexicon. Same hosting pattern as `clock.realm.watch` and
`realm-sigil` — a tiny static bundle deployed to GitHub Pages.

## Layout

```
static/
  index.html      # template — placeholders __VOCAB_BLOCK__ and __CATALOG_ROWS__
  style.css       # dark/light via prefers-color-scheme; CSS custom properties
  app.js          # in-browser roller (mirrors the cross-language seeded RNG)
                  # + catalog filter
  favicon.svg     # open book + floating rune sigil, monochromatic
  build.sh        # reads ../catalog + ../vocabularies → renders dist/
  dist/           # build output (kept in repo so GitHub Pages can serve it)
```

## Build

```sh
./build.sh
```

The script reads:

- `../catalog/projects.yaml` — only entries with `domain != null` go on the page
- `../vocabularies/*.yaml` — adjectives, nouns, scientists, creatures, realms, recipes

…and writes the rendered files into `dist/`. Everything ships pre-baked so the
page loads with zero network calls (no fetches, no CORS).

`build.sh` requires `python3` with `PyYAML`. Override `PYTHON`, `CATALOG`, or
`VOCAB_DIR` env vars if you want to render a different roster.

## How the page works

The header tagline is the realm.watch trio reference:

> There are two hard things in computer science. Clock handles one,
> sigil handles the other, lexicon is what you reach for when they collide.

The roller widget runs entirely client-side:

1. Pick a recipe (project, agent, branch, entity) and the realm/prefix it
   demands (fields auto-show/hide).
2. Click Roll → JS hashes a fresh random seed against each slot index using
   SHA-256 (mirrors `go/seeded.go` and `js/src/seeded.js` byte-for-byte) and
   pulls words from the inlined vocabularies.
3. Five candidates by default, configurable.

The catalog viewer is rendered at build time from `catalog/projects.yaml`,
filtered to entries whose `domain` is set. The realm filter dropdown is built
client-side from whatever realms appear in the rendered rows.

## Aesthetics

Borrows the grimoire palette from `clock.realm.watch` (gold rims on a void
purple-black) but rotates the accent to gem-frost (`#b0e0ff`) so the family
reads as related-but-distinct. Three serif fonts via Google Fonts: Cinzel for
headings, MedievalSharp for the joke footer, JetBrains Mono for rolled names.

## Deploy

Point GitHub Pages at `static/dist/` (or copy `dist/` into `gh-pages` branch).
The path-relative `<link>`/`<script>` tags resolve regardless of subpath.
