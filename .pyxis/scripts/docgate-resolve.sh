#!/bin/sh
# docgate-resolve — the exec-grammar delegate (installer-v1 §3.10 "Shims"; plan
# installer/001 T12). The fetch-verify-cache logic that once lived here now lives
# once in `pyxis exec` over internal/resolve (DINV-1/DINV-2) — this file is a
# locate-and-delegate shim, nothing more. Bash stays the named bootstrap
# exception (a git hook and CI call this before any binary is on PATH). In the
# source repo pyxis is itself unreleased, so source mode runs pyxis from source:
# `go run ./cmd/pyxis`, which then source-runs docgate (the double-go-run leg,
# dev-only). All three call sites (pre-commit, pre-push, CI) delegate here; args
# pass argv-verbatim after `--`, and the tool's exit code is this shim's.
set -e
ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
[ -n "$ROOT" ] || ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
command -v go >/dev/null 2>&1 || { echo "docgate: source mode but no Go toolchain (see AGENTS.md)" >&2; exit 2; }
cd "$ROOT" && exec go run ./cmd/pyxis exec docgate -- "$@"
