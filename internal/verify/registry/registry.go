package registry

import (
	"fmt"
	"sort"
	"sync"

	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// Registry holds all registered checks and derives the ladder from them.
var (
	mu       sync.RWMutex
	checks   = make(map[string]*schema.Check)
	ladder   []*Rung
	ladderOk = false
)

// rungOrder is the canonical, closed set of ladder rungs and their execution
// order (both the numeric id and the name resolve to one order). A check whose
// Rung is absent from this table is rejected at registration — an unrecognized
// rung must not silently sort into the instant band (#402 review).
var rungOrder = map[string]int{
	"0": 0, "instant": 0,
	"1": 1, "build": 1,
	"2": 2, "tests": 2,
	"3": 3, "agent-code": 3,
	"4": 4, "reality": 4,
}

// Rung is the ladder's level with its checks in order.
type Rung struct {
	Name   string
	Order  int
	Checks []*schema.Check
}

// Register adds a check to the global registry.
func Register(check *schema.Check) {
	mu.Lock()
	defer mu.Unlock()

	if check.ID == "" {
		panic("check registration: empty ID")
	}
	if check.Rung == "" {
		panic("check registration: empty rung")
	}
	if _, known := rungOrder[check.Rung]; !known {
		panic(fmt.Sprintf("check registration: unknown rung %q for check %s", check.Rung, check.ID))
	}
	if _, exists := checks[check.ID]; exists {
		panic(fmt.Sprintf("check registration: duplicate ID %s", check.ID))
	}

	checks[check.ID] = check
	ladderOk = false
}

// GetLadder returns the derived ladder with checks ordered by rung.
func GetLadder() []*Rung {
	mu.RLock()

	// Gate on ladderOk alone: a legitimately-empty ladder (no checks registered)
	// is still a valid cached result. Adding `len(ladder) > 0` here would defeat
	// the cache for an empty registry, recomputing under the write lock on every
	// call (#402 review).
	if ladderOk {
		defer mu.RUnlock()
		return copyLadder()
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()

	ladder = deriveLadder()
	ladderOk = true
	return copyLadder()
}

func deriveLadder() []*Rung {
	rungMap := make(map[string]*Rung)

	for _, check := range checks {
		r, ok := rungMap[check.Rung]
		if !ok {
			r = &Rung{
				Name:   check.Rung,
				Checks: []*schema.Check{},
			}
			rungMap[check.Rung] = r
		}
		r.Checks = append(r.Checks, check)
	}

	var rungs []*Rung
	for _, r := range rungMap {
		r.Order = rungOrder[r.Name] // r.Name is a known rung (validated at Register)
		rungs = append(rungs, r)
	}

	sort.Slice(rungs, func(i, j int) bool {
		if rungs[i].Order != rungs[j].Order {
			return rungs[i].Order < rungs[j].Order
		}
		return rungs[i].Name < rungs[j].Name
	})

	for _, r := range rungs {
		sort.Slice(r.Checks, func(i, j int) bool {
			return r.Checks[i].ID < r.Checks[j].ID
		})
	}

	return rungs
}

func copyLadder() []*Rung {
	result := make([]*Rung, len(ladder))
	for i, r := range ladder {
		rung := &Rung{
			Name:   r.Name,
			Order:  r.Order,
			Checks: make([]*schema.Check, len(r.Checks)),
		}
		copy(rung.Checks, r.Checks)
		result[i] = rung
	}
	return result
}

// GetCheck retrieves a registered check by ID.
func GetCheck(id string) *schema.Check {
	mu.RLock()
	defer mu.RUnlock()
	return checks[id]
}

// AllChecks returns all registered checks.
func AllChecks() []*schema.Check {
	mu.RLock()
	defer mu.RUnlock()

	result := make([]*schema.Check, 0, len(checks))
	for _, check := range checks {
		result = append(result, check)
	}
	return result
}
