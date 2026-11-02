package admission

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/installation"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

func TestLockedAuthorityHoldsInstallationTransactionThroughGuard(t *testing.T) {
	repository := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	store, err := installation.Open(repository)
	if err != nil {
		t.Fatal(err)
	}
	manifest := installation.InstallationManifest{SchemaVersion: installation.SchemaVersion, Generation: installation.InstallationGeneration(strings.Repeat("a", 32)),
		State: installation.StateReady, Repository: store.Repository(), Source: jobs.SourceBinding{Remote: "origin", Branch: "main"},
		Host: installation.HostBinding{ExecutablePath: filepath.Join(repository, "pyxis"), ToolSearchPath: filepath.Join(repository, "bin")}, Schedules: []installation.ScheduleFingerprint{}}
	if err := store.WithTransaction(t.Context(), func(transaction *installation.Transaction) error { return transaction.Publish(manifest) }); err != nil {
		t.Fatal(err)
	}
	authority := LockedAuthority{Store: store}
	ready, err := authority.Ready(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.GuardReady(t.Context(), ready, nil); err == nil {
		t.Fatal("nil ready guard callback succeeded")
	}
	entered, release, guardedDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		guardedDone <- authority.GuardReady(context.Background(), ready, func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	replacement := manifest
	replacement.Generation = installation.InstallationGeneration(strings.Repeat("b", 32))
	competingDone := make(chan error, 1)
	go func() {
		competingDone <- store.WithTransaction(context.Background(), func(transaction *installation.Transaction) error { return transaction.Publish(replacement) })
	}()
	select {
	case err := <-competingDone:
		t.Fatalf("installation mutation interleaved with claim proof: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-guardedDone; err != nil {
		t.Fatal(err)
	}
	if err := <-competingDone; err != nil {
		t.Fatal(err)
	}
	if err := authority.GuardReady(t.Context(), ready, func() error { return nil }); !errors.Is(err, installation.ErrGenerationReplaced) {
		t.Fatalf("replaced generation guard = %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	if err := authority.GuardReady(canceled, ready, func() error { called = true; return nil }); !errors.Is(err, context.Canceled) || called {
		t.Fatalf("canceled guard = %v, called=%v", err, called)
	}
}
