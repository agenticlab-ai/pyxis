package reconcile

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/association"
	"github.com/agenticlab-ai/pyxis/internal/memory/capture"
	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/sessionlog"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

var codeTagPathRE = regexp.MustCompile(`^[a-z0-9._][a-z0-9/._-]*$`)

type scopedRecord struct {
	r      *record.Record
	scopes []string
}

type activeLookup struct {
	set  *store.Set
	proj *resolve.Projection
}

func (l activeLookup) Get(id string) *record.Record {
	if l.set == nil || l.proj == nil || l.proj.Status(id) != resolve.Active {
		return nil
	}
	return l.set.Get(id)
}

// Analyze returns a complete, deterministic comparison. Its actions are
// limited to lossless source capture, exact typed PRD rows and questions,
// explicit design Decision blocks, and design-header coverage declarations.
// Relationships that would require interpreting prose
// or guessing implementation meaning remain findings for a human/agent.
func Analyze(s Snapshot, set *store.Set, proj *resolve.Projection, opts Options) Report {
	opts = optionDefaults(opts)
	r := Report{
		Schema: ReportSchema, Generator: Generator, SourceCommit: s.Commit,
		SourceClosureSHA256: s.SourceClosureSHA256, CodeInventorySHA256: s.CodeInventorySHA256,
		DesignIndexSHA256: s.DesignIndexSHA256, MemoryClosureSHA256: MemoryClosureSHA256(set),
	}
	r.Findings = append(r.Findings, s.CompilerFindings...)
	if set == nil || proj == nil {
		r.Findings = append(r.Findings, Finding{Code: "memory-unavailable", Scope: "memory/", Detail: "the memory set or its projection is unavailable", Remedy: "load and validate memory before reconciliation", Blocking: true})
		finishReport(&r, s)
		return r
	}

	for _, problem := range set.Problems {
		r.Findings = append(r.Findings, Finding{Code: "invalid-memory-file", Scope: problem.Path, Detail: problem.Err.Error(), Remedy: "repair the immutable record through a valid successor or restore its exact bytes", Blocking: true})
	}
	for _, d := range set.Dangling {
		r.Findings = append(r.Findings, Finding{Code: "dangling-memory-relation", Scope: d.From, Detail: fmt.Sprintf("%s targets absent record %s", d.Type, d.Target), Remedy: "add the missing target or replace/revoke the record carrying the edge", Blocking: true})
	}
	for _, v := range proj.Violations {
		r.Findings = append(r.Findings, Finding{Code: "invalid-memory-projection", Scope: v.Record, Detail: v.What, Remedy: "append a valid record that retires the offending act or otherwise restores the relation invariant", Blocking: true})
	}
	for _, c := range proj.AnswerConflicts() {
		r.Findings = append(r.Findings, Finding{Code: "answer-conflict", Scope: c.Question, Detail: fmt.Sprintf("question has %d active Answers: %s", len(c.IDs), strings.Join(c.IDs, ", ")), Remedy: "append one replacement Answer that supersedes every active Answer", Blocking: true})
	}
	for _, c := range proj.RevocationConflicts() {
		r.Findings = append(r.Findings, Finding{Code: "revocation-conflict", Scope: c.Target, Detail: fmt.Sprintf("record has %d active Revocations: %s", len(c.IDs), strings.Join(c.IDs, ", ")), Remedy: "revoke redundant Revocation acts so exactly one withdrawal remains active", Blocking: true})
	}
	for _, c := range proj.Conflicts() {
		r.Findings = append(r.Findings, Finding{Code: "memory-family-conflict", Scope: c.Subject + "#" + c.Family, Detail: "more than one active claim: " + strings.Join(c.IDs, ", "), Remedy: "reconcile the competing heads with one explicit successor", Blocking: true})
	}

	sealed := map[string]map[string]bool{}
	if !opts.LegacySourceRetirement {
		var retirementFindings []Finding
		sealed, retirementFindings = sourceRetirementBindings(s, set, proj)
		r.Findings = append(r.Findings, retirementFindings...)
	}
	closeouts, closeoutErr := ActiveCloseouts(set, proj)
	if closeoutErr != nil {
		r.Findings = append(r.Findings, Finding{Code: "invalid-closeout", Scope: "closeout/", Detail: closeoutErr.Error(), Remedy: "repair the active closeout lineage before relying on source retirement", Blocking: true})
	} else {
		for _, closeout := range closeouts {
			for _, source := range closeout.Receipt.SourceDocuments {
				if sealed[source.Path] == nil {
					sealed[source.Path] = map[string]bool{}
				}
				sealed[source.Path][source.DocumentID] = true
			}
		}
	}
	analyzeSources(&r, s, set, proj, opts, sealed)
	analyzePRDsWithNative(&r, s, set, proj, !opts.LegacyNativeServes)
	analyzeDesigns(&r, s, set, proj)
	if opts.LegacyCodeCoverage {
		r.Generator = "memory-reconcile/v6"
		analyzeCode(&r, s, set, proj)
	} else {
		analyzeCodeAssociations(&r, s, set, proj)
	}
	if opts.LegacySourceRetirement {
		r.Generator = "memory-reconcile/v4"
	}
	if opts.LegacyContractCoverage {
		r.Generator = "memory-reconcile/v3"
	} else {
		classifyContractCompletion(&r, s, set, proj)
	}
	if opts.LegacyNativeEvaluator {
		r.Generator = "memory-reconcile/v5"
	}
	finishReport(&r, s)
	return r
}

func finishReport(r *Report, s Snapshot) {
	removeCollidingActions(r)
	sortReport(r)
	if r.Findings == nil {
		r.Findings = []Finding{}
	}
	if r.Actions == nil {
		r.Actions = []Action{}
	}
	r.Counts = Counts{Sources: len(s.AuthoredSources), Code: len(s.Code), Designs: len(s.Designs), Findings: len(r.Findings), Actions: len(r.Actions)}
	if r.Completion != nil {
		r.Counts.Completion = len(r.Completion.Findings)
	}
	for _, prd := range s.PRDs {
		r.Counts.PRDUnits += len(prd.Units)
	}
	for _, f := range r.Findings {
		if f.Blocking {
			r.Counts.Blocking++
		}
		if f.Writable {
			r.Counts.Writable++
		}
	}
}

func removeCollidingActions(r *Report) {
	byAddress := map[string][]int{}
	for i, action := range r.Actions {
		key := action.Draft.Subject + "#" + action.Draft.Family
		byAddress[key] = append(byAddress[key], i)
	}
	badScopes := map[string]bool{}
	for address, indexes := range byAddress {
		if len(indexes) < 2 {
			continue
		}
		var scopes []string
		for _, i := range indexes {
			badScopes[r.Actions[i].Scope] = true
			scopes = append(scopes, r.Actions[i].Scope)
		}
		sort.Strings(scopes)
		r.Findings = append(r.Findings, Finding{
			Code: "reconcile-action-collision", Scope: address,
			Detail: "multiple source units derive the same memory address: " + strings.Join(scopes, ", "),
			Remedy: "assign explicit distinct addresses before writing", Blocking: true,
		})
	}
	if len(badScopes) == 0 {
		return
	}
	actions := r.Actions[:0]
	for _, action := range r.Actions {
		if !badScopes[action.Scope] {
			actions = append(actions, action)
		}
	}
	r.Actions = actions
	for i := range r.Findings {
		if badScopes[r.Findings[i].Scope] && r.Findings[i].Writable {
			r.Findings[i].Writable = false
			r.Findings[i].Remedy = "resolve the derived address collision before writing"
		}
	}
}

func analyzeSources(out *Report, s Snapshot, set *store.Set, proj *resolve.Projection, opts Options, sealed map[string]map[string]bool) {
	bySource := map[string][]*record.Record{}
	historical := map[string][]*record.Record{}
	for _, id := range set.IDs() {
		r := set.Get(id)
		if r.Kind == record.Document && fileProvenance(r.Provenance.Source) && !sessionlog.Path(r.Provenance.Source) {
			clean := path.Clean(r.Provenance.Source)
			historical[clean] = append(historical[clean], r)
			if proj.Status(id) == resolve.Active {
				bySource[clean] = append(bySource[clean], r)
			}
		}
	}
	expected := append([]string(nil), s.AuthoredSources...)
	for p := range historical {
		if _, exists := s.Files[p]; exists {
			expected = append(expected, p)
		}
	}
	sort.Strings(expected)
	expected = compactStrings(expected)
	for _, p := range expected {
		if sessionlog.Path(p) {
			continue
		}
		f, ok := s.Files[p]
		if !ok {
			continue
		}
		active := bySource[p]
		if len(active) > 1 {
			out.Findings = append(out.Findings, Finding{Code: "duplicate-source-binding", Scope: p, Detail: fmt.Sprintf("%d active Documents claim the same source", len(active)), Remedy: "supersede or revoke duplicate bindings so one current Document owns the source", Blocking: true})
			continue
		}
		var exact, current *record.Record
		for _, doc := range active {
			if current == nil || doc.ID > current.ID {
				current = doc
			}
			if documentMatchesSource(doc, set.Bodies[doc.ID], f) {
				exact = doc
			}
		}
		if exact != nil {
			continue
		}
		subject, family := sourceAddress(p, opts)
		supersedes := ""
		code := "missing-document"
		detail := "authored source has no active immutable Document with identical bytes"
		if current != nil {
			subject, family, supersedes = current.Subject, current.Family, current.ID
			code = "stale-document"
			detail = "the active Document body differs from the pinned authored source"
			if !current.LiteralBody {
				out.Findings = append(out.Findings, Finding{Code: "source-mode-conflict", Scope: p, Detail: "the bound Document is native while source reconciliation is lossless/literal", Remedy: "review and author an explicit native Document successor; interpretation mode cannot change inside a lineage", Blocking: true})
				continue
			}
		}
		if head := proj.Head(subject, family); head != nil && head.ID != supersedes {
			out.Findings = append(out.Findings, Finding{Code: "source-address-collision", Scope: p, Detail: fmt.Sprintf("derived address %s#%s is occupied by %s from %s", subject, family, head.ID, head.Provenance.Source), Remedy: "assign an explicit non-conflicting source address", Blocking: true})
			continue
		}
		draft := sourceDraft(f, subject, family, supersedes, s.Commit)
		out.Actions = append(out.Actions, Action{Code: "capture-document", Scope: p, BodySHA256: f.SHA256, Draft: draft})
		out.Findings = append(out.Findings, Finding{Code: code, Scope: p, Detail: detail, Remedy: "run memory reconcile --write to append the exact Document binding", Writable: true, Blocking: true})
	}
	for p, docs := range bySource {
		if _, exists := s.Files[p]; exists {
			continue
		}
		ids := make([]string, 0, len(docs))
		for _, doc := range docs {
			if !sealed[p][doc.ID] {
				ids = append(ids, doc.ID)
			}
		}
		if len(ids) == 0 {
			continue
		}
		sort.Strings(ids)
		out.Findings = append(out.Findings, Finding{Code: "orphan-document-source", Scope: p, Detail: "active Document source is absent at the pinned commit: " + strings.Join(ids, ", "), Remedy: "seal the feature before retiring its source, or append an explicit successor/revocation", Blocking: true})
	}
	for p, docs := range historical {
		if _, exists := s.Files[p]; exists || len(sealed[p]) > 0 || len(bySource[p]) > 0 {
			continue
		}
		ids := make([]string, 0, len(docs))
		for _, doc := range docs {
			ids = append(ids, doc.ID)
		}
		sort.Strings(ids)
		out.Findings = append(out.Findings, Finding{Code: "orphan-document-source", Scope: p, Detail: "historically bound source is absent and no valid closeout authorizes retirement: " + strings.Join(ids, ", "), Remedy: "restore and recapture the source, or create a valid closeout before retiring it", Blocking: true})
	}
}

func documentMatchesSource(r *record.Record, body []byte, f File) bool {
	if r == nil || r.Kind != record.Document || !r.LiteralBody {
		return false
	}
	return sourceDigest(body) == f.SHA256 && r.BodySHA256 == f.SHA256
}

func fileProvenance(p string) bool {
	if p == "" || strings.Contains(p, ":") || strings.Contains(p, "\\") || path.IsAbs(p) {
		return false
	}
	clean := path.Clean(p)
	return clean == p && clean != "." && !strings.HasPrefix(clean, "../")
}

func analyzePRDs(out *Report, s Snapshot, set *store.Set, proj *resolve.Projection) {
	analyzePRDsWithNative(out, s, set, proj, true)
}

func analyzePRDsWithNative(out *Report, s Snapshot, set *store.Set, proj *resolve.Projection, allowNative bool) {
	activeSourceUnits := map[string]bool{}
	requiredDesignUnits := map[string]bool{}
	designStarted := designStartedFeatures(s.Designs, set)
	for _, prd := range s.PRDs {
		if prd.RequirementsFirst && !designStarted[prd.Feature] {
			out.DeferredDesignFeatures = append(out.DeferredDesignFeatures, prd.Feature)
		}
		for _, u := range prd.Units {
			ref := "prd/" + prd.Feature + "#" + u.ID
			head := proj.Head("prd/"+prd.Feature, u.ID)
			if u.Status != "live" {
				if head != nil {
					out.Findings = append(out.Findings, Finding{Code: "stale-prd-memory", Scope: ref, Feature: prd.Feature, Detail: fmt.Sprintf("source unit is %s but memory head %s remains active", u.Status, head.ID), Remedy: "append the source-authorized successor or an explicit revocation", Blocking: true})
					continue
				}
				if u.Prefix == "Q" || u.Prefix == "R" {
					disposition, err := explicitPRDDisposition(prd, u, proj)
					if err != nil {
						out.Findings = append(out.Findings, Finding{Code: "invalid-prd-disposition", Scope: ref, Feature: prd.Feature, Detail: err.Error(), Remedy: "replace or revoke the disposition before retiring the source row", Blocking: true})
					} else if disposition.Record != nil {
						out.Findings = append(out.Findings, Finding{Code: "stale-prd-memory", Scope: ref, Feature: prd.Feature, Detail: fmt.Sprintf("source unit is %s but native disposition %s remains active", u.Status, disposition.Record.ID), Remedy: "append an explicit revocation for the retired disposition", Blocking: true})
					}
				}
				continue
			}
			if !u.Mapped {
				switch u.Prefix {
				case "NG", "WR":
					if head != nil {
						out.Findings = append(out.Findings, Finding{Code: "unexpected-prd-context-memory", Scope: ref, Feature: prd.Feature, Detail: fmt.Sprintf("live %s context row intentionally has no durable memory mapping but active head %s occupies its address", u.Prefix, head.ID), Remedy: "revoke the invented context-row claim; preserve the row only through its exact source Document", Blocking: true})
					}
					continue // declared context, not durable product truth
				}
				disposition, err := explicitPRDDisposition(prd, u, proj)
				if err != nil {
					out.Findings = append(out.Findings, Finding{Code: "invalid-prd-disposition", Scope: ref, Feature: prd.Feature, Detail: err.Error(), Remedy: "replace or revoke the disposition and author one exact current-row disposition", Blocking: true})
					continue
				}
				if disposition.Record != nil {
					if u.Prefix == "Q" && disposition.Answer == nil {
						out.Findings = append(out.Findings, Finding{Code: "open-prd-question", Scope: ref, Feature: prd.Feature, Detail: "the exact Question disposition " + disposition.Record.ID + " has no active Answer", Remedy: "answer the captured Question explicitly before closeout", Blocking: true})
					}
					continue
				}
				if u.Prefix == "Q" {
					if draft, ok := prdQuestionDraft(prd, s.Commit, u); ok {
						out.Actions = append(out.Actions, Action{Code: "add-prd-question", Scope: ref, Draft: draft})
					}
					out.Findings = append(out.Findings, Finding{Code: "open-prd-question", Scope: ref, Feature: prd.Feature, Detail: "the live PRD question has no exact native disposition", Remedy: prdDispositionRemedy(u), Blocking: true})
				} else {
					out.Findings = append(out.Findings, Finding{Code: "unmapped-prd-unit", Scope: ref, Feature: prd.Feature, Detail: "the source unit kind " + u.Prefix + " has no lossless memory-kind mapping or exact native disposition", Remedy: prdDispositionRemedy(u), Blocking: true})
				}
				continue
			}
			activeSourceUnits[ref] = true
			if requiresDesignServe(u) && (!prd.RequirementsFirst || designStarted[prd.Feature]) {
				requiredDesignUnits[ref] = true
			}
			if head == nil {
				draft := prdDraft(prd, s.Commit, u)
				if u.Supersedes != nil {
					previous := proj.Head("prd/"+prd.Feature, *u.Supersedes)
					if previous != nil {
						if previous.Kind != u.Kind {
							out.Findings = append(out.Findings, Finding{Code: "prd-lineage-kind-conflict", Scope: ref, Feature: prd.Feature, Detail: fmt.Sprintf("compiled successor kind %s cannot supersede active %s memory %s", u.Kind, previous.Kind, previous.ID), Remedy: "reconcile the typed lineage explicitly", Blocking: true})
							continue
						}
						draft.Relations = append(draft.Relations, record.Relation{Type: record.Supersedes, Target: previous.ID})
					}
				}
				out.Actions = append(out.Actions, Action{Code: "add-prd-claim", Scope: ref, Draft: draft})
				out.Findings = append(out.Findings, Finding{Code: "missing-prd-memory", Scope: ref, Feature: prd.Feature, Detail: "live source unit has no active typed memory claim", Remedy: "run memory reconcile --write to append the exact typed row", Writable: true, Blocking: true})
				continue
			}
			if !samePRDSemantics(head, u) {
				out.Findings = append(out.Findings, Finding{Code: "prd-memory-drift", Scope: ref, Feature: prd.Feature, Detail: fmt.Sprintf("memory head %s does not equal the live compiled row", head.ID), Remedy: "review the semantic delta and append an explicit successor; reconciliation does not overwrite authored meaning", Blocking: true})
			}
		}
	}

	served, invalidServes := authorizedPRDServes(s, set, proj, activeSourceUnits, allowNative)
	out.Findings = append(out.Findings, invalidServes...)
	for ref := range requiredDesignUnits {
		subject, family, _ := strings.Cut(ref, "#")
		head := proj.Head(subject, family)
		if head != nil && !served[head.ID] {
			feature := strings.TrimPrefix(subject, "prd/")
			out.Findings = append(out.Findings, Finding{Code: "unserved-prd-memory", Scope: ref, Feature: feature, Detail: "the active Must claim is served by no active Decision", Remedy: "capture the implemented design decision with an explicit serves relation", Blocking: true})
		}
	}
}

// authorizedPRDServes projects the source design graph onto current memory.
// A Decision cannot make a PRD claim look designed merely by carrying a
// syntactically valid serves edge: it must live on the exact indexed design
// subject, that design must be live, and its source metadata must declare the
// same feature and family. The edge must also target today's active head for
// that address rather than a superseded version.
func authorizedPRDServes(s Snapshot, set *store.Set, proj *resolve.Projection, activeSourceUnits map[string]bool, allowNative bool) (map[string]bool, []Finding) {
	authorized := authorizedDesignPRDHeads(s.Designs, proj)
	sealedHistorical := sealedHistoricalPRDServes(set, proj)
	ledgerFeatures := map[string]bool{}
	for _, prd := range s.PRDs {
		ledgerFeatures[prd.Feature] = true
	}
	for _, feature := range s.PRDFeatures {
		ledgerFeatures[feature] = true
	}
	for feature := range s.TrustedPRDFeatures {
		ledgerFeatures[feature] = true
	}

	served := map[string]bool{}
	var findings []Finding
	for _, id := range set.IDs() {
		decision := set.Get(id)
		if proj.Status(id) != resolve.Active || decision.Kind != record.Decision {
			continue
		}
		for _, targetID := range decision.Rel(record.Serves) {
			target := set.Get(targetID)
			if target == nil || record.Namespace(target.Subject) != "prd" {
				continue
			}
			currentSource := activeSourceUnits[target.Subject+"#"+target.Family]
			if !currentSource && sealedHistorical[decision.ID+"\x00"+targetID] {
				continue
			}
			feature := strings.TrimPrefix(target.Subject, "prd/")
			if allowNative && !ledgerFeatures[feature] && nativeRequirementKind(target.Kind) && proj.Status(targetID) == resolve.Active && proj.Head(target.Subject, target.Family) == target {
				served[targetID] = true
				continue
			}
			if currentSource && authorized[decision.Subject][targetID] {
				served[targetID] = true
				continue
			}
			detail := "active Decision serves a PRD record without a matching live indexed design declaration for that current feature family"
			if !currentSource {
				detail = "active Decision serves a PRD record whose address is not a live mapped source unit"
			}
			findings = append(findings, Finding{
				Code: "unauthorized-prd-serves", Scope: decision.ID + " -> " + targetID, Feature: feature,
				Detail:   detail,
				Remedy:   "move the Decision to the matching live design and align that design's Serves metadata, or append a successor that removes the unauthorized edge",
				Blocking: true,
			})
		}
	}
	return served, findings
}

func nativeRequirementKind(kind string) bool {
	return kind == record.Need || kind == record.Promise || kind == record.Expectation
}

// sealedHistoricalPRDServes preserves the exact edge set of a valid closeout
// after its development sources are retired. Both endpoints must be active
// native bindings in the same receipt, the target must belong to that receipt's
// PRD subject, and the Decision must belong to one of its sealed designs. An
// invented or stale edge outside that proof remains unauthorized.
func sealedHistoricalPRDServes(set *store.Set, proj *resolve.Projection) map[string]bool {
	out := map[string]bool{}
	closeouts, err := ActiveCloseouts(set, proj)
	if err != nil {
		return out
	}
	for _, closeout := range closeouts {
		native := map[string]bool{}
		for _, binding := range closeout.Receipt.NativeRecords {
			native[binding.ID] = true
		}
		designs := map[string]bool{}
		for _, design := range closeout.Receipt.Designs {
			designs["design/"+design.Slug] = true
		}
		for _, binding := range closeout.Receipt.NativeRecords {
			decision := set.Get(binding.ID)
			if decision == nil || decision.Kind != record.Decision || !designs[decision.Subject] {
				continue
			}
			for _, targetID := range decision.Rel(record.Serves) {
				target := set.Get(targetID)
				if native[targetID] && target != nil && target.Subject == "prd/"+closeout.Receipt.Feature {
					out[decision.ID+"\x00"+targetID] = true
				}
			}
		}
	}
	return out
}

func authorizedDesignPRDHeads(designs []Design, proj *resolve.Projection) map[string]map[string]bool {
	authorized := map[string]map[string]bool{}
	for _, design := range designs {
		if design.Status != "live" {
			continue
		}
		subject := "design/" + design.Slug
		if authorized[subject] == nil {
			authorized[subject] = map[string]bool{}
		}
		for _, ref := range design.Serves {
			prdSubject, family, ok := memoryPRDRef(ref)
			if !ok {
				continue
			}
			if head := proj.Head(prdSubject, family); head != nil {
				authorized[subject][head.ID] = true
			}
		}
	}
	return authorized
}

func prdDispositionRemedy(u PRDUnit) string {
	tags := prdDispositionTag + " and source-unit:" + u.SourceSHA256
	if u.Prefix == "Q" {
		return "capture a Question at family " + u.ID + " tagged " + tags + ", then append exactly one Answer"
	}
	if u.Prefix == "R" {
		return "capture a Decision at family " + u.ID + " tagged " + tags + " with rationale and fit criterion"
	}
	return "repair the source row; only Q and R units support explicit native disposition"
}

func samePRDSemantics(r *record.Record, u PRDUnit) bool {
	return r.Kind == u.Kind && r.Statement == u.Statement && r.Intent == u.Intent && r.Rationale == u.Rationale && r.FitCriterion == u.FitCriterion && r.Consequence == u.Consequence && countSourceUnitTags(r, sourceUnitTags(u)) == 1
}

func analyzeDesigns(out *Report, s Snapshot, set *store.Set, proj *resolve.Projection) {
	for _, d := range s.Designs {
		subject := "design/" + d.Slug
		var decisions []*record.Record
		for _, r := range proj.Active(subject) {
			if r.Kind == record.Decision {
				decisions = append(decisions, r)
			}
		}
		features := designFeatures(d)
		feature := ""
		if len(features) == 1 {
			feature = features[0]
		}
		if d.Status == "superseded" {
			if len(decisions) > 0 {
				out.Findings = append(out.Findings, Finding{Code: "superseded-design-memory", Scope: subject, Feature: feature, Detail: fmt.Sprintf("source design is superseded by %s but has %d active Decision record(s)", d.CurrentSuccessor, len(decisions)), Remedy: "append explicit successor decisions on the live design identity and revoke obsolete decisions", Blocking: true})
			}
			continue
		}
		decisionSourceChecked := analyzeDesignDecisionMemory(out, s, d, set, proj)
		if coverage := proj.Head(subject, designCoverageFamily); coverage != nil {
			expected, ok := expectedDesignCoverage(s, d, set, proj)
			var err error
			if !ok {
				err = fmt.Errorf("the exact current design coverage declaration cannot be reconstructed")
			} else {
				err = validateDesignCoverageMemory(coverage, expected)
			}
			if err != nil {
				out.Findings = append(out.Findings, Finding{Code: "design-coverage-memory-drift", Scope: subject, Feature: feature, Detail: err.Error(), Remedy: "append an explicit successor aligned to the exact live design header", Blocking: true})
			}
		} else if draft, ok := designCoverageDraft(s, d, set, proj); ok {
			out.Actions = append(out.Actions, Action{Code: "add-design-coverage", Scope: subject, Draft: draft})
		}
		if !decisionSourceChecked && len(d.Serves) > 0 && len(decisions) == 0 {
			out.Findings = append(out.Findings, Finding{Code: "missing-design-memory", Scope: subject, Feature: feature, Detail: "live serving design has no native Decision records", Remedy: "distill its binding decisions; the literal imported Document is provenance, not a typed decision", Blocking: true})
		}
		for _, ref := range d.Serves {
			subjectRef, family, ok := memoryPRDRef(ref)
			if !ok {
				continue
			}
			if nonDurablePRDContextRef(s, subjectRef, family) {
				continue
			}
			target := proj.Head(subjectRef, family)
			if target == nil {
				out.Findings = append(out.Findings, Finding{Code: "missing-design-target-memory", Scope: subject + " -> " + ref, Feature: strings.TrimPrefix(subjectRef, "prd/"), Detail: "design Serves target has no active typed memory claim", Remedy: "capture the source requirement before its design relationship", Blocking: true})
				continue
			}
			linked := false
			for _, decision := range decisions {
				for _, id := range decision.Rel(record.Serves) {
					linked = linked || id == target.ID
				}
			}
			if len(decisions) > 0 && !linked {
				out.Findings = append(out.Findings, Finding{Code: "missing-serves-memory", Scope: subject + " -> " + ref, Feature: strings.TrimPrefix(subjectRef, "prd/"), Detail: "source design declares the relationship but no active Decision carries it", Remedy: "append or revise the native Decision with an explicit serves relation", Blocking: true})
			}
		}
	}
}

func designFeatures(d Design) []string {
	seen := map[string]bool{}
	var out []string
	for _, ref := range d.Serves {
		if !strings.HasPrefix(ref, "prd:") {
			continue
		}
		rest := strings.TrimPrefix(ref, "prd:")
		feature, _, ok := strings.Cut(rest, "/")
		if ok && feature != "" && !seen[feature] {
			seen[feature] = true
			out = append(out, feature)
		}
	}
	sort.Strings(out)
	return out
}

func memoryPRDRef(ref string) (string, string, bool) {
	if !strings.HasPrefix(ref, "prd:") {
		return "", "", false
	}
	rest := strings.TrimPrefix(ref, "prd:")
	feature, family, ok := strings.Cut(rest, "/")
	if !ok || feature == "" || family == "" {
		return "", "", false
	}
	return "prd/" + feature, family, true
}

// nonDurablePRDContextRef reports whether an exact design reference resolves
// to one live NG/WR row. Those rows are approved design context, not durable
// product truth, so they intentionally have no native memory head or serves
// relation. Missing, duplicated, retired, or unexpectedly mapped rows return
// false so callers keep failing closed on a stale or malformed design header.
func nonDurablePRDContextRef(s Snapshot, subject, family string) bool {
	feature := strings.TrimPrefix(subject, "prd/")
	if feature == subject || feature == "" || family == "" {
		return false
	}
	featureCount, unitCount := 0, 0
	var matched PRDUnit
	for _, prd := range s.PRDs {
		if prd.Feature != feature {
			continue
		}
		featureCount++
		for _, unit := range prd.Units {
			if unit.ID != family {
				continue
			}
			unitCount++
			matched = unit
		}
	}
	return featureCount == 1 && unitCount == 1 && matched.Status == "live" && !matched.Mapped && (matched.Prefix == "NG" || matched.Prefix == "WR")
}

func analyzeCode(out *Report, s Snapshot, set *store.Set, proj *resolve.Projection) {
	var all, anchors, semantic []scopedRecord
	for _, id := range set.IDs() {
		r := set.Get(id)
		if proj.Status(id) != resolve.Active {
			continue
		}
		scopes := association.Scopes(r, activeLookup{set: set, proj: proj})
		if len(scopes) == 0 {
			continue
		}
		sr := scopedRecord{r: r, scopes: scopes}
		all = append(all, sr)
		if isCodeAnchor(r) {
			anchors = append(anchors, sr)
		} else if isSemanticCodeMemory(r) {
			semantic = append(semantic, sr)
		}
	}
	for _, sr := range all {
		for _, scope := range sr.scopes {
			if !validCodeScope(scope) {
				out.Findings = append(out.Findings, Finding{Code: "invalid-code-scope", Scope: sr.r.ID + " -> " + scope, Detail: "code scope is not a canonical lowercase repository-relative path", Remedy: "append a successor carrying the exact canonical code path", Blocking: true})
				continue
			}
			matched := false
			for _, unit := range s.Code {
				matched = matched || scopeMatchesUnit(scope, unit)
			}
			if !matched {
				out.Findings = append(out.Findings, Finding{Code: "dangling-code-scope", Scope: sr.r.ID + " -> " + scope, Detail: "active memory association matches no production code unit", Remedy: "repoint the association through an explicit successor or revoke obsolete memory", Blocking: true})
			}
		}
	}
	for _, unit := range s.Code {
		exact := exactScopes(unit.Path, anchors)
		switch {
		case len(exact) == 0:
			subject := codeAnchorSubject(unit.Path)
			if !record.ValidSubject(subject) || !codeTagPathRE.MatchString(unit.Path) {
				out.Findings = append(out.Findings, Finding{Code: "unmappable-code-anchor", Scope: unit.Path, Detail: "the exact code path cannot be represented by the closed subject/tag grammar", Remedy: "rename the path or author an explicit stable anchor", Blocking: true})
			} else if head := proj.Head(subject, "anchor"); head != nil {
				out.Findings = append(out.Findings, Finding{Code: "code-anchor-collision", Scope: unit.Path, Detail: fmt.Sprintf("derived anchor address %s#anchor is occupied by %s", subject, head.ID), Remedy: "author an explicit non-conflicting anchor", Blocking: true})
			} else {
				draft := codeAnchorDraft(unit, s.Commit, "")
				out.Actions = append(out.Actions, Action{Code: "add-code-anchor", Scope: unit.Path, Draft: draft})
				out.Findings = append(out.Findings, Finding{Code: "missing-code-anchor", Scope: unit.Path, Detail: "production code has no stable code/* anchor", Remedy: "run memory reconcile --write to append the structural anchor", Writable: true, Blocking: true})
			}
		case len(exact) > 1:
			var ids []string
			for _, sr := range exact {
				ids = append(ids, sr.r.ID)
			}
			sort.Strings(ids)
			out.Findings = append(out.Findings, Finding{Code: "duplicate-code-anchor", Scope: unit.Path, Detail: "multiple active code anchors bind the exact unit: " + strings.Join(ids, ", "), Remedy: "retain one stable anchor identity and supersede or revoke the duplicates", Blocking: true})
		case exact[0].r.HasTag("reconcile:anchor") && (exact[0].r.Provenance.Source != "code:"+unit.Path || exact[0].r.Provenance.Note != "offline code inventory "+unit.SHA256):
			draft := codeAnchorDraft(unit, s.Commit, exact[0].r.ID)
			draft.Subject = exact[0].r.Subject
			draft.Family = exact[0].r.Family
			out.Actions = append(out.Actions, Action{Code: "refresh-code-anchor", Scope: unit.Path, Draft: draft})
			out.Findings = append(out.Findings, Finding{Code: "stale-code-anchor", Scope: unit.Path, Detail: "the structural anchor predates the current code-unit digest", Remedy: "run memory reconcile --write to append the refreshed structural anchor; semantic memories and closeout seals still require explicit reconciliation", Writable: true, Blocking: true})
		}
		if !coveredBy(unit, semantic) {
			out.Findings = append(out.Findings, Finding{Code: "missing-code-memory", Scope: unit.Path, Detail: "production code has no active semantic memory with an explicit code scope", Remedy: "connect a native Promise, Expectation, Decision, Policy, Learning, or Document using code:<path> or about -> code anchor", Blocking: true})
		}
	}
}

// analyzeCodeAssociations validates explicitly authored knowledge. A code unit
// has no obligation to acquire memory merely because it exists or changes.
// Old generated anchors and package summaries are retained as history; their
// digests and paths are historical witnesses, not assertions of current code.
func analyzeCodeAssociations(out *Report, s Snapshot, set *store.Set, proj *resolve.Projection) {
	for _, id := range set.IDs() {
		r := set.Get(id)
		if proj.Status(id) != resolve.Active || capture.Derived(r) {
			continue
		}
		for _, scope := range association.Scopes(r, activeLookup{set: set, proj: proj}) {
			if !validCodeScope(scope) {
				out.Findings = append(out.Findings, Finding{Code: "invalid-code-scope", Scope: r.ID + " -> " + scope, Detail: "code scope is not a canonical lowercase repository-relative path", Remedy: "append a successor carrying the exact canonical code path", Blocking: true})
				continue
			}
			matched := false
			for _, unit := range s.Code {
				matched = matched || scopeMatchesUnit(scope, unit)
			}
			if !matched {
				out.Findings = append(out.Findings, Finding{Code: "dangling-code-scope", Scope: r.ID + " -> " + scope, Detail: "active memory association matches no production code unit", Remedy: "repoint the association through an explicit successor or revoke obsolete memory", Blocking: true})
			}
		}
	}
}

func exactScopes(scope string, records []scopedRecord) []scopedRecord {
	var out []scopedRecord
	for _, sr := range records {
		if slices.Contains(sr.scopes, scope) {
			out = append(out, sr)
		}
	}
	return out
}

func coveredBy(unit codeinventory.Unit, records []scopedRecord) bool {
	for _, sr := range records {
		for _, scope := range sr.scopes {
			if scopeMatchesUnit(scope, unit) {
				return true
			}
		}
	}
	return false
}

func scopeMatchesUnit(scope string, unit codeinventory.Unit) bool {
	if association.Covers(scope, unit.Path) {
		return true
	}
	return slices.Contains(unit.Files, scope)
}

func validCodeScope(scope string) bool {
	return codeTagPathRE.MatchString(scope) && scope != "." && scope != ".." && path.Clean(scope) == scope && !path.IsAbs(scope) && !strings.HasPrefix(scope, "../")
}

func isCodeAnchor(r *record.Record) bool {
	return record.Namespace(r.Subject) == "code" || r.HasTag("reconcile:anchor")
}

func isSemanticCodeMemory(r *record.Record) bool {
	if isCodeAnchor(r) || r.Kind == record.Plan || strings.HasPrefix(r.Subject, "closeout/") {
		return false
	}
	if r.Kind == record.Document && r.LiteralBody {
		return false
	}
	return r.Kind == record.Need || r.Kind == record.Promise || r.Kind == record.Expectation || r.Kind == record.Decision || r.Kind == record.Policy || r.Kind == record.Learning || r.Kind == record.Document
}

func codeAnchorSubject(scope string) string {
	return "code/unit-" + slug(scope) + "-" + sourceDigest([]byte(scope))[:12]
}

func codeAnchorDraft(unit codeinventory.Unit, commit, supersedes string) Draft {
	var relations []record.Relation
	if supersedes != "" {
		relations = append(relations, record.Relation{Type: record.Supersedes, Target: supersedes})
	}
	return Draft{
		Kind: record.Need, Subject: codeAnchorSubject(unit.Path), Family: "anchor",
		Statement:    "Production code unit " + unit.Path + " exists.",
		Intent:       "Provide a stable target for explicit memory-to-code associations.",
		Rationale:    "The offline inventory identified this unit from regular tracked production files at one pinned commit.",
		FitCriterion: "The pinned tree contains the exact code unit at " + unit.Path + ".",
		Tags:         []string{"code:" + unit.Path, "reconcile:anchor"},
		Relations:    relations,
		Provenance:   record.Provenance{Source: "code:" + unit.Path, Commit: commit, Note: "offline code inventory " + unit.SHA256},
	}
}
