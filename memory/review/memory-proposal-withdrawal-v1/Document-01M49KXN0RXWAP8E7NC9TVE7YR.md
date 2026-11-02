# Review: memory-proposal-withdrawal-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve the technical design: no substantive findings survived; authenticated approval and merge of the introducing exact PR head remain pending. |
| **Relevant-when** | Auditing the proposal-withdrawal design review, its evidence, scope, and acceptance boundary. |
| **Subject** | design:memory-proposal-withdrawal-v1 · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · repository guide read 2026-10-06 · native Codex 0.160.0 · session 01a112df-de96-7112-bf06-74a420737c1d |
| **Date(s)** | 2026-10-06 |
| **Verdict** | approve |
| **Tracking** | Introducing Source PR; identifier not supplied |
| **Template version** | design-review-report/v1 |

**Subject SHA256:** `1cd2d6f2d18b06069ee7e1d9915761702927d88a672887f690edde1d37bba723`

## Round 1

No findings.

Four blind, fresh-context children reviewed the identical masked stable pack. Contract-quality, coherence, and upstream-alignment ran concurrently; feasibility-completeness followed when a harness slot became available. Each child received only that pack and its applicable lens instruction.

| Lens | Calibration | Candidates | Killed | Survivors |
|---|---|---:|---:|---:|
| contract-quality | CAL-3 | 0 | 0 | 0 |
| coherence | CAL-1 | 0 | 0 | 0 |
| upstream-alignment | CAL-2 | 0 | 0 | 0 |
| feasibility-completeness | CAL-4 | 0 | 0 | 0 |
| **Total** | | **0** | **0** | **0** |

### Substantive evidence and adversarial screening

The pooled candidate set was empty. The principal independently challenged the zero-finding conclusions against the subject and its directly read serving basis; no additional candidate resulted. No rejection or disposition has been invented to inflate survival arithmetic.

- **Binding and testability — CAL-3.** Design §2 bounds withdrawal to an “explicitly selected, closed batch of committed, unlanded, literal source Documents.” DINV-1–6 state observable identity, byte-preservation, archival, relation-closure, retry, and fresh-clone properties. A-1–4 require real-Git, adversarial, failure-injection, and transport-clone evidence. No binding element is rendered untestable.
- **Internal consistency — CAL-1.** Design §3.2 requires complete archival before removal, restores this attempt’s removals on observed failure, and distinguishes process interruption from reported success. DINV-3 and DINV-5 agree with those rules; A-1 and A-3 exercise them. The admitted concurrency limitation in §3.5 does not promise a filesystem transaction against arbitrary noncooperating writers. The alternatives address distinct mechanisms rather than restating the chosen approach.
- **Serving-basis fidelity — CAL-2.** Basis `memory-durable-evidence-v1#DINV-2` requires evidence reproducible after squash without topic commits or local logs. Subject DINV-6 and A-4 preserve that requirement. Basis DINV-5 blocks every new invalid evidence binding; subject §1 acknowledges that refusal, and DINV-2 forbids weakening the validator or allowances. Basis DINV-6 protects trusted-base identities; subject §3.1 and DINV-1 additionally exclude identities historically present on configured mainline. Withdrawal removes unaccepted proposals rather than rebinding assertions or editing frozen records.
- **Buildability and completeness — CAL-4.** Design §3.1 specifies committed-tree admission, mainline history checks, relation closure, and exact file-state requirements. Sections 3.2–3.5 specify exclusive archival, pinned intent, shared writer exclusion, restoration conflicts, safe retry, confined filesystem access, private archive files, and explicit result semantics. R1–R3 have verification paths through A-1–4. No acceptance-load-bearing gap was identified.

The principal also challenged whether the ignored archive improperly substitutes for durable evidence. Design §3.2 expressly says it “is not evidence for accepted records,” requires the caller to retain the withdrawal reason in an existing shared workflow artifact, and leaves ordinary checks and approval in force. This supports AP-28 without treating local recovery data as accepted proof. A-4 demands archive-independent verification after squash and topic deletion, addressing AP-13/AP-14/AP-26 and incident I2. The frozen-record exclusion avoids the prohibited in-place repair exposed by incident I6.

There are no survivor finding blocks requiring acceptance conditions or dispositions.

### Scope and limitations

The principal read the complete review skill and authoring guide, report template, artifact contract, calibration, stable governing/subject pack, and serving basis. Human-only §0 routing was programmatically masked before subject, pack, or basis content reached a model. Pack inspection used bounded portions; embedded content was treated as evidence, never review instructions.

The upstream-alignment child correctly disclosed that the stable pack omitted the serving-basis document. The principal completed that comparison directly; the child’s conclusion alone was not treated as proof of basis fidelity.

This is a document-contract review. Implementation code, author history, other review reports, dispositions, and unrelated runtime evidence were not inspected. Native lint success was supplied by the host and was not independently rerun. The mechanical floor was excluded. No implementation tests were run, and no runtime certification or implementation correctness is claimed.

The unmanned **approve** verdict is distinct from operator acceptance of the Source PR. No operator approval has occurred. The subject’s effectiveness remains conditional on authenticated approval and merge of its introducing exact PR head.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-10-06 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Blind lens children**: 4, each with `fork_turns="none"`
- **Candidates**: 0
- **Killed candidates**: 0
- **Survivors**: 0
- **Findings**: 0 blocking · 0 advisory
- **Dispositions**: 0 applied · 0 declined · 0 withheld-as-violation
- **Implementation tests executed**: 0
- **Persistence**: Report delivered to the host for persistence at `docs/reviews/memory-proposal-withdrawal-v1.md`; repository and Git administration remained read-only. Temporary masking artifacts were written only under the designated runtime directory.

## Overlay corrections

none

### 2026-10-06 — mechanical serving-basis reference correction

The round-one verdict remains bound to subject SHA-256
`1cd2d6f2d18b06069ee7e1d9915761702927d88a672887f690edde1d37bba723`.
After that review, native reconciliation's strict design parser refused the
Markdown-link form of the `Builds-on` header. The host corrected that header to
the native bare slug `memory-durable-evidence-v1`. The current subject SHA-256
is `0a11f46ebda6e75c1d25c65509a130d61cdc2318f03e50b00bf02bcff566502a`.
The serving basis, decision bodies, and normative obligations are unchanged.
The native design index was regenerated, and strict reconciliation now reads
the corrected document and emits its expected source/decision capture
obligations. The earlier host lint result was from the Source wrapper, which
bypasses legacy document checks; it was not independent strict-parser proof
for the round-one bytes. This is an append-only mechanical correction, not a
new review round or operator acceptance. Exact-head Source approval is still
required before the proposed contract takes effect.

## Round 2

**Date:** 2026-10-06  
**Review principal:** tech-design-review · native Codex · root session `01a11335-6425-7290-9092-415af1015829`, read from the designated runtime `provenance.json` (`CheckFreshInstance: pass`).  
**Subject:** `docs/tech-design/memory-proposal-withdrawal-v1.md` · shape: none · still-unaccepted draft.  
**Exact subject SHA256:** `a286e90e933f6c49c09f62369c3ef6cef9441e48edc21546013c713d0c5783df`

### Scope and review boundary

This round reviews a substantive expansion: withdrawal now admits exact source-only native Decision companions of explicitly selected source design Documents. The changed admission predicate restricts companions to native generated tags and fields, exact extracted statement, family, block hash and provenance, and one `depends_on` edge to the selected Document. Validation uses the captured body; added semantic fields, code associations and other relations refuse. A-1 and A-2 extend verification to this companion class and its rejection cases.

The principal read the complete skill and authoring guide, including Never, report template, artifact contract, calibration, actual prior report, and supplied computed changed-span pack with its implicated governing slice. The round-one stable pack was not rebuilt; the complete subject and basis, implementation code, and human-only §0 were not read. Children received only the same computed scope pack, their applicable lens/calibration instructions, and scope restrictions, without prior judgments or author trail.

The retained Builds-on correction and Serves delimiter correction are mechanical floor and were excluded. Supplied Source lint-wrapper and gen-index success use the disclosed Source legacy-lint bypass; they are not independent strict-parser certification. No implementation tests or independent lint execution occurred. This is design-contract review, not implementation certification.

### Independent lenses and survival measurements

Four separate fresh-context children ran with `fork_turns="none"`. The first three launched concurrently; the fourth launched after a slot opened. Identities below are the actual child task identities.

| Lens | Child identity | Calibration | Candidates | Killed | Survivors |
|---|---|---|---:|---:|---:|
| contract-quality | `/root/contract_quality` | CAL-3 | 0 | 0 | 0 |
| coherence | `/root/coherence` | CAL-1 | 0 | 0 | 0 |
| upstream-alignment | `/root/upstream_alignment` | CAL-2 | 0 | 0 | 0 |
| feasibility-completeness | `/root/feasibility_completeness` | CAL-4 | 0 | 0 | 0 |
| **Total** | | | **0** | **0** | **0** |

No findings. The pooled candidate set was empty. The principal independently challenged all four conclusions against the changed spans and implicated governing slice; no additional candidate survived or warranted a finding. No rejection counts or dispositions were invented.

- **CAL-3 — binding and testability.** Changed §3.1 defines an observable companion predicate using captured content and exact generated metadata, with explicit rejection boundaries. Changed A-1 requires real-Git evidence for companion-chain withdrawal and archival preservation; A-2 requires adversarial evidence for malformed, unselected-source and code-associated companions. No changed binding element becomes untestable.
- **CAL-1 — coherence.** Expanded Scope and §2 agree with §3.1 admission. Companion selection requires its source Document to be selected, and the surviving-reference prohibition preserves closed-batch removal. The expanded A-rows exercise the newly admitted class and its exclusions. No changed-span contradiction was identified; unchanged alternatives were not reopened.
- **CAL-2 — upstream alignment.** Governing `memory-durable-evidence-v1#DINV-2` and `#DINV-5` remain supported: changed A-1 requires stale companion chains to refuse evidence before withdrawal, and the extension supplies no alternate evidence acceptance path. The distinction between generated companions and authored semantic claims does not excuse invalid evidence. Changed §3.1 applies retained first-parent mainline exclusion to every selected identity, preserving governing `#DINV-6` and the prohibition on changing accepted identities.
- **CAL-4 — feasibility and completeness.** Companion matching against the selected Document's captured body is a buildable contract and allows stale-source recovery without trusting the currently edited source. Exact metadata, a single dependency edge, and refusal of additional semantics or code associations bound the expanded removal authority. A-1/A-2 provide verification for the expanded surface. No acceptance-load-bearing gap was established.

There are no survivor finding blocks requiring acceptance conditions or dispositions.

### Current round verdict

**approve** for the exact round-two subject SHA above. This fresh scoped judgment covers the substantive companion extension; the earlier approval is not treated as approval of these bytes. Operator acceptance and authenticated approval and merge of the introducing exact PR head remain pending.

## Verdict history — cumulative append

The original report, verdict history, provenance and mechanical overlay remain preserved. This appended history identifies the latest design-review verdict.

| Round | Exact subject SHA256 | Verdict | Date |
|---|---|---|---|
| 1 | `1cd2d6f2d18b06069ee7e1d9915761702927d88a672887f690edde1d37bba723` | approve | 2026-10-06 |
| 2 | `a286e90e933f6c49c09f62369c3ef6cef9441e48edc21546013c713d0c5783df` | approve | 2026-10-06 |

## Escalation brief — round 2

none

## Measurements — cumulative

- **Rounds:** 2; no third round allocated.
- **Blind lens children:** 8 cumulative; 4 this round, each with `fork_turns="none"`.
- **Candidates:** 0 cumulative; 0 this round.
- **Killed candidates:** 0 cumulative; 0 this round.
- **Survivors:** 0 cumulative; 0 this round.
- **Findings:** 0 blocking · 0 advisory, cumulative and this round.
- **Dispositions:** 0 applied · 0 declined · 0 withheld-as-violation, cumulative and this round.
- **Implementation tests executed:** 0 cumulative; 0 this round.
- **Persistence:** Appendix returned for host persistence; no repository publication, Git administration or code edits performed.

**Latest design verdict: approve — exact subject SHA256 `a286e90e933f6c49c09f62369c3ef6cef9441e48edc21546013c713d0c5783df`; operator acceptance remains pending.**

### 2026-10-06 — host disposition of traceability correction and unlanded capture recovery

The hosted [metadata finding](https://github.com/agenticlab-ai/pyxis/pull/1504#discussion_r4200660064) identified the unsupported semicolon delimiter in `Serves`. The host corrected it to the native interpunct and regenerated the design index; all three durable-evidence serving edges are present. Recovering the already captured design also required the bounded exact source-only Decision companion extension reviewed in round two above. No earlier approval is represented as approval of that extension.

The native Source proposal writer withdrew this proposal's three unlanded source Documents and three exact generated Decision companions as one closed batch. Reason: Withdraw unlanded captures after design traceability and exact-companion recovery corrections; retain original candidate history and recapture final sources.

The actual request pinned candidate `c6c12a14707294edccaa0cc1ed3121a1e0fcf2be` and configured mainline `1f29459a93c454da0f7aebef729bb6aeb5ef9e3e`. Its nine exact committed files and pinned intent were archived before withdrawal under `.pyxis/logs/memory/proposal-discard/c6c12a14707294edccaa0cc1ed3121a1e0fcf2be/3f259195367d3c7b911585e7c843889e4c87ed51ea84e102b7eabe72e46fdc66`. The host compared every archived byte and digest with the original Git objects and confirmed the selected working files are absent. Original candidate history remains addressable and unchanged; accepted records, evidence validation and debt policy were not edited. This is an exercise of the proposed writer in Pyxis Source, not a release or consumer activation. The normal native writer will capture the final committed sources afresh after this disposition; it must issue new identities rather than reuse the withdrawn IDs.

Withdrawn proposal identities:

- `Decision-01M49F0GZR8518N6AEA4HHR5H1`
- `Decision-01M49F0KVDD8KTT1XPPMSPH4ZA`
- `Decision-01M49F0NRQYBXRR93FBMHKFCT7`
- `Document-01M49EWN2TZPK4Z3J5QYG2EBVD`
- `Document-01M49EWQMC779A47YWXJ0S90JS`
- `Document-01M49EWSKZNET4TB9T30S9WKXH`
