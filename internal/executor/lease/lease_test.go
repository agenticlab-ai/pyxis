package lease

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// --- test harness: a real bare "remote" + clones, no network -----------------

// mustGit runs git in dir and fails the test on error, returning trimmed stdout.
func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (in %s): %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRemote creates an empty bare repository — the shared code host both a local
// run and the "fleet" claim against — and returns its path.
func newRemote(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "remote.git")
	mustGit(t, t.TempDir(), "init", "-q", "--bare", dir)
	return dir
}

// newClone creates a working clone with origin set to remote. It needs no
// initial commit or git identity: the Lease adapter supplies its own identity
// for claim commits, and claim refs are independent of any branch.
func newClone(t *testing.T, remote string) string {
	t.Helper()
	dir := t.TempDir()
	mustGit(t, dir, "init", "-q", dir)
	mustGit(t, dir, "remote", "add", "origin", remote)
	return dir
}

// newLease builds a Lease over a fresh clone of remote, with the given holder.
func newLease(t *testing.T, remote, holder string) *Lease {
	t.Helper()
	l, err := New(Config{WorkDir: newClone(t, remote), Remote: "origin", Holder: holder})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return l
}

// claimRefs reads the claim-ref namespace directly off the bare remote, sorted.
func claimRefs(t *testing.T, remote string) []string {
	t.Helper()
	out := mustGit(t, remote, "for-each-ref", "--format=%(refname)", defaultRefPrefix)
	if out == "" {
		return nil
	}
	refs := strings.Split(out, "\n")
	sort.Strings(refs)
	return refs
}

func task(id string, number int) port.Task {
	return port.Task{ID: id, Number: number, Title: id, Brief: "brief for " + id}
}

// --- the ref CAS: exactly one winner across independent processes (A-5) ------

// TestAcquire_RefCAS_ConcurrentAcrossProcesses exercises the shared git-ref
// compare-and-swap under real concurrency: K independent Leases (each its own
// clone + in-process guard, standing in for K separate processes / the fleet)
// race one task. Exactly one wins; every loser is a distinct ErrClaimLost skip.
func TestAcquire_RefCAS_ConcurrentAcrossProcesses(t *testing.T) {
	t.Parallel()
	remote := newRemote(t)
	const K = 8
	leases := make([]*Lease, K)
	for i := range leases {
		leases[i] = newLease(t, remote, "holder-"+string(rune('A'+i)))
	}

	type outcome struct {
		claim port.Claim
		err   error
	}
	results := make(chan outcome, K)
	var wg sync.WaitGroup
	start := make(chan struct{})
	tk := task("T4", 1)
	for _, l := range leases {
		wg.Add(1)
		go func(l *Lease) {
			defer wg.Done()
			<-start // release all racers at once
			c, err := l.Acquire(context.Background(), tk)
			results <- outcome{c, err}
		}(l)
	}
	close(start)
	wg.Wait()
	close(results)

	var winners []port.Claim
	losers := 0
	for r := range results {
		switch {
		case r.err == nil:
			winners = append(winners, r.claim)
		case errors.Is(r.err, port.ErrClaimLost):
			losers++
		default:
			t.Fatalf("loser returned a hard error, want ErrClaimLost skip: %v", r.err)
		}
	}
	if len(winners) != 1 {
		t.Fatalf("want exactly 1 CAS winner, got %d", len(winners))
	}
	if losers != K-1 {
		t.Fatalf("want %d skips, got %d", K-1, losers)
	}
	if got := claimRefs(t, remote); len(got) != 1 || got[0] != defaultRefPrefix+"1" {
		t.Fatalf("want a single held ref %s1, got %v", defaultRefPrefix, got)
	}

	// Every terminal path releases: the winner's Release leaves no ref behind.
	if err := winners[0].Release(context.Background()); err != nil {
		t.Fatalf("winner Release: %v", err)
	}
	if got := claimRefs(t, remote); len(got) != 0 {
		t.Fatalf("want no claim refs after release, got %v", got)
	}
}

// --- the in-process guard: exactly one winner within one process ------------

// TestAcquire_InProcessGuard_Concurrent exercises the local in-process guard
// under real concurrency: one shared Lease, N goroutines racing one task. The
// guard elects one before the CAS; the rest skip with ErrClaimLost. Run under
// -race, this is the guard's data-race and correctness check.
func TestAcquire_InProcessGuard_Concurrent(t *testing.T) {
	t.Parallel()
	remote := newRemote(t)
	l := newLease(t, remote, "holder-solo")

	const N = 16
	type outcome struct {
		claim port.Claim
		err   error
	}
	results := make(chan outcome, N)
	var wg sync.WaitGroup
	start := make(chan struct{})
	tk := task("T4", 2)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			c, err := l.Acquire(context.Background(), tk)
			results <- outcome{c, err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var winners []port.Claim
	for r := range results {
		if r.err == nil {
			winners = append(winners, r.claim)
			continue
		}
		if !errors.Is(r.err, port.ErrClaimLost) {
			t.Fatalf("in-proc loser returned a hard error, want ErrClaimLost: %v", r.err)
		}
	}
	if len(winners) != 1 {
		t.Fatalf("want exactly 1 winner past the guard, got %d", len(winners))
	}
	if err := winners[0].Release(context.Background()); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if got := claimRefs(t, remote); len(got) != 0 {
		t.Fatalf("want no claim refs after release, got %v", got)
	}
}

// TestAcquire_InProcessGuard_Sequential shows the guard blocks a local sibling
// while a claim is held, then frees the key on Release so a re-acquire succeeds.
func TestAcquire_InProcessGuard_Sequential(t *testing.T) {
	t.Parallel()
	remote := newRemote(t)
	l := newLease(t, remote, "holder-solo")
	ctx := context.Background()
	tk := task("T4", 3)

	first, err := l.Acquire(ctx, tk)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	// A local sibling reaching for the same held task skips immediately.
	if c, err := l.Acquire(ctx, tk); c != nil || !errors.Is(err, port.ErrClaimLost) {
		t.Fatalf("sibling Acquire while held: got (%v, %v), want (nil, ErrClaimLost)", c, err)
	}
	if err := first.Release(ctx); err != nil {
		t.Fatalf("Release: %v", err)
	}
	// The key is freed: a re-acquire on the same task now succeeds.
	again, err := l.Acquire(ctx, tk)
	if err != nil {
		t.Fatalf("re-Acquire after Release: %v", err)
	}
	if err := again.Release(ctx); err != nil {
		t.Fatalf("final Release: %v", err)
	}
}

// --- fleet coexistence: a fleet-held task is a skip, never a double-grab (A-5)

// TestAcquire_FleetHolds_LocalSkips proves a local run coexists with the
// external fleet on one repo: a task the fleet already claimed (a separate
// Lease/clone) yields ErrClaimLost locally — no double-grab (NR-2). Releasing
// the fleet claim frees the task for the local run.
func TestAcquire_FleetHolds_LocalSkips(t *testing.T) {
	t.Parallel()
	remote := newRemote(t)
	fleet := newLease(t, remote, "fleet-node-7")
	local := newLease(t, remote, "local-run")
	ctx := context.Background()
	tk := task("T4", 7)

	fleetClaim, err := fleet.Acquire(ctx, tk)
	if err != nil {
		t.Fatalf("fleet Acquire: %v", err)
	}
	if c, err := local.Acquire(ctx, tk); c != nil || !errors.Is(err, port.ErrClaimLost) {
		t.Fatalf("local Acquire of fleet-held task: got (%v, %v), want (nil, ErrClaimLost)", c, err)
	}
	// The fleet releases; the task is now free for the local run.
	if err := fleetClaim.Release(ctx); err != nil {
		t.Fatalf("fleet Release: %v", err)
	}
	localClaim, err := local.Acquire(ctx, tk)
	if err != nil {
		t.Fatalf("local Acquire after fleet release: %v", err)
	}
	if err := localClaim.Release(ctx); err != nil {
		t.Fatalf("local Release: %v", err)
	}
}

// --- release is idempotent and always leaves the goal state (DINV-5) ---------

// TestRelease_IdempotentAndComplete asserts every terminal path leaves no ref
// held: Release removes the ref, a second Release is a safe no-op, and the task
// is re-claimable afterward.
func TestRelease_IdempotentAndComplete(t *testing.T) {
	t.Parallel()
	remote := newRemote(t)
	l := newLease(t, remote, "holder")
	ctx := context.Background()
	tk := task("T4", 4)

	c, err := l.Acquire(ctx, tk)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if got := claimRefs(t, remote); len(got) != 1 {
		t.Fatalf("want 1 held ref after Acquire, got %v", got)
	}
	if err := c.Release(ctx); err != nil {
		t.Fatalf("first Release: %v", err)
	}
	if got := claimRefs(t, remote); len(got) != 0 {
		t.Fatalf("want no ref after Release, got %v", got)
	}
	// Fire-or-release: a double Release is safe and still errors nowhere.
	if err := c.Release(ctx); err != nil {
		t.Fatalf("second (idempotent) Release: %v", err)
	}
	// The task can be claimed again from scratch.
	c2, err := l.Acquire(ctx, tk)
	if err != nil {
		t.Fatalf("re-Acquire after full release: %v", err)
	}
	if err := c2.Release(ctx); err != nil {
		t.Fatalf("Release c2: %v", err)
	}
}

// TestRelease_ConcurrentDouble runs two Releases of one claim at once; under
// -race this proves Release is safe against concurrent terminal paths and the
// goal state (no ref) still holds.
func TestRelease_ConcurrentDouble(t *testing.T) {
	t.Parallel()
	remote := newRemote(t)
	l := newLease(t, remote, "holder")
	ctx := context.Background()
	c, err := l.Acquire(ctx, task("T4", 5))
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) { defer wg.Done(); errs[i] = c.Release(ctx) }(i)
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Fatalf("concurrent Release[%d]: %v", i, e)
		}
	}
	if got := claimRefs(t, remote); len(got) != 0 {
		t.Fatalf("want no ref after concurrent Release, got %v", got)
	}
}

// --- a lost race is a distinct skip, never a hard error ----------------------

// TestAcquire_HardErrorNotClaimLost proves the skip and a genuine operational
// failure are distinguishable: an unreachable remote fails with an error that
// is NOT ErrClaimLost, so a losing racer is never conflated with a real fault.
func TestAcquire_HardErrorNotClaimLost(t *testing.T) {
	t.Parallel()
	remote := newRemote(t)
	// A valid WorkDir clone, but a remote path that does not exist.
	l, err := New(Config{
		WorkDir: newClone(t, remote),
		Remote:  filepath.Join(t.TempDir(), "nonexistent.git"),
		Holder:  "holder",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c, err := l.Acquire(context.Background(), task("T4", 9))
	if c != nil {
		t.Fatalf("want no claim on a hard error, got %v", c)
	}
	if err == nil {
		t.Fatal("want an error for an unreachable remote")
	}
	if errors.Is(err, port.ErrClaimLost) {
		t.Fatalf("an operational failure must not be a lost-race skip: %v", err)
	}
}

// --- claim metadata + input guards -------------------------------------------

// TestAcquire_ClaimRecordsHolder confirms the claim ref points at a commit whose
// metadata records holder identity (the design's "holder in commit metadata").
func TestAcquire_ClaimRecordsHolder(t *testing.T) {
	t.Parallel()
	remote := newRemote(t)
	clone := newClone(t, remote)
	l, err := New(Config{WorkDir: clone, Remote: "origin", Holder: "holder-xyz"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	c, err := l.Acquire(ctx, task("T4", 11))
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer func() { _ = c.Release(ctx) }()

	sha := strings.Fields(mustGit(t, remote, "for-each-ref", "--format=%(objectname)", defaultRefPrefix+"11"))[0]
	body := mustGit(t, clone, "cat-file", "-p", sha)
	if !strings.Contains(body, "holder-xyz") {
		t.Fatalf("claim commit does not record holder identity:\n%s", body)
	}
	if !strings.Contains(body, "task T4 (#11)") {
		t.Fatalf("claim commit does not record the task:\n%s", body)
	}
}

// TestAcquire_NoIssueNumber rejects a task with no backing issue number: there
// is no key to claim on, and it is a wiring error, not a lost-race skip.
func TestAcquire_NoIssueNumber(t *testing.T) {
	t.Parallel()
	l := newLease(t, newRemote(t), "holder")
	c, err := l.Acquire(context.Background(), port.Task{ID: "T4", Number: 0})
	if c != nil {
		t.Fatalf("want no claim, got %v", c)
	}
	if err == nil || errors.Is(err, port.ErrClaimLost) {
		t.Fatalf("want a non-skip wiring error, got %v", err)
	}
}

// TestNew_RequiresWorkDir fails closed on a missing WorkDir.
func TestNew_RequiresWorkDir(t *testing.T) {
	t.Parallel()
	if _, err := New(Config{}); err == nil {
		t.Fatal("want an error for an empty WorkDir")
	}
}

// TestNew_RemoteResolution proves the remote resolution honors config: a blank
// Config.Remote resolves the config schema's single-source default (byte-identical
// to the old "origin" literal, DINV-1), while a configured non-default remote name
// is claimed against verbatim (config-driven-portability lift, FR-2). It CASes a
// real claim to prove the resolved remote is the one git actually pushes to.
func TestNew_RemoteResolution(t *testing.T) {
	t.Parallel()
	if config.Defaults().Remote != "origin" {
		t.Fatalf("config default remote is no longer byte-identical to the old literal (%q)", config.Defaults().Remote)
	}
	remote := newRemote(t)
	ctx := context.Background()

	t.Run("blank resolves the config default origin", func(t *testing.T) {
		clone := newClone(t, remote) // clone's remote is named "origin"
		l, err := New(Config{WorkDir: clone, Remote: "", Holder: "h"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if l.remote != config.Defaults().Remote {
			t.Fatalf("blank Remote resolved to %q, want the config default %q", l.remote, config.Defaults().Remote)
		}
		c, err := l.Acquire(ctx, task("T1", 101))
		if err != nil {
			t.Fatalf("Acquire against the default remote: %v", err)
		}
		_ = c.Release(ctx)
	})

	t.Run("configured non-default remote is honored", func(t *testing.T) {
		// A clone whose sole remote is deliberately named "upstream" (not "origin").
		clone := t.TempDir()
		mustGit(t, clone, "init", "-q", clone)
		mustGit(t, clone, "remote", "add", "upstream", remote)
		l, err := New(Config{WorkDir: clone, Remote: "upstream", Holder: "h"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if l.remote != "upstream" {
			t.Fatalf("configured Remote resolved to %q, want upstream", l.remote)
		}
		c, err := l.Acquire(ctx, task("T2", 102))
		if err != nil {
			t.Fatalf("Acquire against the configured remote: %v", err)
		}
		_ = c.Release(ctx)
	})
}

// TestReleaseDoesNotStompReclaim guards the destructive path (INV-6): after a
// holder's ref is (simulated) reaped and re-claimed by another holder, the
// original holder's Release must not delete the new holder's claim.
func TestReleaseDoesNotStompReclaim(t *testing.T) {
	t.Parallel()
	remote := newRemote(t)
	a := newLease(t, remote, "holder-A")
	b := newLease(t, remote, "holder-B")
	ctx := context.Background()
	tk := task("T4", 13)

	claimA, err := a.Acquire(ctx, tk)
	if err != nil {
		t.Fatalf("A Acquire: %v", err)
	}
	// Simulate a reap: the ref is deleted out from under A (as the fleet's
	// age-reaper would), then re-claimed by B.
	mustGit(t, remote, "update-ref", "-d", defaultRefPrefix+"13")
	claimB, err := b.Acquire(ctx, tk)
	if err != nil {
		t.Fatalf("B Acquire after reap: %v", err)
	}
	// A's Release must be a no-op on B's claim (guarded delete), not a stomp.
	if err := claimA.Release(ctx); err != nil {
		t.Fatalf("A Release (should no-op, not error): %v", err)
	}
	if got := claimRefs(t, remote); len(got) != 1 || got[0] != defaultRefPrefix+"13" {
		t.Fatalf("B's claim must survive A's release, got %v", got)
	}
	if err := claimB.Release(ctx); err != nil {
		t.Fatalf("B Release: %v", err)
	}
	if got := claimRefs(t, remote); len(got) != 0 {
		t.Fatalf("want no refs after B release, got %v", got)
	}
}
