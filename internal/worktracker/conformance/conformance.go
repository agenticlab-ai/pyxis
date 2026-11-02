// Package conformance is the WorkTracker port's conformance kit — the
// executable contract every adapter must pass (docs/tech-design/
// local-work-tracker.md §3.7, AP-3). It is a table of behaviors run against an
// adapter supplied only through the port interface and a medium-neutral fault
// seam; it imports no adapter symbol (A-1, mirroring prd:local-work-tracker/
// NR-1), so the same rows prove the localfs adapter today and any future
// medium unchanged.
//
// An adapter participates by providing a Harness. The first task contributes
// the create/get round-trip, sequential-identifier distinctness, malformed-
// item quarantine, atomic-write survival, and machine-view schema-validity
// rows; later tasks append rows to Run without editing the existing ones (each
// behavior self-contained, the plan's extension-by-addition discipline).
package conformance

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// Harness constructs fresh subjects under test and injects the faults the
// adversarial rows need. An adapter's test implements it and wires itself in.
type Harness interface {
	// NewStore returns a fresh, empty tracker and a Fault handle over that same
	// store. Each call is isolated (its own root); the harness owns cleanup.
	NewStore(t *testing.T) (worktracker.WorkTracker, Fault)
}

// Fault is the medium-neutral failure seam: the kit names a fault, the adapter
// performs it against its own storage. This keeps crash and corruption rows in
// the shared table without the kit importing adapter internals.
type Fault interface {
	// Corrupt overwrites item id's stored representation with malformed bytes,
	// simulating a bad hand-edit, so the next read must fail closed.
	Corrupt(t *testing.T, id int, raw []byte)
	// CrashBeforeRename attempts to rewrite item id's title but suppresses the
	// final rename, simulating a crash mid-write. The prior version must remain
	// the readable one.
	CrashBeforeRename(t *testing.T, id int, newTitle string)
	// CrashAt rewrites item id's title but simulates a crash at cut point cut in
	// the adapter's atomic-write sequence. It returns whether the write committed
	// — false for a pre-commit cut point (the item stays wholly old), true for a
	// post-commit one (the item is wholly new). The item must remain whole and
	// lose no prior log entry either way (DINV-2). The T9 crash-injection suite.
	CrashAt(t *testing.T, id int, newTitle string, cut CutPoint) (committed bool)
	// ParallelCreate runs procs concurrent creators — real OS processes for a
	// filesystem medium, proving the exclusive-create id allocation holds across
	// process boundaries where an in-memory counter would not (DINV-3, UB-5) —
	// each creating perProc items against one shared store, and returns every
	// allocated identifier. The T9 parallel-create stress.
	ParallelCreate(t *testing.T, procs, perProc int) []int
}

// CutPoint names a cut point in an adapter's atomic-write sequence, so the
// crash-injection suite can enumerate them medium-neutrally; the adapter maps
// each to its own write primitive (localfs: stage → fsync → rename → dir-sync).
// A crash at any cut point before the atomic commit leaves the item wholly old;
// one after it leaves the item wholly new — never torn (DINV-2).
type CutPoint int

const (
	CutBeforeWrite  CutPoint = iota // before any bytes are staged → old
	CutAfterStage                   // staged, before it is made durable → old
	CutBeforeCommit                 // durable, immediately before the atomic commit → old
	CutAfterCommit                  // immediately after the atomic commit → new
)

// AllCutPoints is the ordered set the crash suite injects at.
var AllCutPoints = []CutPoint{CutBeforeWrite, CutAfterStage, CutBeforeCommit, CutAfterCommit}

// String names a cut point for a subtest label.
func (c CutPoint) String() string {
	switch c {
	case CutBeforeWrite:
		return "BeforeWrite"
	case CutAfterStage:
		return "AfterStage"
	case CutBeforeCommit:
		return "BeforeCommit"
	case CutAfterCommit:
		return "AfterCommit"
	default:
		return "unknown"
	}
}

// Run executes every conformance row against h. Later tasks append rows here
// without editing the existing ones (extension by addition).
func Run(t *testing.T, h Harness) {
	t.Run("CreateGetRoundTrip", func(t *testing.T) { createGetRoundTrip(t, h) })
	t.Run("SequentialIDsDistinct", func(t *testing.T) { sequentialIDsDistinct(t, h) })
	t.Run("MalformedItemQuarantined", func(t *testing.T) { malformedItemQuarantined(t, h) })
	t.Run("AtomicWriteSurvival", func(t *testing.T) { atomicWriteSurvival(t, h) })
	t.Run("MachineViewSchemaValid", func(t *testing.T) { machineViewSchemaValid(t, h) })
	// T2 — lifecycle: edit and idempotent transitions (lifecycle.go).
	t.Run("EditTouchesOnlyNamedFields", func(t *testing.T) { editTouchesOnlyNamedFields(t, h) })
	t.Run("IdempotentTransition", func(t *testing.T) { idempotentTransition(t, h) })
	t.Run("OutOfSetCloseReasonRefused", func(t *testing.T) { outOfSetCloseReasonRefused(t, h) })
	// T3 — the append log: comments and latest-wins decision-records (log.go).
	t.Run("InterleavedAppendsLoseNothing", func(t *testing.T) { interleavedAppendsLoseNothing(t, h) })
	t.Run("LatestWinsAndIdempotentRepeat", func(t *testing.T) { latestWinsAndIdempotentRepeat(t, h) })
	t.Run("OutOfSetNamespaceRefused", func(t *testing.T) { outOfSetNamespaceRefused(t, h) })
	t.Run("EmptyCommentRefused", func(t *testing.T) { emptyCommentRefused(t, h) })
	// T4 — signals: the governed vocabulary (signal.go).
	t.Run("SeedVocabularyRoundTrip", func(t *testing.T) { seedVocabularyRoundTrip(t, h) })
	t.Run("SignalMutationIdempotent", func(t *testing.T) { signalMutationIdempotent(t, h) })
	t.Run("UndefinedSignalRefused", func(t *testing.T) { undefinedSignalRefused(t, h) })
	t.Run("EngineSignalRedefinitionRefused", func(t *testing.T) { engineSignalRedefinitionRefused(t, h) })
	t.Run("ClassificationExtension", func(t *testing.T) { classificationExtension(t, h) })
	// T5 — hierarchy: parent/child and epic rollup (hierarchy.go).
	t.Run("ParentBidirectional", func(t *testing.T) { parentBidirectional(t, h) })
	t.Run("EpicRollupOpenChildren", func(t *testing.T) { epicRollupOpenChildren(t, h) })
	t.Run("ParentMissingRefused", func(t *testing.T) { parentMissingRefused(t, h) })
	// plan-002 T2 — the reverse-parent query: direct children across all states,
	// narrowable by the list filters, O(k) over the index (hierarchy.go).
	t.Run("ChildrenAllStatesAndStateFilter", func(t *testing.T) { childrenAllStatesAndStateFilter(t, h) })
	// plan-002 T4 — the epic rollup grouped by open and closed (hierarchy.go).
	t.Run("EpicRollupGroupsOpenClosed", func(t *testing.T) { epicRollupGroupsOpenClosed(t, h) })
	// T6 — dependencies: blocked_by edges with cycle refusal (dependency.go).
	t.Run("DependencyBidirectional", func(t *testing.T) { dependencyBidirectional(t, h) })
	t.Run("DependencyCycleRefused", func(t *testing.T) { dependencyCycleRefused(t, h) })
	t.Run("DependencyDanglingRefused", func(t *testing.T) { dependencyDanglingRefused(t, h) })
	t.Run("DependencyMalformedRefused", func(t *testing.T) { dependencyMalformedRefused(t, h) })
	// T7 — queries: list, filter, and exact search (query.go).
	t.Run("ListByStateAndSignal", func(t *testing.T) { listByStateAndSignal(t, h) })
	t.Run("ListByRoleSelectsAuthorization", func(t *testing.T) { listByRoleSelectsAuthorization(t, h) })
	t.Run("SearchExactNoFalsePositives", func(t *testing.T) { searchExactNoFalsePositives(t, h) })
	t.Run("QueryMachineOutputSchemaValid", func(t *testing.T) { queryMachineOutputSchemaValid(t, h) })
	// T8 — the shared predicates: ready-set and triage-set (predicate.go).
	t.Run("ReadySetOverCorpus", func(t *testing.T) { readySetOverCorpus(t, h) })
	t.Run("TriageSetOverCorpus", func(t *testing.T) { triageSetOverCorpus(t, h) })
	t.Run("MembershipReflectsWrite", func(t *testing.T) { membershipReflectsWrite(t, h) })
	// T9 — the fault suites: strong reads, crash safety, and contention (fault.go).
	t.Run("ReadYourWritesUnderInterleave", func(t *testing.T) { readYourWritesUnderInterleave(t, h) })
	t.Run("CrashAtEachCutPointAtomic", func(t *testing.T) { crashAtEachCutPointAtomic(t, h) })
	t.Run("ParallelCreateNoCollisionNoReuse", func(t *testing.T) { parallelCreateNoCollisionNoReuse(t, h) })
	t.Run("ConcurrentAppendStressLosesNothing", func(t *testing.T) { concurrentAppendStressLosesNothing(t, h) })
	// T10 — prove the medium: the defect-quadruple sweep over every file-level
	// failure mode (proof.go). Hermetic-lifecycle, capacity, and survival proofs
	// are medium-specific (network syscalls, wall-clock, the .pyxis/ footprint)
	// and live with the localfs adapter and the Carina CLI, not this kit.
	t.Run("DefectQuadrupleSweep", func(t *testing.T) { defectQuadrupleSweep(t, h) })
}

// createGetRoundTrip: create then get returns every header field and the body
// unchanged (prd:local-work-tracker/FR-1, FR-2, FR-15).
func createGetRoundTrip(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	spec := model.CreateSpec{Type: model.TypeEpic, Title: "Ship the substrate", Body: "The first vertical slice.\n\nWith prose.", Priority: model.PriorityHigh}
	created, defects := store.Create(ctx, spec)
	mustClean(t, "create", defects)

	got, defects := store.Get(ctx, created.ID)
	mustClean(t, "get", defects)

	if got.ID != created.ID || got.Type != spec.Type || got.State != model.StateOpen ||
		got.Title != spec.Title || got.Body != spec.Body || got.Priority != spec.Priority {
		t.Fatalf("round-trip mismatch: created=%+v got=%+v", created, got)
	}
	if !got.Created.Equal(created.Created) || !got.Updated.Equal(created.Updated) {
		t.Fatalf("timestamps did not round-trip: created=%v/%v got=%v/%v", created.Created, created.Updated, got.Created, got.Updated)
	}
}

// sequentialIDsDistinct: N sequential creates yield N distinct, never-reused
// ids, each resolvable by get (prd:local-work-tracker/UB-5, DINV-3).
func sequentialIDsDistinct(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	const n = 200
	seen := make(map[int]bool, n)
	ids := make([]int, 0, n)
	for i := 0; i < n; i++ {
		w, defects := store.Create(ctx, model.CreateSpec{Title: "item"})
		mustClean(t, "create", defects)
		if seen[w.ID] {
			t.Fatalf("id %d reused at create %d", w.ID, i)
		}
		seen[w.ID] = true
		ids = append(ids, w.ID)
	}
	if len(seen) != n {
		t.Fatalf("expected %d distinct ids, got %d", n, len(seen))
	}
	for _, id := range ids {
		if _, defects := store.Get(ctx, id); len(defects) > 0 {
			t.Fatalf("id %d not resolvable: %v", id, defects)
		}
	}
}

// malformedItemQuarantined: a corrupted item yields a defect naming file,
// operation, reason, and remedy, while every other item stays readable
// (prd:local-work-tracker/UB-1, §3.13/R5).
func malformedItemQuarantined(t *testing.T, h Harness) {
	store, fault := h.NewStore(t)
	ctx := context.Background()
	bad, defects := store.Create(ctx, model.CreateSpec{Title: "will be corrupted"})
	mustClean(t, "create", defects)
	good, defects := store.Create(ctx, model.CreateSpec{Title: "stays readable"})
	mustClean(t, "create", defects)

	fault.Corrupt(t, bad.ID, []byte("this is not a work item\n"))

	_, defects = store.Get(ctx, bad.ID)
	if len(defects) == 0 {
		t.Fatalf("corrupted item #%d parsed clean", bad.ID)
	}
	for _, d := range defects {
		if d.File == "" || d.Op == "" || d.Reason == "" || d.Remedy == "" {
			t.Fatalf("defect does not name the full quadruple: %+v", d)
		}
	}
	if _, defects := store.Get(ctx, good.ID); len(defects) > 0 {
		t.Fatalf("sibling item #%d unreadable after corruption of #%d: %v", good.ID, bad.ID, defects)
	}
}

// atomicWriteSurvival: a simulated crash before rename leaves the prior version
// whole (prd:local-work-tracker/UB-4, DINV-2).
func atomicWriteSurvival(t *testing.T, h Harness) {
	store, fault := h.NewStore(t)
	ctx := context.Background()
	orig, defects := store.Create(ctx, model.CreateSpec{Title: "original"})
	mustClean(t, "create", defects)

	fault.CrashBeforeRename(t, orig.ID, "replacement that must not land")

	got, defects := store.Get(ctx, orig.ID)
	mustClean(t, "get-after-crash", defects)
	if got.Title != "original" {
		t.Fatalf("crash before rename mutated the item: title=%q, want %q", got.Title, "original")
	}
}

// requiredViewKeys is the closed set of keys the machine (JSON) projection must
// carry — the golden schema of prd:local-work-tracker/FR-15. A missing key or a
// wrong schema tag fails the row.
var requiredViewKeys = []string{
	"schema", "id", "type", "state", "title", "signals", "assignees",
	"parent", "blocked_by", "created", "updated", "close_reason", "priority", "body", "log",
}

// machineViewSchemaValid: the machine projection validates against its golden
// schema — every required key present and the schema tag pinned.
func machineViewSchemaValid(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	w, defects := store.Create(ctx, model.CreateSpec{Title: "machine view"})
	mustClean(t, "create", defects)
	got, defects := store.Get(ctx, w.ID)
	mustClean(t, "get", defects)

	raw, err := json.Marshal(got.View())
	if err != nil {
		t.Fatalf("view did not marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("view is not a JSON object: %v", err)
	}
	for _, k := range requiredViewKeys {
		if _, ok := m[k]; !ok {
			t.Fatalf("machine view missing required key %q: %s", k, raw)
		}
	}
	var gotSchema string
	if err := json.Unmarshal(m["schema"], &gotSchema); err != nil || gotSchema != model.SchemaVersion {
		t.Fatalf("machine view schema tag = %q, want %q", gotSchema, model.SchemaVersion)
	}
}

// seedVocabularyRoundTrip: a fresh store surfaces every canonical seed signal
// through its vocabulary read, each reporting a role in the closed set
// (prd:local-work-tracker/FR-6, §3.14 — acceptance criterion 1).
func seedVocabularyRoundTrip(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	sigs, defects := store.Signals(context.Background())
	mustClean(t, "signals", defects)

	byName := make(map[string]model.Signal, len(sigs))
	for _, s := range sigs {
		if !model.ValidRole(s.Role) {
			t.Fatalf("signal %q reports out-of-set role %q", s.Name, s.Role)
		}
		byName[s.Name] = s
	}
	for _, want := range model.SeedSignals() {
		got, ok := byName[want.Name]
		if !ok {
			t.Fatalf("seed signal %q missing from the vocabulary", want.Name)
		}
		if got.Role != want.Role {
			t.Fatalf("seed signal %q role = %q, want %q", want.Name, got.Role, want.Role)
		}
	}
}

// signalMutationIdempotent: adding a present signal and removing an absent one
// both succeed as recorded no-ops, and a real add/remove takes effect
// (prd:local-work-tracker/FR-6, AP-22 — acceptance criterion 2).
func signalMutationIdempotent(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	w, defects := store.Create(ctx, model.CreateSpec{Title: "carries signals"})
	mustClean(t, "create", defects)

	// A real add lands the signal.
	got, defects := store.AddSignal(ctx, w.ID, "agent")
	mustClean(t, "add", defects)
	if countSignal(got.Signals, "agent") != 1 {
		t.Fatalf("add did not land the signal: %v", got.Signals)
	}
	// Adding the present signal again is a recorded no-op: success, still one.
	got, defects = store.AddSignal(ctx, w.ID, "agent")
	mustClean(t, "add-again (present is a no-op)", defects)
	if countSignal(got.Signals, "agent") != 1 {
		t.Fatalf("re-adding a present signal duplicated it: %v", got.Signals)
	}
	// Removing an absent signal is a recorded no-op: success, no change.
	got, defects = store.RemoveSignal(ctx, w.ID, "hold")
	mustClean(t, "rm-absent (absent is a no-op)", defects)
	if countSignal(got.Signals, "agent") != 1 || countSignal(got.Signals, "hold") != 0 {
		t.Fatalf("removing an absent signal changed the set: %v", got.Signals)
	}
	// A real remove takes effect; removing again is a recorded no-op.
	got, defects = store.RemoveSignal(ctx, w.ID, "agent")
	mustClean(t, "rm", defects)
	if countSignal(got.Signals, "agent") != 0 {
		t.Fatalf("remove did not take effect: %v", got.Signals)
	}
	if _, defects = store.RemoveSignal(ctx, w.ID, "agent"); len(defects) > 0 {
		t.Fatalf("removing an already-absent signal reported defects: %v", defects)
	}
}

// undefinedSignalRefused: adding a signal not in the vocabulary is refused with
// a defect naming the offense and the full quadruple
// (prd:local-work-tracker/UB-2 — acceptance criterion 3).
func undefinedSignalRefused(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	w, defects := store.Create(ctx, model.CreateSpec{Title: "no undefined signals"})
	mustClean(t, "create", defects)

	_, defects = store.AddSignal(ctx, w.ID, "no-such-signal")
	mustNameOffense(t, "undefined signal add", defects, "no-such-signal")

	// The item did not gain the undefined signal.
	got, defects := store.Get(ctx, w.ID)
	mustClean(t, "get", defects)
	if countSignal(got.Signals, "no-such-signal") != 0 {
		t.Fatalf("a refused add still mutated the item: %v", got.Signals)
	}
}

// engineSignalRedefinitionRefused: redefining an engine-recognized seed signal
// is refused with a defect naming it (DINV-11 — acceptance criterion 3).
func engineSignalRedefinitionRefused(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	defects := store.DefineSignal(context.Background(), model.Signal{Name: "agent", Role: model.RoleClassification})
	mustNameOffense(t, "engine-recognized redefinition", defects, "agent")
}

// classificationExtension: an operand may define a new classification-axis
// signal, which then joins the vocabulary and is addable to an item (§3.14).
func classificationExtension(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	if defects := store.DefineSignal(ctx, model.Signal{Name: "kind:security", Role: model.RoleClassification, Description: "a security change"}); len(defects) > 0 {
		t.Fatalf("defining a classification signal failed: %v", defects)
	}
	sigs, defects := store.Signals(ctx)
	mustClean(t, "signals", defects)
	found := false
	for _, s := range sigs {
		if s.Name == "kind:security" && s.Role == model.RoleClassification {
			found = true
		}
	}
	if !found {
		t.Fatalf("defined classification signal absent from the vocabulary: %v", sigs)
	}
	w, defects := store.Create(ctx, model.CreateSpec{Title: "tagged"})
	mustClean(t, "create", defects)
	if _, defects := store.AddSignal(ctx, w.ID, "kind:security"); len(defects) > 0 {
		t.Fatalf("adding the newly-defined signal failed: %v", defects)
	}

	// A non-classification extension is refused.
	if defects := store.DefineSignal(ctx, model.Signal{Name: "my-lane", Role: model.RoleRouting}); len(defects) == 0 {
		t.Fatalf("defining a non-classification signal was allowed")
	}
}

func countSignal(xs []string, v string) int {
	n := 0
	for _, x := range xs {
		if x == v {
			n++
		}
	}
	return n
}

// mustNameOffense asserts defects are non-empty, every one names the full
// quadruple, and at least one reason names the offending token.
func mustNameOffense(t *testing.T, op string, defects []schema.Defect, offense string) {
	t.Helper()
	if len(defects) == 0 {
		t.Fatalf("%s was not refused", op)
	}
	named := false
	for _, d := range defects {
		if d.File == "" || d.Op == "" || d.Reason == "" || d.Remedy == "" {
			t.Fatalf("%s defect does not name the full quadruple: %+v", op, d)
		}
		if strings.Contains(d.Reason, offense) {
			named = true
		}
	}
	if !named {
		t.Fatalf("%s defect does not name the offense %q: %v", op, offense, defects)
	}
}

func mustClean(t *testing.T, op string, defects []schema.Defect) {
	t.Helper()
	if len(defects) > 0 {
		t.Fatalf("%s reported defects: %v", op, defects)
	}
}

func mustCondition(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Fatalf(format, args...)
	}
}
