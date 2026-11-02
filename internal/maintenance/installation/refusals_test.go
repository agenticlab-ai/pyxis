package installation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRefusalCountersIncrementOnceAndResetOnlyConsecutive(t *testing.T) {
	store, err := open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first := time.Date(2026, 10, 28, 1, 0, 0, 0, time.UTC)
	invocation := strings.Repeat("a", 32)
	if err := store.RecordRefusal(t.Context(), invocation, "daily", CauseExecutionFailed, first); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordRefusal(t.Context(), invocation, "daily", CauseExecutionFailed, first.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordRefusal(t.Context(), strings.Repeat("b", 32), "daily", CauseExecutionFailed, first.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordRefusal(t.Context(), invocation, "daily", CauseAuditFailed, first); err != nil {
		t.Fatal(err)
	}
	state, err := store.LoadRefusals(t.Context())
	if err != nil || len(state.Counters) != 2 || state.Counters[0].Lifetime != 2 || state.Counters[0].Consecutive != 2 || !state.Counters[0].FirstObserved.Equal(first) || !state.Counters[0].LastObserved.Equal(first.Add(2*time.Hour)) {
		t.Fatalf("recorded counters = %+v, %v", state, err)
	}
	if err := store.ResetRefusals(t.Context(), "daily"); err != nil {
		t.Fatal(err)
	}
	state, err = store.LoadRefusals(t.Context())
	if err != nil || state.Counters[0].Lifetime != 2 || state.Counters[0].Consecutive != 0 || state.Counters[1].Lifetime != 1 || state.Counters[1].Consecutive != 0 {
		t.Fatalf("reset counters = %+v, %v", state, err)
	}
}

func TestRefusalReadDoesNotCreateOrRepairAndWriteFaultIsReturned(t *testing.T) {
	stateRoot := t.TempDir()
	store, err := open(t.TempDir(), stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadRefusals(t.Context()); !errors.Is(err, ErrRefusalsAbsent) {
		t.Fatalf("absent read = %v", err)
	}
	if entries, err := os.ReadDir(stateRoot); err != nil || len(entries) != 0 {
		t.Fatalf("read created state: %v %v", entries, err)
	}
	want := errors.New("injected refusal rename failure")
	store.rename = func(*os.Root, string, string) error { return want }
	err = store.RecordRefusal(t.Context(), strings.Repeat("c", 32), "daily", CauseAdmissionRefused, time.Now().UTC())
	if !errors.Is(err, want) {
		t.Fatalf("write fault = %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(store.ManifestPath()), refusalName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed publication exposed state: %v", err)
	}
}

func TestRefusalCountersRejectInvalidInputAndHonorReadCancellation(t *testing.T) {
	store, err := open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := store.LoadRefusals(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read = %v", err)
	}
	if err := store.RecordRefusal(t.Context(), "bad", "daily", CauseExecutionFailed, time.Now().UTC()); err == nil {
		t.Fatal("invalid invocation was accepted")
	}
	if err := store.ResetRefusals(t.Context(), "daily"); err != nil {
		t.Fatalf("absent reset = %v", err)
	}
}

func TestRefusalCodecRejectsMalformedStateAndUnsafeFileMode(t *testing.T) {
	store, err := open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 28, 3, 0, 0, 0, time.UTC)
	valid := RefusalState{SchemaVersion: RefusalSchemaVersion, RepositoryID: store.repositoryID, Counters: []RefusalCounter{{
		JobID: "daily", Cause: CauseExecutionFailed, Lifetime: 1, Consecutive: 1,
		FirstObserved: at, LastObserved: at, LastInvocation: strings.Repeat("e", 32),
	}}}
	canonical, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	invalidCounter := valid
	invalidCounter.Counters = append([]RefusalCounter(nil), valid.Counters...)
	invalidCounter.Counters[0].Lifetime = 0
	invalidCounterBytes, _ := json.Marshal(invalidCounter)
	for _, data := range [][]byte{
		[]byte("{"),
		[]byte("{}\n"),
		append(invalidCounterBytes, '\n'),
		append([]byte(" "), append(canonical, '\n')...),
	} {
		if _, err := decodeRefusals(data, store.repositoryID); err == nil {
			t.Fatalf("malformed refusal state accepted: %q", data)
		}
	}
	directory := filepath.Dir(store.ManifestPath())
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, refusalName)
	if err := os.WriteFile(path, append(canonical, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadRefusals(t.Context()); err == nil || !strings.Contains(err.Error(), "0600 regular file") {
		t.Fatalf("unsafe refusal mode = %v", err)
	}
}
