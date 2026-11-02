package schema

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func validVerdictForLoad(runID string) *Verdict {
	return &Verdict{
		RunID:     runID,
		Workspace: "/workspace",
		Mode:      "full",
		State:     "green",
		StartTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC),
		Rungs: map[string]*Rung{
			"0": {Name: "0", Order: 0, State: "pass", Checks: map[string]*CheckResult{
				"format": {CheckID: "format", State: "pass"},
			}},
		},
		Findings: []Finding{},
		Bindings: Bindings{
			HeadSHA:         "head123",
			DiffBase:        "base456",
			ManifestHash:    "manifest-hash",
			BaselineHash:    "baseline-hash",
			CodegateVersion: "1.0.0",
			GitVersion:      "2.40.0",
		},
	}
}

func TestLoadVerdict_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "verdict.json")
	original := validVerdictForLoad("run-777")
	if err := original.WriteAtomicToFile(path); err != nil {
		t.Fatalf("WriteAtomicToFile: %v", err)
	}

	loaded, err := LoadVerdict(path)
	if err != nil {
		t.Fatalf("LoadVerdict: %v", err)
	}
	if loaded.RunID != original.RunID {
		t.Errorf("RunID = %q, want %q", loaded.RunID, original.RunID)
	}
	if loaded.Bindings != original.Bindings {
		t.Errorf("Bindings = %+v, want %+v", loaded.Bindings, original.Bindings)
	}
	if loaded.State != original.State {
		t.Errorf("State = %q, want %q", loaded.State, original.State)
	}
}

func TestLoadVerdict_MissingFileDegradesToError(t *testing.T) {
	_, err := LoadVerdict(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err == nil {
		t.Fatal("LoadVerdict of a missing file must return an error (the caller's cue to fall back to a full run)")
	}
}

func TestLoadVerdict_MalformedJSONDegradesToError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "verdict.json")
	if err := os.WriteFile(path, []byte("not json at all"), 0o644); err != nil {
		t.Fatalf("write malformed verdict: %v", err)
	}
	if _, err := LoadVerdict(path); err == nil {
		t.Fatal("LoadVerdict of malformed JSON must return an error")
	}
}

func TestLoadVerdict_IncompleteBindingsDegradesToError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "verdict.json")
	v := validVerdictForLoad("run-778")
	v.Bindings.GitVersion = "" // incomplete: DINV-1 bindings-present fails
	// Write directly, bypassing WriteAtomicToFile's own bindings-present
	// guard, so the file on disk actually carries the incomplete bindings
	// LoadVerdict must reject.
	data, err := v.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write verdict: %v", err)
	}

	if _, err := LoadVerdict(path); err == nil {
		t.Fatal("LoadVerdict of a verdict with incomplete bindings must return an error")
	}
}

func TestLoadVerdict_MissingRunIDDegradesToError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "verdict.json")
	v := validVerdictForLoad("")
	data, err := v.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write verdict: %v", err)
	}

	if _, err := LoadVerdict(path); err == nil {
		t.Fatal("LoadVerdict of a verdict with an empty run_id must return an error")
	}
}
