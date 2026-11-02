// Package selfhost is T6's acceptance harness: the self-host proof (A-1), the
// coherence inspection (A-2), and the default-config non-regression assertions
// (A-3) for config-driven portability (design config-driven-portability-v1
// §3.8; plan docs/plans/config-driven-portability/001 T6; prd:config-driven-
// portability/NR-1; #DINV-1, #DINV-2, #A-1, #A-2, #A-3).
//
// It is a NEW e2e-style sibling package rather than a file inside
// internal/converge (the plan's Touches names internal/converge) because the
// self-host proof is a whole-corpus, git-backed, subprocess-optional harness in
// the exact shape of internal/installer/e2e — a peer to that harness, not a
// unit test of the converge library. Placing it here keeps internal/converge a
// focused library package and avoids importing the whole gate + git-repo
// scaffolding into it. The scaffolding is self-contained; it deliberately does
// NOT reuse internal/installer/e2e's harness (that package builds release
// fixtures and drives the packaged exec path — a different deployment context),
// so neither package perturbs the other.
//
// # What the self-host proof does (A-1)
//
// It materializes a THROWAWAY copy of this repository's REAL tracked corpus
// (git archive HEAD | tar -x — the real corpus, so a literal missed anywhere
// that coincidentally matches a default is caught on real content, R1), applies
// a NON-DEFAULT config that varies every target-facing surface §3.8 names (the
// doc corpora moved, the default branch renamed, an explicit remote set, the
// surface pointers moved), PHYSICALLY relocates the corpus to match, sets up the
// git state the full gate's history checks need (a real repo with the non-
// default remote and a merge base over the relocated tree), and runs the full
// doc gate + `doctor` + the branch/remote runtime resolution — asserting green.
//
// # -short gating
//
// The proof archives the real corpus and runs the full gate over ~27 ledger
// directories (seconds, not milliseconds). It is gated behind !testing.Short():
// `go test -short ./...` skips it fast, the full `go test ./...` runs it.
// The coherence inspection (A-2) and the byte-identity assertions (A-3) are pure
// and cheap, so they run even under -short.
//
// Hermetic throughout: a t.TempDir() tree, a local git repo whose "remote" is
// the tree itself (no network), no gh, no release fetch — the config-driven
// resolution surface is exercised entirely in-process over real content.
package selfhost

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// requireGit skips a leg when git is not on PATH — the harness materializes and
// commits a real corpus, so git is a hard prerequisite of the proof (as it is
// of a pyxis-operated repository, UB-7).
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH: the self-host proof needs a real git repo")
	}
}

// writeFile writes b to path (parent dirs created), failing the test on error —
// the harness's only tree mutations (the regenerated generated surfaces).
func writeFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// quietLogger sinks doctor's structured events so the proof's log stays about
// the proof, not the engine's telemetry.
func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// denyFetcher fails the test on ANY artifact fetch — doctor over the relocated
// tree runs in source mode (drift skipped), so it must never reach for the
// network; a fetch would be a real regression this catches.
type denyFetcher struct{ t *testing.T }

func (d denyFetcher) Fetch(context.Context, string) (io.ReadCloser, error) {
	d.t.Error("doctor performed an artifact fetch over the relocated tree (source mode must perform none)")
	return nil, errors.New("denied")
}

// nonDefaultBranch and nonDefaultRemote are the varied mainline surface: a
// repository whose default branch is not "main" and whose remote is not
// "origin" (§3.8 — "the default branch renamed, an explicit remote set"). A
// missed branch/remote literal resolves against these and diverges from the
// default, so the proof catches it (DINV-1).
const (
	nonDefaultBranch = "trunk"
	nonDefaultRemote = "upstream"
)

// corpusPrefix is where the whole doc corpus relocates. The entire docs/ tree
// moves to this prefix preserving its internal structure, and every config doc-
// path is re-rooted here (docsRelocate). Moving the tree WHOLESALE — rather than
// scattering each corpus to an unrelated directory — is what keeps the operand's
// own intra-corpus relative markdown links internally consistent (a relative
// link between two moved docs still resolves), so the proof isolates the
// ENGINE's config resolution from the operand's content topology (design §3.4:
// "placing content there is the operand's concern", not the engine's).
const corpusPrefix = "factory-docs"

// docsRelocate maps a default docs/-rooted config path to its relocated
// location under corpusPrefix, preserving the sub-path (docs/prds ->
// factory-docs/prds). A path not under docs/ is returned unchanged (the
// harness only relocates the docs corpus; system-internal paths like
// .pyxis/templates are not target-facing, NG-1).
func docsRelocate(p string) string {
	if rest, ok := strings.CutPrefix(p, "docs/"); ok {
		return corpusPrefix + "/" + rest
	}
	return p
}

// nonDefaultConfig builds the non-default acceptance profile from the REAL repo
// config at src (so the repo's own grandfather lists and policy travel with the
// relocation — they are config data, not defaults), then varies EVERY target-
// facing surface §3.8 names:
//
//   - the doc corpora — prds, tech_designs, solution_shapes, brainstorms,
//     problems, reviews, wiki, plans, the two rollups, incidents, and the three
//     governing docs (product_vision, architecture_principles, design_principles)
//     — relocated under corpusPrefix, plus the system_maps list;
//   - the default branch — renamed to nonDefaultBranch;
//   - the git remote — set explicitly to nonDefaultRemote;
//   - the surface pointers — the CLI-surface knob set to a non-default surface
//     (T3's derived surface-pointer map is a pure function of this knob).
//
// Fields the engine resolves but that are NOT target-facing doc paths (the
// system-internal templates_dir, the pin surface) are left at their real
// values. The returned config is constructed directly (the T1/T2 in-process
// pattern, e.g. mainline_resolution_test.go's config.Config{Remote:…,Branch:…}):
// the v2 parser still refuses a non-default paths: value by name (the phase-1
// FR-18/UB-8 guard; the operand-override follow-up is deferred), so exercising
// the engine's *resolution* of a relocated layout — the DINV-1 property under
// test — is done by handing the gate the resolved config, exactly as T1's own
// relocated-path gate tests do.
func nonDefaultConfig(t *testing.T, src string) config.Config {
	t.Helper()
	cfg, defects := config.Load(src)
	if len(defects) > 0 {
		msgs := make([]string, len(defects))
		for i, d := range defects {
			msgs[i] = d.String()
		}
		t.Fatalf("the real repo config at %s does not closed-parse: %s", src, strings.Join(msgs, "; "))
	}

	cfg.Branch = nonDefaultBranch
	cfg.Remote = nonDefaultRemote

	cfg.PRDs = docsRelocate(cfg.PRDs)
	cfg.Feedback = docsRelocate(cfg.Feedback)
	cfg.TechDesigns = docsRelocate(cfg.TechDesigns)
	cfg.SolutionShapes = docsRelocate(cfg.SolutionShapes)
	cfg.Brainstorms = docsRelocate(cfg.Brainstorms)
	cfg.Problems = docsRelocate(cfg.Problems)
	cfg.Reviews = docsRelocate(cfg.Reviews)
	cfg.Wiki = docsRelocate(cfg.Wiki)
	cfg.Plans = docsRelocate(cfg.Plans)
	cfg.RollupPRD = docsRelocate(cfg.RollupPRD)
	cfg.RollupTechspec = docsRelocate(cfg.RollupTechspec)
	cfg.Incidents = docsRelocate(cfg.Incidents)
	cfg.ProductVision = docsRelocate(cfg.ProductVision)
	cfg.ArchitecturePrinciples = docsRelocate(cfg.ArchitecturePrinciples)
	cfg.DesignPrinciples = docsRelocate(cfg.DesignPrinciples)

	maps := make([]string, len(cfg.SystemMaps))
	for i, m := range cfg.SystemMaps {
		maps[i] = docsRelocate(m)
	}
	cfg.SystemMaps = maps

	// The surface pointers moved (§3.8): set the CLI-surface knob so T3's
	// derived surface-pointer map (SurfacePointers) resolves the configured
	// surface. codex is a non-default choice for this repo (its .pyxis/config
	// wires none), so a hardcoded surface map would diverge here.
	cfg.Knobs.CLISurfaces = []string{"codex"}

	return cfg
}

// materializeCorpus writes a throwaway copy of the repository's REAL tracked
// corpus (git archive HEAD | tar -x) into dst. The real corpus — not a toy
// fixture — is what makes R1's guarantee real: a missed literal that happens to
// equal a default is only caught on genuine content. Archive omits the
// gitignored .pyxis/bin cache and untracked WIP by construction.
func materializeCorpus(t *testing.T, src, dst string) {
	t.Helper()
	archive := exec.Command("git", "-C", src, "archive", "HEAD")
	extract := exec.Command("tar", "-x", "-C", dst)
	pipe, err := archive.StdoutPipe()
	if err != nil {
		t.Fatalf("archive stdout pipe: %v", err)
	}
	extract.Stdin = pipe
	var archiveErr, extractErr strings.Builder
	archive.Stderr = &archiveErr
	extract.Stderr = &extractErr
	if err := extract.Start(); err != nil {
		t.Fatalf("start tar extract: %v", err)
	}
	// Run the producer, then ALWAYS reap the consumer — even if the producer
	// failed — so a mid-stream `git archive` failure never orphans the started
	// tar child or leaks the pipe. archive.Run closes the pipe's write end, so on
	// producer failure tar sees EOF and exits; extract.Wait then reaps it. The
	// producer error is the primary failure to report.
	archiveRunErr := archive.Run()
	extractWaitErr := extract.Wait()
	if archiveRunErr != nil {
		t.Fatalf("git archive HEAD: %v\n%s", archiveRunErr, archiveErr.String())
	}
	if extractWaitErr != nil {
		t.Fatalf("tar extract: %v\n%s", extractWaitErr, extractErr.String())
	}
}

// relocateCorpus physically moves the whole docs/ tree to corpusPrefix in dst so
// the tree and the non-default config agree, then re-points the one HAND-AUTHORED
// in-content absolute doc-path reference the move invalidates — a
// `doc:docs/wiki/README.md` serves-ref in one design source (found by the
// harness's own dry run, not guessed). The same ref also appears in the generated
// tech-design index (index.md/index.json), but those are DERIVED from the source
// design and are re-rendered wholesale by regenerateGeneratedSurfaces (called
// after this), so they carry the rewritten ref without a manual edit — hence only
// the source doc is rewritten here, and this must run BEFORE the index regen.
// This is the operand's content-placement duty (design §3.4), performed here on
// the throwaway so the ENGINE's config resolution is what the gate then judges.
// A relative markdown link between two moved docs needs no rewrite: it moved
// with its target. check-links walks the literal docs/ path (a system convention,
// not a config surface) with WalkDir errors ignored, so the now-ABSENT docs/
// yields zero files and no link defects.
func relocateCorpus(t *testing.T, dst string) {
	t.Helper()
	from := filepath.Join(dst, "docs")
	to := filepath.Join(dst, filepath.FromSlash(corpusPrefix))
	if err := os.Rename(from, to); err != nil {
		t.Fatalf("relocate docs/ -> %s/: %v", corpusPrefix, err)
	}
	rewriteContentRef(t, filepath.Join(to, "tech-design", "docgate-v2.md"),
		"doc:docs/wiki/README.md", "doc:"+corpusPrefix+"/wiki/README.md")
}

// rewriteContentRef replaces exactly one occurrence of old with new in the file
// at path, failing if the reference is not present (so a corpus change that
// drops the reference is caught rather than silently skipped — the harness must
// stay honest about what it edits).
func rewriteContentRef(t *testing.T, path, old, new string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s to rewrite %q: %v", path, old, err)
	}
	if !strings.Contains(string(b), old) {
		t.Fatalf("expected in-content reference %q not found in %s — the corpus changed; update the harness's content-placement step", old, path)
	}
	out := strings.Replace(string(b), old, new, 1)
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatalf("rewrite %s: %v", path, err)
	}
}

// gitEnv is the deterministic identity every harness commit uses — no
// dependence on the runner's git config.
func gitEnv() []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME=selfhost", "GIT_AUTHOR_EMAIL=selfhost@pyxis",
		"GIT_COMMITTER_NAME=selfhost", "GIT_COMMITTER_EMAIL=selfhost@pyxis")
}

// git runs a git subcommand in root, failing the test with the combined output
// on error. It returns trimmed stdout for the read subcommands the harness uses
// (rev-parse).
func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// initRelocatedRepo initializes a git repo over the relocated tree and commits
// it as ONE base commit, returning that commit's sha as the merge base. It sets
// the NON-DEFAULT branch and remote (§3.8) so the full gate's history checks
// resolve exactly the configured mainline: the branch is created at init (-b
// nonDefaultBranch) and the remote points the tree at itself (hermetic — no
// network). The full relocated corpus lands in one all-ADDITIONS commit, so the
// history checks that flag modified/deleted ledger entries or governing lines
// (check-immutable, check-governing) see nothing to flag and pass, while still
// executing their git diffs against the configured branch/paths — the honest
// green a real, coherent corpus produces on a fresh mainline.
func initRelocatedRepo(t *testing.T, root string) (mergeBase string) {
	t.Helper()
	git(t, root, "init", "-q", "-b", nonDefaultBranch)
	// Committing the full corpus triggers automatic maintenance. Keep it
	// owned by the commit process so it cannot race TempDir cleanup.
	git(t, root, "config", "gc.autoDetach", "false")
	git(t, root, "config", "maintenance.autoDetach", "false")
	// The explicit non-default remote (§3.8): a bare self-reference, so refs
	// like <remote>/<branch> can be created without any network.
	git(t, root, "remote", "add", nonDefaultRemote, root)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "relocated pyxis corpus under a non-default config")
	return git(t, root, "rev-parse", "HEAD")
}

// copySourceLockTags preserves the historical Basis of draft-born plans in the
// relocated proof. git archive copies the current tree but omits refs; pointing
// a lock tag at the synthetic relocated commit would change what it proves.
// Fetch only the source repository's lock tags and their reachable objects from
// the local checkout, without contacting a remote or changing relocated HEAD.
func copySourceLockTags(t *testing.T, src, root string) {
	t.Helper()
	refs := strings.Fields(git(t, src, "for-each-ref", "--format=%(refname)", "refs/tags/lock/"))
	if len(refs) == 0 {
		return
	}
	args := []string{"fetch", "--no-tags", src}
	for _, ref := range refs {
		args = append(args, ref+":"+ref)
	}
	git(t, root, args...)
}

// repoRoot resolves the source repo root from this package directory
// (internal/selfhost -> ../..). The corpus archive and the coherence scan both
// root here.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}
