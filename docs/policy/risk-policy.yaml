# The merge lane's risk policy — versioned, closed-schema, operator-ratified
# data (merge-lane-v1 §3.6; the authoritative shape's seed). The config surface
# carries only this file's path (`paths.risk_policy`); every reader loads it
# through the one shared validator, which refuses a malformed, unknown-key,
# self-unlock-uncovering, build-input-claiming, or unratified policy — the lane
# then stays all-HUMAN naming the defect (DINV-8).
#
# RATIFICATION (the operator's act, never an agent's): this file is trusted
# only from its trailer-ratified mainline lineage — the latest mainline commit
# touching it must carry the `Governing-Change: operator` trailer (DINV-9).
# Until the operator lands that commit, the loader names the unratified-lineage
# defect and the lane runs all-HUMAN shadow — the designed bootstrap state.
# See docs/runbooks/merge-lane-activation.md.
schema_version: risk-policy/v1
ratified_by: ""
floor:
  hard_gated_paths:
    # The SELF-UNLOCK MINIMUM is design content (merge-lane-v1 §3.1): the
    # validator refuses any policy whose floor fails to cover it. Ratification
    # may WIDEN the floor; it can never shrink it below this set.
    - ".github/workflows/**"
    - ".githooks/**"
    - "docs/product-vision.md"
    - "docs/architecture-principles.md"
    - "docs/observability/incidents.md"
    - "docs/policy/**"
    - "AGENTS.md"
    - "CLAUDE.md"
    - ".pyxis/**"
    - ".claude/**"
    - ".codex/**"
    - "cmd/lane/**"
    - "internal/lane/**"
    - "go.mod"
    - "go.sum"
  hard_gated_classes:
    - governing-doc-edit
    - ledger-frozen-entry-edit
auto_classes: []
thresholds:
  shadow_window_min_proposals: 20
  agreement_bar: 0.95
activation:
  enforce: false
  backstop_change_ref: ""
kill_switch: false
