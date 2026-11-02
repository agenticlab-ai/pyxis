#!/bin/sh
# code-gate-resolve — the exec-grammar delegate (code-gate-mirror.md §3.6), the
# sibling of docgate-resolve.sh / codegate-resolve.sh. This file is a locate-and-
# delegate shim, nothing more: it resolves the repo root and delegates into the
# one `pyxis exec` grammar. Bash stays the named bootstrap exception (a git hook
# and CI call this before any binary is on PATH). In the source repo pyxis is
# itself unreleased, so source mode runs pyxis from source: `go run ./cmd/pyxis`,
# which then source-runs code-gate (the double-go-run leg, dev-only). Args pass
# argv-verbatim after `--`, and the tool's exit code is this shim's.
set -e
ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
[ -n "$ROOT" ] || ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
command -v go >/dev/null 2>&1 || { echo "code-gate: source mode but no Go toolchain (see AGENTS.md)" >&2; exit 2; }
cd "$ROOT" && exec go run ./cmd/pyxis exec code-gate -- "$@"
