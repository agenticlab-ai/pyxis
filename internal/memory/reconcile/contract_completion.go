package reconcile

import (
	"path"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

const ContractCompletionSchema = "memory-contract-completion/v1"

// ContractCompletion is unfinished design work, never accepted integrity debt.
type ContractCompletion struct {
	Schema   string              `json:"schema_version"`
	Findings []CompletionFinding `json:"findings"`
}

type CompletionFinding struct {
	State   string `json:"state"`
	Scope   string `json:"scope"`
	Feature string `json:"feature"`
	Action  string `json:"action"`
}

// ContractHistory is pinned by the caller at the trusted predecessor. Candidate
// records cannot declare themselves historical. It is not a serialized input.
type ContractHistory struct {
	Snapshot Snapshot
	Set      *store.Set
	// Projection is the projection of Set when the caller already holds it; a
	// projection of any other set is ignored and computed here instead.
	Projection *resolve.Projection
}

// NeedsContractHistory avoids an additional history read when every declared
// target is live. Unknown addresses never qualify for reopened classification.
func NeedsContractHistory(s Snapshot) bool {
	for _, d := range s.Designs {
		if d.Status != "live" {
			continue
		}
		for _, ref := range d.Serves {
			subject, family, ok := memoryPRDRef(ref)
			if ok {
				_, u, found := contractUnit(s, subject, family)
				if found && (u.Status == "retired" || u.Status == "superseded") {
					return true
				}
			}
		}
	}
	return false
}

func classifyContractCompletion(out *Report, s Snapshot, set *store.Set, proj *resolve.Projection) {
	out.Completion = &ContractCompletion{Schema: ContractCompletionSchema, Findings: []CompletionFinding{}}
	captures := currentSourceCaptures(s, set, proj)
	designed, _ := authorizedPRDServes(s, set, proj, liveContractAddresses(s), true)
	completion := map[string]CompletionFinding{}
	add := func(state, scope, feature, action string) {
		completion[state+"\x00"+scope] = CompletionFinding{state, scope, feature, action}
	}
	undesigned := map[string]bool{}
	for _, prd := range s.PRDs {
		if !capturedContract(prd, s, captures) {
			continue
		}
		for _, u := range prd.Units {
			if u.Status != "live" || !u.Mapped || !requiresDesignServe(u) {
				continue
			}
			subject := "prd/" + prd.Feature
			head := proj.Head(subject, u.ID)
			if head == nil || !samePRDSemantics(head, u) || designed[head.ID] {
				continue
			}
			ref := subject + "#" + u.ID
			undesigned[ref] = true
			add("undesigned", ref, prd.Feature, "author and capture a serving design before planning or feature closeout")
		}
	}
	reopened := map[string]bool{}
	historicalEdges := map[string]bool{}
	historicalCoverage := map[string]bool{}
	var historicalProjection *resolve.Projection
	if s.ContractHistory != nil && s.ContractHistory.Set != nil {
		historicalProjection = projectionOf(s.ContractHistory.Set, s.ContractHistory.Projection)
	}
	for _, d := range s.Designs {
		if !frozenCapturedDesign(s, d, set, proj, historicalProjection) {
			continue
		}
		for _, ref := range d.Serves {
			subject, family, ok := memoryPRDRef(ref)
			if !ok {
				continue
			}
			prd, u, found := contractUnit(s, subject, family)
			if !found || !capturedContract(prd, s, captures) {
				continue
			}
			old := retiredContractClaim(u, subject, s, set, proj)
			if old == nil {
				continue
			}
			scope := "design/" + d.Slug + " -> " + ref
			reopened[scope] = true
			add("reopened", scope, prd.Feature, "supersede the frozen design and reconcile its current declared coverage before closeout")
			for _, decision := range proj.Active("design/" + d.Slug) {
				if decision.Kind != record.Decision || !historicalContractRecord(s, decision) {
					continue
				}
				for _, id := range decision.Rel(record.Serves) {
					if id == old.ID {
						historicalEdges[decision.ID+" -> "+id] = true
					}
				}
			}
		}
		if head := proj.Head("design/"+d.Slug, designCoverageFamily); head != nil && historicalContractRecord(s, head) {
			if expected, ok := stagedDesignCoverage(s, d, set, proj); ok && validateDesignCoverageMemory(head, expected) == nil {
				historicalCoverage[head.Subject] = true
			}
		}
	}
	kept := out.Findings[:0]
	for _, f := range out.Findings {
		if (f.Code == "unserved-prd-memory" && undesigned[f.Scope]) ||
			(f.Code == "missing-design-target-memory" && reopened[f.Scope]) ||
			(f.Code == "unauthorized-prd-serves" && historicalEdges[f.Scope]) ||
			(f.Code == "design-coverage-memory-drift" && historicalCoverage[f.Scope]) {
			continue
		}
		kept = append(kept, f)
	}
	out.Findings = kept
	for _, f := range completion {
		out.Completion.Findings = append(out.Completion.Findings, f)
	}
	sort.Slice(out.Completion.Findings, func(i, j int) bool {
		a, b := out.Completion.Findings[i], out.Completion.Findings[j]
		if a.State != b.State {
			return a.State < b.State
		}
		return a.Scope < b.Scope
	})
}

func currentSourceCaptures(s Snapshot, set *store.Set, proj *resolve.Projection) map[string]bool {
	counts := map[string]int{}
	exact := map[string]bool{}
	for _, id := range set.IDs() {
		r := set.Get(id)
		if r.Kind != record.Document || proj.Status(id) != resolve.Active {
			continue
		}
		f, ok := s.Files[r.Provenance.Source]
		if !ok {
			continue
		}
		counts[f.Path]++
		if documentMatchesSource(r, set.Bodies[id], f) {
			exact[f.Path] = true
		}
	}
	for p := range exact {
		exact[p] = counts[p] == 1
	}
	return exact
}

func capturedContract(prd PRD, s Snapshot, captures map[string]bool) bool {
	prefix := path.Dir(prd.Source) + "/"
	root := optionDefaults(s.SourceOptions).PRDs
	seen := false
	for _, name := range s.AuthoredSources {
		if strings.HasPrefix(name, prefix) && isLedgerEntry(name, root) {
			seen = true
			if !captures[name] {
				return false
			}
		}
	}
	return seen
}

func liveContractAddresses(s Snapshot) map[string]bool {
	out := map[string]bool{}
	for _, prd := range s.PRDs {
		for _, u := range prd.Units {
			if u.Mapped && u.Status == "live" {
				out["prd/"+prd.Feature+"#"+u.ID] = true
			}
		}
	}
	return out
}

func contractUnit(s Snapshot, subject, family string) (PRD, PRDUnit, bool) {
	for _, prd := range s.PRDs {
		if subject != "prd/"+prd.Feature {
			continue
		}
		for _, u := range prd.Units {
			if u.ID == family {
				return prd, u, true
			}
		}
	}
	return PRD{}, PRDUnit{}, false
}

func frozenCapturedDesign(s Snapshot, d Design, set *store.Set, proj, historicalProjection *resolve.Projection) bool {
	if d.Status != "live" || s.ContractHistory == nil || s.ContractHistory.Set == nil || exactDesignDocument(s, d, set, proj) == nil {
		return false
	}
	before, ok := s.ContractHistory.Snapshot.Files[d.Source]
	current := s.Files[d.Source]
	if !ok || before.Mode != current.Mode || before.SHA256 != current.SHA256 {
		return false
	}
	base := s.ContractHistory
	for _, historical := range base.Snapshot.Designs {
		if historical.Slug == d.Slug && historical.Status == "live" && historical.SHA256 == d.SHA256 {
			return exactDesignDocument(base.Snapshot, historical, base.Set, historicalProjection) != nil
		}
	}
	return false
}

func historicalContractRecord(s Snapshot, r *record.Record) bool {
	if s.ContractHistory == nil || s.ContractHistory.Set == nil {
		return false
	}
	old := s.ContractHistory.Set.Get(r.ID)
	return old != nil && digestJSON(old) == digestJSON(r)
}

func retiredContractClaim(u PRDUnit, subject string, s Snapshot, set *store.Set, proj *resolve.Projection) *record.Record {
	if !u.Mapped || (u.Status != "retired" && u.Status != "superseded") || proj.Head(subject, u.ID) != nil {
		return nil
	}
	var matched *record.Record
	for _, id := range set.IDs() {
		r := set.Get(id)
		if r.Subject != subject || r.Family != u.ID || !samePRDSemantics(r, u) {
			continue
		}
		valid := u.Status == "retired" && proj.Status(id) == resolve.Revoked
		if u.Status == "superseded" && proj.Status(id) == resolve.Superseded && u.SupersededBy != nil {
			_, next, ok := contractUnit(s, subject, *u.SupersededBy)
			if ok && next.Status == "live" {
				head := proj.Head(subject, next.ID)
				valid = head != nil && samePRDSemantics(head, next) && contractSupersedes(head, id, set, map[string]bool{})
			}
		}
		if valid {
			if matched != nil {
				return nil
			}
			matched = r
		}
	}
	return matched
}

func contractSupersedes(r *record.Record, target string, set *store.Set, seen map[string]bool) bool {
	if r == nil || seen[r.ID] {
		return false
	}
	seen[r.ID] = true
	for _, id := range r.Rel(record.Supersedes) {
		if id == target || contractSupersedes(set.Get(id), target, set, seen) {
			return true
		}
	}
	return false
}

// Reconstruct a historical header using its exact retired claims, while all
// live references still have to target today's exact heads. The caller admits
// this only for a byte-identical Decision already present at the trusted base.
func stagedDesignCoverage(s Snapshot, d Design, set *store.Set, proj *resolve.Projection) (Draft, bool) {
	return designCoverageWithTargets(s, d, set, proj, func(subject, family string) *record.Record {
		if head := proj.Head(subject, family); head != nil && exactPRDTarget(s, subject, family, head) {
			return head
		}
		_, u, ok := contractUnit(s, subject, family)
		if !ok {
			return nil
		}
		return retiredContractClaim(u, subject, s, set, proj)
	})
}
