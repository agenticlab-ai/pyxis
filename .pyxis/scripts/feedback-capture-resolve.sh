#!/bin/sh
# feedback-capture-resolve — the exec-grammar delegate (installer-v1 §3.10
# "Shims"; plan installer/001 T12). The confirm-time capture logic lives in
# cmd/feedback-capture; this shim only locates the writer and delegates through
# `pyxis exec` — the one acquisition-and-exec path (closing the DINV-2 gap this
# resolver had: it ran an unpinned PATH binary). The verbatim comment arrives on
# stdin and the metadata flags pass argv-verbatim after `--`. This compatibility
# entry point does not launch an editor. Capture is OFF the review critical
# path and FAILS OPEN: a missing Go
# toolchain exits 0 (comment not captured, review NOT blocked), and the writer
# itself always exits 0. In the source repo source mode runs pyxis from source
# (`go run ./cmd/pyxis`), which source-runs feedback-capture (double-go-run).
set -e
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
command -v go >/dev/null 2>&1 || { echo "feedback-capture: no Go toolchain — comment not captured (review not blocked; see AGENTS.md)" >&2; exit 0; }
cd "$ROOT" && exec go run ./cmd/pyxis exec feedback-capture -- "$@"
