// Package lifecycle owns the open-draft and locked-version state machine.
package lifecycle

import (
	"sort"
)

const (
	DraftingFirst = "drafting first version"
	Locked        = "locked"
	DraftingNext  = "drafting next version while prior version binds"
	Documents     = "document pipeline"
)

// Snapshot is a read-only view of the three authoritative stores used by the
// status command: open draft pull requests, lock tags, and PRD ledgers.
type Snapshot struct {
	OpenDrafts     map[string]bool
	LockedVersions map[string]int
	PRDLedgers     map[string]bool
}

type State struct {
	Feature string `json:"feature"`
	Kind    string `json:"kind"`
	Version int    `json:"version,omitempty"`
}

// States derives one contract state per known feature. An open draft takes
// precedence over its previous lock; a PRD ledger is always in the document
// pipeline because existing features are not migrated into drafts.
func States(s Snapshot) []State {
	names := map[string]bool{}
	for feature := range s.OpenDrafts {
		names[feature] = true
	}
	for feature := range s.LockedVersions {
		names[feature] = true
	}
	for feature := range s.PRDLedgers {
		names[feature] = true
	}
	out := make([]State, 0, len(names))
	for feature := range names {
		state := State{Feature: feature}
		switch {
		case s.PRDLedgers[feature]:
			state.Kind = Documents
		case s.OpenDrafts[feature] && s.LockedVersions[feature] > 0:
			state.Kind, state.Version = DraftingNext, s.LockedVersions[feature]+1
		case s.OpenDrafts[feature]:
			state.Kind, state.Version = DraftingFirst, 1
		case s.LockedVersions[feature] > 0:
			state.Kind, state.Version = Locked, s.LockedVersions[feature]
		default:
			continue
		}
		out = append(out, state)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Feature < out[j].Feature })
	return out
}
