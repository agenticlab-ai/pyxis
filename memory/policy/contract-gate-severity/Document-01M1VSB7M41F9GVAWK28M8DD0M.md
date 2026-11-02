# .pyxis/severity-map.yaml — the contract-gate severity-map policy on the pyxis
# config surface (prd:contract-gate/FR-13; docs/tech-design/contract-gate-v1.md
# §3.10). It encodes, PER TRIGGER, which contract-scope check CLASS is blocking
# versus advisory. Closed-parse: an unknown top-level key, trigger, check class,
# or severity is a configuration-error (UB-6). A MISSING file uses the built-in
# default split (this file states that default explicitly). Changing a severity
# here is a DATA edit — no behavior-code change (AP-6/AP-10); the §3.3 wrong-bounce
# demotion path is a human-lane edit to this file.
#
# Triggers (the closed set): planning-entry (the hard precondition before
# plan-new drafts) and contract-doc-pr (the continuous CI surface on PRD/design
# PRs). Check classes (the EVT-2 taxonomy, one per registered check):
# structural · compilability · currency · coverage · link.
# Severities: blocking (a defect fails the run) · advisory (surfaced, non-blocking).
#
# An omitted (trigger, class) inherits the built-in default; the entries below are
# the full default split (the #D-4 seed) written out.
schema_version: "1"
triggers:
  # Planning entry is a HARD gate: a contract entering planning must be whole, so
  # every class blocks.
  planning-entry:
    structural: blocking
    compilability: blocking
    currency: blocking
    coverage: blocking
    link: blocking
  # The continuous contract-doc-PR surface blocks on drift that is always a defect
  # (currency, link, structural, compilability) but keeps inverse-coverage
  # ADVISORY — a PRD legitimately merges before its serving designs exist, so a
  # missing-serving-design gap cannot block a contract-doc PR (#D-4).
  contract-doc-pr:
    structural: blocking
    compilability: blocking
    currency: blocking
    coverage: advisory
    link: blocking
