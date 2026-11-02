package budget

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock is goroutine-safe and its sleep ADVANCES rather than waits, which
// is what makes an hour-scale window assertable in microseconds
// (design:state-reader-v4#A-11).
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func (c *fakeClock) sleep(_ context.Context, d time.Duration) error {
	c.advance(d)
	return nil
}

// newBudget builds a budget on an isolated state dir with the injected clock
// wired through every seam.
func newBudget(t *testing.T, c *fakeClock, host, identity string, hook Hook) *Budget {
	t.Helper()
	b, err := NewForIdentity(host, identity, c.now, hook)
	if err != nil {
		t.Fatalf("NewForIdentity: %v", err)
	}
	b.sleep = c.sleep
	return b
}

// cutOver pays the host's barrier up front, so a test exercises steady state
// rather than the barrier (A-6's precondition).
func cutOver(t *testing.T, host string) {
	t.Helper()
	path, err := MarkerPath(host)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeMarker(path, host); err != nil {
		t.Fatal(err)
	}
}

func readLedger(t *testing.T, path string) ledger {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	var l ledger
	if err := json.Unmarshal(data, &l); err != nil {
		t.Fatalf("decode ledger: %v", err)
	}
	return l
}

func TestBudgetCheapBoundaryPaths(t *testing.T) {
	if err := (&Budget{}).Acquire(t.Context(), 0); err != nil {
		t.Fatalf("zero-size acquire: %v", err)
	}
	if err := (&Budget{}).Acquire(t.Context(), GitHubContentWritesPerMinute+1); err == nil {
		t.Fatal("oversized acquire was accepted")
	}

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := (&Budget{}).identityForAcquire(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled identity resolution = %v", err)
	}
	unresolved := &Budget{host: "github.com", resolve: func(context.Context) (string, string) {
		return "github.com", ""
	}}
	if path, err := unresolved.identityForAcquire(t.Context()); err != nil || path != "" {
		t.Fatalf("unresolved identity path = %q, %v", path, err)
	}
	if err := realSleep(cancelled, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled sleep = %v", err)
	}
	if got := (&Budget{q: &queue{}}).depth(); got != 1 {
		t.Fatalf("empty queue depth = %d", got)
	}
}

// --- A-6: steady state, under a valid marker -------------------------------

// An absent ledger on a CUT-OVER host had no prior holder, so the windows are
// genuinely unspent: the write is granted immediately and nothing is
// fabricated on disk. This is incident I7's repair, and the marker is what
// makes "absent" informative enough to rely on.
func TestAbsentLedgerUnderValidMarkerGrantsImmediately(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	c := newClock()
	start := c.now()

	var deferrals int
	b := newBudget(t, c, "github.com", "octocat", func(e Event) {
		if !e.Granted {
			deferrals++
		}
	})
	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("cold-start acquire: %v", err)
	}

	if deferrals != 0 {
		t.Errorf("cold start deferred %d time(s), want 0", deferrals)
	}
	if elapsed := c.now().Sub(start); elapsed != 0 {
		t.Errorf("cold start advanced the clock by %s, want 0", elapsed)
	}
	l := readLedger(t, b.identityPath)
	if len(l.Minute) != 1 || len(l.Hour) != 1 {
		t.Errorf("ledger = %d minute / %d hour entries, want 1/1 — a full window was fabricated",
			len(l.Minute), len(l.Hour))
	}
}

// Bytes that exist but cannot be read may encode spend this process cannot
// see, so they start conservative — the half of the fail-closed rule that
// stays.
func TestUnreadableLedgerStartsConservative(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	c := newClock()
	b := newBudget(t, c, "github.com", "octocat", nil)

	if err := os.MkdirAll(filepath.Dir(b.identityPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b.identityPath, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	before := c.now()
	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if c.now().Sub(before) == 0 {
		t.Fatal("an unreadable ledger granted immediately, want a conservative wait")
	}
}

// --- A-12 / DINV-9: containment -------------------------------------------

// A resolved and an unresolved writer on one host contend for ONE window, in
// both directions. If the unresolved writer had a ledger of its own, the pair
// would hold two full windows against what may be a single quota — the
// under-throttling DINV-9 exists to prevent.
func TestResolvedAndUnresolvedWritersShareOneWindow(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	c := newClock()

	resolved := newBudget(t, c, "github.com", "octocat", nil)
	unresolved := newBudget(t, c, "github.com", "", nil)

	for i := 0; i < GitHubContentWritesPerMinute; i++ {
		if err := resolved.Acquire(context.Background(), 1); err != nil {
			t.Fatalf("resolved acquire %d: %v", i, err)
		}
	}

	before := c.now()
	if err := unresolved.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("unresolved acquire: %v", err)
	}
	if c.now().Sub(before) == 0 {
		t.Fatal("the unresolved writer wrote immediately against a spent window — the quota was split")
	}
}

func TestUnresolvedSpendIsVisibleToResolvedWriter(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	c := newClock()

	unresolved := newBudget(t, c, "github.com", "", nil)
	resolved := newBudget(t, c, "github.com", "octocat", nil)

	for i := 0; i < GitHubContentWritesPerMinute; i++ {
		if err := unresolved.Acquire(context.Background(), 1); err != nil {
			t.Fatalf("unresolved acquire %d: %v", i, err)
		}
	}
	before := c.now()
	if err := resolved.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("resolved acquire: %v", err)
	}
	if c.now().Sub(before) == 0 {
		t.Fatal("the resolved writer ignored the umbrella's spend — the quota was split")
	}
}

// --- A-13: the unresolved path is admissible on its own terms --------------

func TestUnresolvedWriterChargesUmbrellaAlone(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	c := newClock()
	b := newBudget(t, c, "github.com", "", nil)

	if b.identityPath != "" {
		t.Fatalf("an unresolved writer was given an identity ledger at %s", b.identityPath)
	}
	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if l := readLedger(t, b.umbrellaPath); len(l.Minute) != 1 {
		t.Errorf("umbrella holds %d entries, want 1", len(l.Minute))
	}
	entries, _ := os.ReadDir(filepath.Dir(b.umbrellaPath))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") && e.Name() != "_host.json" && e.Name() != "_cutover.json" {
			t.Errorf("an unresolved writer created a ledger of its own: %s", e.Name())
		}
	}
}

func TestWriterBeginsChargingIdentityAfterResolutionRecovers(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	c := newClock()
	calls := 0
	b, err := NewForIdentityResolver("github.com", "", func(context.Context) (string, string) {
		calls++
		if calls == 1 {
			return "github.com", ""
		}
		return "github.com", "octocat"
	}, c.now, nil)
	if err != nil {
		t.Fatal(err)
	}
	b.sleep = c.sleep

	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("unresolved acquire: %v", err)
	}
	if b.identityPath != "" {
		t.Fatalf("first unresolved write selected %s", b.identityPath)
	}
	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("resolved acquire: %v", err)
	}
	if b.identityPath == "" {
		t.Fatal("successful retry did not attach an identity ledger")
	}
	if got := len(readLedger(t, b.umbrellaPath).Minute); got != 2 {
		t.Errorf("umbrella holds %d writes, want both pre/post-resolution writes", got)
	}
	if got := len(readLedger(t, b.identityPath).Minute); got != 1 {
		t.Errorf("identity holds %d writes, want only the post-resolution write", got)
	}
}

func TestIdentityResolverCannotMoveBudgetToAnotherHost(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	b, err := NewForIdentityResolver("github.com", "", func(context.Context) (string, string) {
		return "ghe.example.com", ""
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Acquire(context.Background(), 1); err == nil {
		t.Fatal("budget admitted after the identity resolver changed medium host")
	}
	if _, err := os.Stat(b.umbrellaPath); !os.IsNotExist(err) {
		t.Fatalf("host-change failure charged the old umbrella: %v", err)
	}
}

// --- A-15 / DINV-13: the barrier ------------------------------------------

// With no marker, an absent ledger means nothing: the host has not cut over,
// and an un-cut-over host is indistinguishable from one whose earlier build
// was writing moments ago. The first initialization is conservative.
func TestUncutHostPaysTheBarrier(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	c := newClock()
	b := newBudget(t, c, "github.com", "octocat", nil)

	before := c.now()
	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	waited := c.now().Sub(before)
	if waited == 0 {
		t.Fatal("an un-cut-over host granted immediately — the barrier was skipped")
	}
	if !markerValid(b.markerPath, "github.com") {
		t.Error("the barrier was paid but no valid marker was recorded")
	}
}

// The barrier is the HOST's, not the caller's: once paid, a second process
// does not pay it again.
func TestBarrierIsPaidOncePerHost(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	c := newClock()

	first := newBudget(t, c, "github.com", "octocat", nil)
	if err := first.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	c.advance(2 * longestWindow) // drain what the barrier charged

	second := newBudget(t, c, "github.com", "hubber", nil)
	before := c.now()
	if err := second.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	if c.now().Sub(before) != 0 {
		t.Error("a second identity on a cut-over host paid the barrier again")
	}
}

func TestCancelledFirstInitializerLeavesPaidBarrier(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	c := newClock()
	b := newBudget(t, c, "github.com", "octocat", nil)
	b.sleep = func(context.Context, time.Duration) error { return context.Canceled }

	if err := b.Acquire(context.Background(), 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("first initializer error = %v, want context.Canceled", err)
	}
	if !markerValid(b.markerPath, "github.com") {
		t.Fatal("cancelled initializer did not leave the host barrier paid")
	}
	if got := len(readLedger(t, b.umbrellaPath).Minute); got != GitHubContentWritesPerMinute {
		t.Fatalf("umbrella holds %d conservative entries, want %d", got, GitHubContentWritesPerMinute)
	}
	if got := len(readLedger(t, b.identityPath).Minute); got != GitHubContentWritesPerMinute {
		t.Fatalf("identity holds %d conservative entries, want %d", got, GitHubContentWritesPerMinute)
	}

	second := newBudget(t, c, "github.com", "octocat", nil)
	markerWrites := 0
	baseWrite := second.writeRecord
	second.writeRecord = func(path string, v any) error {
		if path == second.markerPath {
			markerWrites++
		}
		return baseWrite(path, v)
	}
	if err := second.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("second initializer: %v", err)
	}
	if markerWrites != 0 {
		t.Fatalf("next process repaid the barrier %d time(s)", markerWrites)
	}
}

func TestConcurrentInitializersPayBarrierExactlyOnce(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	c := newClock()
	first := newBudget(t, c, "github.com", "octocat", nil)
	second := newBudget(t, c, "github.com", "hubber", nil)
	var markerWrites atomic.Int32
	for _, b := range []*Budget{first, second} {
		baseWrite := b.writeRecord
		b.writeRecord = func(path string, v any) error {
			if path == b.markerPath {
				markerWrites.Add(1)
			}
			return baseWrite(path, v)
		}
	}
	var wg sync.WaitGroup
	for _, b := range []*Budget{first, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := b.Acquire(context.Background(), 1); err != nil {
				t.Errorf("concurrent initializer: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := markerWrites.Load(); got != 1 {
		t.Fatalf("concurrent initializers wrote the marker %d times, want exactly 1", got)
	}
}

// A second HOST arms its own barrier — the marker is per (machine, host), so
// a machine-global one would wrongly exempt a host never written to.
func TestSecondHostArmsItsOwnBarrier(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	c := newClock()

	first := newBudget(t, c, "github.com", "octocat", nil)
	if err := first.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	c.advance(2 * longestWindow)

	other := newBudget(t, c, "ghe.example.com", "octocat", nil)
	before := c.now()
	if err := other.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("other-host acquire: %v", err)
	}
	if c.now().Sub(before) == 0 {
		t.Error("a second host skipped its barrier")
	}
}

// A marker is never given the benefit of the doubt: absent, unparseable,
// truncated, host-mismatched and unknown-version all arm the barrier.
func TestInvalidMarkersAllArmTheBarrier(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"unparseable", "not json"},
		{"truncated", `{"schema":1,"ho`},
		{"host mismatch", `{"schema":1,"host":"elsewhere.example.com"}`},
		{"unknown version", `{"schema":99,"host":"github.com"}`},
		{"unknown field", `{"schema":1,"host":"github.com","future":true}`},
		{"empty", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			c := newClock()
			b := newBudget(t, c, "github.com", "octocat", nil)
			if err := os.MkdirAll(filepath.Dir(b.markerPath), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(b.markerPath, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			before := c.now()
			if err := b.Acquire(context.Background(), 1); err != nil {
				t.Fatalf("acquire: %v", err)
			}
			if c.now().Sub(before) == 0 {
				t.Errorf("a %s marker was trusted — the barrier was skipped", tc.name)
			}
		})
	}
}

func TestMarkerWithNonPrivateModeArmsBarrier(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	c := newClock()
	b := newBudget(t, c, "github.com", "octocat", nil)
	if err := os.MkdirAll(filepath.Dir(b.markerPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b.markerPath, []byte(`{"schema":1,"host":"github.com"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(b.markerPath, 0o644); err != nil {
		t.Fatal(err)
	}
	before := c.now()
	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if c.now().Sub(before) == 0 {
		t.Fatal("schema-valid marker with non-private mode skipped the barrier")
	}
}

func TestSchemaValidMarkerFromForeignOwnerIsRejected(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path, err := MarkerPath("github.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"schema":1,"host":"github.com"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if !markerValidWith(path, "github.com", os.Geteuid) {
		t.Fatal("owner test fixture is not otherwise a valid marker")
	}

	foreignUID := func() int { return os.Geteuid() ^ 1 }
	if markerValidWith(path, "github.com", foreignUID) {
		t.Fatal("schema-valid marker owned by a different effective user was trusted")
	}
}

// --- A-17: marker lifecycle across a version change ------------------------

func TestInvalidateMarkerReArmsTheBarrier(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	c := newClock()

	b := newBudget(t, c, "github.com", "octocat", nil)
	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	c.advance(2 * longestWindow)

	// A same-build restart retains the marker and costs no second barrier.
	again := newBudget(t, c, "github.com", "octocat", nil)
	before := c.now()
	if err := again.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("restart acquire: %v", err)
	}
	if c.now().Sub(before) != 0 {
		t.Error("a same-build restart re-paid the barrier")
	}

	// Invalidation completes only when it returns nil; a re-upgrade then pays.
	if err := InvalidateMarker("github.com"); err != nil {
		t.Fatalf("invalidate: %v", err)
	}
	if markerValid(b.markerPath, "github.com") {
		t.Fatal("the marker survived a completed invalidation")
	}
	c.advance(2 * longestWindow)
	reupgrade := newBudget(t, c, "github.com", "octocat", nil)
	before = c.now()
	if err := reupgrade.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("re-upgrade acquire: %v", err)
	}
	if c.now().Sub(before) == 0 {
		t.Error("the re-upgrade skipped its barrier after invalidation")
	}
}

// --- A-10 / DINV-8, DINV-10: the deferral is observable --------------------

func TestDeferralNamesLedgerWaitAndReset(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	c := newClock()

	var mu sync.Mutex
	var deferrals []Event
	b := newBudget(t, c, "github.com", "octocat", func(e Event) {
		if e.Granted {
			return
		}
		mu.Lock()
		deferrals = append(deferrals, e)
		mu.Unlock()
	})

	saturate := c.now()
	for i := 0; i < GitHubContentWritesPerMinute; i++ {
		if err := b.Acquire(context.Background(), 1); err != nil {
			t.Fatalf("saturating acquire %d: %v", i, err)
		}
	}
	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("acquire past the minute window: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(deferrals) == 0 {
		t.Fatal("a saturated window produced no deferral event")
	}
	for i, e := range deferrals {
		if e.Deferred <= 0 {
			t.Errorf("deferral %d reported %s — a deferral never lasts zero time", i, e.Deferred)
		}
		if e.ResetAt.IsZero() {
			t.Errorf("deferral %d carried no reset deadline", i)
		}
		if e.Ledger == "" {
			t.Errorf("deferral %d did not name the blocking ledger", i)
		}
	}
	if want := saturate.Add(time.Minute); !deferrals[0].ResetAt.Equal(want) {
		t.Errorf("first deferral resets at %s, want %s", deferrals[0].ResetAt, want)
	}
}

// queue_depth is the backpressure, not the request size. Every waiter is held
// inside its deferral until all have arrived, so the fixture's state is the
// state this assertion names — a clock that advanced on arrival would drain
// the window for whoever got there first and the rest would never queue
// (incident I8).
func TestQueueDepthReportsWaitersNotRequestSize(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	c := newClock()

	var mu sync.Mutex
	maxDepth := 0
	b := newBudget(t, c, "github.com", "octocat", func(e Event) {
		if e.Granted {
			return
		}
		mu.Lock()
		if e.QueueDepth > maxDepth {
			maxDepth = e.QueueDepth
		}
		mu.Unlock()
	})

	for i := 0; i < GitHubContentWritesPerMinute; i++ {
		if err := b.Acquire(context.Background(), 1); err != nil {
			t.Fatalf("saturating acquire %d: %v", i, err)
		}
	}

	const waiters = 9
	arrived := make(chan struct{}, waiters)
	release := make(chan struct{})
	b.sleep = func(_ context.Context, d time.Duration) error {
		arrived <- struct{}{}
		<-release
		c.advance(d)
		return nil
	}

	var wg sync.WaitGroup
	for i := 0; i < waiters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := b.Acquire(context.Background(), 1); err != nil {
				t.Errorf("queued acquire: %v", err)
			}
		}()
	}
	for i := 0; i < waiters; i++ {
		<-arrived
	}
	close(release)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if maxDepth < waiters {
		t.Errorf("deepest reported queue = %d with %d concurrent waiters, want %d — the event is reporting request size",
			maxDepth, waiters, waiters)
	}
}

// A grant that never waited is not backpressure. Comparing clock values would
// report ordinary time passing as a deferral.
func TestUncontendedGrantEmitsNoEvent(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")

	var events []Event
	b, err := NewForIdentity("github.com", "octocat", time.Now, func(e Event) {
		events = append(events, e)
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := b.Acquire(context.Background(), 1); err != nil {
			t.Fatalf("acquire %d: %v", i, err)
		}
	}
	if len(events) != 0 {
		t.Errorf("%d event(s) emitted for uncontended writes, want 0: %+v", len(events), events)
	}
}

// --- A-11: the clock is injectable end to end ------------------------------

func TestHourWindowDrainsWithoutWallClock(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	c := newClock()
	b := newBudget(t, c, "github.com", "octocat", nil)
	started := c.now()
	var slept time.Duration
	b.sleep = func(ctx context.Context, d time.Duration) error {
		slept += d
		return c.sleep(ctx, d)
	}

	for i := 0; i < GitHubContentWritesPerHour; i++ {
		if err := b.Acquire(context.Background(), 1); err != nil {
			t.Fatalf("acquire %d: %v", i, err)
		}
	}
	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("acquire past the hour window: %v", err)
	}
	if advanced := c.now().Sub(started); advanced != longestWindow || slept != longestWindow {
		t.Fatalf("draining an hour window advanced clock=%s sleep=%s, want both %s", advanced, slept, longestWindow)
	}
}

// --- A-14: admission is atomic; a cancelled caller charges nothing ---------

func TestCancelledCallerChargesNothing(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	c := newClock()
	b := newBudget(t, c, "github.com", "octocat", nil)

	for i := 0; i < GitHubContentWritesPerMinute; i++ {
		if err := b.Acquire(context.Background(), 1); err != nil {
			t.Fatalf("saturating acquire %d: %v", i, err)
		}
	}
	umbrellaBefore := len(readLedger(t, b.umbrellaPath).Minute)

	// A sleep that cancels rather than advancing: the caller gives up mid-wait.
	ctx, cancel := context.WithCancel(context.Background())
	b.sleep = func(context.Context, time.Duration) error {
		cancel()
		return context.Canceled
	}
	if err := b.Acquire(ctx, 1); err == nil {
		t.Fatal("a cancelled acquire returned success")
	}

	if got := len(readLedger(t, b.umbrellaPath).Minute); got != umbrellaBefore {
		t.Errorf("umbrella grew from %d to %d across a cancelled acquire — a deferred caller charged a window",
			umbrellaBefore, got)
	}
}

func TestRepeatedCancellationOnIdentityWindowLeavesUmbrellaEmpty(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	c := newClock()
	b := newBudget(t, c, "github.com", "octocat", nil)
	identity := spentLedger(c.now())
	if err := writeDurable(b.identityPath, identity); err != nil {
		t.Fatal(err)
	}
	b.sleep = func(context.Context, time.Duration) error { return context.Canceled }

	for i := 0; i < 3; i++ {
		if err := b.Acquire(context.Background(), 1); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled acquire %d error = %v", i, err)
		}
	}
	if _, err := os.Stat(b.umbrellaPath); !os.IsNotExist(err) {
		t.Fatalf("repeated cancellation created or charged umbrella: %v", err)
	}
}

func TestAdmissionRechecksBothLedgersAcrossRollover(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	c := newClock()
	b := newBudget(t, c, "github.com", "octocat", nil)
	fill := func(at time.Time) ledger {
		l := ledger{Minute: make([]time.Time, GitHubContentWritesPerMinute), Hour: make([]time.Time, GitHubContentWritesPerMinute)}
		for i := range l.Minute {
			l.Minute[i], l.Hour[i] = at, at
		}
		return l
	}
	if err := writeDurable(b.umbrellaPath, fill(c.now().Add(-59*time.Second))); err != nil {
		t.Fatal(err)
	}
	if err := writeDurable(b.identityPath, fill(c.now().Add(-30*time.Second))); err != nil {
		t.Fatal(err)
	}
	started := c.now()
	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if got := c.now().Sub(started); got != 30*time.Second {
		t.Fatalf("admission waited %s, want both ledgers' rollover at 30s", got)
	}
	umbrella := readLedger(t, b.umbrellaPath)
	identity := readLedger(t, b.identityPath)
	if len(umbrella.Minute) != 1 || len(identity.Minute) != 1 {
		t.Fatalf("post-rollover minute entries = umbrella %d / identity %d, want 1/1", len(umbrella.Minute), len(identity.Minute))
	}
	if !umbrella.Minute[0].Equal(identity.Minute[0]) || !umbrella.Minute[0].Equal(c.now()) {
		t.Fatalf("pair committed against different rollover instants: %v / %v", umbrella.Minute, identity.Minute)
	}
}

// --- A-6: the burst drill --------------------------------------------------

func TestBurstDrillNineConcurrentWriteSets(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	c := newClock()

	var mu sync.Mutex
	var deferrals []Event
	b := newBudget(t, c, "github.com", "octocat", func(e Event) {
		if e.Granted {
			return
		}
		mu.Lock()
		deferrals = append(deferrals, e)
		mu.Unlock()
	})

	const tasks, perTask = 9, 10 // 90 > the 80/minute cap: the burst must queue
	var wg sync.WaitGroup
	errs := make(chan error, tasks*perTask)
	for task := 0; task < tasks; task++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perTask; i++ {
				if err := b.Acquire(context.Background(), 1); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("a burst write was dropped: %v", err)
	}

	if got := len(readLedger(t, b.umbrellaPath).Hour); got != tasks*perTask {
		t.Errorf("umbrella recorded %d writes, want %d — the queue did not drain", got, tasks*perTask)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(deferrals) == 0 {
		t.Fatal("a burst past the minute cap produced no deferral events")
	}
	for i, e := range deferrals {
		if e.Deferred <= 0 || e.ResetAt.IsZero() || e.Ledger == "" {
			t.Errorf("burst deferral %d is unobservable: ledger=%q deferred=%s reset_at=%s",
				i, e.Ledger, e.Deferred, e.ResetAt)
		}
	}
}
