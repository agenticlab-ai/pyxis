# Requirement problems

### P-1 — FR-6 — the operator wants to control the harness and model that build a prototype

**Raised by:** Operator, in review of the prepared lock on pull request 1547, 2026-10-10.

**Problem:** The contract leaves a prototype's build to whichever session asked for it, so Pyxis never chooses the harness and model and a repository cannot set them. No requirement lets the operator control what executes a prototype.

**Ruling:** operator · The operator wants to control the harness and model that execute a prototype, without reusing the implement-now flow. · Add FR-6: a Pyxis command launches the build as a separate headless run on the harness and model the repository's configuration names for prototypes, and on the asking session's harness and model when none is configured.

**Source:** Operator messages in the drive session on 2026-10-10. First: "agree with your analysis on not using the implement-now; but i still want to control the harness/model execution of the prototyping. so it should be a cmd in the pyxis ecosystem that based on the config value (if it exists) decides on the harness and the model; if there is no configured value, then it falls back to whather the ambient (current running harness and model)." Then, asked whether an unconfigured prototype is still a headless run on the current session's harness and model, and whether prototypes stay inside feature drafts for this version: "agree with both oof your answers." Recorded as authority entries A-2 and A-3 of this problem's ledger.
