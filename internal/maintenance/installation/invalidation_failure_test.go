package installation

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/audit"
	maintenancecron "github.com/agenticlab-ai/pyxis/internal/maintenance/cron"
)

func TestReconcileRecoversAfterInitialInvalidationRenameFailure(t *testing.T) {
	for _, test := range []struct {
		name          string
		rollbackFails bool
		rollbackWaits bool
	}{
		{name: "verified prior pair"},
		{name: "rollback write fails", rollbackFails: true},
		{name: "rollback deadline expires", rollbackWaits: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newTestStore(t)
			beforeDocument := reconcileDocument("0 2 * * *")
			entries, err := maintenancecron.Render(store.Repository().Root, reconcileBinding(), beforeDocument.Jobs)
			if err != nil {
				t.Fatal(err)
			}
			profile, ok := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
			if !ok {
				t.Fatal("known Cronie profile missing")
			}
			base := &reconcileCron{table: maintenancecron.Table{
				SchemaVersion: maintenancecron.TableSchemaVersion, Profile: profile,
				HostTimezone: "America/Los_Angeles", Bytes: bytes.Clone(entries[0].Text),
			}}
			prior := readyManifest(t, store, "12121212121212121212121212121212")
			prior.Source = beforeDocument.Source
			prior.Host = HostBinding{ExecutablePath: reconcileBinding().Executable, ToolSearchPath: reconcileBinding().ToolPath}
			prior.Schedules, err = ScheduleFingerprints(beforeDocument)
			if err != nil {
				t.Fatal(err)
			}
			publish(t, store, prior)

			invalidationErr := errors.New("one-shot unavailable manifest rename failure")
			renames := 0
			var failedReady ReadyBinding
			store.rename = func(directory *os.Root, oldName, newName string) error {
				renames++
				if renames == 3 {
					manifest, loadErr := store.load()
					if loadErr != nil {
						return loadErr
					}
					failedReady, loadErr = proof(manifest)
					if loadErr != nil {
						return loadErr
					}
					return invalidationErr
				}
				return directory.Rename(oldName, newName)
			}
			port := &reconcileFaultPort{reconcileCron: base, rollbackWait: test.rollbackWaits}
			rollbackErr := errors.New("provider rollback write failed")
			if test.rollbackFails {
				port.rollbackFailure = rollbackErr
			}
			journal := &reconcileJournal{failSuccess: true}
			var recoveryTimeout time.Duration
			if test.rollbackWaits {
				recoveryTimeout = 2 * time.Millisecond
			}
			result, err := (Reconciler{Cron: port, Store: store, Journal: journal, recoveryTimeout: recoveryTimeout}).Reconcile(t.Context(), reconcileRequest(reconcileDocument("30 4 * * *")))
			if !errors.Is(err, invalidationErr) || !FailureOwned(err) || result.Backup == "" || base.installs != 2 {
				t.Errorf("result=%+v err=%v provider writes=%d", result, err, base.installs)
			}
			if test.rollbackFails && !errors.Is(err, rollbackErr) {
				t.Errorf("rollback cause lost: %v", err)
			}
			if test.rollbackWaits && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("rollback deadline lost: %v", err)
			}
			current, loadErr := store.Load(t.Context())
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			if test.rollbackFails || test.rollbackWaits {
				if current.State != StateUnavailable {
					t.Errorf("failed rollback grants authority: %+v", current)
				}
			} else if current.State != StateReady || current.Generation != prior.Generation || !bytes.Equal(base.table.Bytes, entries[0].Text) {
				t.Errorf("verified prior pair not restored: manifest=%+v table=%q", current, base.table.Bytes)
			}
			if failedReady.Generation != result.Generation {
				t.Fatalf("fixture did not capture failed ready generation: proof=%+v result=%+v", failedReady, result)
			}
			validateErr := store.WithTransaction(t.Context(), func(transaction *Transaction) error {
				return transaction.ValidateReady(failedReady)
			})
			if validateErr == nil {
				t.Error("failed generation still passes ValidateReady")
			}
			if len(journal.events) != 1 || journal.events[0].Outcome != audit.OutcomeFailed {
				t.Errorf("failure audit=%+v", journal.events)
			}
		})
	}
}
