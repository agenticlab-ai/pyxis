package reconcile

import (
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/memory/association"
	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// CodeTransitionRegressions preserves the pre-v7 transition rule only when
// reproducing historical evaluation. Current code changes do not establish a
// semantic delta: existing knowledge may still govern the new implementation.
// Revision verification belongs to the normal gates and closeout receipts.
func CodeTransitionRegressions(prior, current Snapshot, priorSet *store.Set, priorProj *resolve.Projection, currentSet *store.Set, currentProj *resolve.Projection) []BaselineDelta {
	if !relatedProjection(priorSet, priorProj) || !relatedProjection(currentSet, currentProj) {
		return []BaselineDelta{codeTransitionDelta("code/", "code transition cannot be checked because a memory set or projection is unavailable")}
	}
	if !current.SourceOptions.LegacyCodeCoverage {
		return nil
	}
	return legacyCodeTransitionRegressions(prior, current, priorSet, priorProj, currentSet, currentProj)
}

// legacyCodeTransitionRegressions retains the v1-v6 rule. New and
// changed units need a newly issued active semantic record that explicitly
// covers the unit. Removed units need an existing semantic/anchor identity to
// become inactive, proving that memory acknowledged the retirement.
//
// The check is intentionally transition-scoped: the first baseline bootstraps
// the repository's existing debt, while every later baseline-ref comparison
// enforces reconciliation for code changed since that trusted inventory.
func legacyCodeTransitionRegressions(prior, current Snapshot, priorSet *store.Set, priorProj *resolve.Projection, currentSet *store.Set, currentProj *resolve.Projection) []BaselineDelta {
	if !relatedProjection(priorSet, priorProj) || !relatedProjection(currentSet, currentProj) {
		return []BaselineDelta{codeTransitionDelta("code/", "code transition cannot be checked because a memory set or projection is unavailable")}
	}
	priorUnits := codeUnitsByKey(prior.Code)
	currentUnits := codeUnitsByKey(current.Code)
	var out []BaselineDelta

	for key, unit := range currentUnits {
		old, existed := priorUnits[key]
		if existed && old.SHA256 == unit.SHA256 {
			continue
		}
		fresh := false
		for _, id := range semanticCodeRecords(unit, currentSet, currentProj) {
			if currentSet.Get(id) != nil && priorSet.Get(id) == nil && CurrentUnitEvidence(currentSet.Get(id), unit) {
				fresh = true
				break
			}
		}
		if fresh {
			continue
		}
		change := "changed"
		if !existed {
			change = "new"
		}
		out = append(out, codeTransitionDelta(unit.Path, change+" production code unit has no newly issued active semantic memory that explicitly covers it"))
	}

	for key, unit := range priorUnits {
		if _, exists := currentUnits[key]; exists {
			continue
		}
		retired := false
		identities := codeIdentityRecords(unit, priorSet, priorProj)
		if len(identities) == 0 {
			retired = hasFreshCodeRetirement(unit, priorSet, currentSet, currentProj)
		} else {
			for _, id := range identities {
				if currentSet.Get(id) != nil && currentProj.Status(id) != resolve.Active {
					retired = true
					break
				}
			}
		}
		if !retired {
			reason := "removed production code unit has no prior semantic or anchor identity retired by current memory"
			if len(identities) == 0 {
				reason = "removed production code unit had no prior memory identity; add a newly issued active Learning tagged reconcile:code-retirement and retired-code-unit:" + unit.SHA256
			}
			out = append(out, codeTransitionDelta(unit.Path, reason))
		}
	}

	sortBaselineDeltas(out)
	return out
}

func relatedProjection(set *store.Set, proj *resolve.Projection) bool {
	return set != nil && proj != nil && proj.Set == set
}

func codeUnitsByKey(units []codeinventory.Unit) map[string]codeinventory.Unit {
	out := make(map[string]codeinventory.Unit, len(units))
	for _, unit := range units {
		out[codeUnitKey(unit.Kind, unit.Path)] = unit
	}
	return out
}

func semanticCodeRecords(unit codeinventory.Unit, set *store.Set, proj *resolve.Projection) []string {
	return matchingCodeRecords(unit, set, proj, func(r *record.Record) bool { return isSemanticCodeMemory(r) })
}

func codeIdentityRecords(unit codeinventory.Unit, set *store.Set, proj *resolve.Projection) []string {
	return matchingCodeRecords(unit, set, proj, func(r *record.Record) bool { return isSemanticCodeMemory(r) || isCodeAnchor(r) })
}

func hasFreshCodeRetirement(unit codeinventory.Unit, priorSet, currentSet *store.Set, currentProj *resolve.Projection) bool {
	for _, id := range currentSet.IDs() {
		r := currentSet.Get(id)
		if priorSet.Get(id) != nil || currentProj.Status(id) != resolve.Active || r.Kind != record.Learning {
			continue
		}
		if r.HasTag("reconcile:code-retirement") && r.HasTag("retired-code-unit:"+unit.SHA256) {
			return true
		}
	}
	return false
}

func matchingCodeRecords(unit codeinventory.Unit, set *store.Set, proj *resolve.Projection, include func(*record.Record) bool) []string {
	lookup := activeLookup{set: set, proj: proj}
	var ids []string
	for _, id := range set.IDs() {
		r := set.Get(id)
		if proj.Status(id) != resolve.Active || !include(r) {
			continue
		}
		for _, scope := range association.Scopes(r, lookup) {
			if scopeMatchesUnit(scope, unit) {
				ids = append(ids, id)
				break
			}
		}
	}
	sort.Strings(ids)
	return ids
}

func codeTransitionDelta(scope, reason string) BaselineDelta {
	return baselineDelta(BaselineAllowance{}, BaselineAllowance{
		Code: "code-memory-transition", Scope: scope, Count: 1, Blocking: true,
	}, reason)
}
