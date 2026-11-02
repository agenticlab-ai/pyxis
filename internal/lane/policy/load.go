package policy

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Lineage is the load-time ratified-provenance read (DINV-8, DINV-9): given
// the on-disk policy content, does it match the trailer-ratified mainline
// lineage? It is the one medium-touching seam in this package — the acting
// layer's thin read surface (merge-lane-v1 §3.8) — so readers substitute it
// locally in tests while production wires the git-backed reader. ok=false
// carries a named reason/remedy for the "unratified" defect.
type Lineage interface {
	Ratified(content []byte) (ok bool, reason, remedy string)
}

// ratificationTrailer is the operator gate-policy trailer, widened to the
// risk-policy home (merge-lane-v1 §3.6) — the same attestation docgate
// already enforces on the governing surface.
const ratificationTrailer = "Governing-Change: operator"

// Load is the single validator every reader shares (plan T1): it resolves
// the policy path from the config surface, reads the file, runs the closed
// parse, the design-bound validation, and the ratified-lineage read against
// the repo's mainline context. A malformed, unknown-key, self-unlock-
// uncovering, build-input-claiming, unratified, or absent policy returns no
// policy and a named defect — the caller then treats every proposal HUMAN
// (DINV-8). A clean policy returns with no defects.
func Load(root string) (*Policy, []schema.Defect) {
	cfg, _ := config.Load(root)
	rel := cfg.RiskPolicy
	return LoadFile(root, rel, gitLineage{root: root, rel: rel, remote: cfg.Remote, branch: cfg.Branch})
}

// LoadFile reads root/rel, parses and validates it, and consults lin for the
// ratified-lineage read. It is Load's core and the substitution point for
// hermetic tests. A nil lin skips the lineage read — used only where a caller
// already holds trusted-context content (never the production path).
func LoadFile(root, rel string, lin Lineage) (*Policy, []schema.Defect) {
	stamp := func(ds []schema.Defect) []schema.Defect {
		for i := range ds {
			ds[i].File = rel
		}
		return ds
	}

	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, stamp([]schema.Defect{def("load",
				"the risk policy is absent",
				"an absent policy is a load defect (the bootstrap/decommission state): the lane stays all-HUMAN until a ratified policy lands (DINV-8)")})
		}
		return nil, stamp([]schema.Defect{def("load",
			fmt.Sprintf("the risk policy is unreadable: %v", err),
			"fix permissions or restore the file; the lane stays all-HUMAN until it loads (DINV-8)")})
	}

	p, defects := Parse(b)
	defects = append(defects, Validate(p)...)
	if len(defects) > 0 {
		return nil, stamp(defects)
	}

	if lin != nil {
		if ok, reason, remedy := lin.Ratified(b); !ok {
			return nil, stamp([]schema.Defect{def("lineage", reason, remedy)})
		}
	}

	return p, nil
}

// gitLineage is the production ratified-lineage reader: the on-disk policy is
// ratified only when it is byte-identical to the policy on the mainline
// lineage AND that lineage's latest change to the policy carries the operator
// ratification trailer. A local un-landed edit, a policy absent from the
// mainline, or a change that skipped the trailer all read unratified — the
// fail-closed provenance bar (DINV-8, DINV-9).
type gitLineage struct {
	root   string
	rel    string
	ref    string // mainline ref; empty → resolved from remote+branch (<remote>/<branch>, then bare <branch>)
	remote string // the configured git remote (config-driven-portability-v1 §3.2, FR-2); blank → "origin"
	branch string // the configured default branch (FR-2); blank → "main"
}

func (g gitLineage) Ratified(content []byte) (bool, string, string) {
	ref := g.ref
	if ref == "" {
		ref = resolveMainline(g.root, g.remote, g.branch)
	}
	if ref == "" {
		return false, "cannot resolve the mainline ref to verify ratified lineage",
			"run in a repository whose configured mainline (<remote>/<branch> or <branch>) is present; the lane stays all-HUMAN otherwise (DINV-9)"
	}
	mainlineContent, err := gitShow(g.root, ref, g.rel)
	if err != nil {
		return false, fmt.Sprintf("the risk policy is absent from the ratified mainline lineage %q", ref),
			"land the policy on the mainline through the operator ratification trailer before it is trusted (DINV-9)"
	}
	if !bytes.Equal(schema.Normalize(content), schema.Normalize(mainlineContent)) {
		return false, "the working-tree risk policy diverges from the ratified mainline content (an un-landed edit)",
			"a policy is trusted only from its trailer-ratified mainline lineage; land the change through ratification (DINV-8, DINV-9)"
	}
	msg, err := gitLog1(g.root, ref, g.rel)
	if err != nil || !strings.Contains(msg, ratificationTrailer) {
		return false, fmt.Sprintf("the policy's latest mainline change lacks the `%s` ratification trailer", ratificationTrailer),
			"every policy change lands only with the operator's recorded ratification trailer (merge-lane-v1 §3.6, AP-24)"
	}
	return true, "", ""
}

// resolveMainline returns the first of "<remote>/<branch>", "<branch>" that git
// can resolve under root, or "" if neither exists. A blank remote/branch takes
// the config schema's single source (config.Defaults — the ONE place the
// origin/main literals live, DINV-1), so a default config probes "origin/main"
// then "main" exactly as before (config-driven-portability-v1 §3.2, FR-2; DINV-2).
func resolveMainline(root, remote, branch string) string {
	if remote == "" {
		remote = config.Defaults().Remote
	}
	if branch == "" {
		branch = config.Defaults().Branch
	}
	for _, ref := range []string{remote + "/" + branch, branch} {
		cmd := exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", ref)
		if err := cmd.Run(); err == nil {
			return ref
		}
	}
	return ""
}

func gitShow(root, ref, rel string) ([]byte, error) {
	return exec.Command("git", "-C", root, "show", ref+":"+rel).Output()
}

func gitLog1(root, ref, rel string) (string, error) {
	out, err := exec.Command("git", "-C", root, "log", "-1", "--format=%B", ref, "--", rel).Output()
	return string(out), err
}
