#!/bin/sh
# maintenance-sweep — the host scheduler's entry to one bounded, read-only
# maintenance sweep pass (maintenance-sweep-v1 §3.9; plan T6's contracted shim;
# the wake tick's delegate-only shim pattern reused, never cohabited — #ALT-3).
# Bash is admissible here as the scheduler-integration exception (AP-25): cron
# and launchd exec a script line, which must run before any binary exists. It is
# delegate-only — it locates the repo and hands off to the Go binary, appending
# nothing but the caller's own flags; every sweep decision lives in Go (DINV-1).
#
# COST STATEMENT (AP-19, prd NFR-2 — nothing resident): one invocation is exactly
# one bounded pass — load the policy, run the three read-only detectors, emit the
# report, exit. Nothing stays resident between invocations; the steady-state cost
# is zero between ticks, one bounded pass per tick. The pass writes nothing beyond
# its own stdout/stderr (no repo, tracker, or network mutation — DINV-1).
#
# SCHEDULER BINDING (the operator installs; the repo never does — the schedule is
# the host's, exactly like the wake tick's shim; AP-19's nothing-resident posture
# is the binary's, the cadence is the operator's):
#   cron:    0 8 * * *  /path/to/repo/.pyxis/scripts/maintenance-sweep.sh
#   launchd: a StartCalendarInterval/StartInterval agent whose ProgramArguments
#            are this script (interval = the operator's chosen sweep cadence).
# Exit codes pass straight through from the binary: 0 clean, 1 findings, 2 fault.
set -e
ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
[ -n "$ROOT" ] || ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
command -v go >/dev/null 2>&1 || { echo "maintenance-sweep: no Go toolchain on this host (see AGENTS.md)" >&2; exit 2; }
cd "$ROOT" && exec go run ./cmd/maintenance-sweep --repo "$ROOT" "$@"
