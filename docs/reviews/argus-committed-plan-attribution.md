# Review: argus-committed-plan-attribution — design-review report

| Field | Value |
|---|---|
| **In one line** | approve — FND-1 is closed; no changed-span regression survives the final scope-locked round. |
| **Relevant-when** | consuming or auditing the successor's two-round design review and committed-delivery-ledger admission predicate |
| **Subject** | design:argus-committed-plan-attribution · shape: none · supersedes: argus-problem-capture-attribution |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · independent Codex reviewer · guide revision dated 2026-09-25 |
| **Date(s)** | 2026-10-05 (rounds 1–2) |
| **Verdict** | approve |
| **Tracking** | Introducing successor PR not supplied; report prepared locally for the subject's introducing PR. |
| **Template version** | design-review-report/v1 |

## Round 1

### Review provenance

- **Repository HEAD:** `0971e29eff1665518b95b3009af42d2f696e9c59`.
- **Subject bytes:** supplied, uncommitted working file [argus-committed-plan-attribution.md](../tech-design/argus-committed-plan-attribution.md); SHA-256 `8373e03676e7657861d5fd2df61a92aef84bdcbb46016d89b6c5634a974ce4ce`. All four lenses confirmed this hash.
- **Frozen target:** [argus-problem-capture-attribution.md](../tech-design/argus-problem-capture-attribution.md) matched its bytes at repository HEAD; SHA-256 `eaf1c1f560b5371f51bf4de06740ce3c5bfdbb812767232eaf5a4abc15132bb6`. Its bindings were evidence, never finding objects. This is the successor's first round; no predecessor review round was imported.
- **Precondition:** `go run ./cmd/docgate lint-tech-designs` exited 0 on the supplied working tree before lens generation. Shape: none. The lint/gate floor was excluded.
- **Stable context:** `go run ./cmd/docgate context-pack --stage tech-design-review --subject argus-committed-plan-attribution` exited 0; pack digest `sha256:ec1759fd20f815b9825e3410e75716b70e6e7ef264e0223f8356d2dc8f37b28d`, built-from fingerprint `content:sha256:c3d05aa3bb6b68902189ca72923077256a1856478d193df56bf8aabff4d70312`. Complete direct reads supplemented truncated pack output: product vision, architecture principles, incident registry, subject and calibration.
- **Direct basis:** [argus-boundary-enforcement/effective.md](../prds/argus-boundary-enforcement/effective.md), as of amendment 002, SHA-256 `9de88889c0f2ce8e3622956649ff5799595b7bfdf51db832143c30d704d53779`; all served anchors read. Additional governing slices: argus-artifacts §3.1–§3.2, feature-draft FR-31 and approval rows, and plan-new's planning-entry precondition.
- **Review protocol:** [SKILL.md](../../.pyxis/skills/tech-design-review/SKILL.md), complete [tech-design-review guide](../../.pyxis/authoring/tech-design-review.md) and referenced sibling protocol; reviewer PRD; both required templates; [artifact contract](../tech-design/tech-design-review-artifacts.md); [calibration](../../.pyxis/skills/tech-design-review/calibration.md); applicable AGENTS.md and memory-capture instructions. Guide SHA-256 `4b87f6ab26ae20e45ec7d5d5c9578bdb6a58701627f9ee617a2728b403650175`; calibration SHA-256 `a41bccee6fb169b55c30c4e331bc84defd19950fb0c4bf6d1a0cc351f22e642d`.
- **Independence and limits:** four fresh `fork_turns="none"` lens agents, without model or reasoning overrides, received subject and governing sources, never another lens's findings. The runtime admitted three concurrent workers; feasibility-completeness started when contract-quality released its slot. Human-only §0 was excluded from judgment and evidence. No application workspace, operator transcript, implementer deliberation or predecessor review report was used. This manual design review asserts no exact-head code review, operator approval, installed-release verification, or native OS guard/admission guarantee.

### Candidate generation and survival

| Lens / fresh agent | Lens-local candidates generated | Findings retained after central survival |
|---|---:|---:|
| contract-quality / `/root/contract_quality` | 6 | 0 |
| coherence / `/root/coherence` | 2 | 1 |
| upstream-alignment / `/root/upstream_alignment` | 3 | 0 |
| feasibility-completeness / `/root/feasibility_completeness` | 6 | 0 |
| **Total** | **17** | **1** |

Counts precede cross-lens deduplication. Discarded candidate identities are not retained. The proposed survivor was independently screened for a concrete conflicting case, governing evidence, calibrated severity and a finite closure condition.

The strongest defense of FND-1 is that §3.3's more specific resolver rule clarifies an informal use of “problem ledger.” That does not reconcile the equally binding Decision, DINV and acceptance row with the repository's distinct versioned ledger classes. A valid `problem.md` with absent or malformed `delivery.md` receives competing admission conditions. Consistent input naming and an explicit refusal fixture close the defect without changing the intended grammar.

Supersession integrity otherwise holds: target DINV-1–DINV-4 and A-1–A-5 are carried forward; the successor changes the target's §3.3 committed-ledger scope by adding fixed future artifact identities, making supersession genuine. Index edges and direct design-header/reference searches found no live dependent on the target. Authoring scope and planning readiness remain distinct: §3.3/DINV-5/A-6 permit bounded artifact paths, while the planning-entry precheck independently refuses an absent, corrupt, stale or unmerged binding lock. Historical exact-path ownership and protected-class precedence remain preserved.

### FND-1 — Name the delivery ledger consistently as the downstream scope input

- **Severity**: blocking (CAL-1 — coherence).
- **Lens**: coherence.
- **Evidence**: design:argus-committed-plan-attribution §2, DINV-5 and A-6 condition admission on a “valid committed problem ledger”; its §3.3 instead names the “valid committed delivery ledger” and says an absent or malformed committed ledger grants no downstream scope. Against design:argus-artifacts §3.1 (`problem.md`, `problem-ledger/v1`) and §3.2 (`delivery.md`, `delivery-ledger/v1`), these are distinct inputs. Target evidence: argus-problem-capture-attribution §3.3 fixes the scope source as the committed delivery ledger; the finding concerns only the successor's new clauses.
- **Acceptance condition**: Name the active problem's committed `delivery-ledger/v1` (`delivery.md`) consistently in §2, DINV-5 and A-6. Explicitly require both scope consumers to refuse the new paths when `problem.md` is valid but the committed delivery ledger is absent or malformed. Preserve the separate planning-readiness predicate.
- **Disposition**: applied by the AI source coordinator — §2, §3.3, DINV-5 and A-6 consistently name the committed delivery ledger. The valid-problem-only case now explicitly refuses numbered plans for absent/malformed committed delivery input, including a valid candidate-only delivery ledger. The unchanged first-proposal problem-capture grammar is stated explicitly. Pure, real Git attribution and write-hook race suites pass with those refusal fixtures; no operator ruling or approval is asserted.

## Round 2

### Review provenance and scope

- **Repository HEAD:** `0971e29eff1665518b95b3009af42d2f696e9c59`.
- **Revised subject SHA-256:** `da8de41fa70e7ea3ac28728cc85785677be519b643d49954066261c9105774f2`, confirmed from the working bytes. The round-1 subject hash and review record above remain preserved.
- **Changed-span input:** supplied computed [round2-subject.diff](../../.pyxis/logs/reviews/argus-plan-scope-design/round2-subject.diff), SHA-256 `f000e48045ce3bd0596b615d317c71bb1ea728eedf9f87eda15379767df88c38`. Review used its changes to §1, §2, §3.3, DINV-5 and A-6, rather than re-deriving the delta or rereading the unchanged subject.
- **Governing slice:** argus-artifacts §3.1–§3.2 only: the distinct `problem.md` / `problem-ledger/v1` and `delivery.md` / `delivery-ledger/v1` classes. The stable context was not rebuilt. Guide and calibration fingerprints remain those recorded in round 1.
- **Lint and verification provenance:** the host supplied the revised lint pass and an exit-0 result for `go test -race -count=1 ./internal/argus/boundary ./internal/argus/attribution ./cmd/argus-writehook`. The host reports real-Git valid-problem/absent-or-malformed-committed-delivery cases for both consumers, including refusal when valid delivery input exists only in the candidate. This reviewer neither executed that test command nor inspected implementation files; the result is host-supplied supporting evidence, not installed-release certification.
- **Independence and limits:** four fresh `fork_turns="none"` agents received only FND-1, the computed delta and the implicated governing slice as review inputs, without model/reasoning overrides or cross-lens findings. They ran within the runtime's three-worker concurrency limit. Only closure and changed-span regressions were judged; unchanged bindings and frozen predecessor rulings were not reopened. This manual design-review verdict establishes no native guard/admission guarantee, operator approval or exact-head code review.

### Candidate generation and survival

| Lens / fresh agent | New lens-local candidates screened | New findings retained |
|---|---:|---:|
| contract-quality / `/root/r2_contract_quality` | 0 | 0 |
| coherence / `/root/r2_coherence` | 1 | 0 |
| upstream-alignment / `/root/r2_upstream_alignment` | 0 | 0 |
| feasibility-completeness / `/root/r2_feasibility_completeness` | 2 | 0 |
| **Total** | **3** | **0** |

All four lenses independently confirm closure. Central adversarial survival and closability checks retained no new finding. Discarded candidate identities are not recorded; no unchanged-content finding was delivered or withheld as a protocol violation.

### Closure of FND-1

The author-applied disposition is accepted. Revised §2, DINV-5 and A-6 consistently require a valid committed delivery ledger; §3.3 explicitly identifies the active problem's `delivery.md` (`delivery-ledger/v1`) and distinguishes `problem.md` (`problem-ledger/v1`), matching governing argus-artifacts §3.1–§3.2. Both consumers refuse numbered plans for absent or malformed committed delivery input, including a valid candidate-only repair. A-6 now explicitly requires the valid-problem-only refusal case and retains its pure, Git-backed attribution and real write-hook verification methods; the host-supplied pass supports that acceptance condition.

The changed §3.3 preserves first-proposal problem captures when committed delivery input is absent, while excluding numbered plans; malformed committed delivery input fails closed. That clarification preserves the existing birth grammar rather than granting downstream plan scope through `problem.md`. Authoring-scope admission remains distinct from the planning-entry lock/readiness predicate. No acceptance-load-bearing gap or changed-span regression survives.

**Round-2 verdict:** approve. The two-round review is complete; FND-1 is closed, no new finding is retained, and no third round is authorized by this protocol.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-05 |
| 2 | approve | 2026-10-05 |

## Escalation brief

none

## Measurements

- **Rounds**: 2.
- **Candidates / survivors**: round 1: 17 / 1; round 2: 3 / 0; cumulative: 20 lens-local candidates / 1 retained finding.
- **Findings**: 1 blocking, closed · 0 advisory · 0 open; no new round-2 findings.
- **Dispositions**: 1 applied · 0 declined · 0 withheld-as-violation · 0 pending author.
- **Completion**: final scope-locked round-2 approve recorded; no third round or human wait.

## Overlay corrections

none

### 2026-10-05 — retain the supplied review delta in Git

The AI source coordinator copied the exact round-2 input delta, SHA-256 `f000e48045ce3bd0596b615d317c71bb1ea728eedf9f87eda15379767df88c38`, to [argus-committed-plan-attribution-round2.diff](argus-committed-plan-attribution-round2.diff). That shared artifact replaces the local-log link as the durable audit location. The original local path records the invocation context only. Review verdicts, findings, provenance, subject hashes and binding content are unchanged; no new review round or operator approval is asserted.
