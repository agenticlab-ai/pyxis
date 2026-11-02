package writehook

import (
	"fmt"
	"io"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// Adapter wires one supported agent CLI's own pre-write hook to the canonical
// Decide (§3.2). It is I/O translation only: it parses that CLI's real hook
// request into a target, and renders a Decision in that CLI's real deny/allow
// contract. It holds NO protected-path knowledge and NO deny logic — those
// live once in Decide/boundary, so the portability check finds each adapter a
// pointer, not a content copy (DINV-5, AP-25).
//
// A CLI whose pre-write hook cannot express a fail-closed deny sets
// Honoring=false and names its Backstop — the merge-boundary layers that are
// its stated guarantee — so it is listed, never silently failing open
// (DINV-4, UB-2). A non-honoring adapter leaves the I/O funcs nil.
type Adapter struct {
	// Name is the CLI's vendor-neutral key (e.g. "claude", "codex"), selected
	// by the shim's --cli flag.
	Name string
	// Hook describes the CLI's own pre-write hook surface this adapter wires
	// (for the guarantee listing and the pointer docs).
	Hook string
	// Honoring reports whether the CLI exposes a honoring, fail-closable
	// pre-write hook. When false, Backstop is its stated guarantee.
	Honoring bool
	// Backstop names the merge-boundary layers that guarantee a non-honoring
	// CLI (empty for a honoring CLI).
	Backstop string

	// parse extracts the pending write's repo-relative target from the CLI's
	// real hook request bytes. A parse failure is a hook-runtime error the shim
	// maps to a fail-closed deny (DINV-3). Nil for a non-honoring CLI.
	parse func(request []byte) (target string, err error)
	// renderDeny writes the CLI's real deny response for d to w and returns the
	// process exit code that blocks the write on this CLI. Nil for non-honoring.
	renderDeny func(w io.Writer, d Decision) (exit int)
	// renderAllow writes the CLI's real allow/proceed response and returns its
	// exit code. Nil for non-honoring.
	renderAllow func(w io.Writer, d Decision) (exit int)
	// failClosedExit is the exit code that blocks the write when the adapter
	// cannot even render a normal deny (a hook-runtime error). It is the CLI's
	// hard-deny exit — the explicit fail-closed setting for a surface whose
	// default is fail-open. Nil-render (non-honoring) adapters leave it 0.
	failClosedExit int
}

// registry holds every registered adapter, keyed by Name. Each adapter file
// (adapter_<cli>.go) self-registers in its init, so onboarding a CLI is a new
// file — never an edit to a shared enumeration line (AP-27, NFR-3).
var registry = map[string]*Adapter{}

// register adds an adapter to the registry; a duplicate name is a programming
// error (two files claimed one CLI).
func register(a *Adapter) {
	if _, dup := registry[a.Name]; dup {
		panic(fmt.Sprintf("writehook: adapter %q registered twice", a.Name))
	}
	registry[a.Name] = a
}

// Lookup returns the adapter for a CLI name, or (nil, false) when none is
// registered — the shim maps an unknown CLI to a fail-closed refusal, never a
// silent allow.
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

// Evaluate runs one pending write through this adapter fail-closed and writes
// the CLI's real response to w, returning the process exit code (§3.2, §3.3).
// Every error path — a non-honoring CLI, an unparseable request, a target the
// parse could not read — denies; nothing here fails open (DINV-3, DINV-4). A
// classified deny renders the CLI's deny contract; only an in-scope target
// allows. cfg carries the target-facing doc roots the classification is keyed
// on (config-driven-portability-v1 §3.2, FR-1).
func (a *Adapter) Evaluate(cfg config.Config, request []byte, drive boundary.Drive, w io.Writer) int {
	if !a.Honoring || a.parse == nil || a.renderDeny == nil {
		// A non-honoring CLI has no fail-closable hook: its guarantee is the
		// merge-boundary backstop, and any evaluation attempt denies rather
		// than pretend to enforce (never a silent fail-open; DINV-4, UB-2).
		d := DenyForHookError("", fmt.Errorf("CLI %q exposes no fail-closable pre-write hook; its guarantee is %s", a.Name, a.Backstop))
		return a.failClosed(w, d)
	}
	target, err := a.parse(request)
	if err != nil {
		return a.failClosed(w, DenyForHookError(target, err))
	}
	d := Decide(cfg, target, drive)
	if d.Allow {
		return a.renderAllow(w, d)
	}
	return a.renderDeny(w, d)
}

// failClosed emits a hook-runtime deny in the CLI's contract when it can, else
// falls back to the CLI's hard-deny exit code with the reason on the writer —
// the write is blocked either way (DINV-3).
func (a *Adapter) failClosed(w io.Writer, d Decision) int {
	if a.renderDeny != nil {
		return a.renderDeny(w, d)
	}
	fmt.Fprintln(w, d.Reason())
	if a.failClosedExit != 0 {
		return a.failClosedExit
	}
	return 2
}
