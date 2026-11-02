package installation

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/audit"
	maintenancecron "github.com/agenticlab-ai/pyxis/internal/maintenance/cron"
)

func TestReconcileRollbackRefusesPriorScheduleDrift(t *testing.T) {
	for _, test := range []struct{ name, actualCron, hostTimezone string }{
		{"timezone changed before reinstall", "0 2 * * *", "UTC"},
		{"cron changed before reinstall", "15 3 * * *", "America/Los_Angeles"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newTestStore(t)
			priorDocument := reconcileDocument("0 2 * * *")
			actualDocument := reconcileDocument(test.actualCron)
			proposedDocument := reconcileDocument("30 4 * * *")
			proposedDocument.Jobs[0].Schedule.Timezone = test.hostTimezone
			entries, err := maintenancecron.Render(store.Repository().Root, reconcileBinding(), actualDocument.Jobs)
			if err != nil {
				t.Fatal(err)
			}
			profile, ok := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
			if !ok {
				t.Fatal("known Cronie profile missing")
			}
			fake := &reconcileCron{failAfterWrite: true, table: maintenancecron.Table{
				SchemaVersion: maintenancecron.TableSchemaVersion, Profile: profile,
				HostTimezone: test.hostTimezone, Bytes: entries[0].Text,
			}}
			prior := readyManifest(t, store, "abababababababababababababababab")
			prior.Source = priorDocument.Source
			prior.Host = HostBinding{ExecutablePath: reconcileBinding().Executable, ToolSearchPath: reconcileBinding().ToolPath}
			prior.Schedules, err = ScheduleFingerprints(priorDocument)
			if err != nil {
				t.Fatal(err)
			}
			publish(t, store, prior)
			result, err := (Reconciler{Cron: fake, Store: store, Journal: &reconcileJournal{}}).Reconcile(t.Context(), reconcileRequest(proposedDocument))
			if !errors.Is(err, maintenancecron.ErrAdmissionRefused) || fake.installs != 2 || result.Backup == "" {
				t.Fatalf("result=%+v err=%v writes=%d", result, err, fake.installs)
			}
			manifest, loadErr := store.Load(t.Context())
			if loadErr != nil || manifest.State != StateUnavailable {
				t.Fatalf("rollback authority=%+v load=%v", manifest, loadErr)
			}
			readyErr := store.WithTransaction(t.Context(), func(tx *Transaction) error {
				_, err := tx.Ready()
				return err
			})
			if !errors.Is(readyErr, ErrInstallationRequired) {
				t.Fatalf("unverified rollback granted ready authority: %v", readyErr)
			}
		})
	}
}

func TestReconcileRollbackRestoresReadyEmptyInstallationWithAbsentTable(t *testing.T) {
	store := newTestStore(t)
	priorDocument := reconcileDocument("0 2 * * *")
	priorDocument.Jobs = priorDocument.Jobs[:0]
	proposedDocument := priorDocument
	proposedDocument.Source.Branch = "stable"
	profile, ok := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	if !ok {
		t.Fatal("known Cronie profile missing")
	}
	fake := &reconcileCron{absent: true, table: maintenancecron.Table{
		SchemaVersion: maintenancecron.TableSchemaVersion, Profile: profile, HostTimezone: "America/Los_Angeles",
	}}
	prior := readyManifest(t, store, "cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd")
	prior.Source = priorDocument.Source
	prior.Host = HostBinding{ExecutablePath: reconcileBinding().Executable, ToolSearchPath: reconcileBinding().ToolPath}
	var err error
	prior.Schedules, err = ScheduleFingerprints(priorDocument)
	if err != nil {
		t.Fatal(err)
	}
	publish(t, store, prior)
	journal := &reconcileJournal{failSuccess: true}
	result, err := (Reconciler{Cron: fake, Store: store, Journal: journal}).Reconcile(t.Context(), reconcileRequest(proposedDocument))
	if err == nil || !FailureAudited(err) || fake.installs != 0 || !fake.absent || result.Backup == "" {
		t.Fatalf("result=%+v err=%v writes=%d absent=%v", result, err, fake.installs, fake.absent)
	}
	restored, loadErr := store.Load(t.Context())
	if loadErr != nil || restored.State != StateReady || restored.Generation != prior.Generation || restored.Source != prior.Source {
		t.Fatalf("restored=%+v load=%v", restored, loadErr)
	}
	if len(journal.events) != 1 || journal.events[0].Outcome != audit.OutcomeFailed {
		t.Fatalf("audit=%+v", journal.events)
	}
}
