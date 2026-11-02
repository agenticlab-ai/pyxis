# skill-command-prefix — `pyxis-` harness skill namespace — plan 001

| Field | Value |
|---|---|
| **Feature** | skill-command-prefix |
| **PRD** | docs/prds/skill-command-prefix |
| **Design** | docs/tech-design/skill-command-prefix-v1.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Namespace every harness skill as pyxis-<basename> with a closing prefix gate |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — Rename `argus` to `pyxis-argus`

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the skills-tree entry for argus · the authoring guide for argus · system-tree identity refs for argus · structural live citations of argus · generated harness views for argus |

**Goal:** The `argus` harness skill carries one canonical `pyxis-argus` identity end to end on the live surface, hard-cut with no alias.

**Provenance:**
- Rename the skills-tree directory and frontmatter `name:` for `argus` to `pyxis-argus` (stem unchanged).
- Rename the matching authoring-guide filename for `argus` to the same `pyxis-argus` stem.
- Rewrite every allowed live occurrence of this skill's identity per the design occurrence-matching rule (structural forms on live surfaces; exhaustive under the system tree); leave frozen merged documents untouched for historical strings.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Hard-cut: leave no alias, symlink, or alternate `name:` restoring `argus`.

**Acceptance criteria:**
- [ ] skills-tree directory basename and frontmatter `name:` both equal `pyxis-argus`, and the matching authoring-guide filename uses the same stem
- [ ] the former unprefixed `argus` skill identity is absent from the skills tree with no alias restoring it
- [ ] generated harness skill views expose `pyxis-argus` for this skill after converge regenerate
- [ ] live structural citations and system-tree references to this skill's identity use the prefixed form

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T2 — Verify the active skill catalog after retired-entry removal

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the active skills tree · generated harness views · structural live skill citations |

**Goal:** The skill namespace migration covers every active skill and does not recreate an entry that has been removed from the catalog.

**Provenance:**
- Enumerate the active skills tree after retired entries are removed.
- Confirm every remaining skill is owned by exactly one ordinary rename task in this plan.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Fail the catalog check on any missing, duplicate, or unplanned active skill identity.

**Acceptance criteria:**
- [ ] every active skill appears in exactly one rename task and has matching directory and frontmatter identities
- [ ] no removed catalog entry is recreated by the namespace migration
- [ ] generated harness skill views contain exactly the active prefixed skill set after converge regenerate
- [ ] live structural citations and system-tree references use the prefixed identities for active skills

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T3 — Rename `e2e-design` to `pyxis-e2e-design`

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the skills-tree entry for e2e-design · the authoring guide for e2e-design · system-tree identity refs for e2e-design · structural live citations of e2e-design · generated harness views for e2e-design |

**Goal:** The `e2e-design` harness skill carries one canonical `pyxis-e2e-design` identity end to end on the live surface, hard-cut with no alias.

**Provenance:**
- Rename the skills-tree directory and frontmatter `name:` for `e2e-design` to `pyxis-e2e-design` (stem unchanged).
- Rename the matching authoring-guide filename for `e2e-design` to the same `pyxis-e2e-design` stem.
- Rewrite every allowed live occurrence of this skill's identity per the design occurrence-matching rule (structural forms on live surfaces; exhaustive under the system tree); leave frozen merged documents untouched for historical strings.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Hard-cut: leave no alias, symlink, or alternate `name:` restoring `e2e-design`.

**Acceptance criteria:**
- [ ] skills-tree directory basename and frontmatter `name:` both equal `pyxis-e2e-design`, and the matching authoring-guide filename uses the same stem
- [ ] the former unprefixed `e2e-design` skill identity is absent from the skills tree with no alias restoring it
- [ ] generated harness skill views expose `pyxis-e2e-design` for this skill after converge regenerate
- [ ] live structural citations and system-tree references to this skill's identity use the prefixed form

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T4 — Rename `e2e-plan` to `pyxis-e2e-plan`

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the skills-tree entry for e2e-plan · the authoring guide for e2e-plan · system-tree identity refs for e2e-plan · structural live citations of e2e-plan · generated harness views for e2e-plan |

**Goal:** The `e2e-plan` harness skill carries one canonical `pyxis-e2e-plan` identity end to end on the live surface, hard-cut with no alias.

**Provenance:**
- Rename the skills-tree directory and frontmatter `name:` for `e2e-plan` to `pyxis-e2e-plan` (stem unchanged).
- Rename the matching authoring-guide filename for `e2e-plan` to the same `pyxis-e2e-plan` stem.
- Rewrite every allowed live occurrence of this skill's identity per the design occurrence-matching rule (structural forms on live surfaces; exhaustive under the system tree); leave frozen merged documents untouched for historical strings.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Hard-cut: leave no alias, symlink, or alternate `name:` restoring `e2e-plan`.

**Acceptance criteria:**
- [ ] skills-tree directory basename and frontmatter `name:` both equal `pyxis-e2e-plan`, and the matching authoring-guide filename uses the same stem
- [ ] the former unprefixed `e2e-plan` skill identity is absent from the skills tree with no alias restoring it
- [ ] generated harness skill views expose `pyxis-e2e-plan` for this skill after converge regenerate
- [ ] live structural citations and system-tree references to this skill's identity use the prefixed form

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T5 — Rename `e2e-review` to `pyxis-e2e-review`

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the skills-tree entry for e2e-review · the authoring guide for e2e-review · system-tree identity refs for e2e-review · structural live citations of e2e-review · generated harness views for e2e-review |

**Goal:** The `e2e-review` harness skill carries one canonical `pyxis-e2e-review` identity end to end on the live surface, hard-cut with no alias.

**Provenance:**
- Rename the skills-tree directory and frontmatter `name:` for `e2e-review` to `pyxis-e2e-review` (stem unchanged).
- Rename the matching authoring-guide filename for `e2e-review` to the same `pyxis-e2e-review` stem.
- Rewrite every allowed live occurrence of this skill's identity per the design occurrence-matching rule (structural forms on live surfaces; exhaustive under the system tree); leave frozen merged documents untouched for historical strings.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Hard-cut: leave no alias, symlink, or alternate `name:` restoring `e2e-review`.

**Acceptance criteria:**
- [ ] skills-tree directory basename and frontmatter `name:` both equal `pyxis-e2e-review`, and the matching authoring-guide filename uses the same stem
- [ ] the former unprefixed `e2e-review` skill identity is absent from the skills tree with no alias restoring it
- [ ] generated harness skill views expose `pyxis-e2e-review` for this skill after converge regenerate
- [ ] live structural citations and system-tree references to this skill's identity use the prefixed form

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T6 — Rename `feature-brainstorm` to `pyxis-feature-brainstorm`

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the skills-tree entry for feature-brainstorm · the authoring guide for feature-brainstorm · system-tree identity refs for feature-brainstorm · structural live citations of feature-brainstorm · generated harness views for feature-brainstorm |

**Goal:** The `feature-brainstorm` harness skill carries one canonical `pyxis-feature-brainstorm` identity end to end on the live surface, hard-cut with no alias.

**Provenance:**
- Rename the skills-tree directory and frontmatter `name:` for `feature-brainstorm` to `pyxis-feature-brainstorm` (stem unchanged).
- Rename the matching authoring-guide filename for `feature-brainstorm` to the same `pyxis-feature-brainstorm` stem.
- Rewrite every allowed live occurrence of this skill's identity per the design occurrence-matching rule (structural forms on live surfaces; exhaustive under the system tree); leave frozen merged documents untouched for historical strings.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Hard-cut: leave no alias, symlink, or alternate `name:` restoring `feature-brainstorm`.

**Acceptance criteria:**
- [ ] skills-tree directory basename and frontmatter `name:` both equal `pyxis-feature-brainstorm`, and the matching authoring-guide filename uses the same stem
- [ ] the former unprefixed `feature-brainstorm` skill identity is absent from the skills tree with no alias restoring it
- [ ] generated harness skill views expose `pyxis-feature-brainstorm` for this skill after converge regenerate
- [ ] live structural citations and system-tree references to this skill's identity use the prefixed form

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T7 — Rename `help` to `pyxis-help`

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the skills-tree entry for help · the authoring guide for help · system-tree identity refs for help · structural live citations of help · generated harness views for help |

**Goal:** The `help` harness skill carries one canonical `pyxis-help` identity end to end on the live surface, hard-cut with no alias.

**Provenance:**
- Rename the skills-tree directory and frontmatter `name:` for `help` to `pyxis-help` (stem unchanged).
- Rename the matching authoring-guide filename for `help` to the same `pyxis-help` stem.
- Rewrite every allowed live occurrence of this skill's identity per the design occurrence-matching rule (structural forms on live surfaces; exhaustive under the system tree); leave frozen merged documents untouched for historical strings.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Hard-cut: leave no alias, symlink, or alternate `name:` restoring `help`.

**Acceptance criteria:**
- [ ] skills-tree directory basename and frontmatter `name:` both equal `pyxis-help`, and the matching authoring-guide filename uses the same stem
- [ ] the former unprefixed `help` skill identity is absent from the skills tree with no alias restoring it
- [ ] generated harness skill views expose `pyxis-help` for this skill after converge regenerate
- [ ] live structural citations and system-tree references to this skill's identity use the prefixed form

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T8 — Rename `impl-review` to `pyxis-impl-review`

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the skills-tree entry for impl-review · the authoring guide for impl-review · system-tree identity refs for impl-review · structural live citations of impl-review · generated harness views for impl-review |

**Goal:** The `impl-review` harness skill carries one canonical `pyxis-impl-review` identity end to end on the live surface, hard-cut with no alias.

**Provenance:**
- Rename the skills-tree directory and frontmatter `name:` for `impl-review` to `pyxis-impl-review` (stem unchanged).
- Rename the matching authoring-guide filename for `impl-review` to the same `pyxis-impl-review` stem.
- Rewrite every allowed live occurrence of this skill's identity per the design occurrence-matching rule (structural forms on live surfaces; exhaustive under the system tree); leave frozen merged documents untouched for historical strings.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Hard-cut: leave no alias, symlink, or alternate `name:` restoring `impl-review`.

**Acceptance criteria:**
- [ ] skills-tree directory basename and frontmatter `name:` both equal `pyxis-impl-review`, and the matching authoring-guide filename uses the same stem
- [ ] the former unprefixed `impl-review` skill identity is absent from the skills tree with no alias restoring it
- [ ] generated harness skill views expose `pyxis-impl-review` for this skill after converge regenerate
- [ ] live structural citations and system-tree references to this skill's identity use the prefixed form

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T9 — Rename `implement-now` to `pyxis-implement-now`

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the skills-tree entry for implement-now · the authoring guide for implement-now · system-tree identity refs for implement-now · structural live citations of implement-now · generated harness views for implement-now |

**Goal:** The `implement-now` harness skill carries one canonical `pyxis-implement-now` identity end to end on the live surface, hard-cut with no alias.

**Provenance:**
- Rename the skills-tree directory and frontmatter `name:` for `implement-now` to `pyxis-implement-now` (stem unchanged).
- Rename the matching authoring-guide filename for `implement-now` to the same `pyxis-implement-now` stem.
- Rewrite every allowed live occurrence of this skill's identity per the design occurrence-matching rule (structural forms on live surfaces; exhaustive under the system tree); leave frozen merged documents untouched for historical strings.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Hard-cut: leave no alias, symlink, or alternate `name:` restoring `implement-now`.

**Acceptance criteria:**
- [ ] skills-tree directory basename and frontmatter `name:` both equal `pyxis-implement-now`, and the matching authoring-guide filename uses the same stem
- [ ] the former unprefixed `implement-now` skill identity is absent from the skills tree with no alias restoring it
- [ ] generated harness skill views expose `pyxis-implement-now` for this skill after converge regenerate
- [ ] live structural citations and system-tree references to this skill's identity use the prefixed form

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T10 — Rename `incident` to `pyxis-incident`

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the skills-tree entry for incident · the authoring guide for incident · system-tree identity refs for incident · structural live citations of incident · generated harness views for incident |

**Goal:** The `incident` harness skill carries one canonical `pyxis-incident` identity end to end on the live surface, hard-cut with no alias.

**Provenance:**
- Rename the skills-tree directory and frontmatter `name:` for `incident` to `pyxis-incident` (stem unchanged).
- Rename the matching authoring-guide filename for `incident` to the same `pyxis-incident` stem.
- Rewrite every allowed live occurrence of this skill's identity per the design occurrence-matching rule (structural forms on live surfaces; exhaustive under the system tree); leave frozen merged documents untouched for historical strings.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Hard-cut: leave no alias, symlink, or alternate `name:` restoring `incident`.

**Acceptance criteria:**
- [ ] skills-tree directory basename and frontmatter `name:` both equal `pyxis-incident`, and the matching authoring-guide filename uses the same stem
- [ ] the former unprefixed `incident` skill identity is absent from the skills tree with no alias restoring it
- [ ] generated harness skill views expose `pyxis-incident` for this skill after converge regenerate
- [ ] live structural citations and system-tree references to this skill's identity use the prefixed form

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T11 — Rename `plan-new` to `pyxis-plan-new`

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the skills-tree entry for plan-new · the authoring guide for plan-new · system-tree identity refs for plan-new · structural live citations of plan-new · generated harness views for plan-new |

**Goal:** The `plan-new` harness skill carries one canonical `pyxis-plan-new` identity end to end on the live surface, hard-cut with no alias.

**Provenance:**
- Rename the skills-tree directory and frontmatter `name:` for `plan-new` to `pyxis-plan-new` (stem unchanged).
- Rename the matching authoring-guide filename for `plan-new` to the same `pyxis-plan-new` stem.
- Rewrite every allowed live occurrence of this skill's identity per the design occurrence-matching rule (structural forms on live surfaces; exhaustive under the system tree); leave frozen merged documents untouched for historical strings.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Hard-cut: leave no alias, symlink, or alternate `name:` restoring `plan-new`.

**Acceptance criteria:**
- [ ] skills-tree directory basename and frontmatter `name:` both equal `pyxis-plan-new`, and the matching authoring-guide filename uses the same stem
- [ ] the former unprefixed `plan-new` skill identity is absent from the skills tree with no alias restoring it
- [ ] generated harness skill views expose `pyxis-plan-new` for this skill after converge regenerate
- [ ] live structural citations and system-tree references to this skill's identity use the prefixed form

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T12 — Rename `prd-amend` to `pyxis-prd-amend`

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the skills-tree entry for prd-amend · the authoring guide for prd-amend · system-tree identity refs for prd-amend · structural live citations of prd-amend · generated harness views for prd-amend |

**Goal:** The `prd-amend` harness skill carries one canonical `pyxis-prd-amend` identity end to end on the live surface, hard-cut with no alias.

**Provenance:**
- Rename the skills-tree directory and frontmatter `name:` for `prd-amend` to `pyxis-prd-amend` (stem unchanged).
- Rename the matching authoring-guide filename for `prd-amend` to the same `pyxis-prd-amend` stem.
- Rewrite every allowed live occurrence of this skill's identity per the design occurrence-matching rule (structural forms on live surfaces; exhaustive under the system tree); leave frozen merged documents untouched for historical strings.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Hard-cut: leave no alias, symlink, or alternate `name:` restoring `prd-amend`.

**Acceptance criteria:**
- [ ] skills-tree directory basename and frontmatter `name:` both equal `pyxis-prd-amend`, and the matching authoring-guide filename uses the same stem
- [ ] the former unprefixed `prd-amend` skill identity is absent from the skills tree with no alias restoring it
- [ ] generated harness skill views expose `pyxis-prd-amend` for this skill after converge regenerate
- [ ] live structural citations and system-tree references to this skill's identity use the prefixed form

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T13 — Rename `prd-new` to `pyxis-prd-new`

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the skills-tree entry for prd-new · the authoring guide for prd-new · system-tree identity refs for prd-new · structural live citations of prd-new · generated harness views for prd-new |

**Goal:** The `prd-new` harness skill carries one canonical `pyxis-prd-new` identity end to end on the live surface, hard-cut with no alias.

**Provenance:**
- Rename the skills-tree directory and frontmatter `name:` for `prd-new` to `pyxis-prd-new` (stem unchanged).
- Rename the matching authoring-guide filename for `prd-new` to the same `pyxis-prd-new` stem.
- Rewrite every allowed live occurrence of this skill's identity per the design occurrence-matching rule (structural forms on live surfaces; exhaustive under the system tree); leave frozen merged documents untouched for historical strings.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Hard-cut: leave no alias, symlink, or alternate `name:` restoring `prd-new`.

**Acceptance criteria:**
- [ ] skills-tree directory basename and frontmatter `name:` both equal `pyxis-prd-new`, and the matching authoring-guide filename uses the same stem
- [ ] the former unprefixed `prd-new` skill identity is absent from the skills tree with no alias restoring it
- [ ] generated harness skill views expose `pyxis-prd-new` for this skill after converge regenerate
- [ ] live structural citations and system-tree references to this skill's identity use the prefixed form

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T14 — Rename `prd-review` to `pyxis-prd-review`

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the skills-tree entry for prd-review · the authoring guide for prd-review · system-tree identity refs for prd-review · structural live citations of prd-review · generated harness views for prd-review |

**Goal:** The `prd-review` harness skill carries one canonical `pyxis-prd-review` identity end to end on the live surface, hard-cut with no alias.

**Provenance:**
- Rename the skills-tree directory and frontmatter `name:` for `prd-review` to `pyxis-prd-review` (stem unchanged).
- Rename the matching authoring-guide filename for `prd-review` to the same `pyxis-prd-review` stem.
- Rewrite every allowed live occurrence of this skill's identity per the design occurrence-matching rule (structural forms on live surfaces; exhaustive under the system tree); leave frozen merged documents untouched for historical strings.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Hard-cut: leave no alias, symlink, or alternate `name:` restoring `prd-review`.

**Acceptance criteria:**
- [ ] skills-tree directory basename and frontmatter `name:` both equal `pyxis-prd-review`, and the matching authoring-guide filename uses the same stem
- [ ] the former unprefixed `prd-review` skill identity is absent from the skills tree with no alias restoring it
- [ ] generated harness skill views expose `pyxis-prd-review` for this skill after converge regenerate
- [ ] live structural citations and system-tree references to this skill's identity use the prefixed form

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T15 — Rename `tech-design-brainstorm` to `pyxis-tech-design-brainstorm`

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the skills-tree entry for tech-design-brainstorm · the authoring guide for tech-design-brainstorm · system-tree identity refs for tech-design-brainstorm · structural live citations of tech-design-brainstorm · generated harness views for tech-design-brainstorm |

**Goal:** The `tech-design-brainstorm` harness skill carries one canonical `pyxis-tech-design-brainstorm` identity end to end on the live surface, hard-cut with no alias.

**Provenance:**
- Rename the skills-tree directory and frontmatter `name:` for `tech-design-brainstorm` to `pyxis-tech-design-brainstorm` (stem unchanged).
- Rename the matching authoring-guide filename for `tech-design-brainstorm` to the same `pyxis-tech-design-brainstorm` stem.
- Rewrite every allowed live occurrence of this skill's identity per the design occurrence-matching rule (structural forms on live surfaces; exhaustive under the system tree); leave frozen merged documents untouched for historical strings.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Hard-cut: leave no alias, symlink, or alternate `name:` restoring `tech-design-brainstorm`.

**Acceptance criteria:**
- [ ] skills-tree directory basename and frontmatter `name:` both equal `pyxis-tech-design-brainstorm`, and the matching authoring-guide filename uses the same stem
- [ ] the former unprefixed `tech-design-brainstorm` skill identity is absent from the skills tree with no alias restoring it
- [ ] generated harness skill views expose `pyxis-tech-design-brainstorm` for this skill after converge regenerate
- [ ] live structural citations and system-tree references to this skill's identity use the prefixed form

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T16 — Rename `tech-design-new` to `pyxis-tech-design-new`

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the skills-tree entry for tech-design-new · the authoring guide for tech-design-new · system-tree identity refs for tech-design-new · structural live citations of tech-design-new · generated harness views for tech-design-new |

**Goal:** The `tech-design-new` harness skill carries one canonical `pyxis-tech-design-new` identity end to end on the live surface, hard-cut with no alias.

**Provenance:**
- Rename the skills-tree directory and frontmatter `name:` for `tech-design-new` to `pyxis-tech-design-new` (stem unchanged).
- Rename the matching authoring-guide filename for `tech-design-new` to the same `pyxis-tech-design-new` stem.
- Rewrite every allowed live occurrence of this skill's identity per the design occurrence-matching rule (structural forms on live surfaces; exhaustive under the system tree); leave frozen merged documents untouched for historical strings.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Hard-cut: leave no alias, symlink, or alternate `name:` restoring `tech-design-new`.

**Acceptance criteria:**
- [ ] skills-tree directory basename and frontmatter `name:` both equal `pyxis-tech-design-new`, and the matching authoring-guide filename uses the same stem
- [ ] the former unprefixed `tech-design-new` skill identity is absent from the skills tree with no alias restoring it
- [ ] generated harness skill views expose `pyxis-tech-design-new` for this skill after converge regenerate
- [ ] live structural citations and system-tree references to this skill's identity use the prefixed form

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T17 — Rename `tech-design-review` to `pyxis-tech-design-review`

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the skills-tree entry for tech-design-review · the authoring guide for tech-design-review · system-tree identity refs for tech-design-review · structural live citations of tech-design-review · generated harness views for tech-design-review |

**Goal:** The `tech-design-review` harness skill carries one canonical `pyxis-tech-design-review` identity end to end on the live surface, hard-cut with no alias.

**Provenance:**
- Rename the skills-tree directory and frontmatter `name:` for `tech-design-review` to `pyxis-tech-design-review` (stem unchanged).
- Rename the matching authoring-guide filename for `tech-design-review` to the same `pyxis-tech-design-review` stem.
- Rewrite every allowed live occurrence of this skill's identity per the design occurrence-matching rule (structural forms on live surfaces; exhaustive under the system tree); leave frozen merged documents untouched for historical strings.
- Regenerate harness skill views via converge reconcile so vendor-visible names match; never hand-edit generated views.
- Hard-cut: leave no alias, symlink, or alternate `name:` restoring `tech-design-review`.

**Acceptance criteria:**
- [ ] skills-tree directory basename and frontmatter `name:` both equal `pyxis-tech-design-review`, and the matching authoring-guide filename uses the same stem
- [ ] the former unprefixed `tech-design-review` skill identity is absent from the skills tree with no alias restoring it
- [ ] generated harness skill views expose `pyxis-tech-design-review` for this skill after converge regenerate
- [ ] live structural citations and system-tree references to this skill's identity use the prefixed form

**Traces:** prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/NR-1 · design:skill-command-prefix-v1#3.1 · design:skill-command-prefix-v1#3.2 · design:skill-command-prefix-v1#3.3 · design:skill-command-prefix-v1#DINV-1 · design:skill-command-prefix-v1#DINV-2 · design:skill-command-prefix-v1#DINV-3 · design:skill-command-prefix-v1#DINV-5

### T18 — Land the prefix conformance gate and close the namespace

| Field | Value |
|---|---|
| **Blocked by** | T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15, T16, T17 |
| **Size band** | 80–160 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the scripts-test prefix-conformance harness · the payload Go wrapper for that harness · the glossary Argus entry · leftover generated harness view drift |

**Goal:** After every skill is prefixed, the mechanical `pyxis-*` prefix conformance test is present and green, the glossary confirms Argus persona vs `/pyxis-argus`, and no unprefixed pyxis skill remains on the live vendor surface.

**Current behavior:** Skills may still be mid-train or fully renamed, but no scripts-test / payload wrapper refuses a non-`pyxis-*` skills-tree directory or frontmatter `name:`; mid-train mixed names are allowed; glossary may already distinguish persona vs skill from the PRD introducing work but M1 confirm is not yet the closing checkpoint.

**Desired behavior:** A conformance test in the scripts-test harness, reached by CI via a Go test wrapper in the payload package (help-purity pattern), fails closed if any skills-tree directory basename or frontmatter `name:` violates `pyxis-*`, and fails if any unprefixed skill remains (closing precondition). The test did not exist before this task (mid-train absence). Glossary Argus entry distinguishes the AI Engineer persona from `/pyxis-argus`. Any leftover harness-view drift is cleared via converge regenerate, not hand edit. Engine CLI verbs and frozen merged docs are unchanged by this close.

**Scope:**
- the scripts-test prefix-conformance suite plus the payload Go wrapper that invokes it under code-gate
- confirmation of the Argus / `/pyxis-argus` glossary split at the M1 checkpoint
- converge backstop for any remaining generated harness view drift after the rename train

**Out of scope:**
- renaming any remaining skill (all rename tasks must already be done)
- adding a docgate skill-prefix check
- rewriting frozen merged documents for historical unprefixed strings
- renaming engine CLI binaries or verbs

**Acceptance criteria:**
- [ ] the prefix conformance test is present under the scripts-test harness and fails a sandbox skill whose directory or `name:` is not `pyxis-*`
- [ ] the live skills tree is entirely `pyxis-*` directories with matching frontmatter `name:` values when the test runs green
- [ ] the glossary Argus entry distinguishes the persona from `/pyxis-argus`
- [ ] no docgate skill-prefix check is added; generated harness views that still drifted are regenerated via converge, not hand-edited

**Key interfaces:**
- the scripts-test harness convention and the payload package Go wrapper pattern (same shape as help purity)
- the skills-tree directory basename + frontmatter `name:` prefix rule (`pyxis-*`)
- the glossary Argus persona vs harness-skill vocabulary split

**Traces:** prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/FR-4 · prd:skill-command-prefix/FR-5 · prd:skill-command-prefix/UB-2 · prd:skill-command-prefix/UB-3 · prd:skill-command-prefix/NR-1 · prd:skill-command-prefix/NFR-1 · design:skill-command-prefix-v1#3.4 · design:skill-command-prefix-v1#DINV-4 · design:skill-command-prefix-v1#DINV-5

## Coverage

| Requirement | Tasks |
|---|---|
| prd:skill-command-prefix/FR-1 | T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15, T16, T17 |
| prd:skill-command-prefix/FR-2 | T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15, T16, T17 |
| prd:skill-command-prefix/FR-3 | T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15, T16, T17, T18 |
| prd:skill-command-prefix/FR-4 | T18 |
| prd:skill-command-prefix/FR-5 | T18 |
| prd:skill-command-prefix/UB-1 | T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15, T16, T17 |
| prd:skill-command-prefix/UB-2 | T18 |
| prd:skill-command-prefix/UB-3 | T18 |
| prd:skill-command-prefix/NR-1 | T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15, T16, T17, T18 |
| prd:skill-command-prefix/NFR-1 | T18 |
