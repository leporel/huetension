#!/usr/bin/env bash
# Fails when the committed web/dist/ differs from a fresh build.
# Run `cd web && bun run build` first; this script only diffs the result
# against what's committed. A diff means `go install` users would ship a
# stale UI.

set -euo pipefail

cd "$(dirname "$0")/.."

if [[ ! -d web/dist ]]; then
    printf 'check-dist: web/dist/ does not exist; run "cd web && bun run build" first.\n' >&2
    exit 1
fi

if ! git diff --quiet -- web/dist; then
    printf 'check-dist: committed web/dist/ is out of sync with web/src/.\n' >&2
    printf 'Run "cd web && bun run build" and commit the regenerated bundle.\n\n' >&2
    git --no-pager diff --stat -- web/dist >&2
    exit 1
fi

if git ls-files --others --exclude-standard -- web/dist | grep -q .; then
    printf 'check-dist: web/dist/ has untracked files after build.\n' >&2
    printf 'Stage and commit them so `go install` users get the same bundle.\n\n' >&2
    git ls-files --others --exclude-standard -- web/dist >&2
    exit 1
fi

printf 'check-dist: web/dist/ is up to date.\n'
