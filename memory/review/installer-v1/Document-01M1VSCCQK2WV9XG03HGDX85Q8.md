# Review: installer-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Round 1: request-changes on 18 findings (7 blocking — the payload shipping the source repo's Go-stepped CI workflow into Go-less operands, a circular bootstrap-anchor comment, FR-16's undesigned argus-off mechanism, the janitor-less binary cache, FR-18's open path set, an unnamed manifest cache, a self-contradicting class-policy entry; 11 advisory); all applied; round 2 verified closure → approve. One lens candidate died on a fabricated evidence quote. |
| **Relevant-when** | auditing or consuming the review of installer-v1; asking why it was approved; tuning tech-design-review calibration |
| **Subject** | design:installer-v1 · shape: present · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · guide as of 2026-07-19 |
| **Date(s)** | 2026-07-19 |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR)* |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — the shape's class policy listed AGENTS.md against its own seeded-never-listed rule

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: shape: `{ glob: "AGENTS.md", class: repo-owned }` beside the comment "seeded skeletons … are instantiated by init, not shipped-in-place … never appear here" — vs — design:§3.5 step 4 / record D-8: the operand manual is instantiated from a template, exactly the excluded class.
- **Acceptance condition**: remove the row; extend the comment to name the operand manual among the instantiated-not-shipped set.
- **Disposition**: applied — row removed, comment now covers skeletons and the operand manual.

### FND-2 — the manifest comment implied a circular bootstrap trust chain

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: shape: `"pyxis": { /* … the bootstrap pins it from here */ }` — vs — design:§3.9's chain (the bootstrap verifies pyxis; pyxis parses the manifest): the bootstrap cannot anchor on an artifact it needs pyxis to read.
- **Acceptance condition**: name the bootstrap's manifest-independent anchor in both halves.
- **Disposition**: applied — design:§3.4 now anchors the bootstrap on the pin documented in the install instructions (version + sha256 in the getting-started line); the shape comment states pyxis is listed for converge/doctor parity only.

### FND-3 — FR-16's argus-off inertness had no designed mechanism

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:§3.5's seeding step never conditioned on the argus knob and never named the authority-contract suppression — vs — prd:installer/FR-16's fit criterion (skill refuses; no authority contract file exists), two concrete testable artifacts.
- **Acceptance condition**: state the mechanism: knob-off skips the authority-contract instantiable; the payload-shipped skill reads the knob at runtime and refuses; knob-on seeds the floor tier.
- **Disposition**: applied — design:§3.5 step 4 carries all three legs.

### FND-4 — the binary cache shipped without its AP-12 janitor, and AP-12 was silently omitted

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:§3.4's cache path (fetch-verify-cache, versioned filenames) with no eviction anywhere; Principles cited AP-20/22/23/27 but not AP-12 — vs — AP-12: a new throwaway-resource type ships with its janitor in the same change.
- **Acceptance condition**: design the reaper or state the deviation with reason; cite AP-12.
- **Disposition**: applied — converge's pre-warm reaps off-version binaries and manifests; doctor names stragglers; AP-12 cited honored.

### FND-5 — FR-18's fit criterion was untestable: the captured-path set deferred to implementation

- **Severity**: blocking (CAL-3 — a binding element went untestable)
- **Lens**: upstream-alignment
- **Evidence**: shape: paths block trailing "…the full set the engine reads, enumerated at implementation from the audit" — vs — prd:installer/FR-18's "zero silent-ignore paths exist for captured keys", a completeness claim needing a closed list.
- **Acceptance condition**: a closure rule making the criterion falsifiable without reading the implementation.
- **Disposition**: applied — design:§3.7: the schema is the authoritative closed list; a conformance check asserts no engine configurable-path read lacks a schema key; the audit feeds the schema before first release. Shape comment matches.

### FND-6 — the payload would ship the source repo's CI workflow (Go-toolchain steps) verbatim into Go-less installed repos

- **Severity**: blocking (CAL-4 — load-bearing for A-3 and FR-20: every GitHub-mode combo would go red on first push)
- **Lens**: feasibility-completeness
- **Evidence**: shape: class policy marks the gate workflow engine-verbatim while the live file at that path runs Go setup, vet, and the race suite — vs — installed repos carry no Go source by the design's own NG (prd:installer/NG-2); design:§3.10's migration covered shims only.
- **Acceptance condition**: specify the installed-repo workflow variant (exec-grammar gate steps, zero source-toolchain steps), keep the source repo's own workflow separate, and gate the first release on the conversion.
- **Disposition**: applied — design:§3.10 gains the CI-workflow bullet and a release-precondition bullet (the release build refuses to publish while any payload hook or workflow carries a retired resolver reference or a source-toolchain step); the shape's class-policy row is annotated as the installed variant.

### FND-7 — the manifest's runtime cache location was unnamed, leaving three A-rows without an injectable artifact

- **Severity**: blocking (CAL-4 — A-5/A-9/A-10 fixtures need the location)
- **Lens**: feasibility-completeness
- **Evidence**: shape: acquisition sequence read only "release-manifest.json (cached)" — vs — the binary cache's path being a named element the same fixtures rely on.
- **Acceptance condition**: name the manifest cache location and its lifecycle.
- **Disposition**: applied — shape names the gitignored cache path, its verify-before-first-use rule, and its converge reaping; design:§3.4's resolution bullet references it.

### FND-8 — DINV-7 (zero telemetry egress) had no covering A-row

- **Severity**: advisory (CAL-3; also raised by coherence, deduplicated)
- **Lens**: contract-quality
- **Evidence**: design:§3.8's DINV-7 — vs — §6: no A-row traced it; A-5's network-denied run covered it only incidentally.
- **Acceptance condition**: trace DINV-7 into A-5 and make the egress assertion explicit.
- **Disposition**: applied — A-5 now proves the zero-egress invariant, evidence extended with a run-log egress scan, #DINV-7 traced.

### FND-9 — A-9's budget "~tens of milliseconds" was not a machine-checkable threshold

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:§3.4/§0/A-9's tilde-range — vs — the fit-criterion rule: a threshold two implementers would encode identically.
- **Acceptance condition**: a concrete bound.
- **Disposition**: applied — ≤ 100 ms p95 per invocation on reference runners, in §3.4, §0, and A-9.

### FND-10 — §3.3 froze a volatile payload-contents enumeration into the design

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:§3.3's inline inventory — vs — the frozen-prose volatility rule; the shape owns the inventory.
- **Acceptance condition**: point at the shape's authoritative inventory; keep only the decision-level commitments.
- **Disposition**: applied — §3.3 now defers to the shape and keeps the un-bundled-vendoring and separate-deps commitments.

### FND-11 — §4's ALT-5 row carried one of the record's three rejection grounds

- **Severity**: advisory (CAL-1, downgraded: abbreviation, not misrepresentation)
- **Lens**: coherence
- **Evidence**: design:§4 ALT-5 — vs — record ALT-5's verdict (appetite; smuggled second decision; I2-shaped migration).
- **Acceptance condition**: carry all three grounds.
- **Disposition**: applied.

### FND-12 — DINV-2's two-key constraint had no A-row exercising refusal of extra or legacy keys

- **Severity**: advisory (CAL-1, downgraded: closed-parse coverage existed via UB-6; the gap was the explicit trace)
- **Lens**: coherence
- **Evidence**: design:§0's sanity row cited DINV-2; no §6 row traced it.
- **Acceptance condition**: extend A-4 to refuse-extra-keys and trace #DINV-2.
- **Disposition**: applied.

### FND-13 — the hand-authored-manifest §4 row lacked its record provenance anchor

- **Severity**: advisory (CAL-1, downgraded)
- **Lens**: coherence
- **Evidence**: design:§4 row without a record citation, unlike its siblings.
- **Acceptance condition**: cite the record's D-4 reasoning.
- **Disposition**: applied.

### FND-14 — AP-28's position on converge/doctor drift events was silently omitted

- **Severity**: advisory (CAL-2, downgraded: AP-28's routine-projection exclusion gives a legitimate reading; the defect was silence, not violation)
- **Lens**: upstream-alignment
- **Evidence**: design:§3.8 routing EVT-2/EVT-3 to local logs; Principles silent on AP-28 — vs — AP-28 naming "a drift snapshot" in its retained set.
- **Acceptance condition**: state the position in Principles.
- **Disposition**: applied — honored-by-scoping bullet: installer events are re-derivable operational telemetry; the teaching surfaces persist as CI artifacts and tracker records; a committed event-record path is an additive follow-up if the operator rules otherwise.

### FND-15 — §3.10's dual-read window did not name the legacy sections it covers

- **Severity**: advisory (CAL-2, downgraded: "per-tool pin sections" was true; the gap was enumeration for a reader without the Builds-on context)
- **Lens**: upstream-alignment
- **Evidence**: design:§3.10 — vs — codegate-v1 §3.1's additive `codegate:` pin section.
- **Acceptance condition**: name the live sections.
- **Disposition**: applied — "every per-tool pin section present in the live config (the docgate and codegate sections today)".

### FND-16 — FR-20 had no dedicated acceptance trace

- **Severity**: advisory (CAL-4; blocking only in combination with FND-6, which closed)
- **Lens**: feasibility-completeness
- **Evidence**: design:§6 A-3's traces omitted FR-20; the first-push check is a post-install event.
- **Acceptance condition**: trace the first-push leg explicitly.
- **Disposition**: applied via FND-6's release-precondition bullet plus A-3's matrix combos exercising the installed workflow's first push (the basis's FR-20 fit criterion); no separate A-row added — the matrix evidence class already includes the first-push run.

### FND-17 — the offline pre-fetch set was not enumerated

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:§3.3 named the payload and deps bundle; the manifest is a separate artifact the offline combo also needs.
- **Acceptance condition**: enumerate the pre-fetch set.
- **Disposition**: applied — exactly four artifacts, each sha256-verified; FR-13's "after the release artifact fetch" defined as after these four.

### FND-18 — the AP-22 claim overreached onto init-from-nothing

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:Principles' unqualified "converge and publish are idempotent and raceable" — vs — AP-22's exactly-one-actor anti-pattern and init's git-init/epoch first-write window.
- **Acceptance condition**: scope the claim.
- **Disposition**: applied — converge/exec/publish raceable; init-from-nothing single-operator by contract with the stated harmless residual.

*Lens-candidate note (measurement, not a finding): one contract-quality candidate ("the design freezes the `internal/resolve` package name") was killed in adversarial screening — its evidence quote does not exist in the design; the package name lives only in the shape, where it belongs.*

## Round 2

Scope-locked re-review over the 18 named defects and the changed spans. All acceptance conditions verified met: the class-policy row removed and its comment extended; the bootstrap anchor stated independently of the manifest in both halves; FR-16's three-leg mechanism present; the cache janitor designed and AP-12 cited; the path-set closure rule in §3.7 and the shape; the installed-workflow variant and release-precondition bullets in §3.10 with the class-policy annotation; the manifest cache named with lifecycle; DINV-7 and DINV-2 traced into A-5/A-4; the ≤100 ms bound consistent across §0/§3.4/A-9; §3.3 shape-authoritative; ALT-5's grounds complete; provenance anchors present; AP-28 and AP-22 positions stated. Regression sweep of changed spans: no new path literals (the content-path check stays green), §0's numbers agree with the body, the gate and index regeneration pass on the revised pair. No new findings; no violation-withheld entries.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-19 |
| 2 | approve | 2026-07-19 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 7 blocking · 11 advisory
- **Dispositions**: 18 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
