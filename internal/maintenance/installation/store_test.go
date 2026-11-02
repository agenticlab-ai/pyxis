package installation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

func TestTransactionSerializesAndCancellationDoesNotEnter(t *testing.T) {
	store := newTestStore(t)
	locked, release, firstDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		firstDone <- store.WithTransaction(context.Background(), func(*Transaction) error {
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked
	ctx, cancel := context.WithCancel(context.Background())
	var entered atomic.Bool
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- store.WithTransaction(ctx, func(*Transaction) error {
			entered.Store(true)
			return nil
		})
	}()
	cancel()
	select {
	case err := <-secondDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled transaction = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled lock acquisition did not return")
	}
	if entered.Load() {
		t.Fatal("canceled acquisition entered its callback")
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first transaction: %v", err)
	}
	if err := store.WithTransaction(context.Background(), func(*Transaction) error { return nil }); err != nil {
		t.Fatalf("lock remained owned after cancellation/release: %v", err)
	}
	ownedContext, ownedCancel := context.WithCancel(context.Background())
	if err := store.WithTransaction(ownedContext, func(transaction *Transaction) error {
		ownedCancel()
		_, err := transaction.Load()
		return err
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("owned cancellation = %v", err)
	}
	if err := store.WithTransaction(context.Background(), func(*Transaction) error { return nil }); err != nil {
		t.Fatalf("canceled owner retained lock: %v", err)
	}
}

func TestTransactionsCannotInterleaveGenerations(t *testing.T) {
	store := newTestStore(t)
	first := readyManifest(t, store, strings.Repeat("1", 32))
	second := readyManifest(t, store, strings.Repeat("2", 32))
	locked, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 2)
	go func() {
		done <- store.WithTransaction(context.Background(), func(transaction *Transaction) error {
			if err := transaction.Publish(first); err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked
	var observed InstallationGeneration
	go func() {
		done <- store.WithTransaction(context.Background(), func(transaction *Transaction) error {
			manifest, err := transaction.Load()
			if err == nil {
				observed = manifest.Generation
				err = transaction.Publish(second)
			}
			return err
		})
	}()
	select {
	case err := <-done:
		t.Fatalf("competing transaction finished while the lock was held: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatalf("transaction: %v", err)
		}
	}
	if observed != first.Generation {
		t.Fatalf("second transaction observed generation %q, want %q", observed, first.Generation)
	}
	manifest, err := store.Load(context.Background())
	if err != nil || manifest.Generation != second.Generation {
		t.Fatalf("final manifest = %+v, %v", manifest, err)
	}
}

func TestAtomicPublicationPreservesAnIntactRecord(t *testing.T) {
	store := newTestStore(t)
	old := readyManifest(t, store, strings.Repeat("3", 32))
	publish(t, store, old)
	store.rename = func(*os.Root, string, string) error { return errors.New("injected rename failure") }
	newer := readyManifest(t, store, strings.Repeat("4", 32))
	err := store.WithTransaction(context.Background(), func(transaction *Transaction) error { return transaction.Publish(newer) })
	if err == nil || !strings.Contains(err.Error(), "injected rename failure") {
		t.Fatalf("publish failure = %v", err)
	}
	got, err := store.Load(context.Background())
	if err != nil || got.Generation != old.Generation {
		t.Fatalf("failed publication exposed %+v, %v", got, err)
	}

	store.rename = (*os.Root).Rename
	store.syncDir = func(*os.Root) error { return errors.New("injected directory sync failure") }
	err = store.WithTransaction(context.Background(), func(transaction *Transaction) error { return transaction.Publish(newer) })
	if !errors.Is(err, ErrPublicationUncertain) {
		t.Fatalf("post-rename failure = %v", err)
	}
	got, loadErr := store.Load(context.Background())
	if loadErr != nil || got.Generation != newer.Generation {
		t.Fatalf("uncertain publication was not an intact new record: %+v, %v", got, loadErr)
	}
}

func TestReadyProofDetectsReplacementAndIntermediateStates(t *testing.T) {
	store := newTestStore(t)
	manifest := readyManifest(t, store, strings.Repeat("5", 32))
	publish(t, store, manifest)
	var saved ReadyBinding
	var retained *Transaction
	if err := store.WithTransaction(context.Background(), func(transaction *Transaction) error {
		retained = transaction
		var err error
		saved, err = transaction.Ready()
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if saved.Generation != manifest.Generation || saved.ManifestSHA256 == "" {
		t.Fatalf("incomplete ready proof: %+v", saved)
	}
	if _, err := retained.Ready(); err == nil {
		t.Fatal("retained transaction remained usable without its lock")
	}

	replacement := readyManifest(t, store, strings.Repeat("6", 32))
	publish(t, store, replacement)
	if err := store.WithTransaction(context.Background(), func(transaction *Transaction) error { return transaction.ValidateReady(saved) }); !errors.Is(err, ErrGenerationReplaced) {
		t.Fatalf("replacement validation = %v", err)
	}

	for _, state := range []State{StateInstalling, StateUnavailable} {
		replacement.State = state
		publish(t, store, replacement)
		err := store.WithTransaction(context.Background(), func(transaction *Transaction) error {
			_, err := transaction.Ready()
			return err
		})
		if !errors.Is(err, ErrInstallationRequired) || !strings.Contains(err.Error(), string(state)) || !strings.Contains(err.Error(), "maintenance install") {
			t.Errorf("%s readiness error = %v", state, err)
		}
	}
}

func TestMissingObservationIsReadOnlyAndNamesRemedy(t *testing.T) {
	repository, stateRoot := t.TempDir(), filepath.Join(t.TempDir(), "not-created")
	store, err := open(repository, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Load(context.Background())
	if !errors.Is(err, ErrInstallationRequired) || !strings.Contains(err.Error(), "absent") || !strings.Contains(err.Error(), "maintenance install") {
		t.Fatalf("missing load = %v", err)
	}
	if _, statErr := os.Stat(stateRoot); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("read-only observation created state: %v", statErr)
	}
}

func TestStrictManifestRefusesUnknownDuplicateAndInvalidFields(t *testing.T) {
	store := newTestStore(t)
	manifest := readyManifest(t, store, strings.Repeat("7", 32))
	publish(t, store, manifest)
	original, err := os.ReadFile(store.ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string][]byte{
		"unknown":   []byte(strings.Replace(string(original), `"state":"ready"`, `"state":"ready","prompt":"secret"`, 1)),
		"duplicate": []byte(strings.Replace(string(original), `"state":"ready"`, `"state":"ready","state":"ready"`, 1)),
		"state":     []byte(strings.Replace(string(original), `"state":"ready"`, `"state":"recovering"`, 1)),
		"schema":    []byte(strings.Replace(string(original), SchemaVersion, "pyxis-maintenance-installation/v2", 1)),
		"malformed": []byte(`{"schema_version":`),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(store.ManifestPath(), data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Load(context.Background()); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("load = %v", err)
			}
		})
	}
}

func TestManifestIsRestrictedAndContainsOnlyDerivedBindings(t *testing.T) {
	store := newTestStore(t)
	manifest := readyManifest(t, store, strings.Repeat("8", 32))
	publish(t, store, manifest)
	info, err := os.Stat(store.ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("manifest mode = %o", info.Mode().Perm())
	}
	data, err := os.ReadFile(store.ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"prompt", "catalog", "credential", "runner", "settings"} {
		if strings.Contains(string(data), forbidden) {
			t.Errorf("manifest contains forbidden owner field %q: %s", forbidden, data)
		}
	}
}

func TestGenerationAndCanonicalRepositoryIdentity(t *testing.T) {
	generation, err := NewGeneration()
	if err != nil || !validGeneration(generation) {
		t.Fatalf("generation = %q, %v", generation, err)
	}
	repository, stateRoot := t.TempDir(), t.TempDir()
	alias := filepath.Join(t.TempDir(), "repository-link")
	if err := os.Symlink(repository, alias); err != nil {
		t.Fatal(err)
	}
	direct, err := open(repository, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	throughAlias, err := open(alias, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if direct.Repository() != throughAlias.Repository() || direct.ManifestPath() != throughAlias.ManifestPath() {
		t.Fatalf("alias changed identity: %+v/%s vs %+v/%s", direct.Repository(), direct.ManifestPath(), throughAlias.Repository(), throughAlias.ManifestPath())
	}
}

func TestScheduleFingerprintsAreEnabledSortedAndSensitive(t *testing.T) {
	document := jobs.Document{SchemaVersion: jobs.SchemaVersion, Source: jobs.SourceBinding{Remote: "origin", Branch: "main"}, Jobs: []jobs.Job{
		validJob("z-job", true, "0 2 * * *"), validJob("a-job", true, "5 1 * * *"), validJob("off", false, "1 3 * * *"),
	}}
	fingerprints, err := ScheduleFingerprints(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(fingerprints) != 2 || fingerprints[0].JobID != "a-job" || fingerprints[1].JobID != "z-job" {
		t.Fatalf("fingerprints = %+v", fingerprints)
	}
	document.Jobs[1].Schedule.Timezone = "America/New_York"
	changed, err := ScheduleFingerprints(document)
	if err != nil {
		t.Fatal(err)
	}
	if changed[0].SHA256 == fingerprints[0].SHA256 {
		t.Fatal("timezone change retained the schedule fingerprint")
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func readyManifest(t *testing.T, store *Store, generation string) InstallationManifest {
	t.Helper()
	document := jobs.Document{SchemaVersion: jobs.SchemaVersion, Source: jobs.SourceBinding{Remote: "origin", Branch: "main"}, Jobs: []jobs.Job{validJob("daily", true, "0 2 * * *")}}
	schedules, err := ScheduleFingerprints(document)
	if err != nil {
		t.Fatal(err)
	}
	return InstallationManifest{SchemaVersion: SchemaVersion, Generation: InstallationGeneration(generation), State: StateReady, Repository: store.Repository(), Source: document.Source, Host: HostBinding{ExecutablePath: "/usr/local/bin/pyxis", ToolSearchPath: "/usr/local/bin:/usr/bin"}, Schedules: schedules}
}

func validJob(id string, enabled bool, schedule string) jobs.Job {
	return jobs.Job{ID: id, Enabled: enabled, Schedule: jobs.Schedule{Cron: schedule, Timezone: "America/Los_Angeles"}, Timeout: "1h", Runner: jobs.Runner{Harness: "codex", Routing: executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityStrong, Deliberation: executiondemand.DeliberationDeep, Context: executiondemand.ContextLarge, Assurance: executiondemand.AssuranceOrdinary}}, Prompt: "owned policy prompt"}
}

func publish(t *testing.T, store *Store, manifest InstallationManifest) {
	t.Helper()
	if err := store.WithTransaction(context.Background(), func(transaction *Transaction) error { return transaction.Publish(manifest) }); err != nil {
		t.Fatalf("publish: %v", err)
	}
}
