package boundary

import (
	"bytes"
	"fmt"
	"path"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// ValidateDeliveryChange prevents unsupported types from being introduced,
// even if a later same-proposal entry attempts to correct them. Historical
// unsupported entries may be corrected only by retaining their exact bytes.
// Neither this validation nor the candidate confers any scope: the resolver
// still reads exclusively the trusted committed ledger.
func ValidateDeliveryChange(verb string, cfg config.Config, slug string, base, candidate []byte) []schema.Defect {
	file := path.Join(cleanRoot(cfg.Problems), slug, "delivery.md")
	bad := func(reason string) []schema.Defect {
		return []schema.Defect{{Verb: verb, File: file, Op: "journal-type", Reason: reason,
			Remedy: "preserve historical events; use canonical types for new entries and docgate correct-journal for historical type errors"}}
	}
	l, defects := argusledger.ParseAs(verb, file, schema.LedgerClasses["delivery-ledger"], candidate)
	if len(defects) > 0 {
		return defects
	}
	var original *argusledger.Ledger
	if len(base) > 0 {
		original, defects = argusledger.ParseAs(verb, file, schema.LedgerClasses["delivery-ledger"], base)
		if len(defects) > 0 {
			return defects
		}
		if len(l.Order) < len(original.Order) {
			return bad("merged journal entries were removed")
		}
		for i, id := range original.Order {
			if l.Order[i] != id || !bytes.Equal(original.RawEntries[id], l.RawEntries[id]) {
				return bad(fmt.Sprintf("merged %s must retain its original order and exact bytes", id))
			}
		}
	}
	for _, id := range l.Order {
		if !argusledger.KnownEntryType(argusledger.EntryType(l.Entries[id])) &&
			(original == nil || len(original.RawEntries[id]) == 0) {
			return bad(fmt.Sprintf("new %s introduces unsupported event type %q", id, argusledger.EntryType(l.Entries[id])))
		}
	}
	if _, defects = argusledger.EffectiveEntryTypes(verb, l); len(defects) > 0 {
		return defects
	}
	if !validDeliveryScopeShape(slug, candidate, l) {
		return bad("delivery ledger must satisfy its complete header, derived-state and typed journal shape; a type correction cannot repair missing historical fields or terminal-event obligations")
	}
	return nil
}
