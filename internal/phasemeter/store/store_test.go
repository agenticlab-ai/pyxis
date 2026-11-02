package store_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/store"
)

func rec(runID, phaseID string) record.Record {
	return record.Record{
		SchemaVersion: record.SchemaVersion,
		RunID:         runID,
		FeatureID:     "pipeline-measurement",
		PhaseID:       phaseID,
		Harness:       "claude-code",
	}
}

// Acceptance: "Writing N records to a run then loading that run yields
// exactly N equal records."
func TestAppendThenLoadRun_RoundTrip(t *testing.T) {
	s := store.New(t.TempDir())
	const n = 25
	var want []record.Record
	for i := 0; i < n; i++ {
		r := rec("run-a", fmt.Sprintf("phase-%d", i))
		if err := s.Append(r); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
		want = append(want, r)
	}

	got, err := s.LoadRun("run-a")
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(got) != n {
		t.Fatalf("want %d records, got %d", n, len(got))
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("loaded records do not match written records:\nwant %+v\ngot  %+v", want, got)
	}
}

func TestLoadRun_NoFileYet(t *testing.T) {
	s := store.New(t.TempDir())
	got, err := s.LoadRun("never-written")
	if err != nil {
		t.Fatalf("expected no error for an unwritten run, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected a nil slice, got %v", got)
	}
}

func TestAppend_OneFilePerRun(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)
	if err := s.Append(rec("run-a", "p1")); err != nil {
		t.Fatal(err)
	}
	if err := s.Append(rec("run-b", "p1")); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"run-a.jsonl", "run-b.jsonl"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("expected %s to exist: %v", want, err)
		}
	}
}

// Acceptance: "Two runs writing concurrently produce two distinct files
// with no lost or interleaved lines."
func TestConcurrentRunsDoNotContend(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)
	const runs = 8
	const perRun = 40

	var wg sync.WaitGroup
	for r := 0; r < runs; r++ {
		runID := fmt.Sprintf("run-%d", r)
		wg.Add(1)
		go func(runID string) {
			defer wg.Done()
			for p := 0; p < perRun; p++ {
				if err := s.Append(rec(runID, fmt.Sprintf("phase-%d", p))); err != nil {
					t.Errorf("append to %s: %v", runID, err)
				}
			}
		}(runID)
	}
	wg.Wait()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, e := range entries {
		files = append(files, e.Name())
	}
	sort.Strings(files)
	var want []string
	for r := 0; r < runs; r++ {
		want = append(want, fmt.Sprintf("run-%d.jsonl", r))
	}
	sort.Strings(want)
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("want exactly one distinct file per run %v, got %v", want, files)
	}

	for r := 0; r < runs; r++ {
		runID := fmt.Sprintf("run-%d", r)
		got, err := s.LoadRun(runID)
		if err != nil {
			t.Fatalf("load %s: %v", runID, err)
		}
		if len(got) != perRun {
			t.Fatalf("run %s: want %d records, got %d (lost or corrupted lines)", runID, perRun, len(got))
		}
		seenPhase := map[string]bool{}
		for _, r := range got {
			if r.RunID != runID {
				t.Fatalf("run %s file contains a record for run %q — cross-run contamination", runID, r.RunID)
			}
			seenPhase[r.PhaseID] = true
		}
		if len(seenPhase) != perRun {
			t.Fatalf("run %s: want %d distinct phases, got %d distinct — interleaved/corrupted line suspected", runID, perRun, len(seenPhase))
		}
	}
}

// AP-23's "collision-free" append is not scoped to cross-run only: any
// concurrent callers appending to one run's file must not tear or lose a
// line either.
func TestConcurrentAppendsToSameRun(t *testing.T) {
	s := store.New(t.TempDir())
	const writers = 16
	const perWriter = 25

	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				phase := fmt.Sprintf("w%d-p%d", w, i)
				if err := s.Append(rec("shared-run", phase)); err != nil {
					t.Errorf("append: %v", err)
				}
			}
		}(w)
	}
	wg.Wait()

	got, err := s.LoadRun("shared-run")
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	want := writers * perWriter
	if len(got) != want {
		t.Fatalf("want %d records, got %d (lost or torn line)", want, len(got))
	}
	seen := map[string]bool{}
	for _, r := range got {
		if seen[r.PhaseID] {
			t.Fatalf("duplicate phase_id %q — a line was corrupted/duplicated in transit", r.PhaseID)
		}
		seen[r.PhaseID] = true
	}
}

// Acceptance: "A record missing an optional field loads with that field
// defaulted, never an error."
func TestLoadRun_MissingOptionalFieldsDefault(t *testing.T) {
	dir := t.TempDir()
	minimal := fmt.Sprintf(`{"schema_version":%q,"run_id":"legacy","feature_id":"f","phase_id":"p","harness":"h"}`, record.SchemaVersion) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "legacy.jsonl"), []byte(minimal), 0o644); err != nil {
		t.Fatal(err)
	}

	s := store.New(dir)
	got, err := s.LoadRun("legacy")
	if err != nil {
		t.Fatalf("a record missing optional fields must load without error, got %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 record, got %d", len(got))
	}
	r := got[0]
	if r.Spend != nil || r.PriceSnapshotVersion != "" || r.Unavailable != nil || r.Efficiency != (record.Efficiency{}) {
		t.Fatalf("expected all optional fields defaulted, got %+v", r)
	}
}

func TestAppend_RejectsIncompleteRecord(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)
	err := s.Append(record.Record{RunID: "run-a"}) // no schema_version, feature_id, phase_id, harness
	if err == nil {
		t.Fatal("expected an error for a record missing identity fields")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "run-a.jsonl")); !os.IsNotExist(statErr) {
		t.Fatalf("a rejected append must not create the run file, stat error: %v", statErr)
	}
}

func TestAppend_RejectsPathLikeRunID(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)
	for _, bad := range []string{"../escape", "nested/run", ".", ".."} {
		if err := s.Append(rec(bad, "p1")); err == nil {
			t.Errorf("run_id %q should be rejected", bad)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("no files should have been created, found %v", entries)
	}
}

func TestLoadRun_RejectsPathLikeRunID(t *testing.T) {
	s := store.New(t.TempDir())
	if _, err := s.LoadRun("../escape"); err == nil {
		t.Fatal("expected an error loading a path-like run_id")
	}
}
