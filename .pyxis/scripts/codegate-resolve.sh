#!/bin/sh
# codegate-resolve — the exec-grammar delegate (installer-v1 §3.10 "Shims"; plan
# installer/001 T12), the sibling of docgate-resolve.sh. The fetch-verify-cache
# and section-scoped config parse that once lived here now live once in `pyxis
# exec` over internal/resolve (DINV-1/DINV-2) — this file is a locate-and-delegate
# shim, nothing more. Bash stays the named bootstrap exception. In the source
# repo pyxis is itself unreleased, so source mode runs pyxis from source: `go run
# ./cmd/pyxis`, which then source-runs codegate (the double-go-run leg, dev-only).
# Args pass argv-verbatim after `--`, and the tool's exit code is this shim's.
set -e
ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
[ -n "$ROOT" ] || ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
command -v go >/dev/null 2>&1 || { echo "codegate: source mode but no Go toolchain (see AGENTS.md)" >&2; exit 2; }
cd "$ROOT" && exec go run ./cmd/pyxis exec codegate -- "$@"
