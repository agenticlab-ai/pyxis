package conformance

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/localfs"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

const conformanceFailureEnv = "PYXIS_CONFORMANCE_FAILURE_CASE"

type coverageHarness struct {
	configure    func(*coverageStore)
	faultFactory func(worktracker.WorkTracker) Fault
}

func (h coverageHarness) NewStore(t *testing.T) (worktracker.WorkTracker, Fault) {
	at := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	base := localfs.New(t.TempDir(), func() time.Time {
		got := at
		at = at.Add(time.Second)
		return got
	})
	store := &coverageStore{WorkTracker: base}
	if h.configure != nil {
		h.configure(store)
	}
	fault := Fault(coverageFault{store: store})
	if h.faultFactory != nil {
		fault = h.faultFactory(store)
	}
	return store, fault
}

type coverageStore struct {
	worktracker.WorkTracker
	get             func(context.Context, int) (model.WorkItem, []schema.Defect)
	record          func(context.Context, int, model.DecisionRecord) (model.WorkItem, []schema.Defect)
	latest          func(context.Context, int, model.Namespace) (model.DecisionRecord, bool, []schema.Defect)
	comment         func(context.Context, int, string, string) (model.WorkItem, []schema.Defect)
	addBlockedBy    func(context.Context, int, string) (model.WorkItem, []schema.Defect)
	removeBlockedBy func(context.Context, int, string) (model.WorkItem, []schema.Defect)
	setParent       func(context.Context, int, int) (model.WorkItem, []schema.Defect)
	children        func(context.Context, int, worktracker.ChildFilter, worktracker.ReadMode) ([]model.WorkItem, []schema.Defect)
	rollup          func(context.Context, int, worktracker.ReadMode) (worktracker.RollupView, []schema.Defect)
	addSignal       func(context.Context, int, string) (model.WorkItem, []schema.Defect)
	readySet        func(context.Context) ([]model.WorkItem, []schema.Defect)
	triageSet       func(context.Context) ([]model.WorkItem, []schema.Defect)
}

func (s *coverageStore) Get(ctx context.Context, id int) (model.WorkItem, []schema.Defect) {
	if s.get != nil {
		return s.get(ctx, id)
	}
	return s.WorkTracker.Get(ctx, id)
}

func (s *coverageStore) Record(ctx context.Context, id int, rec model.DecisionRecord) (model.WorkItem, []schema.Defect) {
	if s.record != nil {
		return s.record(ctx, id, rec)
	}
	return s.WorkTracker.Record(ctx, id, rec)
}

func (s *coverageStore) LatestRecord(ctx context.Context, id int, ns model.Namespace) (model.DecisionRecord, bool, []schema.Defect) {
	if s.latest != nil {
		return s.latest(ctx, id, ns)
	}
	return s.WorkTracker.LatestRecord(ctx, id, ns)
}

func (s *coverageStore) Comment(ctx context.Context, id int, author, body string) (model.WorkItem, []schema.Defect) {
	if s.comment != nil {
		return s.comment(ctx, id, author, body)
	}
	return s.WorkTracker.Comment(ctx, id, author, body)
}

func (s *coverageStore) AddBlockedBy(ctx context.Context, id int, ref string) (model.WorkItem, []schema.Defect) {
	if s.addBlockedBy != nil {
		return s.addBlockedBy(ctx, id, ref)
	}
	return s.WorkTracker.AddBlockedBy(ctx, id, ref)
}

func (s *coverageStore) RemoveBlockedBy(ctx context.Context, id int, ref string) (model.WorkItem, []schema.Defect) {
	if s.removeBlockedBy != nil {
		return s.removeBlockedBy(ctx, id, ref)
	}
	return s.WorkTracker.RemoveBlockedBy(ctx, id, ref)
}

func (s *coverageStore) SetParent(ctx context.Context, childID, parentID int) (model.WorkItem, []schema.Defect) {
	if s.setParent != nil {
		return s.setParent(ctx, childID, parentID)
	}
	return s.WorkTracker.SetParent(ctx, childID, parentID)
}

func (s *coverageStore) Children(ctx context.Context, parentID int, filter worktracker.ChildFilter, mode worktracker.ReadMode) ([]model.WorkItem, []schema.Defect) {
	if s.children != nil {
		return s.children(ctx, parentID, filter, mode)
	}
	return s.WorkTracker.Children(ctx, parentID, filter, mode)
}

func (s *coverageStore) Rollup(ctx context.Context, parentID int, mode worktracker.ReadMode) (worktracker.RollupView, []schema.Defect) {
	if s.rollup != nil {
		return s.rollup(ctx, parentID, mode)
	}
	return s.WorkTracker.Rollup(ctx, parentID, mode)
}

func (s *coverageStore) AddSignal(ctx context.Context, id int, name string) (model.WorkItem, []schema.Defect) {
	if s.addSignal != nil {
		return s.addSignal(ctx, id, name)
	}
	return s.WorkTracker.AddSignal(ctx, id, name)
}

func (s *coverageStore) ReadySet(ctx context.Context) ([]model.WorkItem, []schema.Defect) {
	if s.readySet != nil {
		return s.readySet(ctx)
	}
	return s.WorkTracker.ReadySet(ctx)
}

func (s *coverageStore) TriageSet(ctx context.Context) ([]model.WorkItem, []schema.Defect) {
	if s.triageSet != nil {
		return s.triageSet(ctx)
	}
	return s.WorkTracker.TriageSet(ctx)
}

type coverageFault struct {
	store     worktracker.WorkTracker
	committed bool
	mutate    bool
}

func (coverageFault) Corrupt(*testing.T, int, []byte)           {}
func (coverageFault) CrashBeforeRename(*testing.T, int, string) {}
func (f coverageFault) CrashAt(t *testing.T, id int, title string, _ CutPoint) bool {
	if f.mutate {
		if _, defects := f.store.Edit(t.Context(), id, model.EditPatch{Title: &title}); len(defects) > 0 {
			t.Fatalf("mutate crash fixture: %v", defects)
		}
	}
	return f.committed
}
func (coverageFault) ParallelCreate(*testing.T, int, int) []int { return nil }

func completeDefect(reason string) schema.Defect {
	return schema.Defect{File: "fixture", Op: "fixture", Reason: reason, Remedy: "repair fixture"}
}

func TestCoverage90ConformanceFailureAssertions(t *testing.T) {
	if child := os.Getenv(conformanceFailureEnv); child != "" {
		runConformanceFailureCase(t, child)
		return
	}

	cases := []string{
		"cutpoint-unknown", "must-clean", "must-condition", "offense-missing", "offense-incomplete", "offense-unnamed", "roundtrip-fields", "roundtrip-time",
		"dependency-cycle-clean", "dependency-cycle-incomplete", "dependency-cycle-unnamed", "dependency-remove",
		"fault-record", "fault-latest", "fault-committed-old", "fault-aborted-new",
		"parent-clean", "parent-incomplete", "parent-unnamed", "children-wrong", "rollup-open-count", "rollup-closed-count", "rollup-open-item",
		"lifecycle-unchanged", "log-comment", "log-charter", "log-namespace-clean",
		"predicate-ready", "predicate-triage", "predicate-signal", "proof-seed",
		"query-json", "query-schema", "query-count",
	}
	coverArg := currentCoverDirArg()
	for _, name := range cases {
		name := name
		t.Run(name, func(t *testing.T) {
			args := []string{"-test.run=^TestCoverage90ConformanceFailureAssertions$"}
			if coverArg != "" {
				args = append(args, coverArg)
			}
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), conformanceFailureEnv+"="+name)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("failure fixture %q unexpectedly passed:\n%s", name, out)
			}
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
				t.Fatalf("failure fixture %q exited unexpectedly: %v\n%s", name, err, out)
			}
		})
	}
}

func currentCoverDirArg() string {
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-test.gocoverdir=") {
			return arg
		}
	}
	return ""
}

func runConformanceFailureCase(t *testing.T, name string) {
	h := coverageHarness{}
	switch name {
	case "cutpoint-unknown":
		_ = CutPoint(99).String()
		t.Fatal("covered unknown cut point")
	case "must-clean":
		mustClean(t, "fixture", []schema.Defect{completeDefect("bad")})
	case "must-condition":
		mustCondition(t, false, "fixture condition")
	case "offense-missing":
		mustNameOffense(t, "fixture", nil, "bad")
	case "offense-incomplete":
		mustNameOffense(t, "fixture", []schema.Defect{{}}, "bad")
	case "offense-unnamed":
		mustNameOffense(t, "fixture", []schema.Defect{completeDefect("different")}, "bad")
	case "roundtrip-fields":
		h.configure = func(s *coverageStore) {
			s.get = func(ctx context.Context, id int) (model.WorkItem, []schema.Defect) {
				got, defects := s.WorkTracker.Get(ctx, id)
				got.Title = "wrong"
				return got, defects
			}
		}
		createGetRoundTrip(t, h)
	case "roundtrip-time":
		h.configure = func(s *coverageStore) {
			s.get = func(ctx context.Context, id int) (model.WorkItem, []schema.Defect) {
				got, defects := s.WorkTracker.Get(ctx, id)
				got.Updated = got.Updated.Add(time.Hour)
				return got, defects
			}
		}
		createGetRoundTrip(t, h)
	case "dependency-cycle-clean", "dependency-cycle-incomplete", "dependency-cycle-unnamed":
		h.configure = func(s *coverageStore) {
			calls := 0
			s.addBlockedBy = func(ctx context.Context, id int, ref string) (model.WorkItem, []schema.Defect) {
				calls++
				if calls < 3 {
					return s.WorkTracker.AddBlockedBy(ctx, id, ref)
				}
				got, _ := s.WorkTracker.Get(ctx, id)
				switch name {
				case "dependency-cycle-clean":
					return got, nil
				case "dependency-cycle-incomplete":
					return got, []schema.Defect{{}}
				default:
					return got, []schema.Defect{completeDefect("cycle without identifiers")}
				}
			}
		}
		dependencyCycleRefused(t, h)
	case "dependency-remove":
		h.configure = func(s *coverageStore) {
			s.removeBlockedBy = func(context.Context, int, string) (model.WorkItem, []schema.Defect) {
				return model.WorkItem{}, []schema.Defect{completeDefect("remove refused")}
			}
		}
		dependencyBidirectional(t, h)
	case "fault-record":
		h.configure = func(s *coverageStore) {
			s.record = func(context.Context, int, model.DecisionRecord) (model.WorkItem, []schema.Defect) {
				return model.WorkItem{}, []schema.Defect{completeDefect("record refused")}
			}
		}
		readYourWritesUnderInterleave(t, h)
	case "fault-latest":
		h.configure = func(s *coverageStore) {
			s.latest = func(context.Context, int, model.Namespace) (model.DecisionRecord, bool, []schema.Defect) {
				return model.DecisionRecord{Payload: "stale"}, true, nil
			}
		}
		readYourWritesUnderInterleave(t, h)
	case "fault-committed-old":
		h.faultFactory = func(store worktracker.WorkTracker) Fault { return coverageFault{store: store, committed: true} }
		crashAtEachCutPointAtomic(t, h)
	case "fault-aborted-new":
		h.faultFactory = func(store worktracker.WorkTracker) Fault { return coverageFault{store: store, mutate: true} }
		crashAtEachCutPointAtomic(t, h)
	case "parent-clean", "parent-incomplete", "parent-unnamed":
		h.configure = func(s *coverageStore) {
			s.setParent = func(ctx context.Context, childID, _ int) (model.WorkItem, []schema.Defect) {
				got, _ := s.WorkTracker.Get(ctx, childID)
				switch name {
				case "parent-clean":
					return got, nil
				case "parent-incomplete":
					return got, []schema.Defect{{}}
				default:
					return got, []schema.Defect{completeDefect("missing parent")}
				}
			}
		}
		parentMissingRefused(t, h)
	case "children-wrong":
		h.configure = func(s *coverageStore) {
			s.children = func(context.Context, int, worktracker.ChildFilter, worktracker.ReadMode) ([]model.WorkItem, []schema.Defect) {
				return nil, nil
			}
		}
		childrenAllStatesAndStateFilter(t, h)
	case "rollup-open-count", "rollup-closed-count", "rollup-open-item":
		h.configure = func(s *coverageStore) {
			s.rollup = func(ctx context.Context, id int, mode worktracker.ReadMode) (worktracker.RollupView, []schema.Defect) {
				view, defects := s.WorkTracker.Rollup(ctx, id, mode)
				switch name {
				case "rollup-open-count":
					view.Open = nil
				case "rollup-closed-count":
					view.Closed = nil
				case "rollup-open-item":
					view.Open[0].State = model.StateClosed
				}
				return view, defects
			}
		}
		epicRollupGroupsOpenClosed(t, h)
	case "lifecycle-unchanged":
		before := model.WorkItem{ID: 1, Title: "before"}
		after := before
		after.ID = 2
		assertUnchangedExceptTitleUpdated(t, before, after)
	case "log-comment":
		h.configure = func(s *coverageStore) {
			s.comment = func(context.Context, int, string, string) (model.WorkItem, []schema.Defect) {
				return model.WorkItem{}, []schema.Defect{completeDefect("comment refused")}
			}
		}
		interleavedAppendsLoseNothing(t, h)
	case "log-charter":
		h.configure = func(s *coverageStore) {
			s.record = func(ctx context.Context, id int, rec model.DecisionRecord) (model.WorkItem, []schema.Defect) {
				if rec.Namespace == model.NamespaceCharter {
					return model.WorkItem{}, []schema.Defect{completeDefect("charter refused")}
				}
				return s.WorkTracker.Record(ctx, id, rec)
			}
		}
		latestWinsAndIdempotentRepeat(t, h)
	case "log-namespace-clean":
		h.configure = func(s *coverageStore) {
			s.record = func(ctx context.Context, id int, rec model.DecisionRecord) (model.WorkItem, []schema.Defect) {
				if rec.Namespace == model.Namespace("bogus-ns") {
					got, _ := s.WorkTracker.Get(ctx, id)
					return got, nil
				}
				return s.WorkTracker.Record(ctx, id, rec)
			}
		}
		outOfSetNamespaceRefused(t, h)
	case "predicate-ready":
		h.configure = func(s *coverageStore) {
			s.readySet = func(context.Context) ([]model.WorkItem, []schema.Defect) { return nil, nil }
		}
		readySetOverCorpus(t, h)
	case "predicate-triage":
		h.configure = func(s *coverageStore) {
			s.triageSet = func(context.Context) ([]model.WorkItem, []schema.Defect) { return nil, nil }
		}
		triageSetOverCorpus(t, h)
	case "predicate-signal":
		h.configure = func(s *coverageStore) {
			s.addSignal = func(context.Context, int, string) (model.WorkItem, []schema.Defect) {
				return model.WorkItem{}, []schema.Defect{completeDefect("signal refused")}
			}
		}
		membershipReflectsWrite(t, h)
	case "proof-seed":
		h.configure = func(s *coverageStore) {
			s.addBlockedBy = func(context.Context, int, string) (model.WorkItem, []schema.Defect) {
				return model.WorkItem{}, []schema.Defect{completeDefect("seed refused")}
			}
		}
		defectQuadrupleSweep(t, h)
	case "query-json":
		assertResultShape(t, "fixture", []byte("{"), "want", 0)
	case "query-schema":
		assertResultShape(t, "fixture", []byte(`{"schema":"wrong","count":0,"items":[]}`), "want", 0)
	case "query-count":
		assertResultShape(t, "fixture", []byte(`{"schema":"want","count":1,"items":[]}`), "want", 1)
	default:
		t.Fatalf("unknown failure fixture %q", name)
	}
}

func TestCoverage90ConformancePureHelpers(t *testing.T) {
	if sameStrings([]string{"a"}, nil) {
		t.Fatal("different lengths matched")
	}
	if sameStrings([]string{"a"}, []string{"b"}) {
		t.Fatal("different strings matched")
	}
	a := model.WorkItem{Log: []model.LogEntry{{Kind: model.LogComment, Author: "a"}}}
	b := model.WorkItem{Log: []model.LogEntry{{Kind: model.LogComment, Author: "b"}}}
	if sameItem(a, b) {
		t.Fatal("different log entries matched")
	}
	if got := countAllRecords(model.WorkItem{Log: []model.LogEntry{{Kind: model.LogDecisionRecord}}}); got != 1 {
		t.Fatalf("decision record count = %d", got)
	}
}
