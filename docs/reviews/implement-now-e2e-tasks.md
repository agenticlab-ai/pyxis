# Review: implement-now-e2e-tasks — design-review report

| Field | Value |
|---|---|
| **In one line** | approve-with-advisories after two rounds — round 1 raised eight blocking findings, led by a hermeticity admission that named no action against this repo's suite-level check list, a self-contradictory battery placement that falsified its own invariant, and an executor-written basis artifact the workspace would sweep into the very diff the mandate gate then rejects; all twelve are applied, with `FND-4` closed by declaring the mocked-route contract check absent rather than asserting it. |
| **Relevant-when** | auditing or consuming the review of `implement-now-e2e-tasks`; asking why it was bounced; tuning tech-design-review calibration |
| **Subject** | design:implement-now-e2e-tasks · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · guide 2026-07-28 · the four lenses ran as blind fresh-context sibling sessions seeded only with the stage context pack and the artifacts; adversarial survival, deduplication, and this report were assembled by the session that authored the subject — treat the *selection* as non-independent, the *generation* as independent |
| **Date(s)** | 2026-07-28 |
| **Verdict** | approve-with-advisories |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — The hermeticity admission names no action against this repo's check list

- **Severity**: blocking (CAL-4 — the gap is load-bearing for #DINV-6 and A-7)
- **Lens**: feasibility-completeness
- **Evidence**: design:§3.6 — "A stable verdict is what **admits** the authored check to the repo's canonical check list; an unstable one is never wired in." — vs — `.pyxis/config.yaml` `code-gate:`, whose entries are suite checks (`test-race: go test -race ./...`), the single-source list `code-gate-mirror` §2 defines. A newly authored Go test is inside that predicate from the moment the file exists; there is no separate act by which a check is "wired in", and therefore none to withhold. #DINV-6's "never wired into required checks" and A-7's "a rejected test is never wired into required checks" both name an action the surface does not have.
- **Acceptance condition**: State what admission resolves to for a repo whose canonical list is suite-scoped — quarantine, deletion, a skip marker, an exclusion the list honors, or a per-check entry model — and restate #DINV-6 and A-7 against that mechanism so both name a testable act.
- **Disposition**: applied

### FND-2 — The battery is placed both after and before drive-to-green; #DINV-7 is false under the ordering §3.1 draws

- **Severity**: blocking (CAL-1)
- **Lens**: coherence *(independently raised by upstream-alignment and feasibility-completeness)*
- **Evidence**: design:§3.1 — the diagram `G[drive to green — unchanged] --> M{mandate + route admission} ... M -->|passes| P{hermeticity admission}`, with the prose "Both admissions sit **after genuinely-green and before succeeded**" — vs — design:§3.6 "**Decision:** the battery runs **before the self-heal loop**, and a non-hermetic verdict never enters it" and #DINV-7 "a non-hermetic verdict consumes **zero** self-heal iterations: the battery precedes the loop". `internal/executor/worker/worker.go:251-289` admits no path between the check read and `greenDriver.Drive`; a flaky authored test reds inside drive-to-green and consumes self-heal attempts (`internal/executor/drivetogreen/heal.go:240`) long before §3.1's battery is reached.
- **Acceptance condition**: Choose one placement and state it identically in §3.1, §3.6, #DINV-7, §3.8 and A-7. If the battery fires on the authored test in the worktree before the first publish, the §3.1 diagram must show it there and drop "drive to green — unchanged" on the e2e path; if it genuinely runs post-green, #DINV-7's "zero" must be restated as what that placement delivers.
- **Disposition**: applied

### FND-3 — The executor's own artifact is swept into the proposal, where the scope diff then rejects it

- **Severity**: blocking (CAL-4 — self-refuting against A-5 and #DINV-5)
- **Lens**: feasibility-completeness *(the binding half independently raised by contract-quality)*
- **Evidence**: design:§3.4 — "It is excluded from the proposal's diff: it is executor-written context, not the worker's deliverable" and design:§3.5 — "The perturbation is confined to the task's own worktree and is never proposed" — vs — `internal/executor/workspace/workspace.go:346`, `runGit(ctx, d.dir, "add", "-A")` inside `commitPending`, reached from `Push` and `Checkpoint`. The artifact is written before launch and therefore before push, so it is staged onto the head the proposal opens from. §3.5 then withholds the succeeded class for any changed path outside the task's declared surface — and the executor's own file is by construction outside it, so every recognized e2e task fails its own mandate admission and fires §3.10's loudest alert on a file the executor wrote.
- **Acceptance condition**: Name the mechanism that keeps executor-written worktree content out of the published head — a path outside the worktree, a worktree-scoped exclude the executor owns, or a pathspec-scoped commit — state how it composes with concurrent workers, and bind the exclusion on #DINV-4 or a §6 row rather than asserting it in rationale prose.
- **Disposition**: applied

### FND-4 — The mocked-route double-contract check is bound three times and mechanized nowhere

- **Severity**: blocking (CAL-2 — a critical-risk fit criterion of a cited `Serves` anchor)
- **Lens**: upstream-alignment *(raised by all four lenses)*
- **Evidence**: design:§3.5 — "A `mocked` scenario additionally admits doubles only over the surfaces its recorded mock strategy names, and each double is checked against the real dependency's observed contract", restated in A-6 and #DINV-5 — vs — `prd:implement-now/FR-31` (Must, critical), whose fit criterion is "each double is checked against the real dependency's observed contract, and a stub encoding a wrong guess about that contract fails rather than going green". The design names exactly two mechanical checks, and neither reaches this: falsification perturbs the *subject*, which a mocked scenario replaces by design, so a wrong-guess double still passes. The only other route is inspecting for doubling constructs, which the same subsection explicitly rejects. `internal/e2e/docclass` parses Mock strategy as raw body lines with no per-surface grammar, and §3.12 lists no fixture for it.
- **Acceptance condition**: Either give the clause a mechanism in §3 — how doubled surfaces are enumerated, and what "checked against the real dependency's observed contract" executes — with a §3.12 seam and a wrong-guess-stub fixture; or park it in §7 with an owner and a risk row as Q2 parks the analogous falsification gap, and narrow A-6 and #DINV-5 to what the two named admissions actually deliver.
- **Disposition**: applied — via the second branch. §3.5 now states the obligation is carried to the worker in its basis and is **not** mechanically admitted; §7 Q4 owns the mechanism with @mdarabi named; R8 carries the risk as open-and-stated rather than mitigated; A-6 is narrowed to what falsification and the manual-human route deliver. The gap is additionally parked in the feeding record as `design-brainstorm:implement-now-tech-design-2#P-1` with a `/prd-amend` route named should the check prove unmechanizable from recorded data.

### FND-5 — The slot join is not the pure re-run §3.2 claims; its second input is recorded nowhere

- **Severity**: blocking (CAL-4 — silent mis-briefing that passes #DINV-1 and A-1)
- **Lens**: feasibility-completeness
- **Evidence**: design:§3.2 — "the join is not re-derived: the same rule that decided which slot is the harness and which scenario each task carries is asked again, so the plan's authored graph and the executor's reading of it cannot drift apart (AP-11)" — vs — `internal/e2e/plan/plan.go:90`, `Decide(doc *docclass.Doc, harnessExists bool)`, whose package doc states harness existence "is not this package's call — a repo-inspection judgment the skill makes and passes in". Slot numbering shifts by one on that flag. The design never says how the executor recovers it, and the failure is silent: every task still resolves to exactly one scenario, just the wrong one, so #DINV-1 holds and A-1 reads green while the worker is briefed with another scenario's flow, route, and mock strategy.
- **Acceptance condition**: State how the flag is recovered from recorded data at consumption time — comparing the frozen plan's task count against the document's scenario count at the freeze revision is the obvious candidate — and make a mismatch a refusal under `UB-14` rather than a guess.
- **Disposition**: applied

### FND-6 — The scope diff has no path-bearing surface to read

- **Severity**: blocking (CAL-4 — load-bearing for A-5 and #DINV-5)
- **Lens**: feasibility-completeness
- **Evidence**: design:§3.5 — "the proposal's changed paths must lie within the task's declared surface. A change to the behavior under test outside the charter's stated scope…" — vs — `internal/plangate/plan/reg_durability.go:12-27`, the durability lint that fails any charter region matching a path pattern, with the remedy "plan-internal surfaces belong in Touches". A charter provably cannot state a path. The only path-bearing field is `Touches`, documented at `internal/plangate/plan/plan.go:82` as "plan-internal: overlap sequencing only", optional (`—` accepted), coarse, and absent from `port.Task`.
- **Acceptance condition**: Name the field the scope diff reads and its semantics, and rule the empty case explicitly — a task with no declared surface either has an empty one (every change out of scope) or an unbounded one (the gate silently off). Repurposing an overlap-sequencing hint as a mandate boundary is a decision that belongs in §3, not an implementation detail.
- **Disposition**: applied

### FND-7 — The document-read port is placed on the Worker, but the read precedes the Worker

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design:§3.3 — "the read is a **consumer-side port** … and it joins the Worker's existing port set rather than standing beside it", with design:§3.8 — "the remote dispatch adapter, which reuses this consumption path unchanged because none of it lives in the Dispatcher" — vs — design:§3.2 and #DINV-2, which require resolution before claim with "zero leases acquired, zero worktrees created". `implement-now` §3.6 allocates Workers only after the claim step, so a port on the Worker cannot perform a read that must precede it. The record itself places the seam elsewhere: `design-brainstorm:implement-now-tech-design-2#S-2` calls it "the executor's first inbound document surface", not a Worker port.
- **Acceptance condition**: Name the component that runs pre-claim resolution and holds the read — the frontier walk, which `implement-now` §3.1 assigns to the Dispatcher — and correct §3.3. Then either drop §3.8's claim that a remote adapter reuses the path unchanged, or state how a second Dispatcher adapter inherits the stage without a second implementation of the join.
- **Disposition**: applied

### FND-8 — AP-27 is silently omitted while the design makes list admission a per-scenario-task act

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:§3.6 — "A stable verdict is what **admits** the authored check to the repo's canonical check list", with §7 Q1 keeping "an extension of the canonical check list" a live landing option, and the Principles section closing "**Deviation:** none." — vs — AP-27, "Extension is by addition; a shared append point is a design defect. When a plan fans one 'add an X' out to N sibling tasks, each addition must land as pure addition — a new file, a new self-contained entry — with zero edits to any line a sibling could also be editing." An e2e plan emits one sibling task per scenario by contract, and the check list is authored per-repo config — precisely the fan-out AP-27 names.
- **Acceptance condition**: Cite AP-27 in the Principles section — honored or deviated-with-reason — and bind Q1's resolution to it: admission lands as pure addition, never as N sibling tasks editing one authored list.
- **Disposition**: applied

### FND-9 — EVT-12's "forever in git evidence" retention is not delivered, and AP-28 is uncited

- **Severity**: advisory (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:§3.7 — "The event rides the existing normalized stream to both existing sinks; no new reporting surface appears", with #DINV-8 binding only "append-only" — vs — `prd:implement-now` §5.4, whose e2e-origin-decision row states retention "forever in git evidence", and AP-28, which requires improvement fuel be "kept as a committed artifact in the repository, not written to a gitignored scratch path, an ephemeral CI-run artifact, or a log that ages out". `implement-now` §3.9's two sinks are a live in-session view and argus's delivery journal; neither is stated to be a committed artifact.
- **Acceptance condition**: State where an `EVT-12` record comes to rest as a committed append-only artifact, or cite the design that owns that surface; and cite AP-28 as honored (naming the store) or deviated-with-reason.
- **Disposition**: applied

### FND-10 — Frozen prose states volatile facts, including inside a Decision callout

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:§3.2 (a **Decision:** callout) — "re-running the closed task-graph rule that authored the plan (`internal/e2e/plan`)"; design:§3.8 — "the Worker's five existing ports"; design:§3.3 — "the executor's existing five-port precedent" — vs — the template rule "Frozen prose states no volatile facts. Never write a count, range, enumeration, or repo path a later change can falsify — name the concept and point at the live surface that owns the current value." The count is not merely latent: this design adds a port, so "five existing ports" is resolvable only by a reader who already knows which five predate the document.
- **Acceptance condition**: Replace the package path with the concept and its owning surface, and drop the numeral from both port references.
- **Disposition**: applied

### FND-11 — §3.12's dependency bins are inverted, and the new port runs against the executor's own injection precedent

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:§3.12 bins the document read "remote-but-owned" and the hermeticity battery "in-process" — vs — the reality that a read at a revision against the local checkout is local-substitutable (`internal/executor/workspace/workspace_test.go` already drives real git hermetically), while the battery and falsification execute real subprocesses under a race detector, denied network, and a perturbed clock. On the port itself, the executor's closer precedent is injection, not port growth: `internal/executor/drivetogreen/heal.go:20-41` states that `LogFetcher` and `Pusher` are "INJECTED rather than folded into the frozen port.CodeHost so the self-heal loop stays hermetic-testable and the port surface does not grow."
- **Acceptance condition**: Re-bin both seams, and either justify the port against that precedent by naming what varies across two *production* readers, or take the injection shape and record the deviation. §3.12 says "the bin decides the strategy", so a mis-bin produces a suite that never exercises the seam the two new gates stand on.
- **Disposition**: applied

### FND-12 — R3 verifies the fixtures rather than the battery, and parallel workers are both its load source and its victims

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:§5 R3 — mitigation "each fixture's expected verdict is asserted stable before the fixture is used to assert instability", verification "the hermetic control in the seeded-fixture run" — both about the corpus, never about the battery's verdict on a real authored test — vs — `internal/executor/dispatch/dispatch.go:35`, `DefaultWidth = 4`: up to four Workers share one host, each under a wall-clock budget. §3.6 perturbs with "concurrent load" and "denied network" while siblings fetch, call the code host, and run models; nothing scopes the perturbation to the task's own process tree.
- **Acceptance condition**: Scope each perturbation to the task's own process where it can be, say so explicitly where it cannot, and give R3 a verification that measures the production battery's own verdict stability. Add the sibling-interference case to §5 or §3.11 — a battery that denies network or generates load can push a concurrent Worker past its budget into a spurious respawn.
- **Disposition**: applied

## Round 2

*Scope-locked to the spans round 1's findings cite. No new findings: nothing on unchanged content, and the changed spans introduce no defect this reviewer can name. Two dispositions are recorded as structural rather than mechanical and are called out because a reader should weigh them:*

- **FND-4** closed by declaration, not by mechanism. The mocked-route contract check is now absent-by-design with an owner (§7 Q4), an open risk (R8), and a parked product gap in the feeding record. That is the acceptance condition's own second branch, but it leaves a critical-risk fit criterion of `prd:implement-now/FR-31` undelivered by this design. If Q4 concludes it is unmechanizable from recorded data, the resolution is an amendment, not a design change.
- **AP-28** is now a recorded deviation rather than a silent omission (FND-9), which is what the principles contract asks — but the deviation is real: executor events still have no committed home until the routing supersession provides one (§7 Q5).

*Independence note, restated: round 1's findings were generated by four blind fresh-context lens sessions. This round-2 pass verified each acceptance condition against the revised document but was performed by the authoring session, so it is a disposition check, not an independent re-review.*

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-28 |
| 2 | approve-with-advisories | 2026-07-28 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 8 blocking · 4 advisory
- **Dispositions**: 12 applied · 0 declined · 0 withheld-as-violation
