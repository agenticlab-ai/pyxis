# Merge lane — activation runbook

| Field | Value |
|---|---|
| **Audience** | the operator (@mdarabi) — every act below is an operator act |
| **Contract** | `docs/tech-design/merge-lane-v1.md` (frozen) · shape `docs/solution-shapes/merge-lane-v1.md` · prd:argus-unattended-drive FR-1..FR-9 |
| **Serves** | DOD-6 of prd:argus-unattended-drive — the flip decidable-on-sight |
| **Class** | runbook — descriptive, freely editable via PR; on conflict the cited contracts govern |

## What runs today (shadow, all-HUMAN)

Shadow evaluation runs host-side: `lane sweep --gh-repo agenticlab-ai/pyxis`
from a clone sitting at the mainline evaluates every open proposal — the
**trusted context** is the mainline checkout the binary and policy load from;
each proposal contributes only its `(base, head)` diff, fetched as git objects
(DINV-9). Each evaluation appends one `lane-verdict/v1` record to the
proposal's verdict-block comment (the `<!--lane-verdict-block/v1-->` marker);
an unchanged head is skipped, so the sweep is idempotent. No act ever occurs
in this deployment: no machine identity is wired (DINV-5), and the lane state
cannot resolve `enforcing` until every activation precondition below is
present (FR-9).

**The event-driven trigger is authored but not yet landed**: the exact
workflow content sits at `docs/runbooks/merge-lane.workflow.yml`, ready to
copy verbatim to `.github/workflows/merge-lane.yml`. Landing it requires a
`workflow`-scope push, which the machine identity deliberately lacks — an
operator-ownable act (issue #514 tracks it). Until then the sweep is the
stated sequential fallback and produces identical records.

The seeded policy `docs/policy/risk-policy.yaml` is **deliberately
unratified**: its mainline lineage carries no ratification trailer, so the
loader names the `lineage` defect and the lane runs all-HUMAN (DINV-8). That
is the designed bootstrap state, not a fault.

Read the lane's state any time:

```sh
go run ./cmd/lane state --repo .
```

Read a proposal's verdict history: the marker comment on the PR, or
`lane classify --base <sha> --head <sha>` locally to replay an evaluation
(byte-identical on pinned inputs — DINV-4).

## The shadow evidence bar (Phase B exit — DOD-2)

The lane earns its activation case mechanically, from records alone:

- ≥ `thresholds.shadow_window_min_proposals` (20) evaluated proposals, each
  carrying a recorded verdict;
- agreement between the lane's would-be routing and what humans actually did
  ≥ `thresholds.agreement_bar` (0.95);
- **zero** would-have-been-AUTO floor violations;
- zero acting side effects (structural: the shadow deployment cannot act).

## Activation — the operator's acts, in order

Nothing below is performed by any agent or workflow; the feature reads each act
as data and refuses enforcement while any is absent, naming the missing act
(FR-9, A-3).

1. **Ratify the policy lineage.** Land the risk-policy commit (content of your
   choice — the seeded floor is the design-bound minimum and can only widen)
   on `main` with the commit-message trailer:

   ```
   Governing-Change: operator
   ```

   The loader trusts the policy only when the working-tree content is
   byte-identical to mainline AND the latest mainline change to the file
   carries this trailer (DINV-9).

2. **Ratify the AUTO class set.** In the same (or a later trailered) policy
   commit, populate `auto_classes`. The recorded recommendation
   (brainstorm D-3, shape comment):

   ```yaml
   auto_classes:
     - name: fleet-task              # fleet task PRs with green checks
       paths: ["cmd/**", "internal/**"]   # NOT go.mod/go.sum (floor-gated)
       requires_reviewer_verdict: false
     - name: drive-doc               # drive doc PRs with green gates + approving reviewer verdict
       paths: ["docs/**"]
       requires_reviewer_verdict: true
   ```

   The validator refuses any policy whose AUTO code class claims the lane's
   build inputs (`go.mod`/`go.sum`) or whose floor uncovers the self-unlock
   minimum (§3.1's two-step ratify-then-plant closure).

3. **Flip the org backstop and record it.** Change the org/repo setting that
   today requires a human approval, and write the recorded act's reference
   into the policy: `activation.backstop_change_ref: "<link or ref>"`, plus
   `activation.enforce: true` — in a trailered commit. `enforce: true` with
   the ref absent stays shadow and surfaces a named activation-incomplete
   defect at every read (A-3).

4. **Protect the lane-state namespace.** Add the platform ref-protection
   restricting the lane-state ref namespace to the dedicated machine identity
   (DINV-7's write-authority enforcement). Under `enforce: true` with this
   absent, the lane stays shadow and names it.

5. **Seal the machine identity.** Store the dedicated machine identity's
   credential in the repository's Actions secret store, reachable only by the
   enforcing workflow (DINV-5), and wire `--machine <login>` in that
   workflow's invocation. The shadow workflow never passes it.

## The kill switch

One data change flips the lane HUMAN-only, no deploy (A-4):

```yaml
kill_switch: true
```

## Halt semantics (DINV-7)

A goal-verify failure (a lying merge badge — I2's class) writes a halt record
to the dedicated lane-state ref as the machine identity, before any further
AUTO act. Only a delivery-ledger-recorded **human** disposition clears it; the
act gate refuses a forged or uncited clear and treats ref deletion as still
halted.

## Debugging surface (§3.4)

- `lane state` — resolved state + every named defect (activation-incomplete,
  load defects).
- `lane classify --base <sha> --head <sha>` — replay any evaluation from
  pinned inputs.
- The verdict-block comment on each PR — the append-only evaluation history.
- The workflow run log — one evaluation per event, trusted-context checkout
  visible in the steps.
