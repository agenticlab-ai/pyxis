// Package surfaces answers one question an initialized repository cannot
// otherwise re-ask: which agent CLIs should get pyxis's skills?
//
// init resolves that once. Its `auto` answer is a detect sentinel expanded by
// probing PATH at init time and never stored, so a machine carrying no agent
// CLI records an empty set — legal, and permanent. Installing Codex or Cursor
// afterwards changes nothing: converge materializes the configured set, and an
// empty set materializes nothing. The only route was to hand-edit the config,
// which nothing told the operator about.
//
// This package is that route, made a verb: report what is installed and what
// is wired, take a selection, write the knob, and reconcile. Selection is
// deliberately not limited to what is installed — an operator provisioning a
// repo for a team wires surfaces their own machine has never seen.
package surfaces

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// lookPath is the PATH probe seam; tests set PATH and exercise the real thing.
var lookPath = exec.LookPath

// Supported is every surface this build ships wiring for. One catalog, owned
// by the package that holds the pointers — a name with no convention wires
// nothing, so it could not honestly be called supported.
func Supported() []string { return converge.SupportedSurfaces() }

// Detect reports which supported surfaces are on PATH, sorted. An empty result
// is ordinary: a machine with no agent CLI installed.
func Detect() []string {
	var out []string
	for _, s := range Supported() {
		if _, err := lookPath(s); err == nil {
			out = append(out, s)
		}
	}
	return out
}

// Status is what the repository and the machine currently say.
type Status struct {
	Supported  []string // every surface this build can wire
	Installed  []string // those found on PATH
	Configured []string // those knobs.cli_surfaces names today
}

// Row is one line of the report: a supported surface and its two states.
type Row struct {
	Name      string
	Installed bool
	Wired     bool
}

// Rows renders the status as one row per supported surface, plus a row for any
// configured name that is NOT supported — a typo, or a surface from a newer
// release. Those wire nothing and would otherwise be invisible: the knob's
// value set is open, so nothing rejects them.
func (s Status) Rows() []Row {
	inst := set(s.Installed)
	wired := set(s.Configured)
	var out []Row
	for _, n := range s.Supported {
		out = append(out, Row{Name: n, Installed: inst[n], Wired: wired[n]})
	}
	known := set(s.Supported)
	var extra []string
	for _, n := range s.Configured {
		if !known[n] {
			extra = append(extra, n)
		}
	}
	sort.Strings(extra)
	for _, n := range extra {
		// Wired by construction: these came out of the configured list. They
		// wire nothing, which Unsupported reports separately — the row says
		// the config names them, not that they work.
		out = append(out, Row{Name: n, Installed: inst[n], Wired: true})
	}
	return out
}

// Unsupported is the configured names this build has no wiring for.
func (s Status) Unsupported() []string {
	known := set(s.Supported)
	var out []string
	for _, n := range s.Configured {
		if !known[n] {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// Read gathers the status for the repository at root.
func Read(cfg config.Config) Status {
	return Status{Supported: Supported(), Installed: Detect(), Configured: cfg.Knobs.CLISurfaces}
}

// Resolve turns a selection into the surface set to write. names may be any
// supported surface, installed or not; unknown names are refused by name
// rather than accepted into a knob that would silently wire nothing.
func Resolve(names []string) ([]string, error) {
	known := set(Supported())
	seen := map[string]bool{}
	var out []string
	for _, raw := range names {
		n := strings.ToLower(strings.TrimSpace(raw))
		if n == "" {
			continue
		}
		if !known[n] {
			return nil, fmt.Errorf("surfaces: %q is not a surface pyxis wires — supported: %s", raw, strings.Join(Supported(), ", "))
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out, nil
}

// Apply writes the selection to the repo config and materializes the surfaces,
// returning the actions converge performed. The config write comes first so a
// failed reconcile leaves a config an ordinary `pyxis converge` can finish
// from, rather than pointers no config explains.
func Apply(root string, cfg config.Config, selected []string) ([]converge.Action, error) {
	if err := config.SetCLISurfaces(root, selected); err != nil {
		return nil, err
	}
	cfg.Knobs.CLISurfaces = selected
	actions, err := converge.ReconcileHarnessSurfaces(root, cfg)
	if err != nil {
		return nil, fmt.Errorf("surfaces: materialize: %w (the config is written; `pyxis converge` will finish)", err)
	}
	return actions, nil
}

func set(in []string) map[string]bool {
	m := make(map[string]bool, len(in))
	for _, s := range in {
		m[s] = true
	}
	return m
}
