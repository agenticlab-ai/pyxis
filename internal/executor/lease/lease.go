// Package lease implements the executor's Lease port (port.Lease) as a
// create-or-fail git-ref compare-and-swap, keyed by (repo, task) and carrying
// holder identity in the claim commit's metadata
// (docs/tech-design/implement-now.md §3.1, #S-3).
//
// It is the SAME claim mechanism the external fleet uses, so a local run
// coexists with the fleet on one repo with no double-grab: both sides race a
// claim ref on the shared code host, and git's atomic ref update elects exactly
// one holder (prd:implement-now/FR-2, NR-2). Qualified tasks normally use a scoped
// ref; LegacySourceKey atomically reserves its numeric compatibility ref too.
// A losing
// racer is a skip, not a failure — Acquire returns port.ErrClaimLost, which
// callers match with errors.Is (DINV-5, AP-17, AP-22).
//
// A local in-process guard (a mutex-owned set of reserved refs) short-circuits
// two local Workers that reach for the same task before either hits the CAS, so
// the process never fires two redundant pushes at one ref; correctness still
// rests on the remote CAS, the guard is only the local optimization (NR-2).
//
// Every acquired claim is released on every terminal path (fire-or-release,
// DINV-5): Release deletes the ref and is idempotent and safe to call twice.
// The delete is guarded so a holder only ever removes its OWN claim — a fleet
// reap-and-reclaim by another holder is never stomped (fail-closed on a
// destructive path, INV-6, AP-26).
//
// git is driven through os/exec (AGENTS.md Go rules); the adapter needs no
// repo-side git identity — it supplies its own for the claim commit, so it runs
// hermetically regardless of the checkout's config.
package lease

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// Compile-time proof the adapter satisfies the executor ports.
var (
	_ port.Lease = (*Lease)(nil)
	_ port.Claim = (*claim)(nil)
)

const (
	// defaultRefPrefix is the claim-ref namespace when Config.RefPrefix is empty.
	// The ref for issue N is refs/claim/N, matching the fleet's convention
	// (docs/domain-model.md — the LeaseStore's git refs claim/<N>).
	defaultRefPrefix = "refs/claim/"

	// holderEmail is the synthetic, non-routable author/committer email stamped
	// on every claim commit (RFC 2606 reserved .invalid TLD). Holder identity of
	// substance lives in the holder name and the commit message.
	holderEmail = "lease@pyxis.invalid"
)

// Config configures a git-ref CAS Lease adapter. Unknown-key rejection is not
// applicable (this is a Go struct, not a parsed file); zero-value fields take
// the documented defaults.
type Config struct {
	// WorkDir is a git repository (typically the run's clone) with Remote
	// reachable from it. The adapter runs git here to mint the claim commit and
	// CAS-push it. Required.
	WorkDir string

	// Remote is the name or URL of the shared code host the claim refs live on —
	// the same remote the external fleet claims against (NR-2). Blank resolves the
	// config schema's default remote (config.Defaults, "origin" by default).
	Remote string

	// Holder identifies this process as the claim holder; it is recorded in every
	// claim commit's author/committer and message. Default: pyxis-local/<host>/<pid>.
	Holder string

	// RefPrefix is the claim-ref namespace; a trailing slash is added if absent.
	// Default "refs/claim/".
	RefPrefix string

	// Git is the git binary to invoke. Default "git" (resolved on PATH).
	Git string

	// LegacySourceKey enables atomic reservation of both scoped and numeric
	// claims for this exact task source. Set only when the hosted task and code
	// bindings match; older workers interpret numeric refs in that repository.
	// Empty preserves scoped-only claims (and existing unqualified claims).
	LegacySourceKey string
}

// Lease is the git-ref CAS claim adapter. It is safe for concurrent use by many
// Workers and is meant to be shared process-wide so its in-process guard spans
// them (a per-Worker instance would defeat the guard, though the remote CAS
// would still hold). Build one with New.
type Lease struct {
	gitBin          string
	workDir         string
	remote          string
	holder          string
	refPrefix       string
	legacySourceKey string
	env             []string // os.Environ plus the claim-commit identity overrides

	mu   sync.Mutex
	held map[string]struct{} // refs currently reserved/held by this process
}

// New builds a Lease from cfg, applying defaults. It fails closed on a missing
// WorkDir; it does not probe the remote (operational reachability surfaces at
// Acquire time).
func New(cfg Config) (*Lease, error) {
	if strings.TrimSpace(cfg.WorkDir) == "" {
		return nil, errors.New("lease: New requires a non-empty WorkDir")
	}

	gitBin := cfg.Git
	if gitBin == "" {
		gitBin = "git"
	}
	remote := cfg.Remote
	if remote == "" {
		// The blank-Config fallback resolves from the repo config schema's single
		// source (config.Defaults — "origin" on a default config, the ONE place
		// that literal lives, DINV-1), never an independent literal here. The
		// production caller (cmd/implement-now) resolves the configured remote and
		// passes it explicitly (config-driven-portability-v1 §3.2, FR-2); this
		// fallback only serves a caller that leaves it blank (the hermetic tests).
		remote = config.Defaults().Remote
	}
	refPrefix := cfg.RefPrefix
	if refPrefix == "" {
		refPrefix = defaultRefPrefix
	}
	if !strings.HasSuffix(refPrefix, "/") {
		refPrefix += "/"
	}
	holder := strings.TrimSpace(cfg.Holder)
	if holder == "" {
		holder = defaultHolder()
	}

	// Supply the claim commit's identity ourselves so commit-tree never depends
	// on (or is perturbed by) the checkout's git config — hermetic by construction.
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+holder,
		"GIT_AUTHOR_EMAIL="+holderEmail,
		"GIT_COMMITTER_NAME="+holder,
		"GIT_COMMITTER_EMAIL="+holderEmail,
	)

	return &Lease{
		gitBin:          gitBin,
		workDir:         cfg.WorkDir,
		remote:          remote,
		holder:          holder,
		refPrefix:       refPrefix,
		legacySourceKey: cfg.LegacySourceKey,
		env:             env,
		held:            make(map[string]struct{}),
	}, nil
}

// Holder returns the identity this adapter stamps on its claims. Exposed for
// observability and tests.
func (l *Lease) Holder() string { return l.holder }

// Acquire attempts an at-most-one claim on task via the git-ref CAS. On success
// it returns a Claim the caller must Release. When the task is already held —
// by a local sibling or the external fleet — it returns port.ErrClaimLost
// (wrapped, matchable with errors.Is): a skip, distinct from a hard error.
func (l *Lease) Acquire(ctx context.Context, task port.Task) (port.Claim, error) {
	if task.Number <= 0 {
		return nil, fmt.Errorf("lease: task %q has no backing issue number to key a claim on", task.ID)
	}
	ref := l.refPrefix + strconv.Itoa(task.Number)
	if scope := task.ScopeToken(); scope != "" {
		ref = l.refPrefix + scope + "/" + strconv.Itoa(task.Number)
	}
	refs := []string{ref}
	if task.SourceKey != "" && task.SourceKey == l.legacySourceKey {
		refs = append(refs, l.refPrefix+strconv.Itoa(task.Number))
	}

	// In-process guard first: reserve the ref for this process before touching
	// the remote so two local Workers never race the CAS (NR-2). A sibling that
	// already holds or is mid-acquire on this ref makes us skip immediately — the
	// same no-op a lost CAS yields (AP-22).
	if !l.reserve(refs...) {
		return nil, fmt.Errorf("lease: task #%d already held by a local worker: %w", task.Number, port.ErrClaimLost)
	}

	// Every failure past the reservation must release it, or a retry would be
	// wedged behind a phantom local hold.
	commit, err := l.mintClaim(ctx, task)
	if err != nil {
		l.unreserve(refs...)
		return nil, err
	}

	won, lost, err := l.casPush(ctx, refs, commit)
	switch {
	case err != nil:
		l.unreserve(refs...)
		return nil, err
	case lost:
		l.unreserve(refs...)
		return nil, fmt.Errorf("lease: task #%d already claimed on %s: %w", task.Number, l.remote, port.ErrClaimLost)
	case !won:
		// Unreachable: casPush returns exactly one of {won, lost, err}. Kept as a
		// fail-closed guard rather than a silent fall-through.
		l.unreserve(refs...)
		return nil, fmt.Errorf("lease: cas-push for task #%d returned no verdict", task.Number)
	}

	return &claim{lease: l, refs: refs, commit: commit}, nil
}

// reserve records all refs together, returning false without any reservation
// if a local sibling already holds or is mid-acquire on any of them.
func (l *Lease) reserve(refs ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, ref := range refs {
		if _, ok := l.held[ref]; ok {
			return false
		}
	}
	for _, ref := range refs {
		l.held[ref] = struct{}{}
	}
	return true
}

// unreserve drops this process's hold on ref, freeing it for a later Acquire.
func (l *Lease) unreserve(refs ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, ref := range refs {
		delete(l.held, ref)
	}
}

// mintClaim builds a parentless commit over the empty tree whose author,
// committer and message record this holder's identity and a unique nonce — the
// object the claim ref will point at (holder identity in commit metadata).
func (l *Lease) mintClaim(ctx context.Context, task port.Task) (string, error) {
	// The empty tree, written for this repo's object format (portable across
	// SHA-1/SHA-256 repos — no hard-coded hash, no /dev/null path).
	tree, stderr, err := l.git(ctx, "", "hash-object", "-w", "-t", "tree", "--stdin")
	if err != nil {
		return "", fmt.Errorf("lease: write empty tree: %s: %w", firstLine(stderr), err)
	}
	tree = strings.TrimSpace(tree)

	nonce, err := nonceHex()
	if err != nil {
		return "", fmt.Errorf("lease: generate claim nonce: %w", err)
	}

	msg := fmt.Sprintf(
		"pyxis claim: task %s (#%d)\n\nholder: %s\nnonce: %s\nacquired: %s\n",
		task.ID, task.Number, l.holder, nonce, time.Now().UTC().Format(time.RFC3339),
	)

	commit, stderr, err := l.git(ctx, "", "commit-tree", tree, "-m", msg)
	if err != nil {
		return "", fmt.Errorf("lease: mint claim commit for task #%d: %s: %w", task.Number, firstLine(stderr), err)
	}
	return strings.TrimSpace(commit), nil
}

// casPush performs the create-or-fail compare-and-swap: it pushes commit to all
// refs atomically with leases asserting none may exist. Exactly one of won, lost,
// err is set. A lost race is detected from the porcelain reject status, which
// reflects the ref's existence at push time on the server — no read-after-write
// race (unlike a follow-up existence probe).
func (l *Lease) casPush(ctx context.Context, refs []string, commit string) (won, lost bool, err error) {
	// "<ref>:" with an empty expected value means: the ref must not yet exist.
	args := []string{"push", "--porcelain"}
	if len(refs) > 1 {
		// Both generations must contend on one server transaction. A server
		// without atomic push support fails closed; never fall back to two pushes.
		args = append(args, "--atomic")
	}
	for _, ref := range refs {
		args = append(args, "--force-with-lease="+ref+":")
	}
	args = append(args, l.remote)
	for _, ref := range refs {
		args = append(args, commit+":"+ref)
	}
	stdout, stderr, runErr := l.git(ctx, "", args...)
	if runErr == nil {
		return true, false, nil
	}
	for _, ref := range refs {
		if !pushRejected(stdout, ref) {
			continue
		}
		// The server reached and declined our must-not-exist update: the ref
		// already exists — a sibling or the fleet holds it. A skip, not an error.
		return false, true, nil
	}
	// Anything else (unreachable remote, auth, fatal) is a genuine operational
	// failure, kept distinct from the skip so the Worker never treats a real
	// error as a lost race.
	return false, false, fmt.Errorf("lease: cas-push %v to %s: %s: %w", refs, l.remote, firstLine(stderr), runErr)
}

// releaseRemote deletes ref on the remote, but only while it still points at
// commit (our claim). A rejected delete means the ref is no longer ours — it is
// gone already, or a reap replaced it — which is exactly the released goal
// state, so it is success, not an error (DINV-5, AP-26). Only a delete that
// could not reach the remote is returned as an error.
func (l *Lease) releaseRemote(ctx context.Context, ref, commit string) error {
	lease := ref + ":" + commit
	stdout, stderr, err := l.git(ctx, "",
		"push", "--porcelain", "--force-with-lease="+lease, l.remote, "--delete", ref)
	if err == nil {
		return nil
	}
	if pushRejected(stdout, ref) {
		return nil
	}
	return fmt.Errorf("lease: release %s on %s: %s: %w", ref, l.remote, firstLine(stderr), err)
}

// git runs the git binary in the adapter's WorkDir with the claim-commit
// identity in the environment, returning stdout, stderr and the run error.
// stdin, when non-empty, is fed to the command.
func (l *Lease) git(ctx context.Context, stdin string, args ...string) (string, string, error) {
	// Claims are supervisor-owned metadata, not working-tree commits. Repository
	// hooks must not execute outside verification or gate their parentless CAS
	// commits as feature branches. This override applies only to this subprocess.
	cmd := exec.CommandContext(ctx, l.gitBin, append([]string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false"}, args...)...)
	cmd.Dir = l.workDir
	cmd.Env = l.env
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()
	return out.String(), errBuf.String(), err
}

// claim is a held git-ref claim. Release is idempotent (guarded by once) and
// safe on every terminal path — the Worker defers it (DINV-5).
type claim struct {
	lease  *Lease
	refs   []string
	commit string
	once   sync.Once
	err    error
}

// Release deletes the claim ref and frees the in-process reservation. It runs
// its effect at most once; subsequent calls return the first call's result.
func (c *claim) Release(ctx context.Context) error {
	c.once.Do(func() {
		for _, ref := range c.refs {
			c.err = errors.Join(c.err, c.lease.releaseRemote(ctx, ref, c.commit))
		}
		// Free the local reservation even if the remote delete failed: the claim
		// is terminal for this Worker, and any lingering ref is handled by the
		// fleet's age-reaper (out of scope here), while the remote CAS still
		// blocks any local re-grab until then.
		c.lease.unreserve(c.refs...)
	})
	return c.err
}

// pushRejected reports whether git push --porcelain output declined the update
// to ref. Porcelain ref-status lines are tab-separated "<flag>\t<from>:<to>\t
// <summary>"; a leading '!' flag marks a rejected ref update.
func pushRejected(porcelain, ref string) bool {
	for line := range strings.SplitSeq(porcelain, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || fields[0] != "!" {
			continue
		}
		if loc := strings.SplitN(fields[1], ":", 2); len(loc) == 2 && loc[1] == ref {
			return true
		}
	}
	return false
}

// defaultHolder derives a process-scoped holder identity from host and pid.
func defaultHolder() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown-host"
	}
	return fmt.Sprintf("pyxis-local/%s/%d", host, os.Getpid())
}

// nonceHex returns a random hex nonce, ensuring distinct claim-commit hashes
// even for the same holder within the same second.
func nonceHex() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// firstLine returns the first non-empty line of s, trimmed — a compact stderr
// summary for wrapped errors.
func firstLine(s string) string {
	for line := range strings.SplitSeq(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return "(no output)"
}
