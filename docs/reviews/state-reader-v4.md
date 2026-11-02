# Review: state-reader-v4 — design-review report

| Field | Value |
|---|---|
| **In one line** | Round-1 supersession review of state-reader-v4 over state-reader-v3 — the containment repair and its invariant set are sound and the basis is faithfully carried, but the seed left duplicated content, three §3 realizations of new invariants were missing, R2's latency claim is falsified by the durability model this document adds, and `Serves` claims two anchors whose enforcing half the design itself disclaims; six findings, all applied, approve. |
| **Relevant-when** | auditing or consuming the review of design:state-reader-v4; asking why the v3→v4 supersession was approved; tracing how prd:self-drive-wake amendments 002 and 003 reached the design layer; tuning tech-design-review calibration |
| **Subject** | design:state-reader-v4 · shape: none · supersedes: state-reader-v3 |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · guide 2026-07 (`.pyxis/authoring/tech-design-review.md`) |
| **Date(s)** | 2026-08-04 |
| **Verdict** | approve |
| **Tracking** | pyxis#998 |
| **Template version** | design-review-report/v1 |

## Round 1

*Fresh subject: an earlier v4 draft was closed unmerged, so this report carries no inherited round history. Scope: the successor plus its declared target, against prd:self-drive-wake as amended by 002 and 003 — both merged, so the folded projection governs — the principles registry, and the incident registry.*

*The supersession trigger is genuine and narrow. v3 §3.8's key bullet states that an unresolvable identity "keys by **medium host alone**, the coarser direction this rule already mandates", while v3's own `**Decision:**` line keys one ledger per (host, identity). The first is unimplementable in a way that defeats the second: a host-keyed ledger only unresolved writers touch is a sibling, not a bound, and since identity lookup is retried rather than memoized a resolved and an unresolved process coexist routinely and spend two full windows against what may be one quota. Whole-document supersession is the only legal repair for a merged design (design:tech-design-conventions §2), and the requirements moved first (002, 003), which is the ordering this corpus requires.*

*Integrity walk: every binding element of v3 not touched by the umbrella is byte-identical; DINV-1..5 and DINV-8, §3.1, §3.6, §3.7 and the read-side acceptance rows carry forward verbatim. v3's live-dependent set is empty — no design carries it in `Builds-on` — so the supersession strands nothing, and wake-watch-v2's `stale-builds-on` flag points at state-reader-v2 and predates this change. `Serves` grows by the anchors 002 and 003 introduced.*

### FND-1 — the supersession seed left duplicated content in two sections

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design:state-reader-v4 §3.4 carried two identical `**EVT-4 is served here, and was served nowhere before.**` bullets, and §3.5 two identical paragraphs opening "The bucket's move to the machine state directory adds no data class" — vs — the one-statement-per-claim expectation any reader brings, and the practical consequence that a later edit to one copy silently diverges from the other. The duplication came from seeding v4 from v3 and then re-adding material v3 already carried; it is invisible to `lint-tech-designs`, which checks structure rather than repetition.
- **Acceptance condition**: each claim appears exactly once.
- **Disposition**: applied

### FND-2 — DINV-10 and DINV-12 had no realization in the sections that implement them

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design:state-reader-v4 #DINV-10 binds the deferral event's queue depth to "the callers waiting on **that ledger within the reporting process**" and requires the event to name which ledger blocked — vs — §3.4's Events bullet, which described the emission as "queue depth and the deferred duration" with no scope and no blocking-ledger property, leaving the invariant with no §3 statement an implementer would meet. An invariant whose implementing section does not mention it is a contract only the invariant knows about.
- **Acceptance condition**: §3.4 states EVT-4's properties as DINV-10 defines them — blocking ledger, in-process per-ledger depth (never the request size, which every production call site sets to 1), projected wait, reset deadline.
- **Disposition**: applied

### FND-3 — the must-not-infer list omitted the one inference this document exists to forbid

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design:state-reader-v4 §3.2's "Implementation must not infer" enumerated five prohibited inferences and stopped at "that a missing budget bucket is evidence of anything (DINV-6)" — vs — §1 and §3.8, which record that an implementer read v3's "coarser" as licence to create a *sibling* ledger, shipped it, and split the quota. That is precisely a must-not-infer item, it is the reason this document supersedes v3, and the list that exists to catch it did not name it. Two further inferences the new invariants forbid — that admission always spans two ledgers (DINV-11), that ordering two `write()` calls orders their durability (DINV-12) — were likewise absent, and §3.2 is where an implementer looks for boundaries before reaching §3.8's argument.
- **Acceptance condition**: the list names all three inferences against their invariants.
- **Disposition**: applied

### FND-4 — R2's latency claim is falsified by the durability model this document introduces

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design:state-reader-v4 R2 mitigates lock contention with "each lock is held only for the read-check-write of a tiny record (**µs**)" — a sentence carried forward from v2, when a commit was one `rename` — vs — #DINV-12, which this document adds, requiring every record to be replaced by write-temp → **fsync temp** → rename → **fsync directory**, with a resolved write committing *two* such records under *two* held locks. An fsync is a millisecond-class operation on ordinary storage, so the mitigation understates its own cost by roughly three orders of magnitude, and the risk it is mitigating — contention under wide fan-out — is exactly the one the correction makes worse. A mitigation whose stated basis is false is not a mitigation.
- **Acceptance condition**: R2 states the real cost (fsync-dominated, and now four syncs per resolved write across two held locks), keeps the structural mitigations that remain true (short critical sections, canonical order, nothing held across a sleep), and names the measurement that would falsify it rather than asserting a magnitude.
- **Disposition**: applied

### FND-5 — `Serves` claims two anchors whose enforcing half the design disclaims, with no artifact named as owning the remainder

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment (Serves substance)
- **Evidence**: design:state-reader-v4's header carries `prd:self-drive-wake/UB-8` and `prd:self-drive-wake/DOD-10` — vs — #DINV-13, which states that neither direction of a version change "is enforceable from inside either build … this design implements the barrier half only", and prd:self-drive-wake/UB-8, whose response places the obligation on "a documented operator precondition … enforced by the release gate". The design genuinely implements what it owes those anchors (the marker's semantics, the barrier, A-15/A-17) and honestly says so — but `Serves` is the stage-6 compiler's join key, and an anchor listed there reads as *covered*. Listing UB-8 without naming who owns the unimplemented half makes a requirement whose enforcing mechanism is unowned look satisfied by a design that says in its own body it does not enforce it.
- **Acceptance condition**: the design names, at the boundary where it disclaims the drain, the artifact that owns it — the operator procedure and the release gate's §6.6 record — so a reader (and the compiler's inverse map) can see the anchor is jointly served rather than fully served here.
- **Disposition**: applied

### FND-6 — the boundaries section did not scope the drain it declines to implement

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:state-reader-v4 §3.2's "Owned by a future design" listed the wake-channel re-plumb, further medium adapters, and limit-scope learning — vs — §3.8 and #DINV-13, which decline the downgrade drain's enforcement outright. A boundary a document draws in its argument but omits from its boundaries section is one a reader can miss on the pass that matters, and this is the boundary most likely to be misread as an omission rather than a decision.
- **Acceptance condition**: §3.2 records the drain's enforcement as owned elsewhere, pointing at UB-8 and A-17.
- **Disposition**: applied

**Verdict: approve** — four blocking findings and two advisories applied in the subject before the verdict; the applied revisions were re-checked against `lint-tech-designs`, the full `docgate gate --merge-base origin/main`, and the regenerated index, all post-commit.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-08-04 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 4 blocking · 2 advisory
- **Dispositions**: 6 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

*Post-approve appends only (DINV-5): round 1 and the verdict history above are frozen and are **not** edited here — including its misstated counts, which the first entry corrects by record rather than by rewrite. No round 3 is opened; these are corrections to an approved report, not a new review.*

**2026-08-04 · review-thread:pyxis#998 · raised externally (operator-run review), applied by the author**

1. **Round 1's own accounting was wrong.** Its verdict line, the header summary and the Measurements block all read "4 blocking · 2 advisory" against six findings whose recorded severities are **five blocking and one advisory** — FND-5 cites CAL-2, a row with no advisory tier. The frozen text stands as written; the correct count is 5 · 1.

2. **Round 1 verified that new material was present, not that superseded material was gone.** Five defects in the subject followed from that single gap, all now applied:
   - §2's `**Decision:**`, its rationale, and Principles' AP-11/AP-17 were still v3's single-bucket text — the primary Decision callout described the topology this document exists to replace. The v4 edits meant to replace them had been applied by string anchor against strings v3 no longer contained, so each silently did nothing. §2 is now rewritten by structure rather than by anchor.
   - Both supersession notes described **v3 → v2** and called wake-watch-v2 a v3 dependent; v3 has no live dependents and that edge points at v2. Replaced with the actual v4 → v3 integrity result.
   - `Serves` claimed `prd:self-drive-wake/UB-8` and `DOD-10` while the design declines their enforcing half and carries none of their acceptance. Round 1 accepted this as "jointly served", which was prose offsetting a machine-readable claim: `Serves` is the stage-6 join key. Both anchors left `Serves`; DOD-10 is retained as **row-level provenance** on A-15/A-17, which do implement its upgrade-barrier and marker cases, with the unserved downgrade half named.
   - The header declared `Security/privacy impact: none` on a design that persists state **keyed by an authenticated identity** and gates production content writes — `yes` plus a named reviewer is mandatory for that surface (design:tech-design-conventions §3.1). Now `yes`, with §3.5 agreeing.
   - DINV-11 promised "both committed or neither" while DINV-12 admits *umbrella-only*, which is reachable by an **ordinary fsync/rename error**, not only by crash — and no branch specified its handling. DINV-11 is now scoped to the capacity decision; DINV-12 owns failure outcomes and binds the handling (substrate error returned, no content write sent, umbrella charge retained).

**2026-08-04 · review-thread:pyxis#998, second pass · raised externally, applied by the author**

3. **The cutover marker's contract had gaps the first correction did not close.**
   - Its threat statements disagreed across three places: the header said a deleted marker *grants* a window, the body correctly said deletion *arms* the barrier and costs one, and §3.5 claimed a forged marker is treated as absent — which a **schema-valid** forgery is not, since it passes the stated predicate by construction. Stated consistently now: deletion or malformed content costs a window; a schema-valid forgery skips the barrier; owner-only access is what prevents forgery, and it is the control, not the predicate.
   - A-17 permitted a power loss to leave the marker "present-and-valid or gone" while forbidding any surviving marker that exempts the re-upgrade — a present valid marker being exactly that. Split into **completed** invalidation (returns only after unlink *and* directory fsync) and **interrupted** invalidation (the downgrade is incomplete; legacy writes stay prohibited until it is retried).

4. **Path encoding was not collision-resistant.** Sanitize-and-truncate is not injective — `a/b` and `a?b` both reduce to `a_b`, truncation collides, and `.`/`..` already pass the allowed-character class. Host encoding was not addressed at all. Both components now carry a digest of the raw bytes, so distinct inputs cannot share a path atom, with adversarial fixtures.

5. **The ordinary-failure branch was stated in the invariants and nowhere else.** §2's rationale still claimed no budget is spent for a write that never happens, §3.8 described only crashes, and A-16's evidence column omitted the injected-error fixture. The rationale is qualified, the behaviour is propagated into §3.8, and the fixture is named.

6. **The supersession note overclaimed verbatim carry-forward** of the deferral event, which v4 extends with the blocking-ledger identity and scoped queue-depth semantics amendment 002 introduced. The note now separates the unchanged event baseline from the added fields and their tests.

**2026-08-05 · review-thread:pyxis#998, third pass · raised externally, applied by the author**

7. **The GitHub adapter's read was incomplete against the platform, in two ways the earlier rounds never reached.** These are the first findings in this review that are not internal inconsistency — they are the design being wrong about GitHub.
   - **The required set was read from branch protection alone.** Repository and organization **rulesets** also define required checks and *compose* with branch protection, so a check a ruleset requires read as unrequired: a **false green** under FR-3. And a requirement can be bound to a specific integration, so matching on the context *name* alone lets a same-named check from a different app satisfy it. The effective set is now the union of both mechanisms, and a requirement is a **(context, source)** pair. A-18 adds ruleset-only, union, and wrong-app/same-name fixtures.
   - **The query could not return what it claimed.** It used the plural `pullRequests` connection, which has **no list-of-numbers filter** — so it read the 100 most recently *updated* proposals rather than the ones requested — and it truncated contexts at a fixed page while three nested comment connections had no continuation semantics at all. Now built from **aliased per-ref lookups**, with every nested connection paginated, continuation waves batched, and cursor tie-breaking pinned. A-19 exceeds each nested cap independently.
   - **The round-trip bound is restated honestly.** A-2's "≤ 2" was universal; it is now the no-overflow bound, with the N-independence FR-2 actually requires asserted separately. FR-2's own "within the medium's page size" is what scopes nested overflow out, so no amendment is needed — but the design says that rather than leaving A-2 quietly false.

8. **`States` — a read method — carried the write budget's error contract**, including DINV-12's failed-content-write branch. Restricted to read and cancellation failures; ledger substrate errors are returned by the content-write operation that incurs them.

9. **First-cutover cancellation was doubly specified.** DINV-11/A-14 required a cancelled caller to leave nothing behind while DINV-6/A-15 required the first initializer to persist the conservative ledger and marker *before* waiting. Initialization is now a separate ordered transaction, explicitly exempt: the barrier is the host's, not the caller's, and A-14's no-entry rule is scoped to steady-state admission under a valid marker.

10. **Two claims were unprovable as written, and one was false.**
   - A-17 demanded a durable unlink-plus-directory-fsync guarantee using only crash fixtures, against §3.9's own rule that crash injection establishes call order and never durable order. It now carries the inspection evidence A-16(b) already used.
   - The path encoding claimed distinct inputs yield distinct atoms, which a **truncated digest cannot deliver** — false by pigeonhole. The parameters are now bound (SHA-256, 16 hex characters, 32-character readable budget) and the claim is stated as a probability over the actual population rather than as certainty. A-21 adds hostile-input and canonical-host fixtures.

11. **The lock and scratch lifecycle was undefined, and the omission was a correctness bug.** `flock` binds to an **inode**, and the replacement protocol renames a new inode over the record — so a process locking the record file itself holds the *old* inode while a second process locks the new one and proceeds concurrently. Mutual exclusion would stop existing at the moment a write commits. Locks are now dedicated sidecars, never replaced. Temps additionally get a reaper in the same change, as AP-12 requires. A-20 adds forced-overlap and crash-restart fixtures.

