// This file owns the unwired-packages detector (maintenance-sweep-v1 §3.2): the
// dead-feature rot class — a feature's organs landing reachable from no shipped
// binary — detected mechanically. First-party packages are the module's own
// package list; shipped reachability is the union of the import graphs of every
// main under the binaries' tree; a first-party non-main package outside that
// union is unwired unless a package waiver names it, in which case the finding is
// suppressed and the applied waiver surfaces with its recorded reason (DINV-5,
// prd FR-2/FR-10). The detector is a pure function of (evaluated state, policy):
// it reads only through the runner seam, performs no write (DINV-1), and returns
// a named in-band fault rather than an error out-of-band (DINV-4). Stdlib only.
package sweep

import (
	"context"
	"sort"
	"strings"
)

// binariesTree is the go pattern naming the shipped binaries' tree — the mains
// whose import-graph union defines shipped reachability (design §3.2, shape:
// `go list -deps ./cmd/...`).
const binariesTree = "./cmd/..."

const (
	unwiredEvidence = "reachable from no binary under cmd/ (import-graph union)"
	unwiredRemedy   = "wire it into a shipped binary, or add a reasoned package waiver"
)

// DetectUnwired is the unwired-packages detector. It computes the module's
// first-party non-main package list and the first-party subset of the import
// graphs of every main under the binaries' tree; each first-party non-main
// package outside that reaching set is one finding carrying the module-relative
// package path as subject, the empty-reaching-set evidence, and the wire-or-waive
// remedy — unless a package waiver names it, in which case no finding emits and
// the applied waiver is surfaced with its reason (DINV-5). Any toolchain read
// that fails names an in-band fault and no finding emits (DINV-3/DINV-4).
//
// DetectUnwired satisfies the shared Detector signature (design §3.2).
func DetectUnwired(ctx context.Context, state State, policy Policy) DetectorResult {
	res := DetectorResult{Name: DetectorUnwiredPackages}

	module, fault := modulePath(ctx, state)
	if fault != "" {
		res.Fault = fault
		return res
	}

	firstParty, fault := firstPartyPackages(ctx, state, module)
	if fault != "" {
		res.Fault = fault
		return res
	}

	reaching, fault := reachingSet(ctx, state, module)
	if fault != "" {
		res.Fault = fault
		return res
	}

	// Index the package waivers so suppression is a single lookup per package;
	// each applied waiver is surfaced exactly once, with its recorded reason.
	waivers := make(map[string]string, len(policy.PackageWaivers))
	for _, w := range policy.PackageWaivers {
		waivers[w.Package] = w.Reason
	}

	for _, pkg := range firstParty {
		if reaching[pkg.importPath] {
			continue // a shipped binary reaches it — wired, not rot
		}
		subject := pkg.relative
		if reason, waived := waivers[subject]; waived {
			res.WaiversApplied = append(res.WaiversApplied, AppliedWaiver{
				Kind:    WaiverKindPackage,
				Subject: subject,
				Reason:  reason,
			})
			continue // suppression is visible (DINV-5), never a silent drop
		}
		// NewFinding enforces the four-field contract; the fields here are all
		// non-empty, so the error is unreachable — but honor it rather than
		// discard a finding silently.
		f, err := NewFinding(DetectorUnwiredPackages, subject, unwiredEvidence, unwiredRemedy)
		if err != nil {
			res.Fault = "finding construction: " + err.Error()
			return res
		}
		res.Findings = append(res.Findings, f)
	}

	sortBySubject(res.Findings, res.WaiversApplied)
	return res
}

// firstPartyPkg is one first-party non-main package: its full import path (the
// reaching-set key) and its module-relative path (the finding subject and the
// waiver-match key — the historical, human-facing name like "internal/lane").
type firstPartyPkg struct {
	importPath string
	relative   string
}

// modulePath reads the main module's import path via `go list -m`. A read
// failure (an unreadable repo or absent toolchain) is a named fault so the pass
// never claims cleanliness over a repo it could not evaluate (prd UB-1).
func modulePath(ctx context.Context, state State) (module, fault string) {
	lines, err := state.Runner.Run(ctx, state.Root, "go", "list", "-m")
	if err != nil {
		return "", "reading the module path (go list -m): " + err.Error()
	}
	if len(lines) == 0 {
		return "", "reading the module path (go list -m): no module path reported"
	}
	return strings.TrimSpace(lines[0]), ""
}

// firstPartyPackages lists the module's own packages and keeps the non-main
// ones — main packages are binaries themselves, never unwired. Each kept package
// carries its full import path and its module-relative path.
func firstPartyPackages(ctx context.Context, state State, module string) ([]firstPartyPkg, string) {
	lines, err := state.Runner.Run(ctx, state.Root, "go", "list", "-f", "{{.ImportPath}} {{.Name}}", "./...")
	if err != nil {
		return nil, "listing first-party packages (go list ./...): " + err.Error()
	}
	var out []firstPartyPkg
	for _, line := range lines {
		importPath, name, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || importPath == "" {
			continue
		}
		if name == "main" {
			continue // a main is a shipped binary, not a candidate for unwired
		}
		out = append(out, firstPartyPkg{
			importPath: importPath,
			relative:   moduleRelative(importPath, module),
		})
	}
	return out, ""
}

// reachingSet is the first-party subset of the union of the import graphs of
// every main under the binaries' tree — the packages a shipped binary reaches.
// `go list -deps ./cmd/...` returns each dependency once, including stdlib and
// external modules; only the first-party (module-prefixed) entries are retained.
func reachingSet(ctx context.Context, state State, module string) (map[string]bool, string) {
	lines, err := state.Runner.Run(ctx, state.Root, "go", "list", "-deps", "-f", "{{.ImportPath}}", binariesTree)
	if err != nil {
		return nil, "reading shipped reachability (go list -deps ./cmd/...): " + err.Error()
	}
	reaching := make(map[string]bool, len(lines))
	for _, line := range lines {
		pkg := strings.TrimSpace(line)
		if isFirstParty(pkg, module) {
			reaching[pkg] = true
		}
	}
	return reaching, ""
}

// isFirstParty reports whether pkg belongs to the main module — the module path
// itself or a package beneath it — so external and stdlib deps are excluded.
func isFirstParty(pkg, module string) bool {
	return pkg == module || strings.HasPrefix(pkg, module+"/")
}

// moduleRelative strips the module prefix so the subject is the historical,
// human-facing name (e.g. "internal/lane"); the module root maps to ".".
func moduleRelative(importPath, module string) string {
	if importPath == module {
		return "."
	}
	if rel := strings.TrimPrefix(importPath, module+"/"); rel != importPath {
		return rel
	}
	return importPath
}

// sortBySubject orders findings and applied waivers lexicographically by subject
// so the detector's returned value is deterministic before the report's own
// canonical ordering re-applies it (DINV-2).
func sortBySubject(findings []Finding, waivers []AppliedWaiver) {
	sort.SliceStable(findings, func(i, j int) bool { return findings[i].Subject < findings[j].Subject })
	sort.SliceStable(waivers, func(i, j int) bool { return waivers[i].Subject < waivers[j].Subject })
}
