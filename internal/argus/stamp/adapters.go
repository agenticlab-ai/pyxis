package stamp

import (
	"fmt"
	"sort"
)

// Adapter wires one supported agent CLI's own commit-authoring path to the
// canonical stamp (§3.1, AP-25). Where a CLI authors commits, the producer is
// wired to that path by a pointer — a config that routes the CLI's commits
// through the canonical stamp (cmd/argus-stamp), never a re-implementation of
// the trailer logic. Onboarding a CLI is a new adapter file
// (adapter_<cli>.go + a .pyxis/stamp pointer config), extending by addition —
// never editing the shared stamp decision (AP-27, NFR-3).
//
// The adapter holds NO trailer logic and names NO protected path: it is
// registry metadata describing the CLI's commit path and where the producer is
// wired for it, single-sourcing the `argus-stamp paths` listing so the wiring
// is discoverable and never silently absent.
type Adapter struct {
	// Name is the CLI's vendor-neutral key (e.g. "claude") — the same key the
	// write-hook adapter uses for the CLI.
	Name string
	// CommitPath describes how this CLI authors commits — the surface the
	// producer must reach to stamp them.
	CommitPath string
	// Wiring names the pointer config that routes this CLI's commit path
	// through the canonical stamp (a pointer, never a content copy).
	Wiring string
}

// registry holds every registered stamp adapter, keyed by Name. Each adapter
// file (adapter_<cli>.go) self-registers in its init, so onboarding a CLI is a
// new file — never an edit to a shared enumeration line (AP-27).
var registry = map[string]*Adapter{}

// register adds an adapter to the registry; a duplicate name is a programming
// error (two files claimed one CLI).
func register(a *Adapter) {
	if _, dup := registry[a.Name]; dup {
		panic(fmt.Sprintf("stamp: adapter %q registered twice", a.Name))
	}
	registry[a.Name] = a
}

// Lookup returns the adapter for a CLI name, or (nil, false) when none is
// registered.
func Lookup(name string) (*Adapter, bool) {
	a, ok := registry[name]
	return a, ok
}

// Adapters returns every registered adapter, name-sorted for stable output.
func Adapters() []*Adapter {
	out := make([]*Adapter, 0, len(registry))
	for _, a := range registry {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
