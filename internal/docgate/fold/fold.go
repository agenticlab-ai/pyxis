// Package fold is the deterministic replay of a PRD ledger into its
// projection. It is PURE (DINV-1): ordered entry bytes in, artifact bytes
// out — no filesystem, network, clock, or environment. Identical inputs
// yield identical outputs on every platform; verify-generated is therefore
// recompute-and-compare by construction (design §3.6).
package fold

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// UnitState tracks one unit's lineage for the source map, plus the
// per-requirement fields the contract gate consumes.
type UnitState struct {
	Status       string  `json:"status"` // live | superseded | retired | rescoped
	IntroducedBy string  `json:"introduced_by"`
	Supersedes   *string `json:"supersedes"`
	SupersededBy *string `json:"superseded_by"`
	SourceLoc    string  `json:"source_location"`
	CurrentLoc   string  `json:"current_location"`
	// Per-requirement fields projected from the row cells by header-column
	// name (design contract-gate-v1 §3.6): additive, omitted when the row's
	// table carries no such column, so existing consumers are unaffected.
	Priority        string `json:"priority,omitempty"`
	HasFitCriterion bool   `json:"has_fit_criterion,omitempty"`
	VerifyMethod    string `json:"verify_method,omitempty"`
}

// ResolvedUnit is one unit after the whole ledger has been replayed. It is an
// in-memory compiler face for consumers that need the authored row as well as
// its computed liveness. It is deliberately not emitted into either generated
// artifact: effective.md and effective.source.json remain byte-for-byte the
// public fold outputs.
type ResolvedUnit struct {
	ID           string
	Prefix       string
	Status       string
	IntroducedBy string
	Supersedes   *string
	SupersededBy *string
	Columns      []string // canonical semantic names for this prefix
	// AuthoredColumns preserves the exact table headers from the effective
	// projection. Legacy ledgers encode actors and audiences in those headers;
	// consumers include them in source identity even while using Columns for
	// version-pinned positional interpretation.
	AuthoredColumns []string
	Cells           []string
}

// enrichFields reads the per-requirement fields the contract gate consumes —
// priority, fit-criterion presence, and declared verification method — from a
// unit's row cells, keyed by the kind's header-column names (design
// contract-gate-v1 §3.6, #DINV-7). A column the table does not carry yields the
// zero value; the fold stays the single parser (no separate reader re-parses
// effective.md).
func enrichFields(u *ledger.Unit) (priority, verify string, hasFit bool) {
	cols := schema.TableSchemas[u.Prefix]
	cell := func(name string) string {
		for i, c := range cols {
			if c == name && i < len(u.Cells) {
				return strings.TrimSpace(u.Cells[i])
			}
		}
		return ""
	}
	return cell("Priority"), cell("Verify"), cell("Fit criterion") != ""
}

// AmendMeta is per-amendment computed data (diff, blast radius).
type AmendMeta struct {
	Name         string
	ComputedTier string
	DeclaredTier string
	Affected     []string
}

// Result is a fold's full output.
type Result struct {
	EffectiveMD []byte
	SourceJSON  []byte
	Fingerprint string
	AsOf        string
	Amendments  []AmendMeta
	Resolved    []ResolvedUnit
	Defects     []schema.Defect
}

type removed struct {
	id, by, note, verbatim string
}

// options carries a fold's caller-selected relaxations. The zero value is the
// strict fold every gate path uses; only an explicit Option departs from it.
type options struct {
	allowUnapproved bool
}

// Option configures a fold. Options exist so one implementation serves both
// the persisting fold and a compute-only preview — a second fold would risk
// reviewing a projection that differs from the one that merges.
type Option func(*options)

// AllowUnapproved relaxes the envelope's `Status: Approved` requirement for
// THIS fold only.
//
// Approval gates persistence and merge, not computation: FR-5 binds the check
// to `validate-amendments` "at merge-target". Applying it while merely
// computing a projection makes an amendment unreviewable until it is approved,
// which inverts the authoring flow — the reviewer reads the folded effect, and
// the operator approves after reading the review.
//
// A caller passing this option must not persist the result: the projection on
// disk is what merges, and it stays behind the approval bar. Every other
// envelope, op, and blast-radius defect is still reported — a preview that
// hid real defects would be worse than no preview.
func AllowUnapproved() Option { return func(o *options) { o.allowUnapproved = true } }

// Fold replays normalized-or-raw entries (it normalizes internally) in the
// given order. Entries must be the ledger files only, sorted by name.
func Fold(entries []ledger.Entry, opts ...Option) Result {
	var o options
	for _, apply := range opts {
		apply(&o)
	}
	var res Result
	if len(entries) == 0 {
		res.Defects = append(res.Defects, schema.Defect{Verb: "fold", File: "(ledger)", Op: "replay",
			Reason: "empty ledger", Remedy: "a ledger starts at 000-base.md"})
		return res
	}
	norm := make([][]byte, len(entries))
	for i, e := range entries {
		norm[i] = schema.Normalize(e.Bytes)
	}
	// Replay starts at the latest rebase checkpoint.
	start := 0
	for i, e := range entries {
		if ledger.IsRebase(e.Name) {
			start = i
		}
	}
	replayed := norm[start:]
	res.Fingerprint = schema.Fingerprint(replayed)

	doc, defects := ledger.ParseBase(entries[start].Name, replayed[0])
	res.Defects = append(res.Defects, defects...)
	if doc == nil {
		return res
	}
	baseName := entries[start].Name
	res.AsOf, _ = ledger.Seq(entries[len(entries)-1].Name)

	states := map[string]*UnitState{}
	for _, id := range doc.UnitOrder {
		u := doc.Units[id]
		priority, verify, hasFit := enrichFields(u)
		states[id] = &UnitState{Status: "live", IntroducedBy: baseName,
			SourceLoc:       baseName + "#" + u.Section,
			CurrentLoc:      "effective.md#" + id,
			Priority:        priority,
			VerifyMethod:    verify,
			HasFitCriterion: hasFit}
	}
	history, hdef := newResolvedHistory(doc, baseName)
	res.Defects = append(res.Defects, hdef...)
	var removals []removed

	for i := start + 1; i < len(entries); i++ {
		a, adef := ledger.ParseAmendment(entries[i].Name, norm[i])
		res.Defects = append(res.Defects, adef...)
		if a == nil {
			continue
		}
		meta := applyAmendment(doc, a, states, &removals, &res.Defects)
		res.Amendments = append(res.Amendments, meta)
		validateEnvelope(a, meta, o, &res.Defects)
		res.Defects = append(res.Defects, history.observe(doc, a.Name)...)
	}
	resolved, rdef := resolvedUnits(doc, states, history)
	res.Defects = append(res.Defects, rdef...)
	res.Resolved = resolved

	res.EffectiveMD = emitEffective(doc, res.AsOf, res.Fingerprint, removals)
	res.SourceJSON = emitSource(states, res.AsOf, res.Fingerprint)
	return res
}

type resolvedRow struct {
	ID              string
	Prefix          string
	IntroducedBy    string
	Columns         []string
	AuthoredColumns []string
	Cells           []string
	Present         bool
}

type resolvedHistory struct {
	Rows    map[string]resolvedRow
	Current *ledger.Doc
	Invalid bool
}

func newResolvedHistory(doc *ledger.Doc, introducedBy string) (*resolvedHistory, []schema.Defect) {
	h := &resolvedHistory{Rows: map[string]resolvedRow{}}
	return h, h.observe(doc, introducedBy)
}

// observe reparses one complete replay boundary. This is a separate compiler
// face only: doc and the source-map states remain untouched, preserving the two
// generated artifacts byte-for-byte. Observing each amendment boundary retains
// rows introduced and later removed by section replacement and makes reuse of
// one of those IDs decidable.
func (h *resolvedHistory) observe(doc *ledger.Doc, introducedBy string) []schema.Defect {
	current, defects := parseCurrentDocument(doc)
	if current == nil || len(defects) > 0 {
		h.Invalid = true
		h.Current = nil
		return defects
	}
	now := make(map[string]bool, len(current.Units))
	for _, id := range current.UnitOrder {
		unit := current.Units[id]
		now[id] = true
		row, known := h.Rows[id]
		if known && !row.Present {
			defects = append(defects, def("fold", introducedBy, "resolve-units",
				fmt.Sprintf("non-live unit %s is reintroduced by the effective projection", id),
				"an ID removed by a section replacement is never reused; introduce a fresh ID"))
		}
		if !known {
			row.IntroducedBy = introducedBy
		}
		row.ID = unit.ID
		row.Prefix = unit.Prefix
		row.Columns = unitColumns(current, unit)
		row.AuthoredColumns = authoredUnitColumns(current, unit)
		row.Cells = append([]string(nil), unit.Cells...)
		row.Present = true
		h.Rows[id] = row
	}
	for id, row := range h.Rows {
		if row.Present && !now[id] {
			row.Present = false
			h.Rows[id] = row
		}
	}
	h.Current = current
	return defects
}

func parseCurrentDocument(doc *ledger.Doc) (*ledger.Doc, []schema.Defect) {
	var replayed strings.Builder
	replayed.WriteString("# ")
	replayed.WriteString(doc.Title)
	replayed.WriteByte('\n')
	replayed.WriteString(strings.Join(doc.Lines, "\n"))
	replayed.WriteByte('\n')
	return ledger.ParseProjection("effective.md", schema.Normalize([]byte(replayed.String())))
}

func resolvedUnits(doc *ledger.Doc, states map[string]*UnitState, history *resolvedHistory) ([]ResolvedUnit, []schema.Defect) {
	if history == nil || history.Invalid || history.Current == nil {
		return nil, nil
	}
	var defects []schema.Defect
	ids := make([]string, 0, len(states)+len(history.Rows))
	seen := make(map[string]bool, len(states)+len(history.Rows))
	for id := range states {
		// ParseBase intentionally preserves legacy inference for the emitted
		// source map, including old §0-shaped inputs. Resolved is the current
		// compiler face: a state is addressable only if the projection tracker
		// observed it outside the complete human-only §0 subtree at some replay
		// boundary.
		if _, addressable := history.Rows[id]; !addressable {
			continue
		}
		ids = append(ids, id)
		seen[id] = true
	}
	for id := range history.Rows {
		if !seen[id] {
			ids = append(ids, id)
		}
	}
	schema.SortIDs(ids)
	out := make([]ResolvedUnit, 0, len(ids))
	for _, id := range ids {
		state := states[id]
		row, known := history.Rows[id]
		if state == nil {
			if !known {
				continue
			}
			status := "retired"
			if row.Present {
				status = "live"
			}
			out = append(out, resolvedUnit(row, status, nil))
			continue
		}
		if known && row.Present && state.Status != "live" {
			defects = append(defects, def("fold", "effective.md", "resolve-units",
				fmt.Sprintf("non-live unit %s is reintroduced by the effective projection", id),
				"a retired, superseded, or rescoped ID is never reused; introduce a fresh ID"))
		}
		if known && row.Present && state.Status == "live" {
			out = append(out, resolvedUnit(row, "live", state))
			continue
		}
		if !known {
			historical := doc.Units[id]
			if historical != nil {
				row = rowFromUnit(doc, historical, state.IntroducedBy)
				known = true
			}
		}
		if !known {
			defects = append(defects, def("fold", state.IntroducedBy, "resolve-units",
				fmt.Sprintf("unit %s has source-map state but no historical row", id),
				"restore the row at its recorded introduction or rebuild the rebase checkpoint"))
			continue
		}
		status := state.Status
		if status == "live" {
			// A section replacement may remove a row without a row-level retire
			// operation. It is not current merely because the legacy source-map
			// state still says live.
			status = "retired"
		}
		out = append(out, resolvedUnit(row, status, state))
	}
	return out, defects
}

func rowFromUnit(doc *ledger.Doc, unit *ledger.Unit, introducedBy string) resolvedRow {
	return resolvedRow{
		ID: unit.ID, Prefix: unit.Prefix, IntroducedBy: introducedBy,
		Columns: unitColumns(doc, unit), AuthoredColumns: authoredUnitColumns(doc, unit), Cells: append([]string(nil), unit.Cells...),
	}
}

func authoredUnitColumns(doc *ledger.Doc, unit *ledger.Unit) []string {
	if unit.Table >= 0 && unit.Table < len(doc.Tables) {
		return append([]string(nil), doc.Tables[unit.Table].Cols...)
	}
	return nil
}

func unitColumns(doc *ledger.Doc, unit *ledger.Unit) []string {
	// Resolved is a compiler face: once the parser has safely established the
	// row kind, expose the version-pinned semantic column names. Frozen merged
	// documents carry cosmetic legacy headers (for example "If…" vs
	// "If… (trigger)"); their cell positions retain the registered meaning.
	if cols, ok := schema.TableSchemas[unit.Prefix]; ok {
		return append([]string(nil), cols...)
	}
	if unit.Table >= 0 && unit.Table < len(doc.Tables) {
		return append([]string(nil), doc.Tables[unit.Table].Cols...)
	}
	return nil
}

func resolvedUnit(row resolvedRow, status string, state *UnitState) ResolvedUnit {
	out := ResolvedUnit{
		ID:              row.ID,
		Prefix:          row.Prefix,
		Status:          status,
		Columns:         append([]string(nil), row.Columns...),
		AuthoredColumns: append([]string(nil), row.AuthoredColumns...),
		Cells:           append([]string(nil), row.Cells...),
	}
	if state != nil {
		out.IntroducedBy = state.IntroducedBy
		out.Supersedes = cloneString(state.Supersedes)
		out.SupersededBy = cloneString(state.SupersededBy)
	} else {
		out.IntroducedBy = row.IntroducedBy
	}
	return out
}

func cloneString(in *string) *string {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

func def(verb, file, op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: verb, File: file, Op: op, Reason: reason, Remedy: remedy}
}

func applyAmendment(doc *ledger.Doc, a *ledger.Amendment, states map[string]*UnitState, removals *[]removed, defects *[]schema.Defect) AmendMeta {
	meta := AmendMeta{Name: a.Name, ComputedTier: "additive"}
	raise := func(t string) {
		if schema.TierRank[t] > schema.TierRank[meta.ComputedTier] {
			meta.ComputedTier = t
		}
	}
	opName := func(op ledger.Op) string { return "op: " + op.Verb + " " + op.Arg }

	for _, op := range a.Ops {
		switch {
		case op.Verb == schema.OpSupersede && strings.HasPrefix(op.Arg, "§"):
			anchor := strings.TrimPrefix(op.Arg, "§")
			anchor = strings.Fields(anchor)[0]
			// An empty replacement body is never legal: a section supersede's
			// payload is the full replacement BODY (heading excluded — a heading
			// inside the payload self-truncates the op block at the parser's
			// block boundary). Folding an empty body would hollow the contract
			// section while every byte-compare check stays green — the exact
			// best-effort projection the model forbids (AP-17; #523).
			if len(op.Body) == 0 {
				*defects = append(*defects, def("fold", a.Name, opName(op),
					fmt.Sprintf("the §%s supersede carries an empty replacement body", anchor),
					"the payload is the full replacement section BODY, heading excluded — a '### <n> <title>' line inside the payload truncates the op block (#523)"))
				continue
			}
			if !replaceSection(doc, anchor, op.Body) {
				*defects = append(*defects, def("fold", a.Name, opName(op),
					fmt.Sprintf("section §%s not found", anchor), "target an existing numbered section"))
				continue
			}
			meta.Affected = append(meta.Affected, "§"+anchor)
			raise("contract-changing") // section supersede is contract-changing by default

		case op.Verb == schema.OpSupersede, op.Verb == schema.OpRetire, op.Verb == schema.OpRescope:
			id, _, ok := schema.ParseID(op.Arg)
			if !ok {
				*defects = append(*defects, def("fold", a.Name, opName(op), "target is not a unit ID", "target an existing anchored unit"))
				continue
			}
			st, exists := states[id]
			if !exists {
				*defects = append(*defects, def("fold", a.Name, opName(op),
					fmt.Sprintf("dangling target: %s does not exist", id), "target an existing unit"))
				continue
			}
			if st.Status != "live" {
				*defects = append(*defects, def("fold", a.Name, opName(op),
					fmt.Sprintf("%s is already %s", id, st.Status), "a unit is superseded or retired at most once"))
				continue
			}
			u := doc.Units[id]
			raise(rowTier(doc, u))
			meta.Affected = append(meta.Affected, id)
			switch op.Verb {
			case schema.OpRetire:
				removeRow(doc, u, removals, a.Seq, "retired ("+a.Seq+")")
				st.Status = "retired"
				st.CurrentLoc = ""
			case schema.OpSupersede:
				newID := addPayloadRows(doc, u.Prefix, op, a, states, defects, " *("+a.Seq+" ← "+id+")*", u)
				if newID == "" {
					continue
				}
				removeRow(doc, u, removals, a.Seq, "superseded by "+newID+" ("+a.Seq+")")
				st.Status = "superseded"
				st.SupersededBy = &newID
				st.CurrentLoc = ""
				if ns := states[newID]; ns != nil {
					old := id
					ns.Supersedes = &old
				}
			case schema.OpRescope:
				if u.Prefix != "NG" && u.Prefix != "WR" {
					*defects = append(*defects, def("fold", a.Name, opName(op),
						"rescope targets NG-n or WR-n anchors only", "use supersede for requirement rows"))
					continue
				}
				raise("contract-changing") // a non-goal flip inward
				removeRow(doc, u, removals, a.Seq, "rescoped ("+a.Seq+")")
				st.Status = "rescoped"
				st.CurrentLoc = ""
				if len(op.PayloadRows) > 0 {
					pid, _, _ := schema.ParseID(op.PayloadRows[0][0])
					_, ppre, _ := schema.ParseID(pid)
					addPayloadRows(doc, ppre, op, a, states, defects, " *("+a.Seq+")*", nil)
				}
			}

		case op.Verb == schema.OpAdd:
			kind := strings.ToUpper(strings.TrimSpace(op.Arg))
			if _, ok := schema.TableSchemas[kind]; !ok {
				*defects = append(*defects, def("fold", a.Name, opName(op),
					fmt.Sprintf("unknown add target kind %q", op.Arg), "name the receiving table kind: FR, NR, UB, NFR, DOD, EVAL, EVT, Q"))
				continue
			}
			if id := addPayloadRows(doc, kind, op, a, states, defects, " *("+a.Seq+")*", nil); id != "" {
				meta.Affected = append(meta.Affected, id)
			}

		case op.Verb == schema.OpAnswer:
			id, prefix, ok := schema.ParseID(op.Arg)
			if !ok || prefix != "Q" {
				*defects = append(*defects, def("fold", a.Name, opName(op), "answer targets Q-n anchors", "target an open question"))
				continue
			}
			st, exists := states[id]
			if !exists || st.Status != "live" {
				*defects = append(*defects, def("fold", a.Name, opName(op),
					fmt.Sprintf("dangling target: %s", id), "target an existing open question"))
				continue
			}
			for _, f := range []string{"Answer", "Evidence", "Decided-by", "Effects"} {
				if _, ok := op.Fields[f]; !ok {
					*defects = append(*defects, def("fold", a.Name, opName(op),
						fmt.Sprintf("answer payload missing field %q", f), "the typed answer block is closed: Answer, Evidence, Decided-by, Effects"))
				}
			}
			u := doc.Units[id]
			u.Cells[len(u.Cells)-1] = "resolved (" + a.Seq + ") — " + op.Fields["Answer"]
			doc.Lines[u.Line] = "| " + strings.Join(u.Cells, " | ") + " |"
			meta.Affected = append(meta.Affected, id)
		}
	}
	schema.SortIDs(meta.Affected)
	return meta
}

// addPayloadRows validates a one-or-more-row payload against the kind's
// schema, appends the rows to the kind's table, and returns the first new ID.
func addPayloadRows(doc *ledger.Doc, kind string, op ledger.Op, a *ledger.Amendment, states map[string]*UnitState, defects *[]schema.Defect, lineage string, replace *ledger.Unit) string {
	want := schema.TableSchemas[kind]
	opn := "op: " + op.Verb + " " + op.Arg
	if len(op.PayloadRows) == 0 {
		*defects = append(*defects, def("fold", a.Name, opn, "operation carries no payload row", "payloads are complete and resolved"))
		return ""
	}
	if !colsEqual(op.PayloadCols, want) {
		*defects = append(*defects, def("fold", a.Name, opn,
			fmt.Sprintf("payload columns do not match the %s schema (want %d columns: %s)", kind, len(want), strings.Join(want, " | ")),
			"match prd-template/v1 column names and order exactly"))
		return ""
	}
	// Locate the receiving table: the replaced unit's table, or the first of kind.
	ti := -1
	if replace != nil {
		ti = replace.Table
	} else {
		for i, t := range doc.Tables {
			if t.Kind == kind {
				ti = i
				break
			}
		}
	}
	if ti < 0 {
		*defects = append(*defects, def("fold", a.Name, opn,
			fmt.Sprintf("the base has no %s table to receive the row", kind), "add the table via section supersede first"))
		return ""
	}
	first := ""
	for _, row := range op.PayloadRows {
		id, prefix, ok := schema.ParseID(row[0])
		if !ok || prefix != kind {
			*defects = append(*defects, def("fold", a.Name, opn, "payload row ID does not match the target kind", "fresh IDs use the receiving table's prefix"))
			continue
		}
		if _, dup := states[id]; dup {
			*defects = append(*defects, def("fold", a.Name, opn,
				fmt.Sprintf("ID reuse: %s already exists", id), "IDs are never reused; pick the next free number"))
			continue
		}
		newCells := append([]string{}, row...)
		newCells[0] = id + lineage
		line := "| " + strings.Join(newCells, " | ") + " |"
		at := insertAfter(doc, ti, replace)
		doc.Lines = append(doc.Lines[:at+1], append([]string{line}, doc.Lines[at+1:]...)...)
		shiftAfter(doc, at, 1)
		t := &doc.Tables[ti]
		t.RowLines = append(t.RowLines, at+1)
		u := &ledger.Unit{ID: id, Prefix: prefix, Line: at + 1, Cells: newCells, Table: ti, Section: sectionOf(doc, at+1)}
		doc.Units[id] = u
		doc.UnitOrder = append(doc.UnitOrder, id)
		priority, verify, hasFit := enrichFields(u)
		states[id] = &UnitState{Status: "live", IntroducedBy: a.Name,
			SourceLoc: a.Name + "#" + opn, CurrentLoc: "effective.md#" + id,
			Priority: priority, VerifyMethod: verify, HasFitCriterion: hasFit}
		if first == "" {
			first = id
		}
	}
	return first
}

func insertAfter(doc *ledger.Doc, ti int, replace *ledger.Unit) int {
	if replace != nil {
		return replace.Line
	}
	t := doc.Tables[ti]
	max := t.HeaderLine + 1
	for _, r := range t.RowLines {
		if r > max {
			max = r
		}
	}
	return max
}

func removeRow(doc *ledger.Doc, u *ledger.Unit, removals *[]removed, seq, note string) {
	verbatim := strings.Join(u.Cells, " | ")
	*removals = append(*removals, removed{id: u.ID, by: seq, note: note, verbatim: verbatim})
	doc.Lines = append(doc.Lines[:u.Line], doc.Lines[u.Line+1:]...)
	shiftAfter(doc, u.Line-1, -1)
}

// shiftAfter adjusts recorded line indexes after an insertion/removal at pos.
func shiftAfter(doc *ledger.Doc, pos, delta int) {
	for _, u := range doc.Units {
		if u.Line > pos {
			u.Line += delta
		}
	}
	for i := range doc.Tables {
		if doc.Tables[i].HeaderLine > pos {
			doc.Tables[i].HeaderLine += delta
		}
		for j, r := range doc.Tables[i].RowLines {
			if r > pos {
				doc.Tables[i].RowLines[j] += delta
			}
		}
	}
	for i := range doc.Sections {
		if doc.Sections[i].Start > pos {
			doc.Sections[i].Start += delta
		}
		if doc.Sections[i].End > pos {
			doc.Sections[i].End += delta
		}
	}
}

func sectionOf(doc *ledger.Doc, line int) string {
	best := ""
	for _, s := range doc.Sections {
		if s.Start <= line && line < s.End {
			best = s.Anchor
		}
	}
	return best
}

func replaceSection(doc *ledger.Doc, anchor string, body []string) bool {
	for _, s := range doc.Sections {
		if s.Anchor != anchor {
			continue
		}
		newLines := append([]string{}, doc.Lines[:s.Start+1]...)
		newLines = append(newLines, "")
		newLines = append(newLines, body...)
		tail := doc.Lines[s.End:]
		if len(tail) > 0 && strings.TrimSpace(tail[0]) != "" {
			newLines = append(newLines, "")
		}
		delta := len(newLines) + len(tail) - len(doc.Lines)
		doc.Lines = append(newLines, tail...)
		shiftAfter(doc, s.End-1, delta) // approximate: units inside the section were prose-only
		return true
	}
	return false
}

func rowTier(doc *ledger.Doc, u *ledger.Unit) string {
	cols := schema.TableSchemas[u.Prefix]
	for i, c := range cols {
		if c == "Priority" && i < len(u.Cells) && strings.EqualFold(u.Cells[i], "Must") {
			return "contract-changing"
		}
		if c == "Blocking?" && i < len(u.Cells) && strings.EqualFold(u.Cells[i], "yes") {
			return "contract-changing"
		}
	}
	return "corrective"
}

func colsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x := strings.TrimSpace(strings.Trim(a[i], "* "))
		if x != b[i] {
			return false
		}
	}
	return true
}

func validateEnvelope(a *ledger.Amendment, meta AmendMeta, o options, defects *[]schema.Defect) {
	name := a.Name
	// The approval bar is a merge-target check (FR-5), so a compute-only fold
	// skips it — see AllowUnapproved. Every other envelope check still runs.
	if s := a.Header["Status"]; !o.allowUnapproved && !strings.EqualFold(strings.TrimSpace(s), "Approved") {
		*defects = append(*defects, def("validate-amendments", name, "envelope",
			fmt.Sprintf("Status is %q, not Approved", s), "an amendment merges only with Status: Approved"))
	}
	lf := a.Header["Learned-from"]
	if strings.TrimSpace(lf) == "" {
		*defects = append(*defects, def("validate-amendments", name, "envelope", "Learned-from is empty", "evidence is a typed reference, not a vibe"))
	} else {
		for _, tok := range strings.Split(lf, "·") {
			tok = strings.TrimSpace(tok)
			if tok == "" {
				continue
			}
			cls := tok
			if k := strings.Index(tok, ":"); k > 0 {
				cls = tok[:k]
			}
			if !schema.EvidenceClasses[strings.TrimSpace(cls)] {
				*defects = append(*defects, def("validate-amendments", name, "envelope",
					fmt.Sprintf("unknown evidence class %q", cls), "classes: incident, review-thread, telemetry-query, build-result, user-research, support-ticket, operator-decision, dependency-change, policy-change"))
			}
		}
	}
	declared := parseTier(a.Header["Blast radius"])
	if declared == "" {
		*defects = append(*defects, def("validate-amendments", name, "envelope",
			"Blast radius does not parse (want `tier: <t>; touches: <IDs>; reason: <why>`)", "use the structured form"))
		return
	}
	if schema.TierRank[meta.ComputedTier] > schema.TierRank[declared] {
		*defects = append(*defects, def("validate-amendments", name, "envelope",
			fmt.Sprintf("declared blast radius %q understates computed %q", declared, meta.ComputedTier),
			"declared may exceed computed, never understate"))
	}
	if v := a.Header["Template version"]; !strings.Contains(v, schema.AmendmentTemplateVersion) && !strings.Contains(v, schema.AmendmentTemplateV2) {
		*defects = append(*defects, def("validate-amendments", name, "envelope",
			fmt.Sprintf("Template version %q is not in the supported set (%s, %s)", v, schema.AmendmentTemplateVersion, schema.AmendmentTemplateV2),
			"declare a supported envelope template version (FR-24)"))
	}
}

// parseTier extracts the declared tier from the structured Blast radius
// value (`tier: <t>; touches: …; reason: …`).
func parseTier(s string) string {
	for _, t := range []string{"contract-changing", "corrective", "additive"} {
		if strings.Contains(s, "tier: "+t) || strings.Contains(s, "tier:"+t) {
			return t
		}
	}
	return ""
}

// emitEffective renders the canonical projection markdown.
func emitEffective(doc *ledger.Doc, asOf, fingerprint string, removals []removed) []byte {
	var b strings.Builder
	b.WriteString("# " + doc.Title + " *(effective view)*\n\n")
	b.WriteString("---\n\n")
	b.WriteString("**GENERATED — DO NOT EDIT.** Sole writer: `docgate fold`. As of: **" + asOf + "**. Content fingerprint: `" + fingerprint + "`. Tool version (recorded, never hashed): `" + schema.ToolVersion + "`.\n\n")
	b.WriteString("---\n\n")
	b.WriteString(strings.Join(doc.Lines, "\n"))
	b.WriteString("\n\n## Superseded & retired\n\n")
	if len(removals) == 0 {
		b.WriteString("None — no unit has been superseded or retired; the ledger is its base.\n")
	} else {
		for _, r := range removals {
			b.WriteString("- **" + r.id + "** *(" + r.note + ")* — `" + r.verbatim + "`\n")
		}
	}
	return schema.Normalize([]byte(b.String()))
}

type sourceDoc struct {
	SchemaVersion string                `json:"schema_version"`
	GeneratedBy   string                `json:"generated_by"`
	ToolVersion   string                `json:"tool_version"`
	AsOf          string                `json:"as_of"`
	Fingerprint   string                `json:"content_fingerprint"`
	Units         map[string]*UnitState `json:"units"`
}

func emitSource(states map[string]*UnitState, asOf, fingerprint string) []byte {
	out, err := json.MarshalIndent(sourceDoc{
		SchemaVersion: "1", GeneratedBy: "docgate", ToolVersion: schema.ToolVersion,
		AsOf: asOf, Fingerprint: fingerprint, Units: states,
	}, "", "  ")
	if err != nil {
		return []byte("{}")
	}
	return append(out, '\n')
}
