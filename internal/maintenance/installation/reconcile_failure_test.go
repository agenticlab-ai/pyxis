package installation

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/audit"
	maintenancecron "github.com/agenticlab-ai/pyxis/internal/maintenance/cron"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

type reconcileFaultPort struct {
	*reconcileCron
	calls               int
	readFailureAt       int
	readFailure         error
	rollbackFailure     error
	rollbackReadFailure error
	rollbackWait        bool
}

func (port *reconcileFaultPort) Read(ctx context.Context) (maintenancecron.Table, error) {
	port.calls++
	if port.calls == port.readFailureAt || port.installs >= 2 && port.rollbackReadFailure != nil {
		return maintenancecron.Table{}, errors.Join(port.readFailure, port.rollbackReadFailure)
	}
	return port.reconcileCron.Read(ctx)
}

func (port *reconcileFaultPort) Install(ctx context.Context, profile maintenancecron.Profile, table []byte) error {
	if port.installs != 0 && port.rollbackFailure != nil {
		port.installs++
		return port.rollbackFailure
	}
	if port.installs != 0 && port.rollbackWait {
		port.installs++
		<-ctx.Done()
		return ctx.Err()
	}
	return port.reconcileCron.Install(ctx, profile, table)
}

type failingReconcileJournal struct {
	err   error
	calls int
}

func TestReconcileInvalidPolicyAndBindingRefuseBeforeProviderIO(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Request)
		want   error
	}{
		{"absent request schema", func(request *Request) { request.SchemaVersion = "" }, ErrInvalidRequest},
		{"unknown request schema", func(request *Request) { request.SchemaVersion = "unknown" }, ErrInvalidRequest},
		{"unknown policy schema", func(request *Request) { request.Document.SchemaVersion = "unknown" }, jobs.ErrInvalid},
		{"unattested executable", func(request *Request) { request.Binding.StableExecutable = false }, maintenancecron.ErrUnstableExecutable},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newTestStore(t)
			port := &reconcileFaultPort{reconcileCron: &reconcileCron{}}
			journal := &reconcileJournal{}
			request := reconcileRequest(reconcileDocument("0 2 * * *"))
			test.change(&request)
			_, err := (Reconciler{Cron: port, Store: store, Journal: journal}).Reconcile(t.Context(), request)
			if !errors.Is(err, test.want) || port.calls != 0 || port.installs != 0 || len(journal.events) != 0 {
				t.Fatalf("err=%v reads=%d writes=%d events=%+v", err, port.calls, port.installs, journal.events)
			}
			if _, err := store.Load(t.Context()); !errors.Is(err, ErrInstallationRequired) {
				t.Fatalf("refused request exposed authority: %v", err)
			}
		})
	}
}

func (journal *failingReconcileJournal) Append(context.Context, audit.Event, audit.Durability) (audit.Result, error) {
	journal.calls++
	return audit.Result{}, journal.err
}

func TestReconcileObservationFailuresPreserveExistingAuthority(t *testing.T) {
	for _, test := range []struct {
		name      string
		failureAt int
		malformed bool
	}{
		{"first observation", 1, false},
		{"concurrent-change recheck", 2, false},
		{"ownership refusal", 0, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newTestStore(t)
			prior := readyManifest(t, store, strings.Repeat("d", 32))
			publish(t, store, prior)
			profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
			base := &reconcileCron{table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Profile: profile, HostTimezone: "America/Los_Angeles", Bytes: []byte("# untouched foreign job\n")}}
			if test.malformed {
				base.table.Bytes = []byte("# pyxis-maintenance/v1 malformed\n")
			}
			before := bytes.Clone(base.table.Bytes)
			want := errors.New("injected scheduler observation failure")
			port := &reconcileFaultPort{reconcileCron: base, readFailureAt: test.failureAt, readFailure: want}
			journal := &reconcileJournal{}
			result, err := (Reconciler{Cron: port, Store: store, Journal: journal}).Reconcile(t.Context(), reconcileRequest(reconcileDocument("0 2 * * *")))
			if test.malformed {
				want = maintenancecron.ErrMalformedOwnership
			}
			if !errors.Is(err, want) || !FailureAudited(err) || base.installs != 0 || !bytes.Equal(before, base.table.Bytes) || result.Backup != "" {
				t.Fatalf("result=%+v err=%v writes=%d", result, err, base.installs)
			}
			current, loadErr := store.Load(t.Context())
			if loadErr != nil || current.Generation != prior.Generation || len(journal.events) != 1 || journal.events[0].Outcome != audit.OutcomeFailed {
				t.Fatalf("authority=%+v err=%v events=%+v", current, loadErr, journal.events)
			}
		})
	}
}

func TestReconcileAuditFailureKeepsObservationFailureAndGrantsNoAuthority(t *testing.T) {
	store := newTestStore(t)
	readErr, auditErr := errors.New("scheduler unavailable"), errors.New("audit unavailable")
	port := &reconcileFaultPort{reconcileCron: &reconcileCron{}, readFailureAt: 1, readFailure: readErr}
	journal := &failingReconcileJournal{err: auditErr}
	_, err := (Reconciler{Cron: port, Store: store, Journal: journal}).Reconcile(t.Context(), reconcileRequest(reconcileDocument("0 2 * * *")))
	if !errors.Is(err, readErr) || !errors.Is(err, auditErr) || journal.calls != 1 || port.installs != 0 || FailureAudited(err) || !FailureOwned(err) {
		t.Fatalf("err=%v audits=%d writes=%d", err, journal.calls, port.installs)
	}
	if _, err := store.Load(t.Context()); !errors.Is(err, ErrInstallationRequired) {
		t.Fatalf("failed admission exposed authority: %v", err)
	}
}

func TestReconcileRepeatedAuditFailureRetainsInstallationPhaseAndEvidence(t *testing.T) {
	store := newTestStore(t)
	beforeDocument, afterDocument := reconcileDocument("0 2 * * *"), reconcileDocument("30 4 * * *")
	beforeEntries, _ := maintenancecron.Render(store.Repository().Root, reconcileBinding(), beforeDocument.Jobs)
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	port := &reconcileCron{table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Bytes: beforeEntries[0].Text, Profile: profile, HostTimezone: "America/Los_Angeles"}}
	prior := readyManifest(t, store, strings.Repeat("8", 32))
	prior.Source, prior.Host = beforeDocument.Source, HostBinding{ExecutablePath: reconcileBinding().Executable, ToolSearchPath: reconcileBinding().ToolPath}
	prior.Schedules, _ = ScheduleFingerprints(beforeDocument)
	publish(t, store, prior)
	auditErr := errors.New("audit append and sync unavailable")
	journal := &failingReconcileJournal{err: auditErr}
	result, err := (Reconciler{Cron: port, Store: store, Journal: journal}).Reconcile(t.Context(), reconcileRequest(afterDocument))
	manifest, loadErr := store.Load(t.Context())
	if !errors.Is(err, auditErr) || !FailureOwned(err) || FailureAudited(err) || journal.calls != 2 || !result.Changed || result.Owned != 1 || result.Generation == "" || result.Backup == "" || loadErr != nil || manifest.Generation != prior.Generation {
		t.Fatalf("result=%+v err=%v audits=%d manifest=%+v load=%v", result, err, journal.calls, manifest, loadErr)
	}
}

func TestReconcileRecoveryDeadlineCannotLeaveReadyAuthority(t *testing.T) {
	store := newTestStore(t)
	beforeDocument, afterDocument := reconcileDocument("0 2 * * *"), reconcileDocument("30 4 * * *")
	beforeEntries, _ := maintenancecron.Render(store.Repository().Root, reconcileBinding(), beforeDocument.Jobs)
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	base := &reconcileCron{table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Bytes: beforeEntries[0].Text, Profile: profile, HostTimezone: "America/Los_Angeles"}}
	prior := readyManifest(t, store, strings.Repeat("f", 32))
	prior.Source, prior.Host = beforeDocument.Source, HostBinding{ExecutablePath: reconcileBinding().Executable, ToolSearchPath: reconcileBinding().ToolPath}
	prior.Schedules, _ = ScheduleFingerprints(beforeDocument)
	publish(t, store, prior)
	port := &reconcileFaultPort{reconcileCron: base, rollbackWait: true}
	result, err := (Reconciler{Cron: port, Store: store, Journal: &reconcileJournal{failSuccess: true}, recoveryTimeout: time.Millisecond}).Reconcile(t.Context(), reconcileRequest(afterDocument))
	manifest, loadErr := store.Load(t.Context())
	if !errors.Is(err, context.DeadlineExceeded) || result.Backup == "" || loadErr != nil || manifest.State != StateUnavailable || port.installs != 2 {
		t.Fatalf("result=%+v err=%v manifest=%+v load=%v writes=%d", result, err, manifest, loadErr, port.installs)
	}
	if readyErr := store.WithTransaction(t.Context(), func(tx *Transaction) error { _, err := tx.Ready(); return err }); !errors.Is(readyErr, ErrInstallationRequired) {
		t.Fatalf("expired recovery exposed ready authority: %v", readyErr)
	}
}

func TestReconcileIncompatibleRollbackCannotRepublishReadyAuthority(t *testing.T) {
	store := newTestStore(t)
	beforeDocument, afterDocument := reconcileDocument("0 2 * * *"), reconcileDocument("30 4 * * *")
	beforeEntries, _ := maintenancecron.Render(store.Repository().Root, reconcileBinding(), beforeDocument.Jobs)
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	base := &reconcileCron{failAfterWrite: true, table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Bytes: beforeEntries[0].Text, Profile: profile, HostTimezone: "America/Los_Angeles"}}
	base.onInstall = func() { base.table.HostTimezone = "UTC" }
	prior := readyManifest(t, store, strings.Repeat("9", 32))
	prior.Source, prior.Host = beforeDocument.Source, HostBinding{ExecutablePath: reconcileBinding().Executable, ToolSearchPath: reconcileBinding().ToolPath}
	prior.Schedules, _ = ScheduleFingerprints(beforeDocument)
	publish(t, store, prior)
	_, err := (Reconciler{Cron: base, Store: store, Journal: &reconcileJournal{}}).Reconcile(t.Context(), reconcileRequest(afterDocument))
	manifest, loadErr := store.Load(t.Context())
	if !errors.Is(err, maintenancecron.ErrAdmissionRefused) || loadErr != nil || manifest.State != StateUnavailable {
		t.Fatalf("err=%v manifest=%+v load=%v", err, manifest, loadErr)
	}
}

func TestReconcileUnverifiedRollbackWithholdsReadyAuthorityAndRetainsBackup(t *testing.T) {
	for _, failure := range []string{"rollback write", "rollback read"} {
		t.Run(failure, func(t *testing.T) {
			store := newTestStore(t)
			beforeDoc := reconcileDocument("0 2 * * *")
			entries, err := maintenancecron.Render(store.Repository().Root, reconcileBinding(), beforeDoc.Jobs)
			if err != nil {
				t.Fatal(err)
			}
			profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
			base := &reconcileCron{failAfterWrite: true, table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Profile: profile, HostTimezone: "America/Los_Angeles", Bytes: entries[0].Text}}
			prior := readyManifest(t, store, strings.Repeat("e", 32))
			publish(t, store, prior)
			want := errors.New("injected " + failure)
			port := &reconcileFaultPort{reconcileCron: base}
			if failure == "rollback write" {
				port.rollbackFailure = want
			} else {
				port.rollbackReadFailure = want
			}
			journal := &reconcileJournal{}
			result, err := (Reconciler{Cron: port, Store: store, Journal: journal}).Reconcile(t.Context(), reconcileRequest(reconcileDocument("30 4 * * *")))
			if !errors.Is(err, want) || base.installs != 2 || result.Backup == "" || !FailureAudited(err) {
				t.Fatalf("result=%+v err=%v writes=%d", result, err, base.installs)
			}
			current, loadErr := store.Load(t.Context())
			if loadErr != nil || current.State != StateUnavailable || len(journal.events) != 1 || journal.events[0].Outcome != audit.OutcomeFailed || !strings.Contains(journal.events[0].Cause, want.Error()) {
				t.Fatalf("authority=%+v err=%v events=%+v", current, loadErr, journal.events)
			}
			err = store.WithTransaction(t.Context(), func(tx *Transaction) error { _, err := tx.Ready(); return err })
			if !errors.Is(err, ErrInstallationRequired) {
				t.Fatalf("unverified rollback exposed ready generation: %v", err)
			}
		})
	}
}
