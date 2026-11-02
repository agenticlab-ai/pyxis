package reconcile

import (
	"fmt"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/memory/association"
	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// VerifyOffline performs the ordinary repository-to-memory reconciliation and
// then validates every active closeout against the current memory and code.
// Retired source documents are optional: their exact captured bytes and
// compiler identities remain bound by the receipt, while code and native
// memory must remain current.
func VerifyOffline(s Snapshot, set *store.Set, proj *resolve.Projection, opts Options) Report {
	return verifyOffline(s, set, proj, opts, nil)
}

// VerifyOfflineWithHistory additionally replays every active closeout at its
// recorded source commit. The loader is supplied by the repository adapter so
// this package's deterministic verifier remains independent of Git.
func VerifyOfflineWithHistory(s Snapshot, set *store.Set, proj *resolve.Projection, opts Options, load func(ActiveCloseout) (Snapshot, error)) Report {
	return verifyOffline(s, set, proj, opts, load)
}

func verifyOffline(s Snapshot, set *store.Set, proj *resolve.Projection, opts Options, load func(ActiveCloseout) (Snapshot, error)) Report {
	report := Analyze(s, set, proj, opts)
	active, err := ActiveCloseouts(set, proj)
	if err != nil {
		if !containsFinding(report.Findings, "invalid-closeout", "closeout/") {
			report.Findings = append(report.Findings, Finding{
				Code: "invalid-closeout", Scope: "closeout/", Detail: err.Error(),
				Remedy: "repair the active receipt lineage; no source-retirement allowance is trusted while a seal is invalid", Blocking: true,
			})
		}
		finishReport(&report, s)
		return report
	}
	requiredOwners := map[string]map[string]bool{}
	sealedOwners := map[string]map[string]string{}
	for _, closeout := range active {
		if load != nil {
			historical, historyErr := load(closeout)
			if historyErr != nil {
				report.Findings = append(report.Findings, Finding{
					Code: "closeout-history-unavailable", Scope: "closeout/" + closeout.Receipt.Feature, Feature: closeout.Receipt.Feature,
					Detail: fmt.Sprintf("cannot rebuild source commit %s for receipt %s: %v", closeout.Receipt.SourceCommit, closeout.RecordID, historyErr),
					Remedy: "restore the recorded commit or append a replacement closeout whose source commit can be verified offline", Blocking: true,
				})
			} else if replayErr := ValidateCloseoutAtSnapshot(closeout, historical, set, proj); replayErr != nil {
				report.Findings = append(report.Findings, Finding{
					Code: "closeout-history-invalid", Scope: "closeout/" + closeout.Receipt.Feature, Feature: closeout.Receipt.Feature,
					Detail: replayErr.Error(), Remedy: "restore the exact closeout evidence or append a replacement receipt produced by memory closeout", Blocking: true,
				})
			} else if owners, ownerErr := ContextSourceOwners(historical); ownerErr != nil {
				report.Findings = append(report.Findings, Finding{
					Code: "closeout-history-invalid", Scope: "closeout/" + closeout.Receipt.Feature, Feature: closeout.Receipt.Feature,
					Detail: fmt.Sprintf("cannot derive complete typed source ownership at %s: %v", closeout.Receipt.SourceCommit, ownerErr),
					Remedy: "repair the historical lifecycle metadata or append a replacement closeout from a fully typed source snapshot", Blocking: true,
				})
			} else {
				for _, binding := range closeout.Receipt.SourceDocuments {
					for _, owner := range owners[binding.Path] {
						if requiredOwners[binding.Path] == nil {
							requiredOwners[binding.Path] = map[string]bool{}
						}
						requiredOwners[binding.Path][owner] = true
					}
					if sealedOwners[binding.Path] == nil {
						sealedOwners[binding.Path] = map[string]string{}
					}
					sealedOwners[binding.Path][closeout.Receipt.Feature] = binding.BodySHA256
				}
			}
		}
		verifyReceipt(&report, s, set, proj, closeout)
	}
	if load != nil {
		verifySharedSourceRetirements(&report, s, requiredOwners, sealedOwners)
	}
	finishReport(&report, s)
	return report
}

func verifySharedSourceRetirements(out *Report, s Snapshot, required map[string]map[string]bool, sealed map[string]map[string]string) {
	paths := make([]string, 0, len(required))
	for source := range required {
		paths = append(paths, source)
	}
	sort.Strings(paths)
	for _, source := range paths {
		if _, present := s.Files[source]; present {
			continue
		}
		owners := make([]string, 0, len(required[source]))
		for owner := range required[source] {
			owners = append(owners, owner)
		}
		sort.Strings(owners)
		digests := map[string]bool{}
		for _, owner := range owners {
			digest := sealed[source][owner]
			if digest == "" {
				out.Findings = append(out.Findings, Finding{
					Code: "closeout-shared-source-unsealed", Scope: source, Feature: owner,
					Detail: fmt.Sprintf("retired shared context is owned by %s but has no replay-valid active closeout for that owner", owner),
					Remedy: "restore the source until every typed owner has sealed the same exact bytes", Blocking: true,
				})
				continue
			}
			digests[digest] = true
		}
		if len(digests) > 1 {
			out.Findings = append(out.Findings, Finding{
				Code: "closeout-shared-source-version-conflict", Scope: source,
				Detail: "replay-valid owner closeouts bind different versions of the retired shared context",
				Remedy: "restore the source and replace every owner closeout against one identical source version", Blocking: true,
			})
		}
	}
}

func containsFinding(findings []Finding, code, scope string) bool {
	for _, finding := range findings {
		if finding.Code == code && finding.Scope == scope {
			return true
		}
	}
	return false
}

func verifyReceipt(out *Report, s Snapshot, set *store.Set, proj *resolve.Projection, active ActiveCloseout) {
	receipt := active.Receipt
	add := func(code, item, detail, remedy string) {
		out.Findings = append(out.Findings, Finding{Code: code, Scope: item, Feature: receipt.Feature, Detail: detail, Remedy: remedy, Blocking: true})
	}

	for _, binding := range receipt.SourceDocuments {
		if proj.Status(binding.DocumentID) != resolve.Active {
			add("closeout-source-memory-inactive", binding.Path, fmt.Sprintf("sealed source Document %s is %s", binding.DocumentID, proj.Status(binding.DocumentID)), "append a new valid closeout only after restoring an active exact source capture")
		}
	}
	for _, binding := range receipt.NativeRecords {
		if proj.Status(binding.ID) != resolve.Active {
			add("closeout-native-memory-inactive", binding.Subject+"#"+binding.Family, fmt.Sprintf("sealed native record %s is %s", binding.ID, proj.Status(binding.ID)), "reconcile the successor semantics and append a replacement closeout")
		}
	}

	currentCode := map[string]codeinventory.Unit{}
	for _, unit := range s.Code {
		currentCode[codeUnitKey(unit.Kind, unit.Path)] = unit
	}
	sealedCode := map[string]CloseoutCodeUnit{}
	for _, binding := range receipt.CodeUnits {
		key := codeUnitKey(binding.Kind, binding.Path)
		sealedCode[key] = binding
		unit, ok := currentCode[key]
		switch {
		case !ok:
			add("closeout-code-missing", binding.Path, "a code unit bound by the closeout no longer exists", "reconcile the removal or move in native memory and append a replacement closeout")
		case unit.SHA256 != binding.SHA256:
			add("closeout-code-drift", binding.Path, "the current code-unit digest differs from the sealed digest", "reconcile the changed behavior in native memory and append a replacement closeout")
		}
	}

	// Re-evaluate the receipt's immutable native records against today's code.
	// This catches new subpackages beneath a broad feature scope: their absence
	// from the old receipt is drift even if the same memory happens to cover
	// their path syntactically.
	lookup := activeLookup{set: set, proj: proj}
	selectedNow := map[string]bool{}
	for _, binding := range receipt.NativeRecords {
		r := set.Get(binding.ID)
		if r == nil || proj.Status(binding.ID) != resolve.Active || !semanticForCodeCloseout(r) {
			continue
		}
		for _, codeScope := range association.Scopes(r, lookup) {
			for _, unit := range s.Code {
				if scopeMatchesUnit(codeScope, unit) {
					selectedNow[codeUnitKey(unit.Kind, unit.Path)] = true
				}
			}
		}
	}
	for _, binding := range receipt.CodeUnits {
		key := codeUnitKey(binding.Kind, binding.Path)
		unit, present := currentCode[key]
		associated := false
		if present {
			for _, id := range binding.MemoryRecords {
				r := set.Get(id)
				if r == nil || proj.Status(id) != resolve.Active || !semanticForCodeCloseout(r) {
					continue
				}
				for _, codeScope := range association.Scopes(r, lookup) {
					if scopeMatchesUnit(codeScope, unit) {
						associated = true
						break
					}
				}
			}
		}
		if present && !associated {
			add("closeout-code-unassociated", binding.Path, "the sealed code unit still exists but no active semantic record bound by the closeout explicitly selects it", "restore an explicit code scope on active semantic memory and append a replacement closeout")
		}
	}
	for key := range selectedNow {
		if _, ok := sealedCode[key]; ok {
			continue
		}
		unit := currentCode[key]
		add("closeout-code-unsealed", unit.Path, "current code is covered by the feature's sealed memories but was not present in its receipt", "review the added code and append a replacement closeout that binds it")
	}

	if current, ok := currentPRD(receipt.Feature, s.PRDs); ok {
		if current.Source != receipt.PRD.Source || current.AsOf != receipt.PRD.AsOf || current.Fingerprint != receipt.PRD.Fingerprint || current.SourceMapSHA256 != receipt.PRD.SourceMapSHA256 {
			add("closeout-prd-drift", "prd/"+receipt.Feature, "the current compiled PRD identity differs from the closeout", "reconcile every changed source unit and append a replacement closeout")
		}
	}

	sealedDesigns := map[string]CloseoutDesign{}
	for _, binding := range receipt.Designs {
		sealedDesigns[binding.Slug] = binding
		for _, current := range s.Designs {
			if current.Slug != binding.Slug {
				continue
			}
			if current.Status != "live" || current.Source != binding.Source || current.SHA256 != binding.SHA256 {
				add("closeout-design-drift", "design/"+binding.Slug, "the current design identity or liveness differs from the closeout", "distill the current decisions and append a replacement closeout")
			}
		}
	}
	var currentServing []string
	for _, design := range s.Designs {
		if design.Status != "live" {
			continue
		}
		for _, serves := range design.Serves {
			subject, _, ok := memoryPRDRef(serves)
			if ok && subject == "prd/"+receipt.Feature {
				currentServing = append(currentServing, design.Slug)
				break
			}
		}
	}
	sort.Strings(currentServing)
	if len(currentServing) > 0 {
		for _, slug := range currentServing {
			if _, ok := sealedDesigns[slug]; !ok {
				add("closeout-design-unsealed", "design/"+slug, "a current live design serves the feature but is absent from its closeout", "capture its native Decisions and append a replacement closeout")
			}
		}
	}

}

func currentPRD(feature string, prds []PRD) (PRD, bool) {
	var current PRD
	found := false
	for _, prd := range prds {
		if prd.Feature != feature {
			continue
		}
		if found {
			return PRD{}, false
		}
		current, found = prd, true
	}
	return current, found
}

func codeUnitKey(kind codeinventory.UnitKind, path string) string {
	return string(kind) + "\x00" + path
}
