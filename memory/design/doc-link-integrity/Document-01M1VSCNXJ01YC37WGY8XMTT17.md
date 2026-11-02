# Doc link integrity — dead-link checking and system-map registry stability

| Field | Value |
|---|---|
| **Status** | Approved (2026-07-08 — operator, via the introducing PR review) *(merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-08 |
| **Relevant-when** | adding or changing `check-links`; touching the system-map registry check; triaging a dangling-link or removed-anchor defect; deciding whether a new citation surface needs stability protection |
| **Scope** | Two integrity checks over surfaces the ledger machinery does not govern: relative-link resolution across the documentation corpus (FR-21), and merge-base stability of the `INV-<n>` / `D<n>` registry anchors inside the living system maps (FR-22). **Not**: `Serves`/`corpus:` resolution (serves-hardening), abstract-layer purity (docgate-fence-completion), anchor-fragment resolution (deferred, Q1). |
| **Tracking** | issue #54 · PR #56 (requirements) |
| **Builds-on** | docs/tech-design/docgate-v1.md · docs/tech-design/docgate-v2.md |
| **Supersedes** | — |
| **Serves** | prd:docgate/FR-21 · prd:docgate/FR-22 |
| **Security/privacy impact** | none — hermetic reads of tracked files and git history; no new surface |
| **Template version** | tech-design-template/v1 |

## 1. Problem

Two citation surfaces have no mechanical integrity (#54 A5/C3). Relative
links: the corpus is clean today by discipline alone — nothing checks, and
targets do move (the footprint migration left four frozen documents
pointing at a templates directory that no longer exists; only their
immutability keeps the damage bounded). Registry anchors: the system maps
are deliberately mutable living references, yet they carry the invariant
registry (`INV-<n>` — cited by designs, the graduation target for DINVs)
and the open-decision queue (`D<n>`); an edit can renumber or delete a
cited anchor and nothing objects, while even the non-authoritative wiki
has a metadata lint.

## 2. Decision

**Decision:** Two checks, one bytes-only and one history-backed.
**`check-links`** scans every `.md` under the docs tree, the root
operating manual, and the configured templates directory; every relative
link target (not a URL scheme, not a pure `#fragment`) must resolve to an
existing file, with fenced code blocks skipped. **The map-registry check**
runs with a merge base over the configured system-map documents: the set
of `INV-<n>` and `D<n>` anchors present at the merge base must be a subset
of the set at HEAD — anchors may be added, never removed or renumbered.
Both are pattern-level checks (AP-10's smallest honest form): they prove
resolution and persistence, never meaning.

## 3. The design

### 3.1 Mechanics

- **Link extraction** — inline `](target)` and reference-style
  `[label]: target` links, outside fenced blocks; targets split at `#`
  (the file part resolves relative to the source file's directory; the
  fragment is ignored in v1 — Q1). Scheme-bearing targets (`http:`,
  `https:`, `mailto:`) and pure fragments are skipped. Resolution is
  `os.Stat` of the joined path — files only, no content read.
- **Registry anchor extraction** — over each configured map document at
  merge base and at HEAD: collect `INV-[0-9]+` and `D[0-9]+` tokens that
  begin a table row's first cell (`| INV-33 |`, `| D7 |`) — row-anchored,
  so prose mentions of an INV elsewhere never count. Base-set minus
  HEAD-set non-empty → one defect per missing anchor, naming it and the
  document. A map file absent at HEAD but present at base is reported as
  the sum of its anchors (deletion is maximal removal); a map absent at
  both is skipped (FR-22's letter — `system-design.md` merges later).
- **Homes** — `check-links` is merge-base-free (joins the pre-commit set);
  the registry check needs the merge base (joins pre-push/CI beside
  `check-immutable`). Both live in `internal/docgate/gate`.

### 3.2 Boundaries and non-goals

- **Not solved here** — fragment/anchor resolution inside targets (Q1);
  link *direction* rules (fence-completion's purity scan owns the
  governing docs); wiki content freshness; INV/D *content* stability
  (rows may be edited in place — the maps are living; only the anchors
  are permanent, matching "never renumber — update the numbered content").
- **Owned by a future design** — a general anchors registry if more
  ID-bearing living documents appear; graduation tooling that *moves*
  a DINV into the INV registry (it must append, and this check will
  hold it to that).
- **Implementation must not infer** — that external URLs should be
  fetched (hermetic: NFR-2 stands; a dead external link is review's
  problem); or that the checks may reformat anything (report-only, like
  every gate verb — docgate-v1#DINV-3).

### 3.3 Invariants

- **DINV-1** — `check-links` performs no network IO and no content
  writes: extraction and `Stat` only.
- **DINV-2** — the registry check compares anchor *sets*, never row
  contents: living maps stay freely editable in everything but their
  permanent IDs.
- **DINV-3** — both checks are report-only; remedies name the fix
  (restore the anchor / fix or remove the link), never perform it.

### 3.4 Observability

Unchanged from docgate-v1 §3.4; new catalog entries: dangling link
(source file, line, target), removed registry anchor (document, anchor).
The dangling-link defect includes the resolved absolute-relative form the
checker attempted, so path-vs-cwd confusion is diagnosable from the
message alone.

### 3.5 Security & privacy

none — matches the header row: reads tracked files and git history
already available to every verb; no network (DINV-1), no new data.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| a third-party markdown parser for link extraction | closed, tiny grammar needed; stdlib regex + fence tracking suffices; zero new supply chain (repo Go rules) |
| checking external URLs with HTTP probes | breaks hermeticity (NFR-2, docgate-v1 A-6); availability-of-the-internet becomes gate flake |
| full anchor-fragment resolution now | needs a heading-slug algorithm pinned across renderers; defer until a real dangling-fragment defect exists (Q1) |
| freezing the maps entirely (immutability fence) | the maps are the *living* reference class by design (wiki document-classes); freezing them recreates the drift the class exists to absorb — only the anchors are contract |
| hand-maintained link manifest | a generated-looking artifact maintained by discipline — the exact drift pattern this repo rejects |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | frozen documents with known-stale links (the four `docs/templates/` citations) red the corpus at introduction | unfixable defects on immutable files | the checker's defect for a frozen file names the wiki-crosswalk remedy; introduction ships with either an allowlist of the four known-frozen targets **or** compatibility pointers at the old paths — decided at implementation against the real corpus run (Q2) | corpus run in the introducing PR |
| R2 | fence tracking misses an exotic markdown construct and a code-sample link reds | author friction | fences + inline backticks are the only skip classes the corpus uses; a false positive is a fixture the same day | corpus green at introduction |
| R3 | row-anchored INV/D extraction misses a future table reformat | silent loss of protection | the maps' registry tables are stable four-column shapes; a reformat that breaks extraction also breaks readers — caught at review; extraction is fixture-pinned | golden fixtures over the real maps |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | dangling relative link red naming source file, line, and target; fenced/external/fragment links ignored | Test | fixtures | prd:docgate/FR-21 |
| A-2 | the real corpus's verdict at introduction is recorded: green, or each red named and resolved (allowlist or pointer) in the same PR | Test | corpus run in the introducing PR | prd:docgate/FR-21 · R1 |
| A-3 | removing or renumbering an `INV-<n>`/`D<n>` row red naming the anchor; appending green; prose edits green; absent map file skipped | Test | temp-repo fixtures | prd:docgate/FR-22 |
| A-4 | no network syscalls in either check | Test | the suite's existing network guard | DINV-1 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Fragment resolution: which heading-slug algorithm, and is it worth pinning before a real defect class exists? | implementation | nothing | open | — |
| Q2 | The four known-stale frozen links: allowlist entries or committed compatibility pointers at the old paths? | implementation | A-2 | open | decided at the introducing PR against the real corpus run |

## Principles

- **AP-10 — honored.** "Links happen to be clean" becomes "links are checked"; "please don't renumber INV rows" becomes a diff check.
- **AP-14 — honored.** Acceptance is the real corpus run, not the fixture set alone — the repo is fixture zero again.
- **AP-17 — honored.** Unresolvable is red; there is no best-effort pass. The frozen-link bootstrap case is named and resolved at introduction, not waved through.
- **AP-19 — honored.** Byte scans and set comparisons; zero tokens, sub-second.
- **AP-23 — honored.** Registry anchors are append-only by check, which is what keeps parallel map editors collision-safe on the IDs.
- **Deviation:** none.
