// The write-journal rollback (design init-hardening-v1 §3.6, DINV-3/DINV-4;
// plan init-hardening/001 T4): one journaling write primitive backs every
// init-owned mutation — file, directory, repo-local git config, the git
// remote (T5), git-init, and remote operation — so a stage failure past the
// point mutation begins unwinds in reverse to the pre-run target. Converge's
// own writes (and ReconcileHarnessSurfaces', its shared symlink primitive)
// never go through this primitive — they are INGESTED from their returned
// results the moment the pipeline observes them (stageConverge, stageSeed),
// before the pipeline proceeds, so a later stage's failure still unwinds
// them.
//
// Foreign entries — anything on disk this run never journaled — are left
// untouched by construction (unwind only ever removes a journaled path) and
// named in the RollbackReport. A repository created remotely during wiring
// is a remote-op entry that unwinds to a report line, never a deletion
// (DINV-4) — distinct from the git remote ENTRY itself (journalRemote), a
// local config mutation this run made that unwinds like any other.
package installer

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/gitcredential"
)

var installerRemoveAll = os.RemoveAll

// journalKind classifies one recorded mutation for the reverse unwind.
type journalKind int

const (
	journalFile        journalKind = iota // a file (or symlink) at an absolute path — unwinds by removal
	journalDir                            // a directory this run created directly — unwinds by removal iff still empty
	journalConfig                         // a repo-local `git config <key>` this run set — unwinds by --unset
	journalRemote                         // a git remote this run added (T5) — unwinds by `git remote remove`
	journalGitInit                        // this run ran `git init` — unwinds by removing .git wholesale
	journalRemoteOp                       // a remote mutation this run performed — never unwound, only reported
	journalCredentials                    // selected URL Git credential configuration, restored to exact prior values
	journalRootedFile                     // authority seed: removal remains bounded if a parent becomes a symlink
	journalRootedDir                      // authority parent: root-bounded removal iff empty
)

// journalEntry is one recorded mutation, in the order performed.
type journalEntry struct {
	kind        journalKind
	path        string // file/dir: absolute filesystem path
	key         string // config: the git config key this run set
	cleanup     string // remoteOp: the manual command that removes what this run created
	credentials []gitcredential.Change
	ownedInfo   fs.FileInfo // authority-only: identity of the object this init created
	ownedDigest [32]byte    // authority-only: bytes written before any later-stage failure
}

// journal accumulates every mutation Init performs once apply begins (DINV-1:
// nothing is journaled before then, because nothing is written before then).
// target is the cleaned operand root every entry's path is bounded to.
type journal struct {
	target  string
	entries []journalEntry
}

func newJournal(target string) *journal {
	return &journal{target: filepath.Clean(target)}
}

// writeFile creates rel's parent directories (journaling each one newly
// made) and atomically writes b — the journaling write primitive every
// installer-owned file write in the apply pipeline routes through. rel is
// operand-relative, slash form (the same convention run.seeded uses).
func (j *journal) writeFile(rel string, b []byte, perm os.FileMode) error {
	abs := filepath.Join(j.target, filepath.FromSlash(rel))
	if err := j.mkdirAllAbs(filepath.Dir(abs)); err != nil {
		return err
	}
	if err := writeFileAtomic(abs, b, perm); err != nil {
		return err
	}
	j.entries = append(j.entries, journalEntry{kind: journalFile, path: abs})
	return nil
}

// mkdirAll creates rel (operand-relative, slash form) and any missing
// ancestors, journaling each newly created directory individually — the
// primitive stageSeed's taxonomy directories (never populated by a journaled
// file, so no other unwind path would ever reach them) route through.
func (j *journal) mkdirAll(rel string) error {
	return j.mkdirAllAbs(filepath.Join(j.target, filepath.FromSlash(rel)))
}

// mkdirAllAbs is mkdirAll's absolute-path core: it walks up from path,
// collecting the missing ancestors bounded at j.target, then creates them
// shallowest-first (so each parent exists before its child), journaling each
// in that same chronological order — so the reverse unwind removes them
// deepest-first, exactly mirroring a plain rmdir walk back up.
func (j *journal) mkdirAllAbs(path string) error {
	path = filepath.Clean(path)
	var missing []string
	for p := path; p != j.target; p = filepath.Dir(p) {
		st, err := os.Stat(p)
		if err == nil {
			if !st.IsDir() {
				return fmt.Errorf("mkdir %s: %s exists and is not a directory", path, p)
			}
			break
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat %s: %w", p, err)
		}
		missing = append(missing, p)
		if parent := filepath.Dir(p); parent == p {
			break // filesystem-root guard; unreachable under a real j.target
		}
	}
	for _, path := range slices.Backward(missing) {
		if err := os.Mkdir(path, 0o755); err != nil && !os.IsExist(err) {
			return fmt.Errorf("mkdir %s: %w", path, err)
		}
		j.entries = append(j.entries, journalEntry{kind: journalDir, path: path})
	}
	return nil
}

// recordFile journals a path some OTHER writer already put on disk —
// converge's ingested Report, or ReconcileHarnessSurfaces' returned actions
// (DINV-3: "converge's contribution ingests its returned Report ... before
// the pipeline proceeds"). It performs no I/O itself: that writer already
// created any directories it needed, so only the leaf path is remembered
// (its now-possibly-empty ancestor directories are cleaned up generically by
// removeEmptyAncestors when the file is unwound).
func (j *journal) recordFile(rel string) {
	j.entries = append(j.entries, journalEntry{kind: journalFile, path: filepath.Join(j.target, filepath.FromSlash(rel))})
}

// recordConfig journals a repo-local `git config` mutation this run made
// (the committer-identity halves, core.hooksPath) — unwound by `--unset`
// when the repository survives the run; moot (and skipped) when init
// created the repository, since the whole .git is removed wholesale then.
func (j *journal) recordConfig(key string) {
	j.entries = append(j.entries, journalEntry{kind: journalConfig, key: key})
}

func (j *journal) recordCredentials(changes []gitcredential.Change) {
	if len(changes) > 0 {
		j.entries = append(j.entries, journalEntry{kind: journalCredentials, credentials: changes})
	}
}

// recordRemote journals that this run added the git remote named name
// (design init-hardening-v1 §3.7; plan init-hardening/001 T5's remote-
// materialization step) — unwound by `git remote remove` when the
// repository survives the run; moot (and skipped) when init created the
// repository, since the whole .git is removed wholesale then. Distinct from
// recordRemoteOp/journalRemoteOp: this is a LOCAL git-config mutation this
// run made (the remote entry pointing at a coordinate), never a hosted
// GitHub mutation — DINV-4's never-auto-delete-a-created-repository rule
// does not apply to it.
func (j *journal) recordRemote(name string) {
	j.entries = append(j.entries, journalEntry{kind: journalRemote, key: name})
}

// recordGitInit journals that THIS run created the repository (git init) —
// the one entry that turns unwind's repository leg into "remove .git
// wholesale" rather than "restore the prior ref/index" (DINV-3).
func (j *journal) recordGitInit() {
	j.entries = append(j.entries, journalEntry{kind: journalGitInit})
}

// recordRemoteOp journals a remote mutation this run performed that unwind
// must never reverse (DINV-4) — today exactly a landed create-repo op.
// cleanup is the human-readable command that removes it, named in the
// RollbackReport; unwind never runs it.
func (j *journal) recordRemoteOp(cleanup string) {
	j.entries = append(j.entries, journalEntry{kind: journalRemoteOp, cleanup: cleanup})
}

// hasGitInit reports whether this run's journal recorded creating the
// repository.
func (j *journal) hasGitInit() bool {
	for _, e := range j.entries {
		if e.kind == journalGitInit {
			return true
		}
	}
	return false
}

// RollbackReport is FR-10's account of one unwind: what this run restored,
// what survived because it was never journaled (preserved and named, never
// deleted), and the manual step to remove a repository this run created
// remotely (DINV-4 — never automatic). Init populates Report.Rollback with
// this whenever a stage failure past the point mutation began triggers the
// unwind.
type RollbackReport struct {
	// FailedStage is the stage (a pipelineStages name, or one of the
	// pre-pipeline apply steps "git-init"/"identity"/"gitignore"/"config")
	// whose failure triggered the unwind.
	FailedStage string
	// Restored is true when every journaled mutation was undone without the
	// unwind itself hitting an error (Errors is then empty).
	Restored bool
	// RemovedFiles / RemovedDirs are the journaled (or converge/harness-
	// surface ingested) paths actually removed, operand-relative slash form.
	RemovedFiles []string
	RemovedDirs  []string
	// RepoRemoved is true when the metadata directory was removed wholesale
	// (this run created it — journalGitInit was recorded).
	RepoRemoved bool
	// CacheCleared is true when the local acquisition cache (.pyxis/bin —
	// resolve.go's cacheDir) existed and was removed. It is never journaled
	// per-file (converge writes it through its own primitive, outside the
	// repo tree entirely) but must still go: it is hash-verified and always
	// re-fetchable, so clearing it costs at most a future re-fetch, and
	// leaving it behind would otherwise strand a target dirty once its
	// .gitignore entry is itself unwound.
	CacheCleared bool
	// ForeignPreserved lists what remains under the target after every
	// journaled removal — paths this run never journaled, left untouched by
	// construction (DINV-3's "preserving any foreign entry ... naming it").
	ForeignPreserved []string
	// RemoteCleanup lists the manual command(s) to remove a repository this
	// run created remotely — never auto-run (DINV-4).
	RemoteCleanup []string
	// Errors carries anything the unwind itself could not undo (Restored is
	// then false) — e.g. a filesystem permission error on a journaled path.
	Errors []string
}

// unwind reverses run's journal after a stage failure past the point
// mutation began (FR-10): files (and converge/harness-surface ingested
// paths) are removed, their now-empty ancestor directories are cleaned up,
// directories this run created directly are removed, and — only when this
// run created the repository — the metadata directory itself is removed
// wholesale; otherwise a pre-existing repository's index and branch ref are
// restored to their captured pre-run state instead (never deleted). Foreign
// entries are never touched, only named (DINV-3); a remotely created
// repository is never deleted, only named with its cleanup command (DINV-4).
//
// unwind runs its own git commands against a context stripped of the
// triggering failure's cancellation/deadline (context.WithoutCancel): a
// "signal mid-apply" rollback must still be able to clean up after itself
// even though the context that failed the run is already done.
func (run *initRun) unwind(ctx context.Context, failedStage string) RollbackReport {
	cctx := context.WithoutCancel(ctx)
	target := run.j.target
	rb := RollbackReport{FailedStage: failedStage, Restored: true}
	repoCreated := run.j.hasGitInit()

	// The local acquisition cache (resolve.go's cacheDir: Root/.pyxis/bin) is
	// repo-local, gitignored infrastructure a converge run populates through
	// its own primitive, never run.j — hash-verified and always re-fetchable,
	// so clearing it loses nothing but a future re-fetch. It must go before
	// the journal walk below can find .pyxis itself empty, and it must go
	// regardless of repoCreated: once .gitignore (which is what hides it from
	// `git status`) is removed, a surviving cache would otherwise strand a
	// pre-existing repository dirty and block a subsequent init's admission.
	clearAcquisitionCache(target, &rb)

	for _, e := range slices.Backward(run.j.entries) {
		switch e.kind {
		case journalRootedFile, journalRootedDir:
			root, err := os.OpenRoot(target)
			if err == nil {
				rel := relSlash(target, e.path)
				info, probeErr := root.Lstat(filepath.FromSlash(rel))
				if os.IsNotExist(probeErr) {
					_ = root.Close() // already removed; no object remains to unwind
					continue
				}
				if probeErr != nil {
					err = probeErr
				} else if e.ownedInfo == nil || !os.SameFile(e.ownedInfo, info) || e.ownedInfo.Mode() != info.Mode() {
					err = fmt.Errorf("rollback: preserve replaced authority object %s", rel)
				} else if e.kind == journalRootedFile {
					b, readErr := root.ReadFile(filepath.FromSlash(rel))
					if readErr != nil {
						err = readErr
					} else if sha256.Sum256(b) != e.ownedDigest {
						err = fmt.Errorf("rollback: preserve modified authority policy %s", rel)
					}
				}
				if err == nil {
					err = root.Remove(filepath.FromSlash(rel))
				}
				if err == nil {
					if e.kind == journalRootedFile {
						rb.RemovedFiles = append(rb.RemovedFiles, rel)
					} else {
						rb.RemovedDirs = append(rb.RemovedDirs, rel)
					}
				}
				_ = root.Close() // this entry's bounded removal has completed
			}
			if err != nil && !os.IsNotExist(err) && e.kind == journalRootedFile {
				rb.Restored = false
				rb.Errors = append(rb.Errors, err.Error())
			}
			// A nonempty parent survives exactly as ordinary journalDir does.
		case journalFile:
			switch err := os.Remove(e.path); {
			case err == nil:
				rb.RemovedFiles = append(rb.RemovedFiles, relSlash(target, e.path))
				removeEmptyAncestors(target, filepath.Dir(e.path), &rb.RemovedDirs)
			case os.IsNotExist(err):
				// already gone (e.g. the epoch rewrite journals config.yaml twice)
			default:
				rb.Restored = false
				rb.Errors = append(rb.Errors, err.Error())
			}
		case journalDir:
			if err := os.Remove(e.path); err == nil {
				rb.RemovedDirs = append(rb.RemovedDirs, relSlash(target, e.path))
			}
			// Non-empty (foreign content survives inside) or already gone via
			// removeEmptyAncestors above: never an unwind failure — a
			// directory that cannot be removed is exactly the preserved
			// case, named by the final scan below.
		case journalConfig:
			if !repoCreated {
				if _, err := gitOutput(cctx, target, "config", "--unset", e.key); err != nil {
					rb.Restored = false
					rb.Errors = append(rb.Errors, err.Error())
				}
			}
		case journalCredentials:
			if !repoCreated {
				if err := gitcredential.Restore(cctx, target, e.credentials); err != nil {
					rb.Restored = false
					rb.Errors = append(rb.Errors, err.Error())
				}
			}
		case journalRemote:
			if !repoCreated {
				if _, err := gitOutput(cctx, target, "remote", "remove", e.key); err != nil {
					rb.Restored = false
					rb.Errors = append(rb.Errors, err.Error())
				}
			}
		case journalRemoteOp:
			rb.RemoteCleanup = append(rb.RemoteCleanup, e.cleanup)
		case journalGitInit:
			// handled once, after the reverse walk, below.
		}
	}

	if repoCreated {
		if err := os.RemoveAll(filepath.Join(target, ".git")); err != nil {
			rb.Restored = false
			rb.Errors = append(rb.Errors, err.Error())
		} else {
			rb.RepoRemoved = true
		}
	} else if err := run.restoreGitRef(cctx); err != nil {
		rb.Restored = false
		rb.Errors = append(rb.Errors, err.Error())
	}

	rb.ForeignPreserved = scanForeign(target)
	return rb
}

// clearAcquisitionCache removes the local acquisition cache at
// target/.pyxis/bin (resolve.go's cacheDir — the release manifest, the
// payload archive, and any pre-warmed tool binaries a converge run fetched).
// It is scoped to exactly that subdirectory, never the wider .pyxis tree —
// config.yaml and every other repo-owned or converged path are siblings this
// must not touch. A missing cache (converge never ran, or a preview) is not
// an error.
func clearAcquisitionCache(target string, rb *RollbackReport) {
	dir := filepath.Join(target, ".pyxis", "bin")
	if _, err := os.Stat(dir); err != nil {
		return // never existed — nothing to clear
	}
	if err := installerRemoveAll(dir); err != nil {
		rb.Restored = false
		rb.Errors = append(rb.Errors, err.Error())
		return
	}
	rb.CacheCleared = true
}

// restoreGitRef undoes any commits THIS run made on a repository it did not
// create (DINV-3's git-adjacent half). The index is always cleared back to
// empty — admission required zero tracked files to begin with, so an empty
// index is the as-found state regardless of whether a commit landed. The
// branch ref is touched only when this run actually committed: reset back to
// the captured pre-run sha when HEAD was already born, or deleted back to
// unborn when this run made the branch's first-ever commit. A no-op — beyond
// clearing the index — when this run never committed.
func (run *initRun) restoreGitRef(ctx context.Context) error {
	target := run.j.target
	if _, err := gitOutput(ctx, target, "read-tree", "--empty"); err != nil {
		return fmt.Errorf("clear the index: %w", err)
	}
	if len(run.rep.Commits) == 0 {
		return nil
	}
	ref := "refs/heads/" + run.branch
	if run.headBorn {
		if _, err := gitOutput(ctx, target, "update-ref", ref, run.headSHA); err != nil {
			return fmt.Errorf("restore the branch ref: %w", err)
		}
		return nil
	}
	if _, err := gitOutput(ctx, target, "update-ref", "-d", ref); err != nil {
		return fmt.Errorf("clear the branch ref this run created: %w", err)
	}
	return nil
}

// removeEmptyAncestors walks up from dir toward target, removing each
// directory that is now empty, stopping at the first non-empty directory,
// an already-gone one, or target itself. The supplementary cleanup for
// directories no journalDir entry names directly — converge's own writes,
// and ReconcileHarnessSurfaces', create their parent directories themselves;
// this is the only mechanism that reclaims them once every journaled/
// ingested file beneath them is gone.
func removeEmptyAncestors(target, dir string, removed *[]string) {
	for len(dir) > len(target) && strings.HasPrefix(dir, target) {
		if err := os.Remove(dir); err != nil {
			return
		}
		*removed = append(*removed, relSlash(target, dir))
		dir = filepath.Dir(dir)
	}
}

// scanForeign walks what remains under target after unwind's removals,
// skipping .git (either gone because this run removed it, or the
// pre-existing repository this run never owned, either way not "foreign") —
// anything left is exactly what the journal never recorded: preserved by
// construction, named here (DINV-3). A leaf empty directory is named with a
// trailing slash; every other survivor is a file (or symlink) path.
func scanForeign(target string) []string {
	var out []string
	_ = filepath.WalkDir(target, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // best-effort naming; never fail the unwind over a stat race
		}
		if p == target {
			return nil
		}
		rel, rerr := filepath.Rel(target, p)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			entries, rderr := os.ReadDir(p)
			if rderr == nil && len(entries) == 0 {
				out = append(out, rel+"/")
			}
			return nil
		}
		out = append(out, rel)
		return nil
	})
	sort.Strings(out)
	return out
}

// relSlash renders an absolute path under target as an operand-relative
// slash path for the RollbackReport — the same rendering convention Report's
// other paths use.
func relSlash(target, abs string) string {
	rel, err := filepath.Rel(target, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(rel)
}

// failAndUnwind runs the reverse unwind for a stage failure past the point
// mutation began (FR-10), fires init_rolled_back (EVT-3), and returns the
// ORIGINAL error wrapped with the failed stage's name — never swallowed, so
// the caller learns both why the run failed and what was restored.
func (run *initRun) failAndUnwind(ctx context.Context, stage string, cause error) (Report, error) {
	var uncertain *publicationUncertainError
	if run.published || run.rep.Resumed || errors.As(cause, &uncertain) {
		return *run.rep, fmt.Errorf("init: stage %s: %w; installation retained because publication may have reached the destination; fix the failure and re-run pyxis init to resume", stage, cause)
	}
	rb := run.unwind(ctx, stage)
	run.rep.Rollback = &rb
	emitInitRolledBack(ctx, run.logger, run.rep.RunID, rb)
	return *run.rep, fmt.Errorf("init: stage %s: %w", stage, cause)
}
