# Argus attribution hardening — problem ledger

| Field | Value |
|---|---|
| **Slug** | argus-attribution-hardening |
| **Principal** | @mdarabi (the human problem provider) |
| **Authority-contract version** | authority-contract/v1 · tier-2 (2026-07-11 operator-directed promotion) |
| **Envelope override** | — |
| **Template version** | problem-ledger/v1 |

## Idea

The boundary-enforcement verification found the attribution gate — the layer meant to refuse, at the merge boundary, an Argus-authored diff that touches a protected path (the authority contract, a governance document, or an out-of-declared-scope file) — does not actually bite in practice. Three gaps:
(1) It is LATENT. The gate identifies Argus's commits by an `Argus-Drive` commit trailer, but nothing produces that trailer — not the drive, the skill, any git hook, or the worker — so real Argus commits carry none, classify as Human, and are served. The gate guards a signal that is never emitted.
(2) The trailer is UNAUTHENTICATED. It is a plaintext, author-writable line with no forgery resistance (the attribution-gate design carries exactly this as its own open question, §7 Q1). A misbehaving or injected drive can omit or forge it.
(3) An ABSENT trailer on a protected-path diff is served as Human (DINV-2 as built), so stripping the trailer masquerades as a legitimate human edit — while the legitimate human-author path to those paths must genuinely stay open (NR-4).
Make the attribution gate a real, unbypassable merge-boundary guarantee: Argus authorship must be mechanically produced (unbypassable), forgery-resistant, and fail-closed on absence for the protected subset — without closing the human-author path. Treat the frozen attribution-gate design (`docs/tech-design/argus-attribution-gate.md`) and the boundary-enforcement PRD (`docs/prds/argus-boundary-enforcement/`) as governing prior art to cite and EVOLVE BY SUPERSESSION, never to edit (they are frozen/merged).

## Authority entries

### A-1 — intake envelope: repo default, no override

- **Date**: 2026-07-13
- **Author**: @mdarabi
- **Authority-contract version**: authority-contract/v1 · tier-2
- **Answers**: —
- **Ruling**: No per-problem envelope override supplied at intake; the repo-default envelope governs this drive. Recorded so the derived-block spend watch has its bound. (Intake pre-ruling — not a brief answer.)

---
