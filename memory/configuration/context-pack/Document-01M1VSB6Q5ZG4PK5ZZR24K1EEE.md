# .pyxis/context-pack.yaml — per-stage stable-context declarations for the
# context pack (docs/tech-design/context-pack-v1.md §3.2; PRD FR-3).
#
# Closed parse: each entry is `- <kind>[?] <selector>`. Kinds are the closed
# admissible set — governing · subject · calibration — and NO authoring-trail
# kind is admissible, so a declaration cannot name the delivery journal,
# dispositions, or orchestration (DINV-3, the independence boundary). A `?`
# after the kind marks the source optional: absent → recorded in the manifest,
# never a defect (FR-7); a required source that does not resolve fails closed
# (UB-1, DINV-6). `{subject}` is substituted with the subject slug. Selector
# order is the pack order and is preserved verbatim (DINV-1, DINV-4).
#
# This is data: adding a stage or changing a stage's classes is a config PR,
# never a code change (NFR-4). The admissible-kind set is the one thing that
# is code — enlarging it is a reviewed change, which is what keeps the
# independence boundary structural.

prd-review:
  - governing docs/product-vision.md
  - governing docs/architecture-principles.md
  - governing docs/observability/incidents.md
  - subject docs/prds/{subject}/effective.md
  - calibration .pyxis/skills/prd-review/calibration.md

tech-design-review:
  - governing docs/product-vision.md
  - governing docs/architecture-principles.md
  - governing docs/observability/incidents.md
  - subject docs/tech-design/{subject}.md
  - calibration .pyxis/skills/tech-design-review/calibration.md

# e2e-review (the fourth reviewer-program instance) is deliberately NOT
# declared here: its governing design(s) are named by the subject e2e design
# doc's own "Governing design(s)" header — a two-hop resolution (read one
# source to compute another's selector) this file's single-pass {subject}
# substitution cannot express. It is declared in code instead —
# internal/e2e/review.Declaration — over the SAME three admitted kinds, and
# `docgate context-pack --stage e2e-review --subject <feature>` dispatches to
# it (cmd/docgate/main.go), so the CLI surface still matches its siblings'.
