# The maintenance sweep's policy — the single closed-parse file the sweep reads
# (maintenance-sweep-v1 §3.4; the authoritative shape's seed). One additive
# config path key names its home (`paths.maintenance_policy`, default this file);
# the closed body carries the staleness bound and the two waiver classes. Unknown
# keys, a wrong schema version, or a reasonless waiver refuse to load with a named
# defect and the fault exit code — never a lenient parse (DINV-3, prd UB-2).
#
# WHY WAIVERS EXIST (FR-10, DINV-5): a waiver suppresses a finding the detector
# would otherwise emit, and every applied waiver is surfaced back in the report
# with its recorded reason — suppression is always visible and attributable in
# history. A reasonless entry is a defect. Retuning the bound or adding/removing a
# waiver is a review-routed PR against this file, never a code change.
#
# THE FIRST LIVE TRIAGE (plan T6, prd NR-1/A-5): these entries were triaged from
# the first live sweep on mainline. Every remaining EMITTED finding is a verified
# truth; every suppression below is a reasoned waiver. The one finding left
# standing on purpose — internal/lane/state, a merge-lane plan-T5 organ reachable
# from no cmd/ binary (cmd/lane's `state` verb still resolves through
# internal/lane/acting, not this newer package) — is a genuine "landed but not
# wired" instance the sweep exists to surface. It is deliberately NOT waived: the
# operator wires it or waives it, but the signal stays honest until then.
schema_version: maintenance-policy/v1
staleness_bound: 336h   # rollup-staleness threshold — 14 days (the design default)
waivers:
  packages:
    # --- impl-review: a skill runtime, not a binary (impl-review-v1 §3, #D-3) ---
    # impl-review realizes as the /impl-review skill spawning LLM subagents; a
    # pinned reviewer binary was explicitly rejected. Its Go packages are the
    # skill-runtime + schema library and its test harness, deliberately reachable
    # from no cmd/ binary — library-only under maintenance-sweep-v1 R2.
    - package: internal/implreview/band
      reason: impl-review skill-runtime library (impl-review-v1 D-3 rejected a pinned binary) — deliberately reachable from no cmd/ binary
    - package: internal/implreview/calibration
      reason: impl-review skill-runtime calibration-as-data library — deliberately reachable from no cmd/ binary
    - package: internal/implreview/charter
      reason: impl-review skill-runtime library — deliberately reachable from no cmd/ binary
    - package: internal/implreview/checkpoint
      reason: impl-review checkpoint-harvest library — deliberately reachable from no cmd/ binary
    - package: internal/implreview/e2e
      reason: impl-review end-to-end test harness — exercised by go test, deliberately no binary
    - package: internal/implreview/eval
      reason: impl-review eval-suite library — consumed by the eval CI and tests, deliberately no binary
    - package: internal/implreview/floor
      reason: impl-review skill-runtime library — deliberately reachable from no cmd/ binary
    - package: internal/implreview/harvest
      reason: impl-review measurement-harvest library — deliberately reachable from no cmd/ binary
    - package: internal/implreview/lens
      reason: impl-review lens-runtime library — deliberately reachable from no cmd/ binary
    - package: internal/implreview/pack
      reason: impl-review context-pack library — deliberately reachable from no cmd/ binary
    - package: internal/implreview/persist/github
      reason: impl-review GitHub persistence adapter — consumed via port by the skill and conformance tests, deliberately no binary
    - package: internal/implreview/protocol
      reason: impl-review report-schema/protocol library — deliberately reachable from no cmd/ binary
    - package: internal/implreview/runtime
      reason: impl-review skill-runtime library — deliberately reachable from no cmd/ binary
    - package: internal/implreview/verdict
      reason: impl-review verdict library — deliberately reachable from no cmd/ binary
    # --- test-harness / fixture / library packages, deliberately no binary ---
    - package: internal/argus/corpus
      reason: argus EVAL corpus and defect fixtures (argus-artifacts) — consumed by the argus eval and test suites, deliberately no binary
    - package: internal/docgate/feedback/adapters/stub
      reason: stub CaptureSource adapter (a test double) for the review-feedback conformance and tests — deliberately reachable from no binary
    - package: internal/executor/e2e
      reason: implement-now end-to-end test harness — exercised by go test, deliberately no binary
    - package: internal/verify/matrix
      reason: implementation-verification matrix and catch-suite test drivers — consumed by the verify test suites, deliberately no binary
    - package: internal/worktracker/conformance
      reason: local-work-tracker adapter conformance library — consumed by the WorkTracker adapter suites, deliberately no binary
  plans:
    # Frozen plans whose touches-underdelivery findings are advisory reconciliation
    # signals, not genuine under-delivery (prd FR-9, maintenance-sweep-v1 §3.2, R3):
    # the surfaces shipped (under their base paths, or relocated/renamed at
    # implementation) or are legitimately in flight. The touches detector evaluates
    # each Touches token literally, so a token carrying a descriptive parenthetical
    # or a pre-implementation path cannot stat-match even when the surface exists.
    - plan: installer/001
      reason: frozen 2026-07-19 and fleet-executing — the installer surfaces (cmd/pyxis, internal/wire, internal/converge, internal/relmanifest, internal/resolve, internal/installer) are legitimately pending; the in-flight-plan control (R1)
    - plan: implement-now/001
      reason: frozen and delivered (internal/executor/*, cmd/implement-now) — the touches tokens carry descriptive parentheticals and bare per-package test-file names the literal matcher cannot resolve; surfaces shipped under their base paths
    - plan: implementation-verification/001
      reason: frozen and delivered (internal/verify/*, cmd/codegate) — the touches tokens carry descriptive parentheticals and prose (fixtures, release-workflow) the literal matcher cannot resolve; surfaces shipped under their base paths
    - plan: workbench-ui/001
      reason: frozen and delivered under .pyxis/ui/* — the touches tokens name the pre-relocation ui/* paths; the feature shipped on the vendor-neutral agent surface (AP-25)
    - plan: review-feedback-capture/001
      reason: frozen and delivered as internal/docgate/feedback/* — the touches tokens name the pre-implementation internal/reviewfeedback and docs/feedback paths; the feature shipped under its final home
    - plan: argus-boundary-enforcement/002
      reason: frozen and delivered as cmd/argus-stamp + internal/argus/stamp — the touches tokens name the pre-implementation drivestamp surface; the feature shipped under the renamed home
    - plan: argus-unattended-drive/001
      reason: frozen and delivered — the touches tokens name relocated surfaces (scripts/wake-tick.sh lives at .pyxis/scripts/, the merge-lane workflow); surfaces shipped under their final homes
    - plan: contract-gate/001
      reason: frozen and delivered (internal/docgate/gate, a plan-new ledger amendment) — the touches tokens carry descriptive parentheticals and prose (the determinism/perf tests) the literal matcher cannot resolve; surfaces shipped
    - plan: impl-review/001
      reason: frozen and delivered as internal/implreview/* — the touches token names internal/implnow/verificationgate, a cross-feature reference; the reviewer shipped under its final home
