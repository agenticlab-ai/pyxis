# Argus authority contract — <repo>

| Field | Value |
|---|---|
| **Current tier** | <a tier name declared in the ladder below> |
| **Template version** | authority-contract/v1 |

*Guidance paragraphs look like this — italic. Delete every guidance italic when instantiating. The contract is `docs/tech-design/argus-artifacts.md` §3.3. This file is **data**: grant checks read it at act time; changing it is a human act, by PR, always — **Argus never writes this file** (the checkpoint walk prepares evidence blocks; a human lands them). The never-list is NOT here and cannot be granted by any edit to this file (the artifacts design's DINV-5): self-grant, tier self-promotion, un-parking its own escalation, governance authoring, overriding a human ruling / incident lesson / breaking-call answer, exceeding the effective envelope, and ungranted outward acts refuse structurally.*

## The ladder

*Ordered tiers, each a named bundle of action classes (tokens defined in the artifacts design §3.3). A grant check is one lookup: is the act's class within the current tier's bundle? An act class in no bundle is not grantable by any data state — it escalates (fail-closed); adding a class to the vocabulary is a successor-design decision.*

| Tier (ordered) | Bundle (action classes) |
|---|---|
| `revoked` | ∅ — every act escalates; the kill switch |
| `floor` | dispose-advisory-finding |
| `tier-2` | + dispose-blocking-finding · sign-gate-verdict · trigger-materialization |
| `tier-3` | + rule-acceptance |
| `tier-4` | + approve-merge *(activates only after the operator changes org governance)* |

## Change history

*Append-only `checkpoint` entries — date · `move: promotion | demotion | revocation | none` · the resulting `tier` · evidence refs (checkpoint walk blocks, incident anchors). Prepared by the walk's runner, landed by a human PR; a bare pointer flip without a history entry is invalid (DINV-4).*

| Date | Move | Tier | Evidence |
|---|---|---|---|
| <YYYY-MM-DD> | none | <initial tier> | born at <tier> — the introducing PR |

---

## Appendix — structure checklist *(walked at every change and by review; delete from instantiated contracts)*

- [ ] Current tier names a tier declared in the ladder
- [ ] The ladder is ordered, `revoked` present, every bundle token a **defined action-class token** — never-class acts are absent from the vocabulary by construction, so an attempt to grant one fails here (DINV-5 is structural; there is no separate "no never-grant" scan)
- [ ] Every history row carries date, move (from the closed enum), resulting tier, and evidence refs
- [ ] The latest history row's tier equals Current tier
