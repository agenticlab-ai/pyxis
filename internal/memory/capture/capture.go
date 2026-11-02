// Package capture distinguishes known generated code bookkeeping from durable
// knowledge. It does not attempt to judge arbitrary prose or infer intent from
// code. Authors and reviewers decide whether a new fact adds future value.
package capture

import (
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// Derived recognizes the exact legacy capture classes whose information can
// be reconstructed from source. Historical records remain valid and addressable;
// this classification controls new admission and ordinary retrieval only.
func Derived(r *record.Record) bool {
	if r == nil {
		return false
	}
	if r.Kind == record.Need && r.Family == "anchor" && r.HasTag("reconcile:anchor") {
		return true
	}
	if r.Kind != record.Learning || !r.HasTag("reconcile:code-semantics") ||
		!strings.HasPrefix(r.Intent, "Preserve the implemented responsibility of ") {
		return false
	}
	return r.Rationale == "Source-code audit found production code without a dedicated semantic memory record." ||
		r.Rationale == "Source-code audit found production code without a dedicated semantic memory record covering the current package bytes after the Forgejo projection move."
}

// CheckNew rejects known automatic inventory/summary records without changing
// immutable-record validation. Inherited history must continue to load, resolve,
// and verify with its original evidence.
func CheckNew(r *record.Record) error {
	if Derived(r) {
		return fmt.Errorf("derived-memory: %s records code inventory or a generated implementation summary; derive that information from source and record only a new obligation, reason, constraint, ruling, or reusable lesson", r.ID)
	}
	return nil
}
