package localfs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/conformance"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// harness wires the localfs adapter into the port conformance kit. It is a
// white-box implementation (package localfs) so the fault seam can reach the
// atomic-write primitive directly — the kit itself imports no localfs symbol
// (conformance A-1); only this test does.
type harness struct{}

func (harness) NewStore(t *testing.T) (worktracker.WorkTracker, conformance.Fault) {
	// A thread-safe advancing clock: the lifecycle rows need the updated stamp to
	// move across a mutation, and the T3 interleaved-append row calls the clock
	// from concurrent goroutines. flock serializes those calls in wall-clock time,
	// but Go's race detector cannot see the flock happens-before edge, so the
	// clock guards its own state to keep `go test -race` clean.
	s := New(t.TempDir(), safeAdvancingClock())
	return s, faultSeam{s}
}

// safeAdvancingClock is advancingClock made goroutine-safe with a mutex — a
// deterministic clock stepping one second per call, callable from concurrent
// writers (the fault/contention rows) without a data race.
func safeAdvancingClock() func() time.Time {
	var mu sync.Mutex
	at := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		t := at
		at = at.Add(time.Second)
		return t
	}
}

type faultSeam struct{ s *Store }

func (f faultSeam) Corrupt(t *testing.T, id int, raw []byte) {
	if err := os.WriteFile(itemPath(f.s.itemsDir(), id), raw, 0o644); err != nil {
		t.Fatalf("corrupt #%d: %v", id, err)
	}
}

func (f faultSeam) CrashBeforeRename(t *testing.T, id int, newTitle string) {
	// The T1 atomic-write row, expressed on the T9 cut-point seam: a crash right
	// before the rename must abort, leaving the prior file readable.
	if f.CrashAt(t, id, newTitle, conformance.CutBeforeCommit) {
		t.Fatalf("crash before rename still committed the write for #%d", id)
	}
}

// cutPhase maps a medium-neutral cut point to localfs's atomic-write phase.
func cutPhase(cut conformance.CutPoint) writePhase {
	switch cut {
	case conformance.CutBeforeWrite:
		return phaseBeforeTemp
	case conformance.CutAfterStage:
		return phaseAfterTemp
	case conformance.CutBeforeCommit:
		return phaseBeforeRename
	default: // CutAfterCommit
		return phaseAfterRename
	}
}

func (f faultSeam) CrashAt(t *testing.T, id int, newTitle string, cut conformance.CutPoint) bool {
	w, defects := f.s.Get(context.Background(), id)
	if len(defects) > 0 {
		t.Fatalf("crash setup: get #%d: %v", id, defects)
	}
	w.Title = newTitle
	target := cutPhase(cut)
	// The injector fires only at the requested phase, aborting the write there —
	// the crash. rename() is the commit: an abort before it leaves the prior file
	// whole (wholly old); an abort after it leaves the renamed file (wholly new).
	err := f.s.atomicReplace(itemPath(f.s.itemsDir(), id), Render(w), func(p writePhase) error {
		if p == target {
			return errors.New("simulated crash")
		}
		return nil
	})
	// The injected error must surface at every cut point — it proves the seam
	// actually fired rather than the write completing untouched (which would let
	// a broken injector pass the crash suite silently).
	if err == nil {
		t.Fatalf("crash at %v did not fire the injector for #%d", cut, id)
	}
	// The rename is the commit: after it the item is wholly new (committed),
	// before it wholly old (aborted).
	return cut == conformance.CutAfterCommit
}

// Env keys handing the re-exec'd child its instructions. The parallel-create
// stress spawns real OS processes so the exclusive-create id allocation is
// proven across process boundaries (DINV-3, UB-5) — a goroutine pool would share
// this process and miss the cross-process case an in-memory counter would fail.
const (
	parallelEnvRoot  = "WT_PARALLEL_CREATE_ROOT"
	parallelEnvCount = "WT_PARALLEL_CREATE_COUNT"
)

func (f faultSeam) ParallelCreate(t *testing.T, procs, perProc int) []int {
	type result struct {
		ids []int
		err error
	}
	results := make([]result, procs)
	var wg sync.WaitGroup
	for p := 0; p < procs; p++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			// Re-exec this test binary in child mode: run only the child
			// entrypoint, pointed at the shared store root.
			cmd := exec.Command(os.Args[0], "-test.run=^TestParallelCreateChild$")
			cmd.Env = append(os.Environ(),
				parallelEnvRoot+"="+f.s.root,
				parallelEnvCount+"="+strconv.Itoa(perProc),
			)
			out, err := cmd.Output()
			if err != nil {
				results[p] = result{err: fmt.Errorf("child %d: %v (output: %s)", p, err, out)}
				return
			}
			ids, err := parseChildIDs(out)
			results[p] = result{ids: ids, err: err}
		}(p)
	}
	wg.Wait()

	var all []int
	for _, r := range results {
		if r.err != nil {
			t.Fatalf("parallel-create child failed: %v", r.err)
		}
		all = append(all, r.ids...)
	}
	return all
}

// parseChildIDs pulls the created ids from a child's stdout, tolerating the test
// framework's own lines (PASS, ok …) by keeping only the id-tagged lines.
func parseChildIDs(out []byte) ([]int, error) {
	var ids []int
	for _, line := range strings.Split(string(out), "\n") {
		rest, ok := strings.CutPrefix(line, childIDPrefix)
		if !ok {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(rest))
		if err != nil {
			return nil, fmt.Errorf("unparseable child id line %q: %w", line, err)
		}
		ids = append(ids, n)
	}
	return ids, nil
}

const childIDPrefix = "CREATED-ID "

// TestParallelCreateChild is the re-exec child entrypoint for ParallelCreate:
// when the harness sets parallelEnvRoot, it creates perProc items against that
// shared root and prints their ids (each tagged so the parent can pick them out
// of the test binary's own stdout). It is skipped in a normal test run.
func TestParallelCreateChild(t *testing.T) {
	root := os.Getenv(parallelEnvRoot)
	if root == "" {
		t.Skip("child entrypoint; runs only when re-exec'd by ParallelCreate")
	}
	count, err := strconv.Atoi(os.Getenv(parallelEnvCount))
	if err != nil {
		t.Fatalf("bad %s: %v", parallelEnvCount, err)
	}
	s := New(root, safeAdvancingClock())
	ctx := context.Background()
	var b strings.Builder
	for i := 0; i < count; i++ {
		w, defects := s.Create(ctx, model.CreateSpec{Title: "parallel"})
		if len(defects) > 0 {
			t.Fatalf("child create %d: %v", i, defects)
		}
		fmt.Fprintf(&b, "%s%d\n", childIDPrefix, w.ID)
	}
	fmt.Print(b.String())
}

// TestConformance runs the whole port conformance kit against localfs.
func TestConformance(t *testing.T) {
	conformance.Run(t, harness{})
}
