package baseline

import "sort"

// Result is rung 2's no-new-failures evaluation outcome (design §3.6/§3.7):
// NewFailures are observed identities absent from the baseline — these red
// the rung; Shrunk are baseline identities this run did not reproduce — the
// ratchet's only permitted direction of change, recorded as observed
// shrinkage regardless of whether the rung otherwise passes or fails.
type Result struct {
	NewFailures []string
	Shrunk      []string
}

// Evaluate compares a run's observed failure identities against a bound
// baseline. Pure: no IO, deterministic (sorted) output.
func Evaluate(observed []string, b *Baseline) Result {
	baselineSet := toSet(b.Identities)
	observedSet := toSet(observed)

	newSet := map[string]bool{}
	for id := range observedSet {
		if !baselineSet[id] {
			newSet[id] = true
		}
	}
	shrunkSet := map[string]bool{}
	for id := range baselineSet {
		if !observedSet[id] {
			shrunkSet[id] = true
		}
	}

	return Result{NewFailures: sortedKeys(newSet), Shrunk: sortedKeys(shrunkSet)}
}

func toSet(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func sortedKeys(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
