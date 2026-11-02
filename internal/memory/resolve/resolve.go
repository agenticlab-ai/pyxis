// Package resolve computes what the stored records mean right now: which are
// active, which have been superseded or revoked, which claims conflict, what
// depends on what, why a claim says what it says, and which designs have been
// reopened by a change to what they serve. Status is never stored; it is
// always this projection over the files.
package resolve

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// Status of a record under the projection.
type Status string

// The three statuses. An act can itself be superseded or revoked; an inactive
// act has no effect on the projection.
const (
	Active     Status = "active"
	Superseded Status = "superseded"
	Revoked    Status = "revoked"
)

// Conflict is a subject and family with more than one active claim.
type Conflict struct {
	Subject string
	Family  string
	IDs     []string
}

// Dependent is an active record that rests on another through a relation.
type Dependent struct {
	Record *record.Record
	Type   string
}

// Reopen is a decision or plan whose served claim is no longer active.
type Reopen struct {
	Record    *record.Record // the decision or plan
	Serves    string         // the id it serves
	Status    Status         // what became of that id
	Successor *record.Record // the active head of the same family, if one exists
}

// Violation is an edge the projection refuses to honour: a relation that
// points forward in history, a supersession that would retire something it
// may not, or an answer to a question that is not there to answer. The edge
// is reported and ignored, so the target stays whatever it was.
type Violation struct {
	Code    string // supersedes-across-subjects | supersedes-across-questions | supersedes-out-of-order | supersedes-revoked-record | document-mode-changed | relation-out-of-order | answers-inactive-question | answers-across-subjects | revokes-across-subjects | revokes-out-of-order | revokes-inactive-record | revocation-breaks-projection
	Record  string // the record carrying the edge
	Target  string
	Related []string // other records participating in the defect, in id order
	What    string
}

// Projection is the resolved view of one set.
type Projection struct {
	Set              *store.Set
	Violations       []Violation
	status           map[string]Status
	heads            map[string]map[string][]*record.Record // subject → family → active claims
	supersessions    map[string][]string                    // admitted superseder id → target ids
	revocations      map[string]string                      // admitted Revocation id → target id
	invalidRelations map[relationEdge]bool                  // refused relations
}

type relationEdge struct {
	from   string
	typ    string
	target string
}

// New computes the projection.
func New(set *store.Set) *Projection {
	p := &Projection{
		Set:              set,
		status:           map[string]Status{},
		heads:            map[string]map[string][]*record.Record{},
		supersessions:    map[string][]string{},
		revocations:      map[string]string{},
		invalidRelations: map[relationEdge]bool{},
	}
	records := chronologicalRecords(set)
	seen := make([]*record.Record, 0, len(records))
	supersessions := p.supersessions
	var invalid []Violation

	for _, r := range records {
		seen = append(seen, r)
		invalid = append(invalid, p.rejectForwardRelations(r, set)...)
		status := resolveStatuses(seen, supersessions, p.revocations)
		if r.Kind == record.Revocation {
			targetID := r.Rel(record.Revokes)[0]
			target := set.Records[targetID]
			switch {
			case target == nil:
				// The store owns dangling relations. An absent target never gains
				// a phantom projection status here.
				continue
			case target.Subject != r.Subject:
				invalid = append(invalid, Violation{Code: "revokes-across-subjects", Record: r.ID, Target: targetID,
					What: fmt.Sprintf("%s on %s revokes %s on %s; a revocation stays on the subject it changes", r.ID, r.Subject, targetID, target.Subject)})
				continue
			case !record.IDBefore(targetID, r.ID):
				invalid = append(invalid, Violation{Code: "revokes-out-of-order", Record: r.ID, Target: targetID,
					What: fmt.Sprintf("%s revokes %s, which was minted later; a revocation points backward in time", r.ID, targetID)})
				continue
			case status[targetID] == Superseded:
				superseders := activeSuperseders(targetID, status, supersessions, set)
				invalid = append(invalid, Violation{Code: "revokes-inactive-record", Record: r.ID, Target: targetID,
					Related: superseders,
					What:    fmt.Sprintf("%s revokes %s, which was already superseded by %s; only an active record may be revoked", r.ID, targetID, strings.Join(superseders, ", "))})
				continue
			case status[targetID] == Revoked:
				// Keep both same-target acts. The projection exposes their full
				// membership as a RevocationConflict; revoking either act later
				// leaves the remaining act effective rather than losing intent.
				p.revocations[r.ID] = targetID
				continue
			case status[targetID] != Active:
				invalid = append(invalid, Violation{Code: "revokes-inactive-record", Record: r.ID, Target: targetID,
					What: fmt.Sprintf("%s revokes %s, which is not active", r.ID, targetID)})
				continue
			}

			before := projectionIntegrity(seen, resolveStatuses(seen, supersessions, p.revocations), supersessions, p.revocations, invalid, set)
			p.revocations[r.ID] = targetID
			afterStatus := resolveStatuses(seen, supersessions, p.revocations)
			after := projectionIntegrity(seen, afterStatus, supersessions, p.revocations, invalid, set)
			introduced := newIntegrityIssues(before, after)
			if len(introduced) == 0 {
				continue
			}
			delete(p.revocations, r.ID)
			var details, related []string
			for _, issue := range introduced {
				details = append(details, issue.detail)
				related = append(related, issue.ids...)
			}
			invalid = append(invalid, Violation{Code: "revocation-breaks-projection", Record: r.ID, Target: targetID,
				Related: uniqueIDs(related, r.ID, targetID),
				What:    fmt.Sprintf("%s cannot revoke %s because it would introduce %s", r.ID, targetID, strings.Join(details, "; "))})
			continue
		}

		for _, targetID := range r.Rel(record.Supersedes) {
			target := set.Records[targetID]
			if v := SupersedesViolation(r, target); v != nil {
				p.invalidRelations[relationEdge{from: r.ID, typ: record.Supersedes, target: targetID}] = true
				invalid = append(invalid, *v)
				continue
			}
			if target == nil {
				continue
			}
			status = resolveStatuses(seen, supersessions, p.revocations)
			if status[targetID] == Revoked {
				revokers := activeRevokers(targetID, status, p.revocations)
				p.invalidRelations[relationEdge{from: r.ID, typ: record.Supersedes, target: targetID}] = true
				invalid = append(invalid, Violation{Code: "supersedes-revoked-record", Record: r.ID, Target: targetID, Related: revokers,
					What: fmt.Sprintf("%s supersedes %s, which was already revoked by %s; a successor cannot revive withdrawn truth", r.ID, targetID, strings.Join(revokers, ", "))})
				continue
			}
			supersessions[r.ID] = append(supersessions[r.ID], targetID)
		}
	}

	p.status = resolveStatuses(records, supersessions, p.revocations)
	for _, v := range invalid {
		if p.status[v.Record] != Revoked {
			p.Violations = append(p.Violations, v)
		}
	}
	for _, r := range records {
		if r.Kind != record.Answer || p.status[r.ID] != Active {
			continue
		}
		q := r.Rel(record.Answers)[0]
		edge := relationEdge{from: r.ID, typ: record.Answers, target: q}
		if p.invalidRelations[edge] {
			continue
		}
		if target := set.Records[q]; target != nil && target.Subject != r.Subject {
			p.invalidRelations[edge] = true
			p.Violations = append(p.Violations, Violation{Code: "answers-across-subjects", Record: r.ID, Target: q,
				What: fmt.Sprintf("%s on %s answers %s on %s; an answer stays on its question's subject", r.ID, r.Subject, q, target.Subject)})
			continue
		}
		if p.status[q] != Active {
			p.invalidRelations[edge] = true
			related := activeRevokers(q, p.status, p.revocations)
			what := fmt.Sprintf("%s answers %s, which is %s; an answer needs an open question", r.ID, q, p.status[q])
			if len(related) > 0 {
				what += "; active revocation(s): " + strings.Join(related, ", ")
			}
			p.Violations = append(p.Violations, Violation{Code: "answers-inactive-question", Record: r.ID, Target: q,
				Related: related, What: what})
		}
	}
	for id, r := range set.Records {
		if record.IsClaim(r.Kind) && p.status[id] == Active {
			fam := p.heads[r.Subject]
			if fam == nil {
				fam = map[string][]*record.Record{}
				p.heads[r.Subject] = fam
			}
			fam[r.Family] = append(fam[r.Family], r)
		}
	}
	for _, fam := range p.heads {
		for _, rs := range fam {
			sort.Slice(rs, func(i, j int) bool { return rs[i].ID < rs[j].ID })
		}
	}
	return p
}

type integrityIssue struct {
	key     string
	kind    string
	anchor  string
	members []string
	detail  string
	ids     []string
}

func chronologicalRecords(set *store.Set) []*record.Record {
	out := make([]*record.Record, 0, len(set.Records))
	for _, r := range set.Records {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := idOrderKey(out[i].ID), idOrderKey(out[j].ID)
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// rejectForwardRelations records every present non-act edge whose target was
// minted at or after its carrier. Writers can only name records already in
// their tree, so accepting such an edge at merge/read time would give a
// hand-authored file semantics the CLI could never have admitted. Supersedes
// and revokes retain their more specific diagnostics in the act resolver.
func (p *Projection) rejectForwardRelations(r *record.Record, set *store.Set) []Violation {
	typesByTarget := map[string][]string{}
	for _, rel := range r.Relations {
		if rel.Type == record.Supersedes || rel.Type == record.Revokes || set.Records[rel.Target] == nil || record.IDBefore(rel.Target, r.ID) {
			continue
		}
		p.invalidRelations[relationEdge{from: r.ID, typ: rel.Type, target: rel.Target}] = true
		typesByTarget[rel.Target] = append(typesByTarget[rel.Target], rel.Type)
	}
	targets := make([]string, 0, len(typesByTarget))
	for target := range typesByTarget {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	out := make([]Violation, 0, len(targets))
	for _, target := range targets {
		types := typesByTarget[target]
		sort.Strings(types)
		out = append(out, Violation{
			Code:   "relation-out-of-order",
			Record: r.ID,
			Target: target,
			What: fmt.Sprintf("%s carries %s to %s, which was minted later; every relation points backward to knowledge that already existed",
				r.ID, relationPhrase(types), target),
		})
	}
	return out
}

func relationPhrase(types []string) string {
	if len(types) == 1 {
		return types[0] + " relation"
	}
	return strings.Join(types, ", ") + " relations"
}

// IDs encode chronology in their ULID suffix. Comparing the whole id would
// compare kind names first and misorder cross-kind acts.
func idOrderKey(id string) string {
	if i := strings.IndexByte(id, '-'); i >= 0 {
		return id[i+1:]
	}
	return id
}

func resolveStatuses(records []*record.Record, supersessions map[string][]string, revocations map[string]string) map[string]Status {
	status := make(map[string]Status, len(records))
	for _, r := range records {
		status[r.ID] = Active
	}
	for i := len(records) - 1; i >= 0; i-- {
		r := records[i]
		if r.Kind != record.Revocation || status[r.ID] == Revoked {
			continue
		}
		if target := revocations[r.ID]; target != "" {
			status[target] = Revoked
		}
	}
	for _, r := range records {
		if status[r.ID] == Revoked {
			continue
		}
		for _, target := range supersessions[r.ID] {
			if status[target] != Revoked {
				status[target] = Superseded
			}
		}
	}
	return status
}

func activeRevokers(target string, status map[string]Status, revocations map[string]string) []string {
	var out []string
	for id, revoked := range revocations {
		if revoked == target && status[id] == Active {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func activeSuperseders(target string, status map[string]Status, supersessions map[string][]string, set *store.Set) []string {
	var out []string
	for id, targets := range supersessions {
		if status[id] == Revoked {
			continue
		}
		for _, candidate := range targets {
			if candidate == target && set.Records[id] != nil {
				out = append(out, id)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func projectionIntegrity(records []*record.Record, status map[string]Status, supersessions map[string][]string, revocations map[string]string, invalid []Violation, set *store.Set) map[string]integrityIssue {
	out := map[string]integrityIssue{}
	for _, v := range invalid {
		if status[v.Record] != Active {
			continue
		}
		members := append([]string(nil), v.Related...)
		sort.Strings(members)
		ids := append([]string{v.Record, v.Target}, members...)
		anchor := strings.Join([]string{v.Code, v.Record, v.Target}, "\x00")
		key := strings.Join([]string{"violation", anchor, strings.Join(members, ",")}, "\x00")
		out[key] = integrityIssue{key: key, kind: "violation", anchor: anchor, members: members, detail: v.Code + " on " + v.Record, ids: ids}
	}
	for _, dangling := range set.Dangling {
		if status[dangling.From] != Active {
			continue
		}
		anchor := strings.Join([]string{dangling.From, dangling.Type, dangling.Target}, "\x00")
		key := "dangling\x00" + anchor
		out[key] = integrityIssue{
			key: key, kind: "dangling", anchor: anchor,
			detail: fmt.Sprintf("active dangling %s edge on %s to %s", dangling.Type, dangling.From, dangling.Target),
			ids:    []string{dangling.From},
		}
	}

	families := map[string][]string{}
	for _, r := range records {
		if record.IsClaim(r.Kind) && status[r.ID] == Active {
			key := r.Subject + "#" + r.Family
			families[key] = append(families[key], r.ID)
		}
	}
	for ref, ids := range families {
		if len(ids) < 2 {
			continue
		}
		sort.Strings(ids)
		key := strings.Join([]string{"family-conflict", ref, strings.Join(ids, ",")}, "\x00")
		out[key] = integrityIssue{key: key, kind: "family-conflict", anchor: ref, members: append([]string(nil), ids...), detail: "family conflict at " + ref + " (" + strings.Join(ids, ", ") + ")", ids: append([]string(nil), ids...)}
	}

	answers := map[string][]string{}
	for _, r := range records {
		if r.Kind != record.Answer || status[r.ID] != Active {
			continue
		}
		q := r.Rel(record.Answers)[0]
		target := set.Records[q]
		if target != nil && !record.IDBefore(q, r.ID) {
			// The matching relation-out-of-order violation is already in invalid.
			// Do not also let this refused edge participate in answer conflicts.
			continue
		}
		if target != nil && target.Subject != r.Subject {
			key := strings.Join([]string{"answers-across-subjects", r.ID, q}, "\x00")
			out[key] = integrityIssue{key: key, detail: "cross-subject Answer " + r.ID, ids: []string{r.ID, q}}
			continue
		}
		if status[q] != Active {
			key := strings.Join([]string{"answers-inactive-question", r.ID, q}, "\x00")
			out[key] = integrityIssue{key: key, detail: "active Answer " + r.ID + " on inactive Question " + q, ids: []string{r.ID, q}}
			continue
		}
		answers[q] = append(answers[q], r.ID)
	}
	for q, ids := range answers {
		if len(ids) < 2 {
			continue
		}
		sort.Strings(ids)
		key := strings.Join([]string{"answer-conflict", q, strings.Join(ids, ",")}, "\x00")
		out[key] = integrityIssue{key: key, kind: "answer-conflict", anchor: q, members: append([]string(nil), ids...), detail: "answer conflict at " + q + " (" + strings.Join(ids, ", ") + ")", ids: append([]string{q}, ids...)}
	}

	byTarget := map[string][]string{}
	for id, target := range revocations {
		if status[id] == Active {
			byTarget[target] = append(byTarget[target], id)
		}
	}
	for target, ids := range byTarget {
		if len(ids) < 2 {
			continue
		}
		sort.Strings(ids)
		key := strings.Join([]string{"revocation-conflict", target, strings.Join(ids, ",")}, "\x00")
		out[key] = integrityIssue{key: key, kind: "revocation-conflict", anchor: target, members: append([]string(nil), ids...), detail: "revocation conflict at " + target + " (" + strings.Join(ids, ", ") + ")", ids: append([]string{target}, ids...)}
	}

	for _, r := range records {
		if r.Kind != record.Document || status[r.ID] != Active {
			continue
		}
		for _, ref := range r.Embeds {
			i := strings.IndexByte(ref, '#')
			if i >= 0 && hasActiveHead(records, status, ref[:i], ref[i+1:]) {
				continue
			}
			key := strings.Join([]string{"embed-unresolved", r.ID, ref}, "\x00")
			out[key] = integrityIssue{key: key, kind: "embed-unresolved", anchor: r.ID + "\x00" + ref, detail: "unresolved embed {{" + ref + "}} in " + r.ID, ids: []string{r.ID}}
		}
	}
	_ = supersessions // retained in the signature to make the state inputs explicit
	return out
}

func hasActiveHead(records []*record.Record, status map[string]Status, subject, family string) bool {
	for _, r := range records {
		if record.IsClaim(r.Kind) && r.Subject == subject && r.Family == family && status[r.ID] == Active {
			return true
		}
	}
	return false
}

func newIntegrityIssues(before, after map[string]integrityIssue) []integrityIssue {
	var keys []string
	for key, issue := range after {
		if !integrityIssueCovered(before, issue) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	out := make([]integrityIssue, 0, len(keys))
	for _, key := range keys {
		out = append(out, after[key])
	}
	return out
}

func integrityIssueCovered(before map[string]integrityIssue, after integrityIssue) bool {
	if _, existed := before[after.key]; existed {
		return true
	}
	if len(after.members) == 0 {
		return false
	}
	for _, prior := range before {
		if prior.kind == after.kind && prior.anchor == after.anchor && strictStringSubset(after.members, prior.members) {
			return true
		}
	}
	return false
}

func strictStringSubset(candidate, prior []string) bool {
	if len(candidate) >= len(prior) {
		return false
	}
	want := make(map[string]bool, len(prior))
	for _, value := range prior {
		want[value] = true
	}
	for _, value := range candidate {
		if !want[value] {
			return false
		}
	}
	return true
}

func uniqueIDs(ids []string, exclude ...string) []string {
	skip := map[string]bool{}
	for _, id := range exclude {
		skip[id] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, id := range ids {
		if id == "" || skip[id] || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// SupersedesViolation says why r may not supersede target, or nil when it
// may. A supersession stays on its subject (a claim retires nothing elsewhere;
// a family rename is a supersession on the same subject), and it points
// backwards in time: the target's id sorts before the superseder's, which is
// what makes a cycle impossible and history causally ordered. A Document's
// literal/native interpretation mode is stable across that lineage: changing
// it could silently turn claim markers into inert text, or the reverse. A
// missing target is not a violation here; the store reports it as dangling.
func SupersedesViolation(r, target *record.Record) *Violation {
	if target == nil {
		return nil
	}
	if target.Subject != r.Subject {
		return &Violation{Code: "supersedes-across-subjects", Record: r.ID, Target: target.ID,
			What: fmt.Sprintf("%s on %s supersedes %s on %s; a claim retires nothing outside its own subject", r.ID, r.Subject, target.ID, target.Subject)}
	}
	if r.Kind == record.Document && target.Kind == record.Document && r.LiteralBody != target.LiteralBody {
		return &Violation{Code: "document-mode-changed", Record: r.ID, Target: target.ID,
			What: fmt.Sprintf("%s changes %s from literal_body=%t to literal_body=%t; a Document lineage keeps one interpretation mode", r.ID, target.ID, target.LiteralBody, r.LiteralBody)}
	}
	if r.Kind == record.Answer && target.Kind == record.Answer {
		mine, theirs := r.Rel(record.Answers), target.Rel(record.Answers)
		if len(mine) == 1 && len(theirs) == 1 && mine[0] != theirs[0] {
			return &Violation{Code: "supersedes-across-questions", Record: r.ID, Target: target.ID,
				What: fmt.Sprintf("%s answers %s but supersedes %s, an answer to %s; a correction replaces an answer to the same question", r.ID, mine[0], target.ID, theirs[0])}
		}
	}
	if !record.IDBefore(target.ID, r.ID) {
		return &Violation{Code: "supersedes-out-of-order", Record: r.ID, Target: target.ID,
			What: fmt.Sprintf("%s supersedes %s, which was minted later; a successor is newer than what it replaces (check the clock that minted the id)", r.ID, target.ID)}
	}
	return nil
}

// Status of a record id; an unknown id reads as "" .
func (p *Projection) Status(id string) Status { return p.status[id] }

// Targets returns the targets of r's honoured relations of one type, in file
// order. A dangling, causally forward, or otherwise rejected edge is absent:
// callers that render the resolved memory must not fall back to Record.Rel,
// which deliberately exposes the immutable bytes including invalid history.
func (p *Projection) Targets(r *record.Record, typ string) []string {
	if r == nil {
		return nil
	}
	var out []string
	for _, rel := range r.Relations {
		if rel.Type == typ && p.relationHonored(r.ID, rel.Type, rel.Target) {
			out = append(out, rel.Target)
		}
	}
	return out
}

func (p *Projection) relationHonored(from, typ, target string) bool {
	if p.Set.Records[target] == nil {
		return false
	}
	switch typ {
	case record.Supersedes:
		for _, admitted := range p.supersessions[from] {
			if admitted == target {
				return true
			}
		}
		return false
	case record.Revokes:
		return p.revocations[from] == target
	default:
		return !p.invalidRelations[relationEdge{from: from, typ: typ, target: target}]
	}
}

// Active lists the active records of a subject in id order.
func (p *Projection) Active(subject string) []*record.Record {
	var out []*record.Record
	for _, r := range p.Set.BySubject[subject] {
		if p.status[r.ID] == Active {
			out = append(out, r)
		}
	}
	return out
}

// Head returns the active claim of a family, the newest when the family is in
// conflict, or nil.
func (p *Projection) Head(subject, family string) *record.Record {
	rs := p.heads[subject][family]
	if len(rs) == 0 {
		return nil
	}
	return rs[len(rs)-1]
}

// Families lists the families with an active claim on a subject, sorted.
func (p *Projection) Families(subject string) []string {
	out := make([]string, 0, len(p.heads[subject]))
	for f := range p.heads[subject] {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// Conflicts lists every family with more than one active claim, sorted.
func (p *Projection) Conflicts() []Conflict {
	var out []Conflict
	for sub, fams := range p.heads {
		for fam, rs := range fams {
			if len(rs) > 1 {
				c := Conflict{Subject: sub, Family: fam}
				for _, r := range rs {
					c.IDs = append(c.IDs, r.ID)
				}
				out = append(out, c)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Subject != out[j].Subject {
			return out[i].Subject < out[j].Subject
		}
		return out[i].Family < out[j].Family
	})
	return out
}

// Dependents lists the active records that serve, depend on, are about, or
// refine the given id, in id order.
func (p *Projection) Dependents(id string) []Dependent {
	var out []Dependent
	for _, e := range p.Set.To[id] {
		switch e.Type {
		case record.Serves, record.DependsOn, record.About, record.Refines:
			if r := p.Set.Records[e.From]; r != nil && p.status[r.ID] == Active && p.relationHonored(r.ID, e.Type, id) {
				out = append(out, Dependent{Record: r, Type: e.Type})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Record.ID < out[j].Record.ID })
	return out
}

// Chain walks a record back through what it superseded: the why-chain of a
// claim, newest first. It stops at a missing target or after limit records.
func (p *Projection) Chain(id string, limit int) []*record.Record {
	var out []*record.Record
	seen := map[string]bool{}
	for cur := p.Set.Records[id]; cur != nil && len(out) < limit && !seen[cur.ID]; {
		out = append(out, cur)
		seen[cur.ID] = true
		sup := p.Targets(cur, record.Supersedes)
		if len(sup) == 0 {
			break
		}
		cur = p.Set.Records[sup[0]]
	}
	return out
}

// Successors lists the active records that supersede id directly or through
// a chain, i.e. the current heads that descend from it.
func (p *Projection) Successors(id string) []*record.Record {
	var out []*record.Record
	seen := map[string]bool{}
	var walk func(string)
	walk = func(cur string) {
		for _, r := range p.Set.From(cur, record.Supersedes) {
			if !p.relationHonored(r.ID, record.Supersedes, cur) {
				continue
			}
			if seen[r.ID] {
				continue
			}
			seen[r.ID] = true
			if p.status[r.ID] == Active {
				out = append(out, r)
			}
			walk(r.ID)
		}
	}
	walk(id)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Reopened lists active decisions and plans that serve a claim that is no
// longer active. With no subjects every subject is scanned.
func (p *Projection) Reopened(subjects []string) []Reopen {
	scan := subjects
	if len(scan) == 0 {
		scan = p.Set.Subjects()
	}
	var out []Reopen
	for _, sub := range scan {
		for _, r := range p.Active(sub) {
			if r.Kind != record.Decision && r.Kind != record.Plan {
				continue
			}
			for _, t := range p.Targets(r, record.Serves) {
				target := p.Set.Records[t]
				if target == nil || p.status[t] == Active {
					continue
				}
				out = append(out, Reopen{Record: r, Serves: t, Status: p.status[t], Successor: p.Head(target.Subject, target.Family)})
			}
		}
	}
	return out
}

// Undesigned lists the active promises and expectations of a subject that no
// active decision serves.
func (p *Projection) Undesigned(subject string) []*record.Record {
	var out []*record.Record
	for _, r := range p.Active(subject) {
		if r.Kind != record.Promise && r.Kind != record.Expectation {
			continue
		}
		if !p.served(r.ID) {
			out = append(out, r)
		}
	}
	return out
}

func (p *Projection) served(id string) bool {
	for _, e := range p.Set.To[id] {
		if e.Type != record.Serves {
			continue
		}
		if from := p.Set.Records[e.From]; from != nil && from.Kind == record.Decision && p.status[from.ID] == Active && p.relationHonored(from.ID, e.Type, id) {
			return true
		}
	}
	return false
}

// Orphaned lists active decisions that served something and now serve nothing
// active: the decision made for a feature that was never built or was
// withdrawn.
func (p *Projection) Orphaned() []*record.Record {
	var out []*record.Record
	for _, id := range p.Set.IDs() {
		r := p.Set.Records[id]
		if r.Kind != record.Decision || p.status[id] != Active {
			continue
		}
		serves := p.Targets(r, record.Serves)
		if len(serves) == 0 {
			continue
		}
		alive := false
		for _, t := range serves {
			if p.status[t] == Active {
				alive = true
				break
			}
			// a served claim with a successor is a reopen (re-point the decision), not an orphan
			if target := p.Set.Records[t]; target != nil && p.Head(target.Subject, target.Family) != nil {
				alive = true
				break
			}
		}
		if !alive {
			out = append(out, r)
		}
	}
	return out
}

// Answer returns the active answer to a question, or nil. A revoked or
// superseded answer no longer answers; when several are active (a conflict
// the gate refuses), the newest stands until it is resolved.
func (p *Projection) Answer(questionID string) *record.Record {
	active := p.ActiveAnswers(questionID)
	if len(active) == 0 {
		return nil
	}
	return active[len(active)-1]
}

// ActiveAnswers lists the active answers of a question in id order; more
// than one is a conflict. A question that is not active has no answers.
func (p *Projection) ActiveAnswers(questionID string) []*record.Record {
	if p.status[questionID] != Active {
		return nil
	}
	question := p.Set.Records[questionID]
	var out []*record.Record
	for _, a := range p.Set.From(questionID, record.Answers) {
		if p.status[a.ID] == Active && question != nil && a.Subject == question.Subject && p.relationHonored(a.ID, record.Answers, questionID) {
			out = append(out, a)
		}
	}
	return out
}

// AnswerConflict is a question with more than one active answer.
type AnswerConflict struct {
	Question string
	IDs      []string
}

// AnswerConflicts lists every question answered more than once, in id order.
func (p *Projection) AnswerConflicts() []AnswerConflict {
	var out []AnswerConflict
	for _, id := range p.Set.IDs() {
		q := p.Set.Records[id]
		if q.Kind != record.Question {
			continue
		}
		if active := p.ActiveAnswers(id); len(active) > 1 {
			c := AnswerConflict{Question: id}
			for _, a := range active {
				c.IDs = append(c.IDs, a.ID)
			}
			out = append(out, c)
		}
	}
	return out
}

// RevocationConflict is a target with more than one active revocation. All
// revokers are carried so a merge gate can attribute a concurrent duplicate
// to either side rather than whichever id happens to sort last.
type RevocationConflict struct {
	Target string
	IDs    []string
}

// RevocationConflicts lists every multiply-revoked target in target-id order.
func (p *Projection) RevocationConflicts() []RevocationConflict {
	byTarget := map[string][]string{}
	for id, target := range p.revocations {
		if p.status[id] == Active {
			byTarget[target] = append(byTarget[target], id)
		}
	}
	var targets []string
	for target, ids := range byTarget {
		if len(ids) > 1 {
			targets = append(targets, target)
		}
	}
	sort.Strings(targets)
	out := make([]RevocationConflict, 0, len(targets))
	for _, target := range targets {
		ids := byTarget[target]
		sort.Strings(ids)
		out = append(out, RevocationConflict{Target: target, IDs: ids})
	}
	return out
}

// OpenQuestions lists active questions with no answer, oldest first. With no
// subjects every subject is scanned.
func (p *Projection) OpenQuestions(subjects []string) []*record.Record {
	scan := subjects
	if len(scan) == 0 {
		scan = p.Set.Subjects()
	}
	var out []*record.Record
	for _, sub := range scan {
		for _, r := range p.Active(sub) {
			if r.Kind == record.Question && p.Answer(r.ID) == nil {
				out = append(out, r)
			}
		}
	}
	return out
}

// Expired lists active assumptions whose expiry has passed.
func (p *Projection) Expired(now time.Time) []*record.Record {
	var out []*record.Record
	for _, id := range p.Set.IDs() {
		r := p.Set.Records[id]
		if r.Kind != record.Assumption || p.status[id] != Active {
			continue
		}
		if t, err := time.Parse(time.RFC3339, r.Expires); err == nil && t.Before(now) {
			out = append(out, r)
		}
	}
	return out
}
