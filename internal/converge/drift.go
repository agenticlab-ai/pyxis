package converge

// The one drift predicate (AP-11): scanTree classifies every manifest-listed
// file against the tree per ownership class. Run repairs from it, Drift
// exposes it read-only, and doctor (T6) reads the same classification —
// never a second implementation.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

// State classifies one divergent file. Converged files carry no Finding.
type State string

const (
	// StateMissing — the manifest-listed file is absent from the tree. A
	// repair writes it for engine-verbatim, regenerates it for generated,
	// and refuses by name for repo-owned (converge never creates repo-owned
	// content, DINV-3).
	StateMissing State = "missing"
	// StateModified — an engine-verbatim file's sha256 differs from the
	// manifest's pin; repaired byte-exact.
	StateModified State = "modified"
	// StateStale — a generated file's bytes differ from what its generator
	// produces right now; regenerated.
	StateStale State = "stale"
	// StateInvalid — a repo-owned file fails its closed-schema validation;
	// named refusal, file untouched (DINV-3).
	StateInvalid State = "invalid"
	// StateUngeneratable — a generated file whose bytes cannot be computed:
	// no generator is registered for the path, or the generator refuses the
	// corpus (defects). Named refusal, file untouched.
	StateUngeneratable State = "ungeneratable"
)

// Finding is one classified divergence — the drift predicate's output row
// (shared verbatim with doctor, T6).
type Finding struct {
	Path       string
	Class      relmanifest.Class
	State      State
	Detail     string // the named reason for invalid/ungeneratable states; "" otherwise
	WantSHA256 string // the manifest's pinned sha256 (EngineVerbatim findings only; "" otherwise) — RepairEngineVerbatim's target hash
}

// Classification is converge's read-only classification result (exported
// for callers independent of Run, e.g. adopt: existing-repo-onboarding-v2
// §3.1): every divergence the drift predicate found, plus — for the
// generated class — the desired bytes a write needs, computed once so a
// write primitive writes exactly what the scan compared against and never
// re-renders (AP-11).
type Classification struct {
	Findings  []Finding
	Generated map[string][]byte // manifest path → generator output (generated class, generator clean)
}

// scanTree is THE drift predicate: read-only classification of every
// manifest-listed file, findings sorted by path.
//
// archivePath is the verified payload archive a MUTATING run (Run) has ensured;
// it is "" for the read-only faces (Drift/Doctor/preview), which never fetch or
// extract. It is the authoritative source for a generated file that must be
// rendered from an engine-verbatim template the tree may not yet carry: a
// fresh-install converge runs the generated scan BEFORE it materializes the
// engine-verbatim payload, so the installed-workflow template is absent from
// the tree on the first pass. With the archive available, that render falls
// back to the archive's verified template bytes (the SAME bytes engine-verbatim
// repair trusts — DINV-1), so a fresh tree materializes the workflow in ONE
// pass; without it (the read-only faces auditing the tree AS IS), an absent
// template is honestly named as ungeneratable drift.
func scanTree(ctx context.Context, cfg resolve.Config, m relmanifest.Manifest, archivePath string, authoritySeedPending bool) (Classification, error) {
	if err := ctx.Err(); err != nil {
		return Classification{}, err
	}
	var projectErr error
	m, projectErr = ProjectWorkflowManifest(cfg.Root, m, cfg.Integrations)
	if projectErr != nil {
		return Classification{}, projectErr
	}
	sc := Classification{Generated: map[string][]byte{}}
	if f, found := authorityHealth(ctx, cfg, m); found && (!authoritySeedPending || f.State != StateMissing) {
		sc.Findings = append(sc.Findings, f)
	}
	genRefuse := desiredGenerated(cfg.Root, cfg.Repo, m, archivePath, sc.Generated, cfg.Integrations)
	for _, pf := range m.Payload.Files {
		if err := ctx.Err(); err != nil {
			return Classification{}, err
		}
		// Earlier releases incorrectly listed the operator's live authority as
		// engine-verbatim. Its ownership does not depend on that legacy label:
		// the enabled operand's health is checked above as repository-owned,
		// and convergence neither replaces nor creates it. Apply this at the
		// shared scan used by every read/write face.
		if pf.Path == payload.AuthorityContractTarget {
			continue
		}
		abs := treePath(cfg.Root, pf.Path)
		switch pf.Class {
		case relmanifest.EngineVerbatim:
			sum, err := hashFile(abs)
			switch {
			case errors.Is(err, fs.ErrNotExist):
				sc.Findings = append(sc.Findings, Finding{Path: pf.Path, Class: pf.Class, State: StateMissing, WantSHA256: *pf.SHA256})
			case err != nil:
				return Classification{}, fmt.Errorf("hash %s: %w", pf.Path, err)
			case sum != *pf.SHA256:
				sc.Findings = append(sc.Findings, Finding{Path: pf.Path, Class: pf.Class, State: StateModified,
					Detail: fmt.Sprintf("sha256 %s, manifest pins %s", sum, *pf.SHA256), WantSHA256: *pf.SHA256})
			}
		case relmanifest.Generated:
			if reason, refused := genRefuse[pf.Path]; refused {
				sc.Findings = append(sc.Findings, Finding{Path: pf.Path, Class: pf.Class, State: StateUngeneratable, Detail: reason})
				continue
			}
			desired := sc.Generated[pf.Path]
			have, err := os.ReadFile(abs)
			switch {
			case errors.Is(err, fs.ErrNotExist):
				sc.Findings = append(sc.Findings, Finding{Path: pf.Path, Class: pf.Class, State: StateMissing})
			case err != nil:
				return Classification{}, fmt.Errorf("read %s: %w", pf.Path, err)
			case !bytes.Equal(have, desired):
				sc.Findings = append(sc.Findings, Finding{Path: pf.Path, Class: pf.Class, State: StateStale})
			}
		case relmanifest.RepoOwned:
			state, detail, err := validateRepoOwned(cfg.Root, pf.Path)
			if err != nil {
				return Classification{}, err
			}
			if state != "" {
				sc.Findings = append(sc.Findings, Finding{Path: pf.Path, Class: pf.Class, State: state, Detail: detail})
			}
		}
	}
	slices.SortFunc(sc.Findings, func(a, b Finding) int { return strings.Compare(a.Path, b.Path) })
	return sc, nil
}

// desiredGenerated computes every generated file's desired bytes through its
// registered generator, filling genBytes; paths whose bytes cannot be
// computed land in the returned refusal map with the named reason. Two
// generators are registered:
//
//   - the docgate tech-design index (index.md + index.json under the
//     configured tech-designs directory) — the same GenIndexBytes
//     `docgate gen-index` writes, so converge and the verb can never disagree
//     (AP-11). When the design directory does not exist yet (a fresh operand
//     before any design), the SAME generator runs over the empty corpus
//     (techdesigns.GenIndex with zero designs — byte-identical to
//     GenIndexBytes over an existing empty directory), so a fresh tree
//     materializes valid empty indexes rather than refusing.
//   - the installed CI workflow (config-driven-portability-v1 §3.1, FR-4):
//     converge renders it from the SHIPPED template (payload.InstalledWorkflow,
//     the engine-verbatim overlay source that always ships in the operand tree)
//     with branch+remote substituted from the repo config and the bootstrap pins
//     from the verified release manifest, reusing the seed-time substitution
//     mechanism (payload.Substitute, installer-v1 §3.5). A config or release
//     change re-renders it on the next converge; a hand-edit is StateStale drift
//     doctor names and converge repairs (A-6).
//     The one template is the single source (DINV-4/AP-11) — never a second
//     embedded copy; an unreadable template or a failed substitution is a
//     named refusal, never an invented file.
//
// A generated path matching neither generator is a named refusal (converge
// regenerates only through an existing sole generator, never an invented file).
func desiredGenerated(root string, repo config.Config, m relmanifest.Manifest, archivePath string, genBytes map[string][]byte, profiles ...*integrations.Config) map[string]string {
	refuse := map[string]string{}
	var indexPaths []string
	for _, pf := range m.Payload.Files {
		if pf.Class != relmanifest.Generated {
			continue
		}
		if pf.Path == payload.ForgejoWorkflowTarget {
			if b, reason := desiredForgejoWorkflow(root, repo, m, archivePath, profiles...); reason != "" {
				refuse[pf.Path] = reason
			} else {
				genBytes[pf.Path] = b
			}
			continue
		}
		if isIndexPath(repo, pf.Path) {
			indexPaths = append(indexPaths, pf.Path)
			continue
		}
		if isInstalledWorkflowPath(pf.Path) {
			if b, reason := desiredWorkflow(root, repo, m, archivePath, profiles...); reason != "" {
				refuse[pf.Path] = reason
			} else {
				genBytes[pf.Path] = b
			}
			continue
		}
		refuse[pf.Path] = "no generator is registered for this path — converge regenerates only through an existing sole generator, never an invented file"
	}
	if len(indexPaths) == 0 {
		return refuse
	}
	md, js, defects := desiredIndex(root, repo)
	if len(defects) > 0 {
		reason := "the index generator refuses the design corpus: " + summarizeDefects(defects)
		for _, p := range indexPaths {
			refuse[p] = reason
		}
		return refuse
	}
	for _, p := range indexPaths {
		if path.Base(p) == "index.md" {
			genBytes[p] = md
		} else {
			genBytes[p] = js
		}
	}
	return refuse
}

// isIndexPath reports whether a generated manifest path is one of the two
// docgate index artifacts under the configured tech-designs directory.
func isIndexPath(repo config.Config, rel string) bool {
	base := path.Base(rel)
	return (base == "index.md" || base == "index.json") && path.Dir(rel) == path.Clean(repo.TechDesigns)
}

// desiredIndex runs the sole index generator: GenIndexBytes over the tree
// when the design directory exists, the same generator over the empty
// corpus when it does not (zero designs make the env irrelevant — the
// outputs are byte-identical to an existing empty directory's).
func desiredIndex(root string, repo config.Config) (md, js []byte, defects []schema.Defect) {
	dir := filepath.Join(root, filepath.FromSlash(repo.TechDesigns))
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return techdesigns.GenIndex(nil, techdesigns.Env{})
	}
	return gate.GenIndexBytes(root, repo)
}

// isInstalledWorkflowPath reports whether a generated manifest path is the
// installed CI workflow — the operand-side overlay target the class policy
// declares generated (config-driven-portability-v1 §3.1).
func isInstalledWorkflowPath(rel string) bool {
	return rel == payload.InstalledWorkflowTarget || rel == payload.ForgejoWorkflowTarget
}

// hostedWorkflowIncluded reports whether the resolved ci_mode knob places the
// hosted docs-gate workflow in THIS operand's desired set (prd:init-hardening/
// FR-11, design:init-hardening-v1#3.8): "actions" and "both" include it;
// "local" and the uncaptured zero value exclude it. The class-policy row
// itself (.pyxis/class-policy.yaml) stays unconditional — one release manifest
// serves every install mode; this is the consumption-side gate converge/doctor
// apply when they turn that manifest into an operand's desired set.
func hostedWorkflowIncluded(repo config.Config) bool {
	return repo.Knobs.CIMode == "actions" || repo.Knobs.CIMode == "both"
}

// desiredFindings narrows scanTree's classification to this operand's
// knob-conditional desired set (§3.8): every finding scanTree produced, minus
// the hosted workflow's when ci_mode excludes it. Excluded means UNLISTED —
// the caller acts on and reports nothing for that path, and no deletion
// machinery exists anywhere (#D-4; the never-touch complement property). Run,
// preview, and Doctor's drift check narrow through this. Drift and
// ClassifyWithArchive — the shared classification truth existing-repo-
// onboarding-v2's adopt consumes — deliberately do NOT: this is a converge/
// doctor-only policy layer over the one shared predicate (AP-11), not a second
// evaluator of it.
func desiredFindings(findings []Finding, repo config.Config, roots ...string) []Finding {
	if len(roots) > 0 {
		repo = workflowMode(roots[0], repo)
	}
	if hostedWorkflowIncluded(repo) {
		return findings
	}
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if isInstalledWorkflowPath(f.Path) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// desiredWorkflow renders the installed CI workflow from the SHIPPED template
// (payload.InstalledWorkflow — the engine-verbatim overlay source, the SINGLE
// source for the workflow's branch/remote: DINV-4/AP-11, never a second
// embedded copy), substituting branch+remote from the repo config via the
// seed-time substitution mechanism (payload.Substitute, installer-v1 §3.5), and
// the bootstrap pins from m — which its caller verified against
// system.manifest_sha256, so the binary the workflow's shim accepts traces to the
// repository's committed pin. It returns the rendered bytes, or a named refusal
// reason when the template cannot be sourced or a substitution fails — mirroring
// the index generator's ungeneratable posture (never an invented file).
//
// The template source depends on the caller, because the template is an
// engine-verbatim file converge itself owns and may be repairing THIS pass:
//
//   - A MUTATING run (archivePath set, the verified payload archive) renders
//     from the ARCHIVE'S template bytes — the SAME authoritative bytes
//     engine-verbatim repair trusts (DINV-1). This is correct in one pass no
//     matter the tree's template state: a fresh install (whose generated scan
//     runs BEFORE it materializes the engine-verbatim payload) materializes the
//     workflow, and a hand-deleted or hand-MODIFIED tree template cannot
//     corrupt the render (the archive, not the tree, is authoritative).
//   - A read-only face (archivePath "" — Drift/Doctor/preview never fetch or
//     extract) renders from the OPERAND TREE, auditing it AS IS (parallel to
//     desiredIndex reading operand design content); an absent or unreadable
//     tree template is a named refusal — the drift the tree carries.
func desiredWorkflow(root string, repo config.Config, m relmanifest.Manifest, archivePath string, profiles ...*integrations.Config) (rendered []byte, reason string) {
	var tmpl []byte
	if archivePath != "" {
		// Mutating run: the verified archive is authoritative.
		got, err := extractFiles(archivePath, map[string]bool{payload.InstalledWorkflow: true})
		ent, ok := got[payload.InstalledWorkflow]
		if err != nil || !ok {
			return nil, fmt.Sprintf("the verified payload archive carries no installed-workflow template %s (%v) — the pinned release metadata is defective; pin a coherent release", payload.InstalledWorkflow, err)
		}
		tmpl = ent.b
	} else {
		// Read-only face: audit the tree's template as it stands.
		b, err := os.ReadFile(treePath(root, payload.InstalledWorkflow))
		if err != nil {
			return nil, fmt.Sprintf("the installed-workflow template %s cannot be read from the tree: %v — run `pyxis converge` to materialize/repair it (it ships engine-verbatim) so the workflow can be rendered", payload.InstalledWorkflow, err)
		}
		tmpl = b
	}
	var override *integrations.Config
	if len(profiles) > 0 {
		override = profiles[0]
	}
	source, err := resolve.RepositorySource(resolve.Config{Root: root, Repo: repo, Integrations: override}, m.SystemVersion)
	if err != nil {
		return nil, fmt.Sprintf("the installed workflow's artifact source cannot be resolved: %v", err)
	}
	checksums := m.Binaries["pyxis"]
	out, err := payload.RenderInstalledWorkflow(tmpl, repo.Branch, repo.Remote, payload.WorkflowPins{
		Version:        m.SystemVersion,
		LinuxARM64:     checksums["linux-arm64"],
		LinuxAMD64:     checksums["linux-amd64"],
		ArtifactSource: source,
	})
	if err != nil {
		return nil, fmt.Sprintf("rendering the installed workflow from %s failed: %v — the template takes branch and remote from the repo config and the version and both Linux pyxis hashes from the release manifest; a coherent release pins them all", payload.InstalledWorkflow, err)
	}
	return out, ""
}

// validateRepoOwned validates one repo-owned file WITHOUT ever writing it
// (DINV-3). The validator registry is closed and keyed by path:
//
//   - .pyxis/config.yaml — must parse under the closed pyxis-config schema
//     (internal/docgate/config.Parse; unknown keys are errors).
//   - everything else (today .pyxis/stamp/**) — existence-only: no schema
//     exists for those files, so presence is the only falsifiable check
//     (documented posture; doctor surfaces the same).
//
// Absence is a refusal-grade state for every repo-owned path: converge may
// not create repo-owned content, so the remedy is the operator's.
func validateRepoOwned(root, rel string) (State, string, error) {
	abs := treePath(root, rel)
	if rel == ".pyxis/config.yaml" {
		b, err := os.ReadFile(abs)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return StateMissing, "repo-owned file is absent and converge never writes repo-owned content (DINV-3); restore it from your history or re-run init", nil
		case err != nil:
			return "", "", fmt.Errorf("read %s: %w", rel, err)
		}
		if _, defects := config.Parse(b); len(defects) > 0 {
			return StateInvalid, "closed-schema parse failed: " + summarizeDefects(defects) + " — repo-owned content is never rewritten (DINV-3); fix the file by hand", nil
		}
		return "", "", nil
	}
	_, err := os.Stat(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return StateMissing, "repo-owned file is absent and converge never writes repo-owned content (DINV-3); restore it from your history or re-run init", nil
	case err != nil:
		return "", "", fmt.Errorf("stat %s: %w", rel, err)
	}
	return "", "", nil
}

// summarizeDefects renders at most three defects for a named refusal.
func summarizeDefects(defects []schema.Defect) string {
	const limit = 3
	var parts []string
	for i, d := range defects {
		if i == limit {
			parts = append(parts, fmt.Sprintf("(+%d more)", len(defects)-limit))
			break
		}
		parts = append(parts, d.String())
	}
	return strings.Join(parts, "; ")
}

// treePath maps a manifest's slash-relative path onto the operand tree.
func treePath(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// hashFile streams one file through sha256; fs.ErrNotExist passes through
// for the missing branch.
func hashFile(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }() // read-only handle
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
