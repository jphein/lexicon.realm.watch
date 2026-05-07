#!/usr/bin/env bash
# Pre-commit hook for lexicon.realm.watch.
#
# Runs `lexicon validate` against the repo's catalog and vocabularies.
# Aborts the commit (non-zero exit) if validation fails.
#
# Install by symlinking into .git/hooks:
#   ln -s ../../hooks/lexicon-validate-precommit.sh \
#         .git/hooks/pre-commit
#
# Resolution order for the lexicon binary:
#   1. $LEXICON_BIN
#   2. `lexicon` on PATH
#   3. <repo>/go/lexicon (built via `cd go && go build ./cmd/lexicon`)
#   4. `go run ./cmd/lexicon` from <repo>/go (slow but always works)

set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
catalog="$repo_root/catalog/projects.yaml"
vocabularies="$repo_root/vocabularies"

# Skip silently if this isn't a lexicon-shaped repo.
if [[ ! -f "$catalog" || ! -d "$vocabularies" ]]; then
    exit 0
fi

run_validate() {
    if [[ -n "${LEXICON_BIN:-}" && -x "$LEXICON_BIN" ]]; then
        "$LEXICON_BIN" validate \
            --catalog "$catalog" \
            --vocabularies "$vocabularies"
        return
    fi
    if command -v lexicon >/dev/null 2>&1; then
        lexicon validate \
            --catalog "$catalog" \
            --vocabularies "$vocabularies"
        return
    fi
    if [[ -x "$repo_root/go/lexicon" ]]; then
        "$repo_root/go/lexicon" validate \
            --catalog "$catalog" \
            --vocabularies "$vocabularies"
        return
    fi
    if command -v go >/dev/null 2>&1; then
        (
            cd "$repo_root/go"
            go run ./cmd/lexicon validate \
                --catalog "../catalog/projects.yaml" \
                --vocabularies "../vocabularies"
        )
        return
    fi
    echo "pre-commit: no lexicon binary and no go toolchain found; skipping validate" >&2
    return 0
}

if ! run_validate; then
    echo "pre-commit: lexicon validate failed — commit aborted" >&2
    exit 1
fi
