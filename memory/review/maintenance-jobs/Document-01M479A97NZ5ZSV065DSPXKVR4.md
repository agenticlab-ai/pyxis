# Review: maintenance-jobs draft — operator revision design re-review

| Field | Value |
|---|---|
| **In one line** | Approve: the restrictive credential policy and bounded CI evidence capability close the named engineering findings. |
| **Relevant-when** | Auditing closure of the operator-revision design findings before exact-head approval. |
| **Subject** | design:maintenance-jobs · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | tech-design-review · repository guide at 2fb924d6e22b026ccf975ff4f26683f9bfc89ca9 |
| **Date(s)** | 2026-10-05 |
| **Verdict** | approve |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1443 |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec
**Draft digest:** sha256:acc0d3addd7608c1e93a7b75a8ef57078b7d2a0ca6e5616e3ca448d18ae01bc0

Reviewed committed head 2fb924d6e22b026ccf975ff4f26683f9bfc89ca9. This is
round 2 of the operator-revision episode begun in reviews/006.md. Parallel
lenses examined the changed spans at fb5bb0c4ecc499cd336ad98bb9b9ff3cdb85ee1e;
an author-history correction normalized problem serialization before push.
An exact Git diff confirms both contract blobs are identical at the named
current head, so this mapping creates no third contract revision or review round.

## Round 2

The same four lenses re-reviewed only the repairs and their implicated
governing slices. The lead independently screened the closures:

- FND-1: D-5 explicitly distinguishes the existing write policy from the new
  optional host-owned restrictive read/credential policy. It covers selected
  provider auth projection, credential-free clone configuration, positive read
  grants, host credential services and process inspection, inherited handles,
  and descendant/delegated narrowing. The minimal pre-claim fixture and actual
  execution-clone probes have distinct lifecycle ownership; unsupported
  enforcement refuses. D-7 tests denied secret access and allowed source/tool/
  model-auth access through all four harnesses on Darwin/Linux, preserving
  legacy launch behavior. This is an implementation obligation, not a claim
  that today's runtime already provides read isolation.
- FND-2: D-1/D-5 assign the new historical CI capability to the maintenance-owned
  EvidenceReader port on the existing code-host adapter axis, reusing proposal,
  check, and authenticated transport owners. Its versioned result records
  identities, source timestamps, settings availability, and measurement limits.
  Retrieval has a deadline, 200-candidate and 10 MiB bounds; cutoffs remain
  explicit. D-7 covers pagination, mismatch, unavailable settings, and partial
  evidence. No harness receives code-host authority.

No new finding or changed-span regression survived calibration. No unchanged
content, recorded operator ruling, human-only PRD routing section, or lint floor
was reopened. The requirements' existing acceptance contract remains served.
Approval is of this design; four-adapter platform proof and live adoption
evidence still require implementation after the human lock.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 (reviews/006.md) | request-changes | 2026-10-05 |
| 2 | approve | 2026-10-05 |

## Escalation brief

none

## Measurements

- **Rounds**: 2 in this episode
- **Findings**: 1 blocking · 1 advisory in the prior report · 0 new findings
- **Dispositions**: 2 applied in the prior report · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
