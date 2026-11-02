# demo — a dangling-citation fixture PRD ledger base

A minimal ledger base whose body carries one typed citation that does not
resolve: it points at design:ghost-design, a design slug that does not exist in
this fixture (only demo-v1 does). The link check reuses the citation resolver
scoped to the feature's own documents and reds `unresolved`, naming the offender
and its source location.

The live serving design demo-v1 serves prd:demo/FR-1, which resolves — so only
the dangling reference is reported, not the sound one.
