#!/bin/sh
# wake-tick — the host scheduler's entry to the wake channel's bounded
# backstop pass (wake-channel-v1 §3.2; plan T8's contracted shim; #515).
# Bash is admissible here as the scheduler-integration exception: launchd and
# cron exec a script line, which must run before any binary exists. It is
# delegate-only — every flag passes through to the Go binary; the front-door
# CLI argv follows `--` and the tick appends the normalized wire signal
# (`wake:<slug>:<reason>`) as its one argument (DINV-1).
#
# Scheduler binding (the operator installs; the repo never does — AP-19's
# nothing-resident posture is the binary's; the schedule is the host's):
#   cron:    */60 * * * *  /path/to/repo/.pyxis/scripts/wake-tick.sh --gh-repo owner/name -- claude -p
#   launchd: a StartInterval agent whose ProgramArguments are this script +
#            the same arguments (interval = channel.tick_interval).
set -e
ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
[ -n "$ROOT" ] || ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
command -v go >/dev/null 2>&1 || { echo "wake-tick: no Go toolchain on this host (see AGENTS.md)" >&2; exit 2; }
cd "$ROOT" && exec go run ./cmd/wake-tick --repo "$ROOT" "$@"
