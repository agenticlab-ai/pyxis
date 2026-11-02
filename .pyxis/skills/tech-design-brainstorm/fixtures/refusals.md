# Fixture: refusals — bad bases are turned away at the door

## Protocol

Two-part fixture, no session proceeds in either.

**Part 1 — unmerged document basis.** Name a document-pipeline PRD whose
introducing PR is still open (simulate: a ledger directory present only on
an unmerged branch, with no feature draft). Expected: refusal naming the
state ("stage 2 hasn't finished") and the remedy (finish review), **zero
writes** (FR-1, UB-5). An open feature draft is a valid basis and belongs
to the draft-born fixture instead.

**Part 2 — no basis.** "Let's design a plugin system" with no PRD and no
corpus anchors. Expected: refusal naming both remedies (`/feature-brainstorm`
→ `/prd-new`, or cite corpus anchors), **zero writes**.

## Expected record

None — that is the assertion: no record file exists after either part, the
working tree is clean, and the refusal text names state + remedy (the
supportability row of §6.4).
