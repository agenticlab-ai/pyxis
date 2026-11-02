package reconcile

import (
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// requirementsFirst is an explicit opt-in on the original immutable base,
// never inferred from a free-form approval status. Amendments and rebase
// checkpoints cannot opt a legacy contract into deferred design coverage.
// Unmarked ledgers keep their original behavior, including accepted debt.
func requirementsFirst(base, effective []byte) (bool, error) {
	values := make([]string, 0, 2)
	for _, body := range [][]byte{base, effective} {
		doc, defects := ledger.ParseProjection("PRD design coverage", body)
		if len(defects) != 0 {
			return false, fmt.Errorf("parse header: %s", defects[0].String())
		}
		value, present := doc.Header["Design coverage"]
		if present && value != "requirements-first/v1" {
			return false, fmt.Errorf("unknown Design coverage %q; omit the field or use requirements-first/v1", value)
		}
		values = append(values, value)
	}
	if values[0] != values[1] {
		return false, fmt.Errorf("Design coverage must remain equal to the original 000-base.md header")
	}
	return values[0] == "requirements-first/v1", nil
}

// designStartedFeatures is a one-way boundary. Even a superseded indexed
// design starts coverage for the whole feature, not just its first served
// row. Immutable Decision history retains that boundary after source
// retirement or revocation; only live, authorized edges satisfy coverage.
func designStartedFeatures(designs []Design, set *store.Set) map[string]bool {
	started := map[string]bool{}
	for _, design := range designs {
		for _, ref := range design.Serves {
			if subject, _, ok := memoryPRDRef(ref); ok {
				started[strings.TrimPrefix(subject, "prd/")] = true
			}
		}
	}
	for _, id := range set.IDs() {
		decision := set.Get(id)
		if decision.Kind != record.Decision {
			continue
		}
		for _, id := range decision.Rel(record.Serves) {
			if target := set.Get(id); target != nil && record.Namespace(target.Subject) == "prd" {
				started[strings.TrimPrefix(target.Subject, "prd/")] = true
			}
		}
	}
	return started
}
