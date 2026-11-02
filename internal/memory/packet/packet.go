// Package packet turns a branch into something a reviewer can rule on.
//
// A proposal is the set of records a branch adds relative to its base. The
// packet says what those records add or move, what they supersede, whether a
// superseded head has moved on the base since the branch was cut, which
// designs the change would reopen, which new claims nothing serves, what
// other branches are touching the same families, and how much scrutiny the
// change earns. The same computation is the merge check: blockers refuse the
// merge, warnings are for the reviewer to weigh. Approval itself lives on the
// pull request, not here.
package packet

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/capture"
	"github.com/agenticlab-ai/pyxis/internal/memory/policy"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// Input is everything the packet is computed from. The git adapter supplies
// the sets; the packet never touches git.
type Input struct {
	Base        *store.Set            // the memory at the base ref (its tip)
	Ancestor    *store.Set            // the memory at the merge base of branch and base; nil means Base
	Head        *store.Set            // the memory on the branch (working tree or ref)
	InFlight    map[string]*store.Set // branch → the records that branch adds relative to the base
	Policy      policy.Policy         // the candidate's policy
	BasePolicy  policy.Policy         // the base's policy: the one that judges the candidate
	Branch      string
	BaseRef     string
	Description string
}

// Finding is one thing the reviewer or the merge check must know.
type Finding struct {
	Code     string // stable, kebab-case
	What     string
	Resolver string // who or what resolves it
}

// Superseded is a base record an added record replaces.
type Superseded struct {
	Target *record.Record
	By     *record.Record
}

// Moved is a supersedes target that is no longer the head on the base: the
// effective set moved under the branch.
type Moved struct {
	By          *record.Record
	Target      string
	Status      resolve.Status
	CurrentHead *record.Record // nil when the family has no head any more
}

// Concurrent is another branch touching the same family.
type Concurrent struct {
	Branch string
	Ours   *record.Record
	Theirs *record.Record
	What   string
}

// Packet is the computed proposal.
type Packet struct {
	Branch        string
	BaseRef       string
	Description   string
	Consequence   string
	PolicyChanged bool // the candidate changes policy.json; every such change is high
	Added         []*record.Record
	Modified      []string
	Deleted       []string
	Superseded    []Superseded
	Moved         []Moved
	Reopens       []resolve.Reopen
	Undesigned    []*record.Record
	Conflicts     []resolve.Conflict
	Concurrent    []Concurrent
	Blockers      []Finding
	Warnings      []Finding
	Union         *resolve.Projection // the memory as it would be after the merge
	BaseView      *resolve.Projection
}

// Compute builds the packet.
func Compute(in Input) *Packet {
	p := &Packet{Branch: in.Branch, BaseRef: in.BaseRef, Description: in.Description, Consequence: record.Low}
	hasAncestor := in.Ancestor != nil
	ancestor := in.Ancestor
	if ancestor == nil {
		ancestor = in.Base
	}
	// immutability is judged against what the branch started from: a branch
	// that is merely behind the base has not deleted the base's newer records
	for path, b := range in.Head.Bytes {
		if old, ok := ancestor.Bytes[path]; ok && path != store.PolicyFile && string(old) != string(b) {
			p.Modified = append(p.Modified, path)
		}
	}
	for path := range ancestor.Bytes {
		if _, ok := in.Head.Bytes[path]; !ok && path != store.PolicyFile {
			p.Deleted = append(p.Deleted, path)
		}
	}
	// The union is what the memory would be after the merge: start at the
	// current base tip, then apply only the changes the branch made since its
	// merge base. Overlaying the whole (possibly stale) branch tree would
	// resurrect files deleted on the base and overwrite files changed there.
	merged := store.MapSource{}
	var addCollisions []string
	addCollisionPaths := map[string]bool{}
	maps.Copy(merged, in.Base.Bytes)
	for path, b := range in.Head.Bytes {
		old, existed := ancestor.Bytes[path]
		if !existed || !sameTextBytes(old, b) {
			if !existed && path != store.PolicyFile {
				if baseBytes, onBase := in.Base.Bytes[path]; onBase && !sameTextBytes(baseBytes, b) {
					addCollisionPaths[path] = true
					addCollisions = append(addCollisions, fmt.Sprintf("%s was independently added on the branch and %s with different bytes; rebase and keep one immutable file", path, in.BaseRef))
				}
			}
			merged[path] = b
		}
	}
	for path := range ancestor.Bytes {
		if _, exists := in.Head.Bytes[path]; !exists {
			delete(merged, path)
		}
	}
	// policy is three-way: the branch changed it only if it differs from what
	// the branch started with; otherwise the merge keeps the base's policy,
	// whatever the base did to it since
	ancestorPolicy, basePolicy, headPolicy := string(ancestor.Bytes[store.PolicyFile]), string(in.Base.Bytes[store.PolicyFile]), string(in.Head.Bytes[store.PolicyFile])
	branchChangedPolicy := headPolicy != ancestorPolicy
	if !branchChangedPolicy {
		delete(merged, store.PolicyFile)
		if b, ok := in.Base.Bytes[store.PolicyFile]; ok {
			merged[store.PolicyFile] = b
		}
	}
	sort.Strings(p.Modified)
	sort.Strings(p.Deleted)
	sort.Strings(addCollisions)
	union, _ := store.Load(context.Background(), merged) // MapSource never fails; problems are reported on the set
	p.Union = resolve.New(union)
	p.BaseView = resolve.New(in.Base)
	ancestorView := resolve.New(ancestor)
	addedIDs := map[string]bool{}
	for _, id := range in.Head.IDs() {
		_, onAncestor := ancestor.Records[id]
		_, onBase := in.Base.Records[id]
		if !onAncestor && (!onBase || !sameRecordBytes(id, in.Head, in.Base)) {
			p.Added = append(p.Added, in.Head.Records[id])
			addedIDs[id] = true
			if onBase && !addCollisionPaths[in.Head.Paths[id]] {
				addCollisions = append(addCollisions, fmt.Sprintf("%s was independently added on the branch and %s with different bytes (%s vs %s); rebase and keep one immutable record", id, in.BaseRef, in.Head.Paths[id], in.Base.Paths[id]))
			}
		}
	}
	for _, path := range p.Modified {
		p.Blockers = append(p.Blockers, Finding{"record-modified", path + " changed after it was stored; records are immutable — run pyxis draft reopen <feature> for a locked feature or use the owning workflow to add a superseding record", "author"})
	}
	var retired map[string]bool
	if len(p.Deleted) > 0 {
		retired = BookkeepingRetirements(ancestor, in.Base, in.Head, union)
	}
	for _, path := range p.Deleted {
		if retired[path] || retiredSessionLog(path, ancestor, in.Base, in.Head, union) {
			continue
		}
		p.Blockers = append(p.Blockers, Finding{"record-deleted", path + " was deleted; the memory is append-only — revoke it with a Revocation record instead", "author"})
	}
	for _, collision := range addCollisions {
		p.Blockers = append(p.Blockers, Finding{"record-add-collision", collision, "author"})
	}
	for _, r := range p.Added {
		if err := capture.CheckNew(r); err != nil {
			p.Blockers = append(p.Blockers, Finding{"derived-memory", err.Error(), "author"})
		}
	}
	for _, pr := range in.Head.Problems {
		p.Blockers = append(p.Blockers, Finding{"record-invalid", pr.Path + ": " + pr.Err.Error(), "author"})
	}
	for _, d := range p.Union.Set.Dangling {
		finding := Finding{"dangling-target", fmt.Sprintf("%s %s %s, which does not exist", d.From, d.Type, d.Target), "author"}
		if danglingCovered(in.Base.Dangling, d) {
			finding.Code = "dangling-target-on-base"
			finding.Resolver = "memory owner"
			p.Warnings = append(p.Warnings, finding)
		} else {
			p.Blockers = append(p.Blockers, finding)
		}
	}
	for _, c := range p.Union.AnswerConflicts() {
		if !answerConflictCovered(p.BaseView.AnswerConflicts(), c) {
			p.Blockers = append(p.Blockers, Finding{"answer-conflict", fmt.Sprintf("%s would have %d active answers (%s); a correction supersedes the answer it replaces (memory answer --replace)",
				c.Question, len(c.IDs), strings.Join(c.IDs, ", ")), "author"})
		} else {
			p.Warnings = append(p.Warnings, Finding{"answer-conflict-on-base", fmt.Sprintf("%s remains with %d active answers after the merge; the base defect was not worsened", c.Question, len(c.IDs)), "owner of " + p.Union.Set.Records[c.Question].Subject})
		}
	}
	for _, c := range p.Union.RevocationConflicts() {
		if !revocationConflictCovered(p.BaseView.RevocationConflicts(), c) {
			p.Blockers = append(p.Blockers, Finding{"revocation-conflict", fmt.Sprintf("%s would have %d active revocations (%s); revoke the redundant act so exactly one withdrawal remains",
				c.Target, len(c.IDs), strings.Join(c.IDs, ", ")), "author"})
		} else {
			target := p.Union.Set.Records[c.Target]
			owner := "memory owner"
			if target != nil {
				owner = "owner of " + target.Subject
			}
			p.Warnings = append(p.Warnings, Finding{"revocation-conflict-on-base", fmt.Sprintf("%s remains with %d active revocations after the merge; the base defect was not worsened", c.Target, len(c.IDs)), owner})
		}
	}
	for _, v := range p.Union.Violations {
		if v.Code == "supersedes-revoked-record" && addedIDs[v.Record] && p.Union.Status(v.Record) == resolve.Active && p.BaseView.Status(v.Target) == resolve.Revoked {
			// The moved-set pass below reports this stale candidate in the
			// branch author's terms. Do not duplicate it as a projection error.
			continue
		}
		if v.Code == "revokes-inactive-record" && !addedIDs[v.Record] && p.BaseView.Status(v.Target) == resolve.Revoked && activeAddedReplaces(v.Target, p.Added, addedIDs, in.Base, p.Union) {
			// The base's revocation was valid before this branch's stale
			// superseder was overlaid. Attribute the merge defect to the moved
			// candidate below even when skewed clocks make its ULID sort first.
			continue
		}
		if !violationCovered(p.BaseView.Violations, v) {
			p.Blockers = append(p.Blockers, Finding{v.Code, v.What, "author"})
		} else {
			owner := "memory owner"
			if carrier := p.Union.Set.Records[v.Record]; carrier != nil {
				owner = "owner of " + carrier.Subject
			}
			p.Warnings = append(p.Warnings, Finding{v.Code + "-on-base", v.What, owner})
		}
	}
	for _, violation := range resolve.HistoricalTransitionViolations(p.BaseView, p.Union) {
		p.Blockers = append(p.Blockers, Finding{violation.Code, violation.What, "author"})
	}
	// every document that would be active after the merge must still resolve:
	// a proposal that revokes or renames a claim an existing document embeds
	// breaks that document just as surely as a new document with a bad marker
	for _, sub := range p.Union.Set.Subjects() {
		for _, d := range p.Union.Active(sub) {
			if d.Kind != record.Document {
				continue
			}
			for _, ref := range d.Embeds {
				i := strings.IndexByte(ref, '#')
				if p.Union.Head(ref[:i], ref[i+1:]) != nil {
					continue
				}
				switch {
				case addedIDs[d.ID]:
					p.Blockers = append(p.Blockers, Finding{"embed-unresolved", fmt.Sprintf("%s embeds {{%s}}, which has no active head after the merge", d.ID, ref), "author"})
				case p.BaseView.Status(d.ID) == resolve.Active && p.BaseView.Head(ref[:i], ref[i+1:]) == nil:
					if hasAncestor && (ancestorView.Status(d.ID) != resolve.Active || ancestorView.Head(ref[:i], ref[i+1:]) != nil) {
						// The base became broken after this branch was cut. A stale
						// branch must not hide that regression by resurrecting the old
						// target in its working tree.
						p.Blockers = append(p.Blockers, Finding{"embed-unresolved", fmt.Sprintf("the merged memory leaves %s (%s#%s) active while it embeds {{%s}}, which lost its active head on %s after this branch was cut; rebase and resolve the document",
							d.ID, d.Subject, d.Family, ref, in.BaseRef), "author"})
					} else {
						// The same document was active and unresolved before this
						// proposal: the owner's problem, not this change's.
						p.Warnings = append(p.Warnings, Finding{"embed-unresolved-on-base", fmt.Sprintf("%s (%s#%s) embeds {{%s}}, which already has no active head on %s", d.ID, d.Subject, d.Family, ref, in.BaseRef),
							"owner of " + d.Subject})
					}
				default:
					// this change either broke the marker or made a broken document active again (say, by revoking its successor)
					p.Blockers = append(p.Blockers, Finding{"embed-unresolved", fmt.Sprintf("this change leaves %s (%s#%s) active while it embeds {{%s}}, which would have no active head; revise the document in the same proposal",
						d.ID, d.Subject, d.Family, ref), "author"})
				}
			}
		}
	}
	// what the branch replaces on the base: every active added record is traced
	// back through the records the branch itself adds (B supersedes A, C
	// supersedes B) to the base records at the end of the chain, so a moved
	// head is caught however many hops the branch put in front of it
	superseded := map[string]*record.Record{} // base target → the active added record that replaces it
	forkedSupersessions := map[string][]*record.Record{}
	for _, r := range p.Added {
		if p.Union.Status(r.ID) != resolve.Active {
			continue // revoked or superseded within the branch itself: its supersession is void
		}
		targets := baseTargets(r, addedIDs, in.Base, p.Union.Set)
		for _, target := range targets {
			// An explicit reconciliation may retain a branch's old ancestry
			// while also replacing the current base head. That older target is
			// already covered by the current head; it is not a stale proposal.
			// Revoked targets, invalid chains, and missing current-head edges
			// still follow the ordinary moved-set refusal below.
			current := p.BaseView.Head(target.Subject, target.Family)
			if p.BaseView.Status(target.ID) == resolve.Superseded && current != nil &&
				!invalidChain(r, target, p.Union) && !invalidChain(r, current, p.Union) &&
				slices.ContainsFunc(targets, func(candidate *record.Record) bool { return candidate.ID == current.ID }) {
				continue
			}
			superseded[target.ID] = r
			p.Superseded = append(p.Superseded, Superseded{Target: target, By: r})
			chainInvalid := invalidChain(r, target, p.Union)
			if !chainInvalid && p.BaseView.Status(target.ID) == resolve.Active && validActiveClaimForFork(r, p.Union) {
				forkedSupersessions[target.ID] = append(forkedSupersessions[target.ID], r)
			}
			if chainInvalid {
				continue // already a blocker; do not also read it as a move
			}
			if st := p.BaseView.Status(target.ID); st != resolve.Active {
				m := Moved{By: r, Target: target.ID, Status: st, CurrentHead: p.BaseView.Head(target.Subject, target.Family)}
				p.Moved = append(p.Moved, m)
				head := "the family has no active head"
				if m.CurrentHead != nil {
					head = "the head is now " + m.CurrentHead.ID + " (" + clip(m.CurrentHead.Statement, 100) + ")"
				}
				p.Blockers = append(p.Blockers, Finding{"effective-set-moved", fmt.Sprintf("%s replaces %s, which is %s on %s; %s — rebase the claim on the current head",
					r.ID, target.ID, st, in.BaseRef, head), "author"})
			}
		}
	}
	var forkTargets []string
	for target, candidates := range forkedSupersessions {
		if len(candidates) > 1 {
			forkTargets = append(forkTargets, target)
		}
	}
	sort.Strings(forkTargets)
	for _, target := range forkTargets {
		candidates := forkedSupersessions[target]
		ids := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			ids = append(ids, candidate.ID)
		}
		sort.Strings(ids)
		p.Blockers = append(p.Blockers, Finding{
			"supersession-fork",
			fmt.Sprintf("%s would be superseded by multiple active candidate claims (%s); exactly one active successor may replace a current target", target, strings.Join(ids, ", ")),
			"author",
		})
	}
	for _, c := range p.Union.Conflicts() {
		if !familyConflictCovered(p.BaseView.Conflicts(), c) {
			p.Conflicts = append(p.Conflicts, c)
			p.Blockers = append(p.Blockers, Finding{"conflict-at-admission", fmt.Sprintf("%s#%s would have %d active claims after the merge (%s); the new claim must name what it supersedes",
				c.Subject, c.Family, len(c.IDs), strings.Join(c.IDs, ", ")), "author"})
		} else {
			p.Warnings = append(p.Warnings, Finding{"conflict-on-base", fmt.Sprintf("%s#%s remains with %d active claims after the merge; the base defect was not worsened", c.Subject, c.Family, len(c.IDs)), "owner of " + c.Subject})
		}
	}
	for _, re := range p.Union.Reopened(nil) {
		if addedIDs[re.Record.ID] || superseded[re.Serves] == nil {
			continue
		}
		p.Reopens = append(p.Reopens, re)
		p.Warnings = append(p.Warnings, Finding{"reopens", fmt.Sprintf("%s (%s#%s) serves %s, which this change supersedes; carry a superseding decision that re-points it, or accept that the design reopens",
			re.Record.ID, re.Record.Subject, re.Record.Family, re.Serves), "author or owner of " + re.Record.Subject})
	}
	for _, r := range p.Added {
		if (r.Kind == record.Promise || r.Kind == record.Expectation) && len(r.Rel(record.Supersedes)) == 0 {
			for _, u := range p.Union.Undesigned(r.Subject) {
				if u.ID == r.ID {
					p.Undesigned = append(p.Undesigned, r)
					p.Warnings = append(p.Warnings, Finding{"undesigned", fmt.Sprintf("no decision serves %s#%s (%s); it will read as undesigned until one does", r.Subject, r.Family, r.ID), "engineering"})
				}
			}
		}
	}
	branches := make([]string, 0, len(in.InFlight))
	for b := range in.InFlight {
		branches = append(branches, b)
	}
	sort.Strings(branches)
	for _, b := range branches {
		other := in.InFlight[b]
		otherView := resolve.New(other)
		for _, oid := range other.IDs() {
			o := other.Records[oid]
			if otherView.Status(o.ID) != resolve.Active {
				continue
			}
			for _, r := range p.Added {
				if p.Union.Status(r.ID) != resolve.Active {
					continue
				}
				if o.ID == r.ID {
					continue
				}
				switch {
				case record.IsClaim(r.Kind) && o.Subject == r.Subject && o.Family == r.Family:
					c := Concurrent{Branch: b, Ours: r, Theirs: o, What: fmt.Sprintf("%s also changes %s#%s", b, r.Subject, r.Family)}
					p.Concurrent = append(p.Concurrent, c)
					p.Warnings = append(p.Warnings, Finding{"concurrent", c.What + " (" + o.ID + "); whichever merges second must rebase on the other's head", "both authors"})
				case sharesTarget(o, r):
					c := Concurrent{Branch: b, Ours: r, Theirs: o, What: fmt.Sprintf("%s also supersedes what %s supersedes", b, r.ID)}
					p.Concurrent = append(p.Concurrent, c)
					p.Warnings = append(p.Warnings, Finding{"concurrent", c.What + " (" + o.ID + ")", "both authors"})
				}
			}
		}
	}
	// consequence is judged under the base policy and under the candidate's,
	// and the stricter verdict stands: a candidate cannot lower its own bar
	p.Consequence = policy.Max(consequence(in.BasePolicy, p.Added), consequence(in.Policy, p.Added))
	if len(retired) > 0 {
		p.Consequence = record.High
		p.Warnings = append(p.Warnings, Finding{"bookkeeping-retirement", fmt.Sprintf("%d exact records retire under the operator's bounded bookkeeping cleanup; retained references and historical status are unchanged", len(retired)), "operator"})
	}
	if branchChangedPolicy {
		p.PolicyChanged = true
		p.Consequence = record.High
		what := "policy.json changes in this proposal; a policy change is always high consequence and is judged under the base policy"
		if headPolicy == "" {
			what = "policy.json is deleted in this proposal; a policy change is always high consequence and is judged under the base policy"
		}
		p.Warnings = append(p.Warnings, Finding{"policy-changed", what, "operator"})
		if basePolicy != ancestorPolicy {
			p.Warnings = append(p.Warnings, Finding{"policy-conflict", "policy.json changed on " + in.BaseRef + " since this branch was cut, and this branch changes it too; rebase and reconcile the two", "author"})
		}
	}
	return p
}

func sameRecordBytes(id string, a, b *store.Set) bool {
	ap, bp := a.Paths[id], b.Paths[id]
	if ap == "" || ap != bp || !sameTextBytes(a.Bytes[ap], b.Bytes[bp]) {
		return false
	}
	ar, br := a.Records[id], b.Records[id]
	if ar.Kind != record.Document || br.Kind != record.Document {
		return true
	}
	abp, bbp := ar.BodyPath(), br.BodyPath()
	return abp == bbp && sameTextBytes(a.Bytes[abp], b.Bytes[bbp])
}

func sameTextBytes(a, b []byte) bool {
	return bytes.Equal(record.NormalizeBody(a), record.NormalizeBody(b))
}

func consequence(pol policy.Policy, added []*record.Record) string {
	out := record.Low
	claims := 0
	for _, r := range added {
		out = policy.Max(out, pol.ForRecord(r))
		if record.IsClaim(r.Kind) {
			claims++
		}
	}
	if pol.Consequence.ManyClaims > 0 && claims >= pol.Consequence.ManyClaims {
		out = policy.Raise(out)
	}
	return out
}

// baseTargets follows supersedes edges from r through records the branch adds
// until it reaches records that exist on the base: those are what r replaces
// as far as the base is concerned.
func baseTargets(r *record.Record, added map[string]bool, base, union *store.Set) []*record.Record {
	var out []*record.Record
	seen := map[string]bool{r.ID: true}
	var walk func(id string)
	walk = func(id string) {
		cur := union.Records[id]
		if cur == nil {
			return
		}
		for _, t := range cur.Rel(record.Supersedes) {
			if seen[t] {
				continue
			}
			seen[t] = true
			if b := base.Records[t]; b != nil {
				out = append(out, b)
			} else if added[t] {
				walk(t)
			}
		}
	}
	walk(r.ID)
	return out
}

// invalidChain reports whether a supersedes hop between r and target is a
// violation the projection already refuses (and the gate already blocks).
func invalidChain(r, target *record.Record, union *resolve.Projection) bool {
	chain := supersessionPath(r.ID, target.ID, union.Set)
	delete(chain, target.ID) // defects already carried by the base target are not candidate-chain defects
	for _, v := range union.Violations {
		touches := chain[v.Record]
		for _, id := range v.Related {
			touches = touches || chain[id]
		}
		if !touches {
			continue
		}
		if v.Code == "supersedes-revoked-record" && v.Record == r.ID {
			continue
		}
		if v.Code == "revokes-inactive-record" && stringIn(v.Related, r.ID) {
			continue
		}
		if (v.Code == "supersedes-revoked-record" || (v.Code == "revokes-inactive-record" && !chain[v.Record])) && v.Target == target.ID {
			continue
		}
		return true
	}
	return false
}

func danglingCovered(prior []store.Dangling, current store.Dangling) bool {
	for _, candidate := range prior {
		if candidate.From == current.From && candidate.Type == current.Type && candidate.Target == current.Target {
			return true
		}
	}
	return false
}

func stringIn(values []string, want string) bool {
	return slices.Contains(values, want)
}

// supersessionPath returns the records on any supersedes path from start to
// target. invalidChain removes the terminal base target: only candidate-side
// carriers can invalidate the path. A defect already carried by target, or by
// some unrelated record that happens to point at it, must not hide a moved-head
// blocker for this candidate chain.
func supersessionPath(start, target string, set *store.Set) map[string]bool {
	path := map[string]bool{}
	done := map[string]bool{}
	reaches := map[string]bool{}
	visiting := map[string]bool{}
	var walk func(string) bool
	walk = func(id string) bool {
		if id == target {
			path[id] = true
			return true
		}
		if done[id] {
			return reaches[id]
		}
		if visiting[id] {
			return false
		}
		visiting[id] = true
		r := set.Records[id]
		if r != nil {
			for _, next := range r.Rel(record.Supersedes) {
				if walk(next) {
					reaches[id] = true
					path[id] = true
				}
			}
		}
		delete(visiting, id)
		done[id] = true
		return reaches[id]
	}
	walk(start)
	return path
}

func validActiveClaimForFork(r *record.Record, projection *resolve.Projection) bool {
	if !record.IsClaim(r.Kind) || projection.Status(r.ID) != resolve.Active {
		return false
	}
	for _, violation := range projection.Violations {
		if violation.Record == r.ID {
			return false
		}
	}
	for _, dangling := range projection.Set.Dangling {
		if dangling.From == r.ID {
			return false
		}
	}
	for _, conflict := range projection.Conflicts() {
		if stringIn(conflict.IDs, r.ID) {
			return false
		}
	}
	if r.Kind == record.Document {
		for _, ref := range r.Embeds {
			i := strings.IndexByte(ref, '#')
			if i <= 0 || projection.Head(ref[:i], ref[i+1:]) == nil {
				return false
			}
		}
	}
	return true
}

func activeAddedReplaces(target string, added []*record.Record, addedIDs map[string]bool, base *store.Set, projection *resolve.Projection) bool {
	for _, r := range added {
		if projection.Status(r.ID) != resolve.Active {
			continue
		}
		for _, candidate := range baseTargets(r, addedIDs, base, projection.Set) {
			if candidate.ID == target {
				return true
			}
		}
	}
	return false
}

func violationCovered(prior []resolve.Violation, current resolve.Violation) bool {
	for _, candidate := range prior {
		if candidate.Code == current.Code && candidate.Record == current.Record && candidate.Target == current.Target && stringSetSubset(current.Related, candidate.Related) {
			return true
		}
	}
	return false
}

func familyConflictCovered(prior []resolve.Conflict, current resolve.Conflict) bool {
	for _, candidate := range prior {
		if candidate.Subject == current.Subject && candidate.Family == current.Family && stringSetSubset(current.IDs, candidate.IDs) {
			return true
		}
	}
	return false
}

func answerConflictCovered(prior []resolve.AnswerConflict, current resolve.AnswerConflict) bool {
	for _, candidate := range prior {
		if candidate.Question == current.Question && stringSetSubset(current.IDs, candidate.IDs) {
			return true
		}
	}
	return false
}

func revocationConflictCovered(prior []resolve.RevocationConflict, current resolve.RevocationConflict) bool {
	for _, candidate := range prior {
		if candidate.Target == current.Target && stringSetSubset(current.IDs, candidate.IDs) {
			return true
		}
	}
	return false
}

func stringSetSubset(candidate, prior []string) bool {
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

func sharesTarget(a, b *record.Record) bool {
	for _, x := range a.Rel(record.Supersedes) {
		if slices.Contains(b.Rel(record.Supersedes), x) {
			return true
		}
	}
	return false
}

// OK reports whether the merge check passes.
func (p *Packet) OK() bool { return len(p.Blockers) == 0 }

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// Render writes the packet as the pull request body: what the product side
// signs, what the engineering side signs, and what the memory computed.
func (p *Packet) Render() string {
	var b strings.Builder
	title := p.Description
	if title == "" {
		title = "Memory change on " + p.Branch
	}
	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "branch `%s` against `%s` · consequence **%s** · %d records added\n\n", p.Branch, p.BaseRef, p.Consequence, len(p.Added))
	fmt.Fprintf(&b, "> Generated by `memory packet` from the records this branch adds. Nothing below is hand-written except the records' own intent and rationale.\n\n")
	if p.PolicyChanged {
		b.WriteString("## Policy change — operator sign-off\n\nThis proposal changes `memory/policy.json`. Its consequence is high regardless of what the new policy says, and it was judged under the base policy.\n\n")
	}
	if len(p.Blockers) > 0 {
		b.WriteString("## Blockers — this cannot merge as it stands\n\n")
		for _, f := range p.Blockers {
			fmt.Fprintf(&b, "- **%s**: %s _(resolver: %s)_\n", f.Code, f.What, f.Resolver)
		}
		b.WriteString("\n")
	}
	product := filter(p.Added, record.Need, record.Promise, record.Expectation)
	engineering := filter(p.Added, record.Decision, record.Plan)
	documents := filter(p.Added, record.Document)
	rest := filter(p.Added, record.Learning, record.Assumption, record.Policy, record.Question, record.Answer, record.Revocation)
	b.WriteString("## Product sign-off — the promise delta\n\nWhat the product commits to differently, why, and what that makes stale.\n\n")
	if len(product) == 0 {
		b.WriteString("- no product claims change\n\n")
	}
	for _, r := range product {
		p.renderClaim(&b, r)
	}
	b.WriteString("## Engineering sign-off — the design delta\n\nWhich decisions change and which claims they serve.\n\n")
	if len(engineering) == 0 {
		b.WriteString("- no decisions or plans change\n\n")
	}
	for _, r := range engineering {
		p.renderClaim(&b, r)
	}
	if len(documents) > 0 {
		b.WriteString("## Narrative delta — the documents\n\n")
		for _, r := range documents {
			p.renderDocument(&b, r)
		}
	}
	if len(rest) > 0 {
		b.WriteString("## Also carried\n\n")
		for _, r := range rest {
			fmt.Fprintf(&b, "- **%s** on %s: %s", r.Kind, r.Subject, clip(r.Statement, 220))
			if r.Kind == record.Learning && len(r.LearnedFrom) > 0 {
				fmt.Fprintf(&b, " · learned from %s", strings.Join(r.LearnedFrom, ", "))
			}
			for _, t := range r.Rel(record.Revokes) {
				fmt.Fprintf(&b, " · revokes %s", t)
			}
			for _, t := range r.Rel(record.Answers) {
				fmt.Fprintf(&b, " · answers %s", t)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("## What it supersedes\n\n")
	if len(p.Superseded) == 0 {
		b.WriteString("- nothing\n")
	}
	for _, s := range p.Superseded {
		fmt.Fprintf(&b, "- %s#%s since %s: %s → %s\n", s.Target.Subject, s.Target.Family, s.Target.CreatedAt[:10], clip(s.Target.Statement, 120), s.By.ID)
	}
	b.WriteString("\n## What it reopens\n\n")
	if len(p.Reopens) == 0 {
		b.WriteString("- nothing: every decision serving a moved claim rides this change\n")
	}
	for _, re := range p.Reopens {
		fmt.Fprintf(&b, "- %s#%s serves %s, which this change supersedes\n", re.Record.Subject, re.Record.Family, re.Serves)
	}
	b.WriteString("\n## Undesigned after this change\n\n")
	if len(p.Undesigned) == 0 {
		b.WriteString("- nothing: every new promise is served by a decision\n")
	}
	for _, r := range p.Undesigned {
		fmt.Fprintf(&b, "- %s#%s\n", r.Subject, r.Family)
	}
	b.WriteString("\n## Concurrent work\n\n")
	if len(p.Concurrent) == 0 {
		b.WriteString("- no other branch touches these families\n")
	}
	for _, c := range p.Concurrent {
		fmt.Fprintf(&b, "- %s (%s)\n", c.What, c.Theirs.ID)
	}
	var other []Finding
	for _, w := range p.Warnings {
		if strings.HasSuffix(w.Code, "-on-base") {
			other = append(other, w)
		}
	}
	if len(other) > 0 {
		b.WriteString("\n## Already wrong on the base\n\n")
		for _, w := range other {
			fmt.Fprintf(&b, "- %s _(resolver: %s)_\n", w.What, w.Resolver)
		}
	}
	return b.String()
}

func (p *Packet) renderClaim(b *strings.Builder, r *record.Record) {
	head := fmt.Sprintf("### %s %s on %s", r.Kind, r.Family, r.Subject)
	if r.Family == "" {
		head = fmt.Sprintf("### %s on %s", r.Kind, r.Subject)
	}
	sup := r.Rel(record.Supersedes)
	switch {
	case len(sup) == 0:
		head += " — new"
	default:
		head += " — replaces the current " + strings.ToLower(r.Kind)
		for _, t := range sup {
			if old := p.Union.Set.Records[t]; old != nil && old.Family != r.Family {
				head += fmt.Sprintf(" (renamed from %s)", old.Family)
			}
		}
	}
	fmt.Fprintf(b, "%s\n", head)
	for _, t := range sup {
		old := p.Union.Set.Records[t]
		if old == nil {
			fmt.Fprintf(b, "- **before**: %s (not found)\n", t)
			continue
		}
		fmt.Fprintf(b, "- **before** (since %s): %s\n", old.CreatedAt[:10], clip(old.Statement, 300))
	}
	fmt.Fprintf(b, "- **after**: %s\n", clip(r.Statement, 400))
	if r.Intent != "" {
		fmt.Fprintf(b, "- **intent**: %s\n", clip(r.Intent, 300))
	}
	if r.Rationale != "" {
		fmt.Fprintf(b, "- **why**: %s\n", clip(r.Rationale, 300))
	}
	if r.FitCriterion != "" {
		fmt.Fprintf(b, "- **fit criterion**: %s\n", clip(r.FitCriterion, 300))
	}
	if len(r.LearnedFrom) > 0 {
		fmt.Fprintf(b, "- **learned from**: %s\n", strings.Join(r.LearnedFrom, ", "))
	}
	if len(r.Alternatives) > 0 {
		fmt.Fprintf(b, "- **alternatives rejected**: %s\n", strings.Join(r.Alternatives, "; "))
	}
	if serves := r.Rel(record.Serves); len(serves) > 0 {
		var names []string
		for _, t := range serves {
			if s := p.Union.Set.Records[t]; s != nil {
				names = append(names, s.Subject+"#"+s.Family)
			} else {
				names = append(names, t)
			}
		}
		fmt.Fprintf(b, "- **serves**: %s\n", strings.Join(names, ", "))
	}
	if about := r.Rel(record.About); len(about) > 0 {
		var names []string
		for _, t := range about {
			if s := p.Union.Set.Records[t]; s != nil {
				names = append(names, s.Subject)
			} else {
				names = append(names, t)
			}
		}
		fmt.Fprintf(b, "- **about**: %s\n", strings.Join(names, ", "))
	}
	if len(r.Tags) > 0 {
		fmt.Fprintf(b, "- **tags**: %s\n", strings.Join(r.Tags, ", "))
	}
	b.WriteString("\n")
}

// renderDocument shows a document's version, what it embeds, and the line
// diff of its narrative against the version it replaces.
func (p *Packet) renderDocument(b *strings.Builder, r *record.Record) {
	version := len(p.Union.Chain(r.ID, 1000))
	fmt.Fprintf(b, "### %s · %s \"%s\" — version %d\n", r.Subject, r.Family, clip(r.Statement, 120), version)
	if r.Intent != "" {
		fmt.Fprintf(b, "- **intent**: %s\n", clip(r.Intent, 300))
	}
	if r.Rationale != "" {
		fmt.Fprintf(b, "- **why**: %s\n", clip(r.Rationale, 300))
	}
	if r.LiteralBody {
		b.WriteString("- **mode**: literal source body (brace-shaped text is not interpreted as claim embeds)\n")
	}
	if len(r.Embeds) > 0 {
		fmt.Fprintf(b, "- **embeds**: %s\n", strings.Join(r.Embeds, ", "))
	}
	body := string(p.Union.Set.Bodies[r.ID])
	var previous string
	sup := r.Rel(record.Supersedes)
	if len(sup) > 0 {
		previous = string(p.Union.Set.Bodies[sup[0]]) // the first predecessor is the one the version history follows
	}
	if len(sup) > 1 {
		fmt.Fprintf(b, "- **also replaces**: %s (reconciling more than one lineage; the diff below is against %s)\n", strings.Join(sup[1:], ", "), sup[0])
	}
	lines := documentLines(body)
	if len(sup) == 0 {
		diff := lineDiff(nil, lines, 0)
		fence := fenceFor(diff)
		fmt.Fprintf(b, "- **new document**: %d lines\n\n%sdiff\n%s\n%s\n\n", len(lines), fence, strings.Join(diff, "\n"), fence)
		return
	}
	diff := lineDiff(documentLines(previous), lines, 2)
	added, removed := 0, 0
	for _, l := range diff {
		switch {
		case strings.HasPrefix(l, "+"):
			added++
		case strings.HasPrefix(l, "-"):
			removed++
		}
	}
	fence := fenceFor(diff)
	fmt.Fprintf(b, "- **changed**: %d lines added, %d removed\n\n%sdiff\n%s\n%s\n\n", added, removed, fence, strings.Join(diff, "\n"), fence)
}

// documentLines treats an empty body as zero lines and removes at most the
// final line delimiter. Additional trailing blank lines remain part of the
// narrative and therefore part of its diff.
func documentLines(body string) []string {
	if body == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(body, "\n"), "\n")
}

// fenceFor picks a code fence longer than any run of backticks in the lines,
// so a fenced example inside a document cannot close the packet's diff.
func fenceFor(lines []string) string {
	longest := 0
	for _, l := range lines {
		run := 0
		for _, c := range l {
			if c == '`' {
				run++
				if run > longest {
					longest = run
				}
			} else {
				run = 0
			}
		}
	}
	if longest < 3 {
		longest = 2
	}
	return strings.Repeat("`", longest+1)
}

type lineOp struct {
	kind byte
	text string
}

// maxLineDiffTraceInts caps the Myers history at roughly 8 MiB on a 64-bit
// machine. Documents whose edit path would exceed it get a linear coarse
// diff instead of growing memory with the product of both line counts.
const maxLineDiffTraceInts = 1 << 20

// lineDiff renders a bounded Myers diff over lines with +, - and space
// prefixes and the given lines of context around changes.
func lineDiff(a, b []string, context int) []string {
	ops := diffOperations(a, b, maxLineDiffTraceInts)
	keep := make([]bool, len(ops))
	for k, o := range ops {
		if o.kind == ' ' {
			continue
		}
		for c := k - context; c <= k+context; c++ {
			if c >= 0 && c < len(ops) {
				keep[c] = true
			}
		}
	}
	var out []string
	gap := false
	for k, o := range ops {
		if !keep[k] {
			gap = true
			continue
		}
		if gap && len(out) > 0 {
			out = append(out, "@@ …")
		}
		gap = false
		out = append(out, string(o.kind)+o.text)
	}
	return out
}

// diffOperations trims equal edges before running Myers. That makes the
// common case of a small edit in a large document depend on the size of the
// edit, not on the size of the unchanged narrative.
func diffOperations(a, b []string, traceLimit int) []lineOp {
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(a)-prefix && suffix < len(b)-prefix && a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}

	amid := a[prefix : len(a)-suffix]
	bmid := b[prefix : len(b)-suffix]
	middle, ok := myersLineOperations(amid, bmid, traceLimit)
	if !ok {
		middle = coarseLineOperations(amid, bmid)
	}

	ops := make([]lineOp, 0, prefix+len(middle)+suffix)
	for _, line := range a[:prefix] {
		ops = append(ops, lineOp{' ', line})
	}
	ops = append(ops, middle...)
	for _, line := range a[len(a)-suffix:] {
		ops = append(ops, lineOp{' ', line})
	}
	return preferEarlierMatches(deletionsFirst(ops))
}

// myersLineOperations returns a shortest edit script. The trace contains a
// frontier snapshot for each edit depth so the path can be reconstructed;
// the caller-supplied integer budget bounds those snapshots.
func myersLineOperations(a, b []string, traceLimit int) ([]lineOp, bool) {
	n, m := len(a), len(b)
	max := n + m
	size := 2*max + 3
	if traceLimit < size {
		return nil, false
	}
	offset := max + 1
	frontier := make([]int, size)
	frontier[offset+1] = 0
	traceCapacity := max + 1
	if budgeted := traceLimit / size; traceCapacity > budgeted {
		traceCapacity = budgeted
	}
	trace := make([][]int, 0, traceCapacity)
	used := 0
	for depth := 0; depth <= max; depth++ {
		if used > traceLimit-size {
			return nil, false
		}
		trace = append(trace, append([]int(nil), frontier...))
		used += size
		for diagonal := -depth; diagonal <= depth; diagonal += 2 {
			at := offset + diagonal
			var x int
			if diagonal == -depth || (diagonal != depth && frontier[at-1] < frontier[at+1]) {
				x = frontier[at+1]
			} else {
				x = frontier[at-1] + 1
			}
			y := x - diagonal
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			frontier[at] = x
			if x >= n && y >= m {
				return backtrackLineOperations(a, b, trace, depth, offset), true
			}
		}
	}
	return nil, false
}

func backtrackLineOperations(a, b []string, trace [][]int, depth, offset int) []lineOp {
	x, y := len(a), len(b)
	reversed := make([]lineOp, 0, x+y)
	for d := depth; d > 0; d-- {
		frontier := trace[d]
		diagonal := x - y
		previous := diagonal - 1
		if diagonal == -d || (diagonal != d && frontier[offset+diagonal-1] < frontier[offset+diagonal+1]) {
			previous = diagonal + 1
		}
		previousX := frontier[offset+previous]
		previousY := previousX - previous
		for x > previousX && y > previousY {
			reversed = append(reversed, lineOp{' ', a[x-1]})
			x--
			y--
		}
		if x == previousX {
			reversed = append(reversed, lineOp{'+', b[y-1]})
			y--
		} else {
			reversed = append(reversed, lineOp{'-', a[x-1]})
			x--
		}
	}
	for x > 0 && y > 0 {
		reversed = append(reversed, lineOp{' ', a[x-1]})
		x--
		y--
	}
	for x > 0 {
		reversed = append(reversed, lineOp{'-', a[x-1]})
		x--
	}
	for y > 0 {
		reversed = append(reversed, lineOp{'+', b[y-1]})
		y--
	}
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return reversed
}

// coarseLineOperations is the bounded fallback. Equal edges were already
// removed, so replacing the remaining middle is linear in its output size.
func coarseLineOperations(a, b []string) []lineOp {
	ops := make([]lineOp, 0, len(a)+len(b))
	for _, line := range a {
		ops = append(ops, lineOp{'-', line})
	}
	for _, line := range b {
		ops = append(ops, lineOp{'+', line})
	}
	return ops
}

// deletionsFirst keeps replacement blocks stable and familiar even when an
// equally short Myers path happens to discover their insertions first.
func deletionsFirst(ops []lineOp) []lineOp {
	out := make([]lineOp, 0, len(ops))
	for start := 0; start < len(ops); {
		if ops[start].kind == ' ' {
			out = append(out, ops[start])
			start++
			continue
		}
		end := start
		for end < len(ops) && ops[end].kind != ' ' {
			end++
		}
		for _, kind := range []byte{'-', '+'} {
			for _, op := range ops[start:end] {
				if op.kind == kind {
					out = append(out, op)
				}
			}
		}
		start = end
	}
	return out
}

// preferEarlierMatches shifts an equal line left across an adjacent run of
// identical insertions or deletions. This keeps repeated blank lines anchored
// where a forward LCS scan would first encounter them.
func preferEarlierMatches(ops []lineOp) []lineOp {
	for start := 0; start < len(ops); {
		if ops[start].kind == ' ' {
			start++
			continue
		}
		end := start
		for end < len(ops) && ops[end].kind != ' ' {
			end++
		}
		if end == len(ops) {
			break
		}

		firstAddition := start
		for firstAddition < end && ops[firstAddition].kind == '-' {
			firstAddition++
		}
		switch {
		case firstAddition < end && ops[firstAddition].text == ops[end].text:
			ops[firstAddition].kind = ' '
			ops[end].kind = '+'
			start = firstAddition + 1
		case firstAddition == end && ops[start].text == ops[end].text:
			ops[start].kind = ' '
			ops[end].kind = '-'
			start++
		default:
			start = end + 1
		}
	}
	return ops
}

func filter(rs []*record.Record, kinds ...string) []*record.Record {
	var out []*record.Record
	for _, r := range rs {
		for _, k := range kinds {
			if r.Kind == k {
				out = append(out, r)
			}
		}
	}
	return out
}

// Summary is the one-screen form for the terminal.
func (p *Packet) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "proposal on %s against %s: %d records added, consequence %s\n", p.Branch, p.BaseRef, len(p.Added), p.Consequence)
	for _, r := range p.Added {
		fam := r.Family
		if fam == "" {
			fam = "-"
		}
		line := fmt.Sprintf("  + %-11s %-28s %-8s %s", r.Kind, r.Subject, fam, clip(r.Statement, 90))
		if n := len(r.Rel(record.Supersedes)); n > 0 {
			line += fmt.Sprintf(" (supersedes %d)", n)
		}
		b.WriteString(line + "\n")
	}
	if len(p.Blockers) > 0 {
		b.WriteString("blockers:\n")
		for _, f := range p.Blockers {
			fmt.Fprintf(&b, "  ✗ %s: %s\n", f.Code, f.What)
		}
	}
	if len(p.Warnings) > 0 {
		b.WriteString("for the reviewer:\n")
		for _, f := range p.Warnings {
			fmt.Fprintf(&b, "  ! %s: %s\n", f.Code, f.What)
		}
	}
	if p.OK() {
		b.WriteString("check: ok\n")
	} else {
		b.WriteString("check: blocked\n")
	}
	return b.String()
}
