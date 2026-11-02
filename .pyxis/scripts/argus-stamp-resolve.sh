#!/bin/sh
# argus-stamp-resolve — the exec-grammar delegate (installer-v1 §3.10 "Shims";
# plan installer/001 T12). The stamp decision lives in cmd/argus-stamp; this
# shim only locates the stamper and delegates through `pyxis exec` — the one
# acquisition-and-exec path (closing the DINV-2 gap this resolver had: it ran an
# unpinned PATH binary). Bash stays the named bootstrap exception (the
# prepare-commit-msg hook runs before any binary is on PATH). ROOT is the
# stamper's own tree from this script's path (<root>/.pyxis/scripts/), NEVER the
# invoking repo's toplevel — so a commit in any repo resolves cmd/argus-stamp
# here — and the commit-message file is absolutized (git hands it relative to the
# invoking repo) before the cd. No Go toolchain is a fail-closed abort of the
# commit: a drive commit must not land silently un-attributed (merge-boundary
# gate is the backstop, DINV-2). In the source repo source mode runs pyxis from
# source (`go run ./cmd/pyxis`), which source-runs argus-stamp (double-go-run).
set -e
command -v go >/dev/null 2>&1 || { echo "argus-stamp: no Go toolchain (see AGENTS.md)" >&2; exit 2; }
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MSG="$1"; shift
case "$MSG" in /*) ;; *) MSG="$(pwd)/$MSG" ;; esac
cd "$ROOT" && exec go run ./cmd/pyxis exec argus-stamp -- "$MSG" "$@"
