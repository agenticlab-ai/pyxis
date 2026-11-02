# Slice-scoped plan coverage — solution shape

| Field | Value |
|---|---|
| **Design** | plangate-slice-coverage |
| **Status** | authoritative |

*Concrete forms for the design's decisions — authoritative until the code exists, then `superseded-by-code`. Names shown are the intended shape; final spellings fix at implementation. Each element is captioned with the `DINV-n`/`A-n` it realizes.*

## Per-plan coverage — the plan's own matrix slice

`internal/plangate/plan/gates.go` — the coverage gate keeps its internal-consistency + fog checks but **drops the "every live feature Must" loop**; the plan's matrix *is* its slice, fixed at freeze (**DINV-1**, **DINV-4**, **A-1**, **A-2**):

```go
// coverageGate validates a plan's matrix against ITS OWN CLAIMS — the slice —
// never against the whole feature (DINV-1). Internal consistency + fog: kept.
func coverageGate(g governing, p *Plan) []schema.Defect {
    var defs []schema.Defect
    covered := map[string]bool{}
    for _, row := range p.Coverage {            // each claimed requirement…
        if !g.live[row.Req] {                   // …must be a LIVE unit (superseded/retired → defect)
            defs = append(defs, def(p, "coverage", "claims a non-live requirement "+row.Req, "…"))
        }
        if len(row.Tasks) == 0 {                // …and map to ≥1 task
            defs = append(defs, def(p, "coverage", row.Req+" maps to no task", "…"))
        }
        covered[row.Req] = true
    }
    for _, t := range p.Tasks {                 // every task → ≥1 requirement/anchor (fog, FR-16)
        if !taskIsCovered(t, p.Coverage) {
            defs = append(defs, def(p, "coverage", "task "+t.ID+" maps to no requirement or design anchor", "…"))
        }
    }
    // Should/Could covered-or-Deferred: unchanged.
    // REMOVED: the loop over g.must demanding every feature Must appear here.
    return defs
}
```

## Feature-level completeness — the union across plans

A separate feature-scoped check unions every plan's matrix and reports a live Must claimed by no plan — **never a per-plan fold block** (**DINV-2**, **DINV-3**, **A-3**, **A-4**):

```go
// FeatureCoverage unions the coverage matrices of every plan under a feature
// and reports each live Must FR/UB that NO plan claims (DINV-2). It is
// feature-scoped and independent of any single plan's fold (DINV-3) — invoked
// as a `coverage <feature>` verb / fold-mode / project guard (design Q1).
func FeatureCoverage(root string, cfg config.Config, feature string) []schema.Defect {
    g := loadGoverning(root, cfg, feature)      // the feature's PRD live-Must set
    claimed := map[string]bool{}
    for _, rel := range planFiles(root, cfg, feature) {
        for _, row := range foldMatrix(root, cfg, rel) {
            claimed[row.Req] = true             // the UNION of all plans' slices
        }
    }
    var defs []schema.Defect
    for req := range g.must {                   // every live Must FR/UB…
        if !claimed[req] {                      // …covered by SOME plan, or a feature-level defect
            defs = append(defs, featureDef(feature, "coverage", "Must "+req+" is covered by no plan", "add a plan whose matrix covers it"))
        }
    }
    return defs
}
```

## Why the separation matters

A single plan's `fold` calls **only** `coverageGate` (its slice) — it never calls `FeatureCoverage` — so a slice-plan or a frozen base plan folds green even while the feature has an unplanned Must (**DINV-3**, **A-4**). Completeness is verified where it lives — over the feature's union — and where it cannot block the very plans that close the gaps:

```
plangate fold docs/plans/<feature>        # per-plan slice only (DINV-1) — green for slice/frozen plans
plangate coverage docs/plans/<feature>    # feature union (DINV-2) — names any Must covered by no plan
```
