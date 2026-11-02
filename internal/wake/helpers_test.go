package wake

import (
	"context"
	"errors"
	"strings"
)

// testProblemsRoot is the default problems root the wake tests key their
// fixture delivery-ledger paths (deliveryDoc) and their Push/Manual/dispatch
// calls on — the relocation case is TestPush_RelocatedProblemsRoot.
const testProblemsRoot = "docs/problems"

// stubLedgers is a fixed set of delivery docs, or a list error.
type stubLedgers struct {
	docs []Doc
	err  error
}

func (s stubLedgers) List(context.Context) ([]Doc, error) { return s.docs, s.err }

// stubProber answers each exact read from a fixed map; a non-nil err faults
// every probe (the infrastructure-fault path).
type stubProber struct {
	ancestor map[string]bool
	exists   map[string]bool
	err      error
}

func (p stubProber) AncestorOfMainline(_ context.Context, ref string) (bool, error) {
	if p.err != nil {
		return false, p.err
	}
	return p.ancestor[ref], nil
}

func (p stubProber) RecordExists(_ context.Context, ref string) (bool, error) {
	if p.err != nil {
		return false, p.err
	}
	return p.exists[ref], nil
}

// recordDoor records every front-door touch it receives; failSlugs makes the
// touch fail for the named slugs, so a per-touch failure can be exercised
// without failing the whole sweep.
type recordDoor struct {
	touches   []Signal
	failSlugs map[string]bool
}

func (d *recordDoor) Touch(_ context.Context, s Signal) error {
	if d.failSlugs[s.Slug] {
		return errors.New("front door unreachable")
	}
	d.touches = append(d.touches, s)
	return nil
}

// deliveryDoc builds a valid delivery-ledger/v1 doc for slug with the given
// journal body (dense `### E-n` entries). The derived block is present and
// minimal — the tick reads the journal, never the projection (DINV-6).
func deliveryDoc(slug, journal string) Doc {
	var b strings.Builder
	b.WriteString("# " + slug + " — delivery ledger\n\n")
	b.WriteString("| Field | Value |\n|---|---|\n")
	b.WriteString("| **Slug** | " + slug + " |\n")
	b.WriteString("| **Template version** | delivery-ledger/v1 |\n\n")
	b.WriteString("## Derived state\n\n")
	b.WriteString("| Field | Value |\n|---|---|\n")
	b.WriteString("| **Status** | open |\n")
	b.WriteString("| **Open items** | none |\n\n")
	b.WriteString("## Journal\n\n")
	b.WriteString(strings.TrimSpace(journal))
	b.WriteString("\n")
	return Doc{Path: "docs/problems/" + slug + "/delivery.md", Bytes: []byte(b.String())}
}

// stageEntered is a plain journal entry that registers no await — the neutral
// filler for a ledger the tick should find idle.
const stageEntered = "### E-1 — stage-entered — began\n\n- **Date**: 2026-07-10\n- **Body**: entered the stage\n\n---\n"

// awaitEntry builds a stop entry registering one await on goalObject.
func awaitEntry(anchor, goalObject, condition, since string) string {
	return "### " + anchor + " — proposal-disposed — stop\n\n" +
		"- **Date**: " + since + "\n" +
		"- **Body**: the drive stops awaiting a completion\n" +
		"- **Await**: " + goalObject + " · " + condition + " · since " + since + "\n\n---\n"
}
