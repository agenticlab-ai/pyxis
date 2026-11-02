---
name: impl-review
description: The unmanned implementation reviewer — spawned as a fresh sibling session of the implementing worker (never a child), it runs four blind fresh-context lenses (spec-fidelity · design-conformance · security-abuse · correctness-critic) over one shared review context pack, adversarially screens their candidates, and persists one self-contained machine-payload review on the proposal's own medium. Invoked by the host at a proposal's ladder-green boundary with the proposal reference and the reviewed revision ("/impl-review <proposal-ref> <reviewed-revision>").
---

Follow `.pyxis/authoring/impl-review.md` — the vendor-neutral review protocol
(AP-25: this file is invocation glue only; all protocol knowledge lives there).
Read that guide fully and execute it, including its Never list.

Input: the **proposal reference** (its medium reference — a PR the review is
raised on) and the **reviewed revision** (the head sha this review evaluates —
the DINV-7 binding); ask if either is missing. The Go core this skill drives is
`internal/implreview/runtime` (the sibling-session provenance check, the four
blind lenses over the T3 pack, the adversarial screen, round assembly); the
payload schema is `internal/implreview/report`, the context pack is
`internal/implreview/pack`, and persistence is `internal/implreview/persist`.
The behavior contract is `docs/prds/impl-review/`; the realization design is
`docs/tech-design/impl-review-v1.md` (§3.4 governs this runtime). The four
lenses' **judgment content is authored as reviewed data** — one file per lens
under `.pyxis/skills/impl-review/lenses/`, loaded by `internal/implreview/lens`
— and the **severity dial** each finding cites is
`.pyxis/skills/impl-review/calibration.md`
(`impl-review-calibration/v1`, parsed by `internal/implreview/calibration`); the
mechanical-floor exclusion is `internal/implreview/floor`. This skill owns the
topology, the blindness, the screen, and the provenance record; the lens
instructions and calibration are data it loads, not prose it carries.

When the invoking host declares a one-shot authenticated artifact broker, obey
that delivery boundary: return the exact self-contained artifact as the terminal
response and perform no direct proposal/code-host write. The host validates,
publishes, and exact-reads its receipt. This changes only who holds the write
credential; the artifact remains the single proposal-medium record.
