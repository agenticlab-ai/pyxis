# demo — a ledger with no folded projection (fixture)

This ledger directory exists but was never folded, so no
`effective.source.json` machine face is present. Contract resolution must
fail closed with `configuration-error` naming the missing input — never
`clean` on a missing machine face.
