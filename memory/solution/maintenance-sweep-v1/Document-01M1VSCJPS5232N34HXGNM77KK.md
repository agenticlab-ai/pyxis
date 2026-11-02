# Maintenance sweep v1 — solution shape

| Field | Value |
|---|---|
| **Design** | maintenance-sweep-v1 |
| **Status** | authoritative |

**The homes** — the binary, package, policy, and shim locations the design's roles resolve to; the config surface gains exactly one path key (design §3.1, §3.4; A-8's no-write-client import audit applies to the one package — DINV-1):

```text
cmd/maintenance-sweep/            # the thin main (verb-less: one pass per invocation)
internal/sweep/                   # the one focused package: policy, detectors, report, pass
docs/policy/maintenance-sweep.yaml  # the single policy file (waivers + bounds)
.pyxis/scripts/maintenance-sweep.sh # the delegate-only scheduler shim (+ its test under
                                    # .pyxis/scripts/tests/test-maintenance-sweep-shim.sh)
.pyxis/config.yaml                # gains: paths.maintenance_policy (default the file above)
```

**The CLI contract** — fail-closed invocation per DINV-3 (unknown flag / flag-like positional → usage + exit 2 before any detector; incident I3), exit codes per the design §3.3 and A-6:

```text
maintenance-sweep [--repo <root>]
  stdout: ONE maintenance-report/v1 JSON object (the machine face; EVT-1's record)
  stderr: the human face (per-finding lines; explicit "0 findings" per clean detector)
  exit 0: clean         — every detector ran, zero findings, zero faults
  exit 1: findings      — at least one finding emitted (a coexisting detector fault
                          stays named in the report — the design §3.3 ranking: findings win)
  exit 2: fault         — zero findings AND any fault: usage/misparse, unreadable repo state, absent toolchain,
                          malformed policy (incl. reasonless waiver), or any detector fault (with zero findings — a partial-fault pass with findings exits 1 per the ranking)
  never: any write beyond stdout/stderr (DINV-1) — no repo, tracker, or network mutation
```

**The report machine face** — `maintenance-report/v1`, one object per pass; deterministic field order, fixed detector order, lexicographic subjects, no wall-clock field anywhere (DINV-2; A-5's golden byte-identity), faults named in-band so a partial pass is honest (DINV-4), the evaluated state pinned with the design §3.3's dirty annotation (a non-clean tree never certifies silently):

```json
{
  "schema_version": "maintenance-report/v1",
  "evaluated_state": { "commit": "<HEAD sha of the invoked checkout>", "dirty": false },
  "outcome": "clean|findings|fault",
  "detectors": [
    {
      "name": "unwired-packages",
      "findings": [
        {
          "schema_version": "finding/v1",
          "detector": "unwired-packages",
          "subject": "<package import path>",
          "evidence": "reachable from no binary under cmd/ (import-graph union)",
          "remedy": "wire it into a shipped binary, or add a reasoned package waiver"
        }
      ],
      "waivers_applied": [
        { "kind": "package", "subject": "<import path>", "reason": "<the recorded reason>" }
      ],
      "fault": ""
    },
    {
      "name": "touches-underdelivery",
      "findings": [
        {
          "schema_version": "finding/v1",
          "detector": "touches-underdelivery",
          "subject": "<plan>/<task-id>: <path token>",
          "evidence": "the frozen plan's task names the surface; no file or directory prefix matches it in the evaluated state",
          "remedy": "reconcile: deliver the surface, correct the plan by its own channel, or add a reasoned plan waiver (advisory signal — planning metadata, never a breach verdict)"
        }
      ],
      "waivers_applied": [
        { "kind": "plan", "subject": "<plan>", "reason": "<the recorded reason>" }
      ],
      "fault": ""
    },
    {
      "name": "rollup-staleness",
      "findings": [
        {
          "schema_version": "finding/v1",
          "detector": "rollup-staleness",
          "subject": "<rollup path>",
          "evidence": "newest capability-path commit is <N days> newer than the rollup's newest commit (bound: <bound>)",
          "remedy": "refresh the rollup to the shipped structure, or retune the bound in the policy file"
        }
      ],
      "waivers_applied": [],
      "fault": "<empty, or the named detector fault — the pass continued (UB-3)>"
    }
  ]
}
```

**The policy file** — `maintenance-policy/v1`, closed parse (unknown keys are errors — UB-2), every waiver reason-required (a reasonless entry refuses to load — DINV-3; FR-10's visibility contract is the `waivers_applied` surfacing above, DINV-5):

```yaml
schema_version: maintenance-policy/v1   # closed parse: unknown keys are defects (UB-2)
staleness_bound: 336h                   # rollup-staleness threshold; default 14 days when absent
waivers:
  packages:                             # unwired-packages suppressions — each visible when applied
    - package: "internal/worktracker/conformance"
      reason: "test-harness library: consumed by adapter suites, deliberately no binary"
  plans:                                # touches-underdelivery suppressions (in-flight plans)
    - plan: "installer/001"
      reason: "frozen 2026-07-19, fleet executing — surfaces legitimately pending"
# absent file  = empty policy + default bound (legal: the sweep is advisory)
# malformed    = UB-2 fault: named defect, exit 2, zero finding output (DINV-3)
```

**The detector reads** — every input is a read of a goal object, behind an injectable runner seam; the temp-git replay fixtures substitute the repo, the runner substitutes the toolchain (design §3.9; A-2/A-3/A-4's replay bar):

```text
unwired-packages:      go list ./...  +  go list -deps ./cmd/...   (module-prefix filtered)
touches-underdelivery: docs/plans/*/NNN-plan.json  →  tasks[].touches, middot-split,
                       path-like tokens only (contains "/" or ".")  →  stat/prefix check
rollup-staleness:      git log -1 --format=%ct -- <capability_paths from .pyxis/config.yaml>
                       vs  <paths.rollup_prd | paths.rollup_techspec from the same surface>
fixtures:              hermetic temp-git repos; the unwired fixture reproduces the
                       historical package names; the Touches fixture names the two
                       never-shipped wiring surfaces verbatim; staleness constructed
```
