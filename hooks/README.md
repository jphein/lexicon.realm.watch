# lexicon hooks

Sample hooks for working in lexicon.realm.watch. Nothing here installs
itself — pick the ones you want and wire them up by hand.

## Files

- `catalog-status.py` — Claude Code `SessionStart` hook. Reads
  `catalog/projects.yaml` and prints a one-line banner so the model
  starts each session knowing how many projects/realms are recorded.
  Requires `python3` and PyYAML; no-ops cleanly if either is missing.
- `lexicon-validate-precommit.sh` — git `pre-commit` hook. Runs
  `lexicon validate` and aborts the commit if the catalog,
  vocabularies, or recipes are inconsistent.
- `../.claude/settings.json` — example project-local Claude Code
  settings showing how the SessionStart hook (and a small `Edit`
  guard for `tests/fixtures/seeded-recipes.json`) wire in.

## Install: Claude Code SessionStart

Claude Code reads project-local settings from `.claude/settings.json`
or `.claude/settings.local.json` next to the repo. The shipped
`.claude/settings.json` already wires `catalog-status.py` to fire on
`SessionStart`. To enable it in your own checkout:

1. Copy or merge the shipped file into your local settings:
   ```sh
   cp .claude/settings.json .claude/settings.local.json
   ```
   (`settings.local.json` is per-checkout and shouldn't be committed;
   `settings.json` is the shared example.)
2. Make sure `python3` and PyYAML are available — `pip install --user
   pyyaml` is enough.
3. Open the repo in Claude Code. The first message of each session
   should now include a banner like
   `lexicon — 14 projects in 5 realms; 3 prior names recorded`.

The hook reads `$CLAUDE_PROJECT_DIR/catalog/projects.yaml`, falling
back to `$LEXICON_CATALOG` and then to a path relative to the script
itself. Override either env var if your catalog lives elsewhere.

### Other Claude Code events

The same script makes sense as a `UserPromptSubmit` reminder if you
want the banner on every prompt, not just session start. Add another
entry to the `hooks` block in `.claude/settings.json`:

```json
"UserPromptSubmit": [
  { "hooks": [
    { "type": "command",
      "command": "python3 \"$CLAUDE_PROJECT_DIR/hooks/catalog-status.py\"",
      "timeout": 5000 }
  ]}
]
```

## Install: git pre-commit

`lexicon-validate-precommit.sh` blocks commits that would leave the
catalog or vocabularies in an invalid state.

```sh
ln -s ../../hooks/lexicon-validate-precommit.sh .git/hooks/pre-commit
chmod +x hooks/lexicon-validate-precommit.sh
```

The hook resolves `lexicon` in this order:
1. `$LEXICON_BIN` if set and executable
2. `lexicon` on `PATH`
3. `go/lexicon` (built via `cd go && go build ./cmd/lexicon`)
4. `go run ./cmd/lexicon` from `go/`

Build the binary once for fast pre-commits:

```sh
(cd go && go build -o lexicon ./cmd/lexicon)
```

To test the hook without committing:

```sh
bash hooks/lexicon-validate-precommit.sh
```

A clean repo exits 0 silently. A broken catalog or vocabulary file
prints the validate error and exits non-zero.

## Customising

Both scripts are intentionally tiny so they read at a glance. Fork
freely:

- Change the banner format in `catalog-status.py`'s
  `_format_banner` function.
- Tighten the pre-commit to only run when staged files touch
  `catalog/`, `vocabularies/`, or `version.json` — currently it runs
  validate on every commit.
