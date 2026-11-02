# Amendment 008 — integrity checks: dead links, map registries, and the gate's own exemption keys

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 + 001 + 002 + 003 + 004 + 005 + 006 + 007) |
| **Status** | Approved |
| **Date** | 2026-07-08 |
| **Owner** | @mdarabi |
| **Learned-from** | review-thread:issue-54 |
| **Blast radius** | tier: additive; touches: FR-21, FR-22, FR-23; reason: three new checks over previously ungated surfaces; no existing row changes |
| **Template version** | amendment-template/v1 |

## Operations

### op: add FR

Reason: the corpus has zero broken relative links today by discipline, not by
law — nothing checks, and targets still move (the `.pyxis` footprint move left
four frozen documents pointing at `docs/templates/`) (#54 A5).

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-21 | When invoked as `check-links`, docgate shall resolve every relative markdown link target in `docs/**/*.md`, `AGENTS.md`, and the configured templates directory to an existing file — skipping fenced code blocks, external URLs, and pure `#fragment` links — and shall fail on any dangling target, naming source file, line, and target. | inbound links to frozen docs never go stale by design; outbound links rot silently when targets move, and nothing checked (#54 A5) | Should | low | a dangling-link fixture red naming file, line, and target; fenced-block, external, and fragment-only links ignored; the real corpus green | Test | G1, J3 |

### op: add FR

Reason: the system maps are freely editable living references, yet they carry
the invariant registry (`INV-<n>`, cited by designs and the DINV graduation
target) and the open-decision queue (`D<n>`) — citable IDs with no stability
enforcement, while even the non-authoritative wiki has a metadata lint
(#54 C3).

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-22 | When invoked with a merge base, docgate shall fail any diff to the configured system-map documents (defaults `docs/domain-model.md` and `docs/system-design.md`) that removes or renumbers an `INV-<n>` invariant-registry row or a `D<n>` open-decision row present at the merge base; a map file absent from the tree is skipped without defect. | registry IDs are permanent citation anchors inside deliberately mutable documents — the one quasi-authoritative class with zero protection (#54 C3) | Must | high | fixtures: removing INV-33 red; renumbering an INV row red; appending a fresh INV row green; editing non-registry prose green; absent `system-design.md` produces no defect; the real corpus green | Test | G1, J3 |

### op: add FR

Reason: the gate reads its own grandfather and exemption lists from the
ordinary, ungated config surface — a PR can add a design **and** add that design's slug
to `grandfathered_designs` in the same diff, and the gate evaluates the PR
under its self-granted exemption. This is the self-unlock pattern INV-19
forbids for risk policy, unapplied to the doc gate's own policy (#54 D1).

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-23 | When invoked with a merge base, docgate shall fail any diff modifying the gate-policy keys of the pyxis config surface — `grandfathered_designs`, `precedence_grandfathered`, and `capability_paths` — unless the commits carry the `Governing-Change: operator` trailer. | the gate's exemption surface is policy, and policy changes are never self-serve (INV-19's pattern applied to the doc gate; #54 D1) | Must | high | an untrailered diff adding a slug to `grandfathered_designs` red; the same diff trailered green; untrailered edits to `paths`/`templates`/`system` keys green; the real config green | Test | G1, J3 |
