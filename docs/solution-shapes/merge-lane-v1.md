# Merge lane v1 — solution shape

| Field | Value |
|---|---|
| **Design** | merge-lane-v1 |
| **Status** | authoritative |

**The risk-policy file** — one closed-schema, operator-ratified file; the config surface carries only its path key (`paths.risk_policy: docs/policy/risk-policy.yaml` — final key/home fixed at implementation). Floor before classes; the self-unlock minimum is design-bound and validator-enforced — a policy whose floor fails to cover it refuses to load, and loading verifies the trailer-ratified mainline lineage; each AUTO class is a data-defined predicate; the kill switch and activation live here as data (DINV-2, DINV-6, DINV-8, DINV-9, A-4):

```yaml
schema_version: risk-policy/v1        # closed parse: unknown keys are defects (DINV-8)
ratified_by: "@mdarabi"               # + the operator trailer on every changing commit;
                                      #   loaded only from the mainline lineage the trailer
                                      #   check gated (DINV-9) — a fingerprint off that
                                      #   lineage refuses to load (DINV-8)
floor:                                # DINV-2: evaluated first; any hit -> HUMAN, terminal.
  hard_gated_paths:                   # path globs, matched against the whole diff.
    # The set below is the SELF-UNLOCK MINIMUM — design content (design §3.1): the
    # shared validator refuses any policy whose floor fails to cover it (DINV-6).
    # Ratification may WIDEN the floor; it can never shrink it below this set.
    - ".github/workflows/**"          # control plane
    - ".githooks/**"
    - "docs/product-vision.md"        # governing documents
    - "docs/architecture-principles.md"
    - "docs/observability/incidents.md"
    - "docs/policy/**"                # the risk policy + activation state (DINV-6) — NOT the lane
                                      #   state, which lives on the dedicated lane-state ref below,
                                      #   never in this diff-borne working-tree path
    - "AGENTS.md"                     # the operating manual — the merge convention's home (DINV-6)
    - "CLAUDE.md"                     #   …and its symlinked face
    - ".pyxis/**"                     # the agent-control surface: drive + reviewer guides, skills,
                                      #   templates, the pipeline map, the authority contract +
                                      #   agent home (DINV-6)
    - ".claude/**"                    # vendor-dir pointers into it
    - ".codex/**"
    - "cmd/lane/**"                   # the lane's OWN code only (DINV-6) — the classifier, the
    - "internal/lane/**"              #   validator, the acting glue, AND the review-verdict check
                                      #   arm DINV-3 trusts; final package names at implementation;
                                      #   engine/feature code outside the lane's packages is
                                      #   deliberately floor-clear (design §3.1)
    - "go.mod"                        # the lane's OWN build inputs (DINV-6, DINV-9): a dependency
    - "go.sum"                        #   substitution here poisons the next mainline build of the
                                      #   lane's binaries; NO AUTO code class may claim these in its
                                      #   perimeter (validator refuses — §3.6), closing the two-step
                                      #   ratify-then-plant path
  hard_gated_classes:                 # diff-derived classes, never author-declared
    - governing-doc-edit
    - ledger-frozen-entry-edit
auto_classes: []                      # closed set; EMPTY until the operator ratifies (FR-9).
  # Each ratified class is data: name + whole-diff path perimeter + precondition
  # flags; membership = the ENTIRE diff inside ONE class's perimeter (design §3.1).
  # NO code class may claim the lane's build inputs (go.mod/go.sum) in its
  # perimeter — they are the lane's trust base and the validator refuses a policy
  # that does (design §3.1/§3.6, DINV-6); a fleet PR touching them floor-hits.
  # The basis's recorded recommendation, as it would land at ratification
  # (go.mod/go.sum carved out of the perimeter per the trust-base rule above —
  # a fleet PR that touches only source under cmd/internal is floor-clear; one
  # that also bumps a dependency floor-hits HUMAN):
  #   - name: fleet-task              # fleet task PRs with green checks
  #     paths: ["cmd/**", "internal/**"]   # NOT go.mod/go.sum (floor-gated)
  #     requires_reviewer_verdict: false
  #   - name: drive-doc               # drive doc PRs with green gates + approving reviewer verdict
  #     paths: ["docs/**"]
  #     requires_reviewer_verdict: true
thresholds:
  shadow_window_min_proposals: 20
  agreement_bar: 0.95
activation:
  enforce: false                      # shadow by default; true only by operator ratification
  backstop_change_ref: ""             # the operator's recorded org-backstop act (FR-9 reads it);
                                      #   enforce:true with this absent => shadow + a NAMED
                                      #   activation-incomplete defect at every state read (A-3)
kill_switch: false                    # true -> HUMAN-only lane in one data change (A-4)
```

**The lane verdict record** — one structured block per evaluation, riding the proposal, append-only across evaluations; replaying the pinned inputs — the base+head pair the diff derives from, the policy version, the check states — reproduces the verdict byte-identically, merge or base-move notwithstanding; a class's reviewer-verdict precondition enters through the check-state snapshot, never parsed from the proposal (DINV-1, DINV-3, DINV-4, A-1, A-2, A-5):

```json
{
  "record": "lane-verdict/v1",
  "proposal": "<medium-neutral proposal id>",
  "head_revision": "<sha evaluated>",
  "base_revision": "<merge-base sha the diff derives from — pinned for replay>",
  "policy_version": "risk-policy/v1@<content fingerprint>",
  "check_states": [
    { "context": "docs-gate", "state": "green" },
    { "context": "review-verdict", "state": "green" }
  ],
  "floor_hits": [],
  "class": "drive-doc|fleet-task|<ratified class>|none",
  "lane": "AUTO|HUMAN",
  "reason": "<named: floor class hit, out-of-class hunk, fault, or clean>",
  "mode": "shadow|enforce"
}
```

**The act self-description** — the approval and merge the machine identity submits; durable in the medium's history and cold-readable years later; the consumed reviewer verdict is pinned where the class carries one (DINV-3, A-3, A-6):

```text
approved-by: <the dedicated machine identity>       # distinguishable from any human
head-revision: <sha the verdict evaluated>          # a post-evaluation push voids the act (DINV-3)
policy-version: risk-policy/v1@<fingerprint>
verdict-record: <pointer to the evaluation block>
reviewer-verdict: <consumed ref: report + verdict + subject fingerprint> | n/a
                                                    # pinned where the class carries one (DINV-3)
goal-verify: pass|fail                              # fail => lane halted, no further AUTO acts (DINV-7)
```

**The classifier verb interface** — the single lane predicate; hermetic, deterministic, fail-closed; the binary and the policy load from the trusted mainline context, never the proposal's tree (DINV-1, DINV-4, DINV-8, DINV-9, A-1):

```text
lane classify --proposal <id> --base <sha> --head <sha> --policy <path> --checks <snapshot> [--mode shadow|enforce]
  exit 0: verdict record emitted (JSON, schema above)
  exit 2: refusal — misparse, malformed/absent policy, uncovered self-unlock floor,
          off-lineage (unratified) policy, unclassifiable input => lane HUMAN, defect named
  inputs: binary + policy from the TRUSTED MAINLINE context (DINV-9); the proposal
          contributes only its (base, head) diff, consumed as data
  never:  network writes, credential reads, medium mutations (classification is pure)
```

**The credential, provenance, and state boundary** — where the identity lives, whose code runs, who writes the halt and where, and who can reach what; the drive-side probe fixture must fail to cross the line (DINV-5, DINV-7, DINV-9, A-6, A-8):

```text
executing medium's secret store          repo working tree / any drive session
  └─ machine-identity credential           └─ risk policy (read)
     · readable only by the lane's           └─ verdict records (read)
       acting workflow step                  └─ lane state (read: shadow|enforcing|halted|killed)
     · never exported to jobs the            └─ NO credential, NO acting path (DINV-5)
       drive can influence
trusted context (DINV-9): the acting workflow's logic, the classifier binary, the
review-verdict check arm whose state DINV-3 consumes, and the policy load from the
mainline; the evaluated proposal contributes only its diff; no proposal-derived
code — and no proposal-substituted dependency — executes or is built in any step
of the acting job or the trusted check arm. The arm's code home and the lane's
build inputs (go.mod/go.sum) are in the self-unlock minimum above (DINV-6), so a
diff touching them floor-hits HUMAN under the mainline policy even inside a
ratified code class — the two-step ratify-then-plant path is closed.
the halt record (DINV-7): written SYNCHRONOUSLY by the acting workflow step — as
the machine identity — at goal-verify failure, to the dedicated lane-state ref
(repo-resident, outside the protected branch namespace; final ref name at
implementation), BEFORE any further AUTO act; every act gate reads it first.
write/clear authority is ENFORCED, not merely asserted (branch protection does
not guard a ref outside the protected namespace):
  (1) a platform REF-PROTECTION restricts the lane-state namespace to the machine
      identity — an activation-time operator precondition beside the backstop ref
      (read, never performed by this design — FR-9's pattern); absent under
      enforce:true it surfaces a NAMED activation-incomplete defect at every read.
  (2) the act gate honors a "cleared" state only after validating the record's
      writer IS the machine identity AND each clear entry cites a human
      disposition recorded on the affected drive's delivery ledger (the named
      recording surface); a non-machine write or an uncited clear is refused —
      a forged clear or ref deletion does NOT un-halt the lane (A-6's forged-clear
      red fixture). Only a delivery-ledger-recorded human disposition clears it.
lane state resolution (NFR-5, from the repo alone — the policy file + the
lane-state ref): killed if kill_switch; halted if an unresolved halt record
exists (validated per (2) above — a forged/uncited clear leaves it halted);
enforcing if activation.enforce + the backstop ref present + the lane-state
ref-protection present; else shadow — and enforce:true with the backstop ref
OR the ref-protection absent additionally surfaces a NAMED activation-incomplete
defect at every state read (FR-9's refusal names the missing act). One data
change flips killed (A-4).
```
