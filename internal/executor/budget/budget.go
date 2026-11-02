// Package budget provides the machine-shared content-write budget used by
// metered adapters. The limits follow GitHub's published secondary limits:
// https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#about-secondary-rate-limits
//
// The shape is design:state-reader-v4 §3.8, and it follows from one idea: a
// coarser bound is safe only when it CONTAINS the finer ones. A coarse ledger
// standing beside them is not a fallback, it is a second full window.
//
//   - A resolved identity charges its own ledger AND the host umbrella; an
//     unresolved one charges the umbrella alone and never gets a ledger of
//     its own (DINV-9). So no set of processes writing as one identity holds
//     more than one window between them, whatever any of them resolved.
//   - Admission is atomic over the set a write charges — never "always two"
//     (DINV-11).
//   - Recovery is gated on a valid cutover marker: an absent ledger is free
//     only on a host whose barrier is paid, because an un-cut-over host is
//     exactly what an absent ledger looks like (DINV-6, DINV-13).
//
// Incident I7 is why the absent case matters: collapsing "missing" into
// "spent" made every ephemeral checkout charge itself a full hour-window for
// spend that never happened.
package budget

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	GitHubContentWritesPerMinute = 80
	GitHubContentWritesPerHour   = 500
)

// longestWindow is the widest window the budget meters, and therefore the
// span a conservative start must cover and a reaper must respect.
const longestWindow = time.Hour

// Ledger names which of the two windows a deferral event refers to. EVT-4
// carries it because "the queue is deep" is not actionable without knowing
// which bound produced it.
type Ledger string

const (
	LedgerIdentity Ledger = "identity"
	LedgerUmbrella Ledger = "host-umbrella"
)

// Event is one backpressure observation (prd:self-drive-wake/EVT-4).
//
// A deferral is announced at the moment it is DECIDED, carrying the projected
// wait and the deadline the blocking window refills — never elapsed time
// alone, because a wait an operator cannot distinguish from a hang is
// unobservable (DINV-8). Incident I7's only evidence read "deferred=0s" while
// the real wait was 59m59s.
type Event struct {
	// Ledger is which window blocked.
	Ledger Ledger
	// QueueDepth is how many callers are inside the guard for this budget in
	// this process — the backpressure, not the size of one request. Every
	// production call site acquires a single write, so reporting the request
	// size would pin this at 1 however deep the queue got (DINV-10).
	QueueDepth int
	// Deferred is the projected wait on a deferral, and the elapsed wait on
	// the grant that ends one.
	Deferred time.Duration
	// ResetAt is when the blocking window refills. Zero on a grant.
	ResetAt time.Time
	// Granted distinguishes the deferral decision from the grant resolving it.
	Granted bool
}

type Hook func(Event)

// IdentityResolver retries an identity lookup for a writer that began on the
// umbrella-only path. A successful result is sticky; an empty identity is not.
// The host is returned too so a changed medium target cannot silently charge
// the ledger selected at construction.
type IdentityResolver func(context.Context) (host, identity string)

type ledger struct {
	Minute []time.Time `json:"minute"`
	Hour   []time.Time `json:"hour"`
}

// queue counts callers inside Acquire on one logical budget. Shared between
// an identity ledger and its umbrella, because a caller waiting on either is
// one waiter in one queue. The count is this process's: the file lock
// coordinates spend, not intent, and a durable registry of intent would be
// shared mutable state whose staleness after a crash costs more than the
// number is worth (DINV-10).
type queue struct{ waiting atomic.Int64 }

// Budget admits content writes against one host's ledgers.
type Budget struct {
	host string
	// umbrellaPath is always set; identityPath is empty for an unresolved
	// identity, which charges the umbrella alone (DINV-9).
	umbrellaPath string
	identityPath string
	markerPath   string
	identityMu   sync.Mutex
	resolve      IdentityResolver

	q           *queue
	now         func() time.Time
	hook        Hook
	sleep       func(ctx context.Context, d time.Duration) error
	writeRecord func(path string, v any) error
}

// NewForIdentity builds the budget for one (medium host, authenticated
// identity). An empty identity is the documented unresolved fallback: it
// charges the umbrella alone, which is the same window every resolved
// identity on that host also charges — not a sibling ledger beside it. A
// sibling would hand two processes writing as one identity two full windows,
// which is under-throttling, the one direction the rule forbids.
func NewForIdentity(host, identity string, now func() time.Time, hook Hook) (*Budget, error) {
	return NewForIdentityResolver(host, identity, nil, now, hook)
}

// NewForIdentityResolver is NewForIdentity plus the transition A-13 binds: if
// identity is initially empty, each later write retries resolve until one
// succeeds, then all subsequent writes charge that identity and the umbrella.
func NewForIdentityResolver(host, identity string, resolve IdentityResolver, now func() time.Time, hook Hook) (*Budget, error) {
	umbrella, err := HostPath(host)
	if err != nil {
		return nil, err
	}
	marker, err := MarkerPath(host)
	if err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	b := &Budget{
		host: host, umbrellaPath: umbrella, markerPath: marker,
		q: &queue{}, now: now, hook: hook, sleep: realSleep,
		resolve: resolve, writeRecord: writeDurable,
	}
	if identity != "" {
		if b.identityPath, err = Path(host, identity); err != nil {
			return nil, err
		}
	}
	return b, nil
}

// identityForAcquire fixes one admission's ledger set before any lock is
// taken. A failed lookup remains retryable for the next write; a success is
// memoized under the mutex so concurrent first resolvers cannot choose two
// identity ledgers for one Budget.
func (b *Budget) identityForAcquire(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	b.identityMu.Lock()
	defer b.identityMu.Unlock()
	if b.identityPath != "" || b.resolve == nil {
		return b.identityPath, nil
	}
	host, identity := b.resolve(ctx)
	if host != b.host {
		return "", fmt.Errorf("budget: identity resolver changed host from %q to %q", b.host, host)
	}
	if identity == "" {
		return "", nil
	}
	path, err := Path(host, identity)
	if err != nil {
		return "", err
	}
	b.identityPath = path
	b.resolve = nil
	return path, nil
}

func realSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Acquire admits n content writes, blocking until every window this write
// charges has room.
//
// The capacity decision is atomic over that set (DINV-11): both locks are
// held in a canonical order — umbrella first, which is what makes holding two
// deadlock-free — both windows are tested against one `now`, and the decision
// is taken for both together or for neither. Nothing is charged while
// waiting: a blocked caller records nothing anywhere before releasing.
//
// What survives a failure mid-commit is DINV-12's, not this function's
// promise: see commit.
func (b *Budget) Acquire(ctx context.Context, n int) error {
	if n <= 0 {
		return nil
	}
	if n > GitHubContentWritesPerMinute || n > GitHubContentWritesPerHour {
		return fmt.Errorf("budget: request of %d writes exceeds the shared window", n)
	}
	identityPath, err := b.identityForAcquire(ctx)
	if err != nil {
		return err
	}

	b.q.waiting.Add(1)
	defer b.q.waiting.Add(-1)

	uLock, err := lockFile(b.umbrellaPath)
	if err != nil {
		return err
	}
	defer uLock.Close()

	var iLock *os.File
	if identityPath != "" {
		if iLock, err = lockFile(identityPath); err != nil {
			return err
		}
		defer iLock.Close()
	}

	started := b.now()
	deferred := false
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		wait, reset, blocking, err := b.attempt(ctx, uLock, iLock, identityPath, n)
		if err != nil {
			return err
		}
		if wait == 0 {
			if deferred && b.hook != nil {
				b.hook(Event{QueueDepth: b.depth(), Deferred: b.now().Sub(started), Granted: true})
			}
			return nil
		}
		deferred = true
		if b.hook != nil {
			// The projected wait, announced at every deferral decision — so a
			// long block re-announces itself instead of falling silent behind
			// one timer (DINV-8; AP-16).
			b.hook(Event{Ledger: blocking, QueueDepth: b.depth(), Deferred: wait, ResetAt: reset})
		}
		if err := b.sleep(ctx, wait); err != nil {
			return err
		}
	}
}

// attempt takes both locks, tests every window once, and either commits or
// reports the wait. wait == 0 means admitted.
func (b *Budget) attempt(ctx context.Context, uLock, iLock *os.File, identityPath string, n int) (time.Duration, time.Time, Ledger, error) {
	if err := lockContext(ctx, uLock); err != nil {
		return 0, time.Time{}, "", err
	}
	defer unlock(uLock)
	if iLock != nil {
		if err := lockContext(ctx, iLock); err != nil {
			return 0, time.Time{}, "", err
		}
		defer unlock(iLock)
	}

	now := b.now()
	// The barrier, before any capacity test: an absent ledger means nothing
	// on a host that has not cut over (DINV-13).
	if err := b.ensureBarrier(now, identityPath); err != nil {
		return 0, time.Time{}, "", err
	}
	if err := reapTemps(b.umbrellaPath, now); err != nil {
		return 0, time.Time{}, "", err
	}

	umbrella, replaceUmbrella := b.read(b.umbrellaPath, now)
	if replaceUmbrella {
		if err := b.writeRecord(b.umbrellaPath, umbrella); err != nil {
			return 0, time.Time{}, "", err
		}
	}
	umbrella.prune(now)

	var identity ledger
	if identityPath != "" {
		var replaceIdentity bool
		identity, replaceIdentity = b.read(identityPath, now)
		if replaceIdentity {
			if err := b.writeRecord(identityPath, identity); err != nil {
				return 0, time.Time{}, "", err
			}
		}
		identity.prune(now)
	}

	// One test over every window this write charges.
	if w, reset := umbrella.wait(now, n); w > 0 {
		return w, reset, LedgerUmbrella, nil
	}
	if identityPath != "" {
		if w, reset := identity.wait(now, n); w > 0 {
			return w, reset, LedgerIdentity, nil
		}
	}

	return 0, time.Time{}, "", b.commit(umbrella, identity, identityPath, now, n)
}

// commit records the admitted write. The umbrella is made durable FIRST, so
// the reachable outcomes are neither / umbrella-only / both — identity-only
// is unreachable, because it would leave a write charged to no host bound
// (DINV-12).
//
// Umbrella-only is reachable by ORDINARY FAILURE, not only by crash: an fsync
// or rename error on the identity ledger produces it during a healthy run.
// The handling is bound rather than left open — the error is returned, the
// caller sends no content write, and the umbrella's charge is RETAINED.
// Retaining over-throttles the host by at most n and drains on the clock;
// rolling back would need a second durable write that can fail in turn,
// trading a bounded over-charge for an unbounded under-charge.
func (b *Budget) commit(umbrella, identity ledger, identityPath string, now time.Time, n int) error {
	umbrella.add(now, n)
	if err := b.writeRecord(b.umbrellaPath, umbrella); err != nil {
		return err
	}
	if identityPath == "" {
		return nil
	}
	identity.add(now, n)
	if err := b.writeRecord(identityPath, identity); err != nil {
		return fmt.Errorf("budget: umbrella charged but identity ledger not persisted (charge retained, no write sent): %w", err)
	}
	return nil
}

// ensureBarrier pays this host's first-initialization barrier if the cutover
// marker does not vouch for it.
//
// This is a SEPARATE ORDERED TRANSACTION, not part of admission, and it is
// deliberately exempt from Acquire's "nothing is charged while waiting" rule
// (DINV-11): it persists conservative ledgers and then the marker BEFORE any
// caller waits. The barrier belongs to the host, not to the caller, so a
// caller cancelled mid-wait leaves it paid rather than making the next
// process pay again.
//
// Ordering is the safe asymmetry: ledgers durable, then the marker. A crash
// can leave the barrier unpaid and repayable; it can never leave a marker
// claiming a barrier that was not paid.
func (b *Budget) ensureBarrier(now time.Time, identityPath string) error {
	if markerValid(b.markerPath, b.host) {
		return nil
	}
	spent := spentLedger(now)
	if err := b.writeRecord(b.umbrellaPath, spent); err != nil {
		return err
	}
	if identityPath != "" {
		if err := b.writeRecord(identityPath, spent); err != nil {
			return err
		}
	}
	return b.writeRecord(b.markerPath, cutoverMarker{Schema: markerSchema, Host: b.host})
}

// read loads a ledger. The two failure states are deliberately NOT the same
// (DINV-6): an ABSENT ledger on a cut-over host had no prior holder, so the
// windows are genuinely unspent and it starts EMPTY, persisting nothing
// before the grant. An UNREADABLE one may encode spend this process cannot
// see, so it starts conservative.
//
// The barrier has already run by the time this is called, so "absent" here
// always means absent on a host whose marker vouches for it.
// The bool reports that the on-disk bytes were unusable and the conservative
// ledger returned here must be PERSISTED before the caller proceeds. Without
// that replacement every process re-reads the same corruption forever: the
// projected wait is recomputed from a fresh conservative ledger on each pass,
// so the caller sleeps a window, wakes, reads the same bad bytes, and sleeps
// again — a livelock that never converges, and one the tests caught.
func (b *Budget) read(path string, now time.Time) (ledger, bool) {
	data, err := readPrivateFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ledger{Minute: []time.Time{}, Hour: []time.Time{}}, false
		}
		return spentLedger(now), true
	}
	var l ledger
	if decodeClosedJSON(data, &l) != nil || !l.valid() {
		return spentLedger(now), true
	}
	return l, false
}

func (b *Budget) depth() int {
	if d := b.q.waiting.Load(); d > 0 {
		return int(d)
	}
	return 1
}

func lockContext(ctx context.Context, f *os.File) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			return fmt.Errorf("budget: flock: %w", err)
		}
		t := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

// valid rejects a decoded but unusable ledger. A missing window or a zero
// timestamp is indistinguishable from corruption at this boundary; fail
// closed rather than grant an unbudgeted burst.
func (l ledger) valid() bool {
	if l.Minute == nil || l.Hour == nil {
		return false
	}
	for _, ts := range append(append([]time.Time{}, l.Minute...), l.Hour...) {
		if ts.IsZero() {
			return false
		}
	}
	return true
}

func (l *ledger) prune(now time.Time) {
	l.Minute = prune(l.Minute, now.Add(-time.Minute))
	l.Hour = prune(l.Hour, now.Add(-longestWindow))
}

func (l *ledger) add(now time.Time, n int) {
	for i := 0; i < n; i++ {
		l.Minute = append(l.Minute, now)
		l.Hour = append(l.Hour, now)
	}
}

// wait projects how long until this ledger has room for n more writes, and
// when the blocking window refills. Every duration is measured against the
// caller-supplied now — the same clock that stamps the entries — so a test
// can drive an hour-scale window in microseconds and assert the wait as a
// value. Reading the process clock here while entries came from an injected
// one is what made incident I7's fabricated 59m59s wait untestable.
func (l ledger) wait(now time.Time, n int) (time.Duration, time.Time) {
	var w time.Duration
	var reset time.Time
	if len(l.Minute)+n > GitHubContentWritesPerMinute && len(l.Minute) > 0 {
		if d := l.Minute[0].Add(time.Minute).Sub(now); d > w {
			w, reset = d, l.Minute[0].Add(time.Minute)
		}
	}
	if len(l.Hour)+n > GitHubContentWritesPerHour && len(l.Hour) > 0 {
		if d := l.Hour[0].Add(longestWindow).Sub(now); d > w {
			w, reset = d, l.Hour[0].Add(longestWindow)
		}
	}
	if w <= 0 {
		return 0, time.Time{}
	}
	return w, reset
}

func spentLedger(now time.Time) ledger {
	l := ledger{
		Minute: make([]time.Time, GitHubContentWritesPerMinute),
		Hour:   make([]time.Time, GitHubContentWritesPerHour),
	}
	for i := range l.Minute {
		l.Minute[i] = now
	}
	for i := range l.Hour {
		l.Hour[i] = now
	}
	return l
}

func prune(in []time.Time, cutoff time.Time) []time.Time {
	out := in[:0]
	for _, t := range in {
		if t.After(cutoff) {
			out = append(out, t)
		}
	}
	return out
}
