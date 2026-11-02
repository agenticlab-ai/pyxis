package codegatemirror

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	repositoryconfig "github.com/agenticlab-ai/pyxis/internal/docgate/config"
	executorconfig "github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
)

// cacheFormatVersion is folded into every fingerprint; bumping it invalidates
// every cached verdict at once (used when the fingerprint or verdict schema
// changes shape).
const cacheFormatVersion = "code-gate-cache/v6"

// verdictSchema tags the on-disk cached verdict; a file carrying any other schema
// is treated as no cache (closed, AP-6).
const verdictSchema = "code-gate-verdict/v1"

var cacheCreateTemp = os.CreateTemp

// cachedVerdict is the on-disk record of the last GREEN run: the content
// fingerprint the gate was green for. Only a pass is ever cached — a red tree
// always re-runs to show fresh output. Schema-versioned + closed.
type cachedVerdict struct {
	Schema       string `json:"schema"`
	Fingerprint  string `json:"fingerprint"`
	Passed       bool   `json:"passed"`
	GitProtected bool   `json:"git_protected,omitempty"`
}

// RunCached runs the gate over workspace with content-fingerprint caching
// (DINV-6): a fresh cached GREEN verdict for the exact working-tree content and
// this check set is reused WITHOUT running any check, so the gate runs once per
// distinct tree across the local surfaces — the executor's drive-to-green and the
// pre-push share the cache, and an executor-gated tree makes its push's pre-push a
// no-op. The cache lives under the git dir, per checkout, and is never committed,
// so a fresh remote checkout has none and CI always re-runs (AP-13). It fails
// SAFE, never closed: any fingerprint or cache-IO problem degrades to a fresh Run
// (hit=false) — never a stale reuse, never a gate error. The bool reports a hit.
func (c Config) RunCached(ctx context.Context, workspace string) (Report, bool, error) {
	gitDir, err := gitDir(workspace)
	if err != nil {
		return c.Run(ctx, workspace), false, nil // not a git repo → run fresh, no cache
	}
	fp, err := c.fingerprint(ctx, workspace, gitDir)
	if err != nil {
		return c.Run(ctx, workspace), false, nil // cannot fingerprint → run fresh, no cache
	}
	protected := runtime.VerificationAccessRequired(ctx)
	if v, ok := loadVerdict(gitDir); ok && v.Passed && v.Fingerprint == fp && (!protected || v.GitProtected) {
		// A tree/ref fingerprint cannot prove continued availability of local
		// historical objects (pruning, shallow boundaries, or changed alternate
		// stores). Re-run the memory predicate even when other checks are cached.
		memory := c
		memory.Checks = nil
		for _, chk := range c.Checks {
			if chk.Name == "memory" {
				memory.Checks = append(memory.Checks, chk)
			} else {
				emitProgress(ctx, chk, "cached", Outcome{})
			}
		}
		if len(memory.Checks) > 0 {
			return memory.Run(ctx, workspace), false, nil
		}
		return Report{Passed: true}, true, nil // cache hit — no check runs
	}
	rep := c.Run(ctx, workspace)
	if rep.Passed {
		writeVerdict(gitDir, cachedVerdict{Schema: verdictSchema, Fingerprint: fp, Passed: true, GitProtected: protected})
	}
	return rep, false, nil
}

// fingerprint is a content hash that changes iff a check input changes: the
// working tree's git content (tracked + untracked, honoring .gitignore), the
// cache-format version, and each check's identity. It never touches the real index
// (a temporary index is used, §treeContentHash).
func (c Config) fingerprint(ctx context.Context, workspace, gitDir string) (string, error) {
	tree, err := treeContentHash(ctx, workspace, gitDir)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, _ = io.WriteString(h, cacheFormatVersion)                                        // hash.Hash.Write never returns an error.
	_, _ = io.WriteString(h, "\x00tree\x00"+tree)                                       // hash.Hash.Write never returns an error.
	_, _ = io.WriteString(h, "\x00verification-base\x00"+runtime.VerificationBase(ctx)) // hash writes cannot fail
	// History-sensitive checks read HEAD and trusted base refs even when the
	// working-tree bytes have not changed. A fetch or same-tree commit must
	// invalidate their verdict. Operational claim/review refs and the current
	// branch's remote publication are not code inputs; memory availability is
	// independently rechecked on every cache hit below.
	refs, err := gitCmd(ctx, workspace, nil, "show-ref", "--head")
	if err != nil {
		return "", fmt.Errorf("fingerprint git refs: %w", err)
	}
	branch, err := gitCmd(ctx, workspace, nil, "symbolic-ref", "--quiet", "HEAD")
	if err != nil {
		branch = nil // detached HEAD still appears in show-ref --head
	}
	policy, err := executorconfig.Load(workspace)
	if err != nil {
		return "", fmt.Errorf("fingerprint executor evidence policy: %w", err)
	}
	operational := map[string]bool{policy.ReviewEvidenceRef(): true, policy.ReviewProcessEvidenceRef(): true, policy.ReviewPublicationEvidenceRef(): true, policy.IssueEvidenceRef(): true}
	repository, defects := repositoryconfig.Load(workspace)
	if len(defects) > 0 {
		return "", fmt.Errorf("fingerprint repository policy: %v", defects)
	}
	for line := range strings.SplitSeq(string(refs), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		ref := fields[1]
		if strings.HasPrefix(ref, "refs/claim/") || operational[ref] {
			continue
		}
		if strings.HasPrefix(ref, "refs/remotes/") && len(branch) > 0 {
			_, tracked, ok := strings.Cut(strings.TrimPrefix(ref, "refs/remotes/"), "/")
			if ok && tracked != repository.Branch && tracked == strings.TrimPrefix(strings.TrimSpace(string(branch)), "refs/heads/") {
				continue
			}
		}
		_, _ = io.WriteString(h, "\x00ref\x00"+line) // hash.Hash.Write never returns an error.
	}
	for _, chk := range c.Checks {
		_, _ = fmt.Fprintf(h, "\x00chk\x00%s\x00%s\x00%s\x00%s\x00%d\x00%s\x00%d", chk.Name, chk.Command, chk.FailOn, chk.Class, chk.TimeoutS, chk.ExecutionGroup(), chk.ExecutionPhase()) // hash.Hash.Write never returns an error.
		for _, name := range append([]string{"PATH", "HOME"}, chk.Env...) {
			value, present := os.LookupEnv(name)
			_, _ = fmt.Fprintf(h, "\x00env\x00%s\x00%t\x00%s", name, present, value) // hash.Hash.Write never returns an error.
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// treeContentHash returns the git tree SHA of workspace's current working tree —
// every tracked and untracked non-ignored file's content, exactly what the checks
// read. It stages into a TEMPORARY index (GIT_INDEX_FILE) seeded from the real
// index for git's stat cache, so the real index is never mutated.
func treeContentHash(ctx context.Context, workspace, gitDir string) (string, error) {
	tmp, err := os.CreateTemp("", "code-gate-index-*")
	if err != nil {
		return "", fmt.Errorf("code-gate: temp index: %w", err)
	}
	_ = tmp.Close()                              // The file is empty; Git reports any unusable temporary index.
	defer func() { _ = os.Remove(tmp.Name()) }() // Best-effort scratch cleanup.
	// Seed the temp index from the real one so `git add -A` re-hashes only changed
	// files (git's stat cache). A missing real index is fine — an empty temp index
	// makes `git add -A` stage the whole working tree, the same result.
	if data, err := os.ReadFile(filepath.Join(gitDir, "index")); err == nil {
		_ = os.WriteFile(tmp.Name(), data, 0o600)
	}
	env := append(os.Environ(), "GIT_INDEX_FILE="+tmp.Name())
	if out, err := gitCmd(ctx, workspace, env, "add", "-A"); err != nil {
		return "", fmt.Errorf("code-gate: git add -A: %w: %s", err, strings.TrimSpace(string(out)))
	}
	out, err := gitCmd(ctx, workspace, env, "write-tree")
	if err != nil {
		return "", fmt.Errorf("code-gate: git write-tree: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// gitDir resolves workspace's absolute git directory — where the per-checkout
// verdict cache lives, OUTSIDE the working tree so writing it never perturbs the
// content fingerprint. An error means "not a git repo", handled fail-safe.
func gitDir(workspace string) (string, error) {
	out, err := exec.Command("git", "-C", workspace, "rev-parse", "--absolute-git-dir").Output()
	if err != nil {
		return "", fmt.Errorf("code-gate: resolve git dir: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func gitCmd(ctx context.Context, dir string, env []string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = env
	return cmd.CombinedOutput()
}

// cachePath is the per-checkout verdict cache: <git-dir>/code-gate/verdict.json.
func cachePath(gitDir string) string {
	return filepath.Join(gitDir, "code-gate", "verdict.json")
}

// loadVerdict reads the cached verdict; ok is false on any problem (missing,
// unreadable, malformed, wrong schema) — the caller then runs fresh.
func loadVerdict(gitDir string) (cachedVerdict, bool) {
	data, err := os.ReadFile(cachePath(gitDir))
	if err != nil {
		return cachedVerdict{}, false
	}
	var v cachedVerdict
	if err := json.Unmarshal(data, &v); err != nil || v.Schema != verdictSchema {
		return cachedVerdict{}, false
	}
	return v, true
}

// writeVerdict atomically writes the verdict cache (temp + rename). Every error is
// swallowed — a cache-write failure must never fail the gate (fail-safe, not
// fail-closed: the worst case is a future re-run, never a wrong verdict).
func writeVerdict(gitDir string, v cachedVerdict) {
	path := cachePath(gitDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	tmp, err := cacheCreateTemp(filepath.Dir(path), "verdict-*.tmp")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()        // The cache write already failed; a cache miss is safe.
		_ = os.Remove(tmpName) // Best-effort cleanup of an unusable cache entry.
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName) // Best-effort cleanup of an unusable cache entry.
		return
	}
	_ = os.Rename(tmpName, path)
}
