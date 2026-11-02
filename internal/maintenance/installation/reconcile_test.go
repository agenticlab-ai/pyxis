package installation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/audit"
	maintenancecron "github.com/agenticlab-ai/pyxis/internal/maintenance/cron"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

type reconcileCron struct {
	table          maintenancecron.Table
	absent         bool
	installs       int
	failAfterWrite bool
	onInstall      func()
	nextRead       []byte
	driftReads     bool
	reads          int
	afterRead      func(int)
}

func (fake *reconcileCron) Read(ctx context.Context) (maintenancecron.Table, error) {
	if err := ctx.Err(); err != nil {
		return maintenancecron.Table{}, err
	}
	fake.reads++
	if fake.driftReads {
		fake.table.Bytes = fmt.Appendf(fake.table.Bytes, "# drift-%d\n", fake.reads)
	}
	table := fake.table
	table.Bytes = bytes.Clone(table.Bytes)
	if fake.nextRead != nil {
		table.Bytes, fake.nextRead = bytes.Clone(fake.nextRead), nil
	}
	if fake.absent {
		return table, maintenancecron.ErrNoUserTable
	}
	if fake.afterRead != nil {
		fake.afterRead(fake.reads)
	}
	return table, nil
}

func (fake *reconcileCron) Install(_ context.Context, profile maintenancecron.Profile, table []byte) error {
	fake.installs++
	fake.absent, fake.table.Profile, fake.table.Bytes = false, profile, bytes.Clone(table)
	if fake.onInstall != nil {
		fake.onInstall()
	}
	if fake.failAfterWrite {
		fake.failAfterWrite = false
		return errors.New("injected cron write failure")
	}
	return nil
}

type reconcileJournal struct {
	events       []audit.Event
	failSuccess  bool
	afterSuccess func()
}

func (journal *reconcileJournal) Append(_ context.Context, event audit.Event, _ audit.Durability) (audit.Result, error) {
	if journal.failSuccess && event.Outcome == audit.OutcomeSucceeded {
		journal.failSuccess = false
		return audit.Result{}, errors.New("injected audit sync failure")
	}
	journal.events = append(journal.events, event)
	if journal.afterSuccess != nil && event.Outcome == audit.OutcomeSucceeded {
		journal.afterSuccess()
	}
	return audit.Result{}, nil
}

func reconcileDocument(cron string) jobs.Document {
	return jobs.Document{SchemaVersion: jobs.SchemaVersion, Source: jobs.SourceBinding{Remote: "origin", Branch: "main"}, Jobs: []jobs.Job{{
		ID: "daily", Enabled: true, Schedule: jobs.Schedule{Cron: cron, Timezone: "America/Los_Angeles"}, Timeout: "1h", Prompt: "maintain",
		Runner: jobs.Runner{Harness: entity.Codex, Routing: executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityStrong, Deliberation: executiondemand.DeliberationDeep, Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceElevated}},
	}}}
}

func reconcileBinding() maintenancecron.InstalledBinding {
	return maintenancecron.InstalledBinding{Executable: "/opt/pyxis", ToolPath: "/usr/bin:/bin", StableExecutable: true}
}

func reconcileRequest(document jobs.Document) Request {
	return Request{SchemaVersion: RequestSchemaVersion, Document: document, Binding: reconcileBinding(), Identity: audit.Identity{TraceID: "11111111111111111111111111111111", InvocationID: "22222222222222222222222222222222"}}
}

func TestReconcileRollbackRestoresVerifiedPairAndInstallingIsNotReady(t *testing.T) {
	store := newTestStore(t)
	beforeDocument, afterDocument := reconcileDocument("0 2 * * *"), reconcileDocument("30 4 * * *")
	beforeEntries, err := maintenancecron.Render(store.Repository().Root, reconcileBinding(), beforeDocument.Jobs)
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	fake := &reconcileCron{table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Bytes: beforeEntries[0].Text, Profile: profile, HostTimezone: "America/Los_Angeles"}, failAfterWrite: true}
	prior := readyManifest(t, store, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	prior.Source = beforeDocument.Source
	prior.Host = HostBinding{ExecutablePath: reconcileBinding().Executable, ToolSearchPath: reconcileBinding().ToolPath}
	prior.Schedules, _ = ScheduleFingerprints(beforeDocument)
	publish(t, store, prior)
	var intermediate State
	installCalls := 0
	fake.onInstall = func() {
		installCalls++
		if installCalls != 1 {
			return
		}
		manifest, loadErr := store.Load(t.Context())
		if loadErr == nil {
			intermediate = manifest.State
		}
	}
	journal := &reconcileJournal{}
	result, err := (Reconciler{Cron: fake, Store: store, Journal: journal}).Reconcile(t.Context(), reconcileRequest(afterDocument))
	if err == nil || !FailureAudited(err) || fake.installs != 2 || !bytes.Equal(fake.table.Bytes, beforeEntries[0].Text) || intermediate != StateInstalling {
		t.Fatalf("reconcile = (%+v, %v), installs=%d intermediate=%s", result, err, fake.installs, intermediate)
	}
	restored, loadErr := store.Load(t.Context())
	if loadErr != nil || restored.Generation != prior.Generation || len(journal.events) != 1 || journal.events[0].Outcome != audit.OutcomeFailed {
		t.Fatalf("restored=%+v load=%v events=%+v", restored, loadErr, journal.events)
	}
	info, statErr := os.Stat(result.Backup)
	if statErr != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode = %v, %v", info, statErr)
	}
}

func TestReconcileRefreshesNoChangeResultAfterLockWait(t *testing.T) {
	store := newTestStore(t)
	document := reconcileDocument("0 2 * * *")
	entries, _ := maintenancecron.Render(store.Repository().Root, reconcileBinding(), document.Jobs)
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	fake := &reconcileCron{table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Profile: profile, HostTimezone: "America/Los_Angeles"}}
	prior := readyManifest(t, store, "abababababababababababababababab")
	prior.Source, prior.Host = document.Source, HostBinding{ExecutablePath: reconcileBinding().Executable, ToolSearchPath: reconcileBinding().ToolPath}
	prior.Schedules, _ = ScheduleFingerprints(document)
	fake.afterRead = func(read int) {
		if read == 2 {
			fake.table.Bytes = bytes.Clone(entries[0].Text)
			publish(t, store, prior)
		}
	}
	journal := &reconcileJournal{}
	result, err := (Reconciler{Cron: fake, Store: store, Journal: journal}).Reconcile(t.Context(), reconcileRequest(document))
	if err != nil || result.Changed || result.Generation != prior.Generation || fake.installs != 0 || len(journal.events) != 1 || journal.events[0].Outcome != audit.OutcomeNoChange || *journal.events[0].Changed {
		t.Fatalf("result=%+v err=%v writes=%d events=%+v", result, err, fake.installs, journal.events)
	}
}

func TestReconcileRechecksAfterBackupAndPreservesExternalEdit(t *testing.T) {
	store := newTestStore(t)
	document := reconcileDocument("0 2 * * *")
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	fake := &reconcileCron{table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Profile: profile, HostTimezone: "America/Los_Angeles", Bytes: []byte("# original foreign entry\n")}}
	originalSync := store.syncDir
	backupSyncs := 0
	store.syncDir = func(root *os.Root) error {
		backupSyncs++
		if backupSyncs == 1 {
			fake.table.Bytes = append(fake.table.Bytes, []byte("# edit completed during backup\n")...)
		}
		return originalSync(root)
	}
	result, err := (Reconciler{Cron: fake, Store: store, Journal: &reconcileJournal{}}).Reconcile(t.Context(), reconcileRequest(document))
	if err != nil || fake.installs != 1 || result.Backup == "" || !bytes.Contains(fake.table.Bytes, []byte("# original foreign entry\n# edit completed during backup\n")) {
		t.Fatalf("result=%+v err=%v writes=%d table=%q", result, err, fake.installs, fake.table.Bytes)
	}
}

func TestReconcileAuditFailureRollsBackInsteadOfAcknowledgingReady(t *testing.T) {
	store := newTestStore(t)
	beforeDocument, afterDocument := reconcileDocument("0 2 * * *"), reconcileDocument("30 4 * * *")
	beforeEntries, _ := maintenancecron.Render(store.Repository().Root, reconcileBinding(), beforeDocument.Jobs)
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	fake := &reconcileCron{table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Bytes: beforeEntries[0].Text, Profile: profile, HostTimezone: "America/Los_Angeles"}}
	prior := readyManifest(t, store, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	prior.Source = beforeDocument.Source
	prior.Host = HostBinding{ExecutablePath: reconcileBinding().Executable, ToolSearchPath: reconcileBinding().ToolPath}
	prior.Schedules, _ = ScheduleFingerprints(beforeDocument)
	publish(t, store, prior)
	journal := &reconcileJournal{failSuccess: true}
	_, err := (Reconciler{Cron: fake, Store: store, Journal: journal}).Reconcile(t.Context(), reconcileRequest(afterDocument))
	restored, loadErr := store.Load(t.Context())
	if err == nil || fake.installs != 2 || loadErr != nil || restored.Generation != prior.Generation || len(journal.events) != 1 || journal.events[0].Outcome != audit.OutcomeFailed {
		t.Fatalf("err=%v installs=%d restored=%+v load=%v events=%+v", err, fake.installs, restored, loadErr, journal.events)
	}
}

func TestReconcileCancellationAfterAcknowledgedSuccessDoesNotRollback(t *testing.T) {
	store := newTestStore(t)
	document := reconcileDocument("0 2 * * *")
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	fake := &reconcileCron{table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Profile: profile, HostTimezone: "America/Los_Angeles"}}
	ctx, cancel := context.WithCancel(t.Context())
	journal := &reconcileJournal{afterSuccess: cancel}
	result, err := (Reconciler{Cron: fake, Store: store, Journal: journal}).Reconcile(ctx, reconcileRequest(document))
	manifest, loadErr := store.Load(t.Context())
	if err != nil || result.Generation != manifest.Generation || loadErr != nil || manifest.State != StateReady || fake.installs != 1 || len(journal.events) != 1 || journal.events[0].Outcome != audit.OutcomeSucceeded {
		t.Fatalf("result=%+v err=%v manifest=%+v load=%v writes=%d events=%+v", result, err, manifest, loadErr, fake.installs, journal.events)
	}
}

func TestReconcileFailedFirstInstallRetainsUnavailableAuthority(t *testing.T) {
	store := newTestStore(t)
	document := reconcileDocument("0 2 * * *")
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	fake := &reconcileCron{absent: true, failAfterWrite: true, table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Profile: profile, HostTimezone: "America/Los_Angeles"}}
	journal := &reconcileJournal{}
	result, err := (Reconciler{Cron: fake, Store: store, Journal: journal}).Reconcile(t.Context(), reconcileRequest(document))
	manifest, loadErr := store.Load(t.Context())
	if err == nil || fake.installs != 1 || loadErr != nil || manifest.State != StateUnavailable || result.Backup == "" || len(journal.events) != 1 || journal.events[0].Outcome != audit.OutcomeFailed {
		t.Fatalf("result=%+v err=%v installs=%d manifest=%+v load=%v events=%+v", result, err, fake.installs, manifest, loadErr, journal.events)
	}
}

func TestReconcileReadyPublishFaultRestoresPriorPair(t *testing.T) {
	store := newTestStore(t)
	beforeDocument, afterDocument := reconcileDocument("0 2 * * *"), reconcileDocument("30 4 * * *")
	beforeEntries, _ := maintenancecron.Render(store.Repository().Root, reconcileBinding(), beforeDocument.Jobs)
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	fake := &reconcileCron{table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Bytes: beforeEntries[0].Text, Profile: profile, HostTimezone: "America/Los_Angeles"}}
	prior := readyManifest(t, store, "cccccccccccccccccccccccccccccccc")
	prior.Source, prior.Host = beforeDocument.Source, HostBinding{ExecutablePath: reconcileBinding().Executable, ToolSearchPath: reconcileBinding().ToolPath}
	prior.Schedules, _ = ScheduleFingerprints(beforeDocument)
	publish(t, store, prior)
	renames := 0
	store.rename = func(root *os.Root, oldName, newName string) error {
		renames++
		if renames == 2 {
			return errors.New("injected ready publish failure")
		}
		return root.Rename(oldName, newName)
	}
	journal := &reconcileJournal{}
	_, err := (Reconciler{Cron: fake, Store: store, Journal: journal}).Reconcile(t.Context(), reconcileRequest(afterDocument))
	restored, loadErr := store.Load(t.Context())
	if err == nil || fake.installs != 2 || loadErr != nil || restored.Generation != prior.Generation || !bytes.Equal(fake.table.Bytes, beforeEntries[0].Text) || len(journal.events) != 1 {
		t.Fatalf("err=%v installs=%d restored=%+v load=%v events=%+v", err, fake.installs, restored, loadErr, journal.events)
	}
}

func TestReconcileVerificationFaultAndCancellationRestorePriorPair(t *testing.T) {
	for _, test := range []struct {
		name  string
		fault func(*reconcileCron, context.CancelFunc)
	}{
		{name: "read-back mismatch", fault: func(fake *reconcileCron, _ context.CancelFunc) { fake.nextRead = []byte("corrupt\n") }},
		{name: "post-write cancellation", fault: func(_ *reconcileCron, cancel context.CancelFunc) { cancel() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newTestStore(t)
			beforeDocument, afterDocument := reconcileDocument("0 2 * * *"), reconcileDocument("30 4 * * *")
			beforeEntries, _ := maintenancecron.Render(store.Repository().Root, reconcileBinding(), beforeDocument.Jobs)
			profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
			fake := &reconcileCron{table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Bytes: beforeEntries[0].Text, Profile: profile, HostTimezone: "America/Los_Angeles"}}
			prior := readyManifest(t, store, "dddddddddddddddddddddddddddddddd")
			prior.Source, prior.Host = beforeDocument.Source, HostBinding{ExecutablePath: reconcileBinding().Executable, ToolSearchPath: reconcileBinding().ToolPath}
			prior.Schedules, _ = ScheduleFingerprints(beforeDocument)
			publish(t, store, prior)
			ctx, cancel := context.WithCancel(t.Context())
			calls := 0
			fake.onInstall = func() {
				calls++
				if calls == 1 {
					test.fault(fake, cancel)
				}
			}
			_, err := (Reconciler{Cron: fake, Store: store, Journal: &reconcileJournal{}}).Reconcile(ctx, reconcileRequest(afterDocument))
			restored, loadErr := store.Load(t.Context())
			if err == nil || fake.installs != 2 || loadErr != nil || restored.Generation != prior.Generation || !bytes.Equal(fake.table.Bytes, beforeEntries[0].Text) {
				t.Fatalf("err=%v installs=%d restored=%+v load=%v", err, fake.installs, restored, loadErr)
			}
		})
	}
}

func TestReconcileBoundsExternalTableChangeWithoutWriting(t *testing.T) {
	store := newTestStore(t)
	document := reconcileDocument("0 2 * * *")
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	fake := &reconcileCron{driftReads: true, table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Profile: profile, HostTimezone: "America/Los_Angeles"}}
	journal := &reconcileJournal{}
	_, err := (Reconciler{Cron: fake, Store: store, Journal: journal}).Reconcile(t.Context(), reconcileRequest(document))
	if err == nil || fake.reads != tableRechecks*2 || fake.installs != 0 || len(journal.events) != 1 || journal.events[0].Outcome != audit.OutcomeFailed {
		t.Fatalf("err=%v reads=%d installs=%d events=%+v", err, fake.reads, fake.installs, journal.events)
	}
}

func TestReconcileIdenticalTableWithoutAuthorityPublishesReadyWithoutCronWrite(t *testing.T) {
	store := newTestStore(t)
	document := reconcileDocument("0 2 * * *")
	entries, _ := maintenancecron.Render(store.Repository().Root, reconcileBinding(), document.Jobs)
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	fake := &reconcileCron{table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Bytes: entries[0].Text, Profile: profile, HostTimezone: "America/Los_Angeles"}}
	result, err := (Reconciler{Cron: fake, Store: store, Journal: &reconcileJournal{}}).Reconcile(t.Context(), reconcileRequest(document))
	manifest, loadErr := store.Load(t.Context())
	if err != nil || fake.installs != 0 || !result.Changed || result.SchemaVersion != ResultSchemaVersion || loadErr != nil || manifest.State != StateReady {
		t.Fatalf("result=%+v err=%v installs=%d manifest=%+v load=%v", result, err, fake.installs, manifest, loadErr)
	}
}
