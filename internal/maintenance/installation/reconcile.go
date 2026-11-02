package installation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/audit"
	maintenancecron "github.com/agenticlab-ai/pyxis/internal/maintenance/cron"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

const providerTimeout = 10 * time.Second
const tableRechecks = 3

const RequestSchemaVersion = "pyxis-maintenance-installation-request/v1"
const ResultSchemaVersion = "pyxis-maintenance-installation-result/v1"

var ErrInvalidRequest = errors.New("invalid maintenance installation request schema or dependencies")

type Journal interface {
	Append(context.Context, audit.Event, audit.Durability) (audit.Result, error)
}

type Request struct {
	SchemaVersion string                           `json:"schema_version"`
	Document      jobs.Document                    `json:"document"`
	Binding       maintenancecron.InstalledBinding `json:"binding"`
	Identity      audit.Identity                   `json:"identity"`
	Uninstall     bool                             `json:"uninstall"`
	Preview       bool                             `json:"preview"`
}

type ReconcileResult struct {
	SchemaVersion string                 `json:"schema_version"`
	Changed       bool                   `json:"changed"`
	Owned         int                    `json:"owned"`
	Generation    InstallationGeneration `json:"generation"`
	Backup        string                 `json:"backup"`
}

type reconciliationFailure struct {
	error
	audited bool
}

func (failure *reconciliationFailure) Unwrap() error { return failure.error }

func FailureAudited(err error) bool {
	var failure *reconciliationFailure
	return errors.As(err, &failure) && failure.audited
}

// FailureOwned reports that reconciliation, rather than CLI admission, owns
// the failure phase even when its failure-audit append also failed.
func FailureOwned(err error) bool {
	var failure *reconciliationFailure
	return errors.As(err, &failure)
}

type Reconciler struct {
	Cron    maintenancecron.Port
	Store   *Store
	Journal Journal

	recoveryTimeout time.Duration
}

type observedPlan struct {
	before, after maintenancecron.Table
	present       bool
	changed       bool
}

// Reconcile composes one repository's cron and authority transition. Preview
// performs observations only and never enters the state/audit locks.
func (r Reconciler) Reconcile(ctx context.Context, request Request) (ReconcileResult, error) {
	if request.SchemaVersion != RequestSchemaVersion || r.Cron == nil || r.Store == nil || (!request.Preview && r.Journal == nil) {
		return ReconcileResult{}, ErrInvalidRequest
	}
	if !request.Uninstall {
		if err := request.Document.Validate(); err != nil {
			return ReconcileResult{}, err
		}
	}
	root := r.Store.Repository().Root
	configured := request.Document.Jobs
	if request.Uninstall {
		configured = nil
	}
	var desired []maintenancecron.CronEntry
	if !request.Uninstall {
		var err error
		desired, err = maintenancecron.Render(root, request.Binding, configured)
		if err != nil {
			return ReconcileResult{}, err
		}
	}
	hash := r.Store.Repository().SHA256
	requirements := make([]maintenancecron.EntryRequirement, 0, len(desired))
	for _, entry := range desired {
		for _, job := range configured {
			if job.ID == entry.JobID {
				requirements = append(requirements, maintenancecron.EntryRequirement{RepositoryHash: hash, JobID: job.ID, Timezone: job.Schedule.Timezone})
			}
		}
	}
	host := HostBinding{ExecutablePath: request.Binding.Executable, ToolSearchPath: request.Binding.ToolPath}
	schedules, err := ScheduleFingerprints(request.Document)
	if request.Uninstall {
		schedules, err = []ScheduleFingerprint{}, nil
	}
	if err != nil {
		return ReconcileResult{}, err
	}
	started := time.Now()
	result := ReconcileResult{SchemaVersion: ResultSchemaVersion, Owned: len(desired)}
	plan, err := r.stablePlan(ctx, hash, desired, requirements)
	if err != nil {
		if !request.Preview {
			err = r.report(request, result, err, started)
		}
		return result, err
	}
	result.Changed = plan.changed
	if request.Preview {
		manifest, loadErr := r.Store.Load(ctx)
		if loadErr != nil && !errors.Is(loadErr, ErrInstallationRequired) {
			return result, loadErr
		}
		if request.Uninstall {
			result.Changed = result.Changed || loadErr == nil
		} else {
			result.Changed = result.Changed || loadErr != nil || !matches(manifest, request.Document, host, schedules)
		}
		return result, nil
	}
	err = r.Store.WithTransaction(ctx, func(transaction *Transaction) error {
		var prior *InstallationManifest
		if manifest, loadErr := transaction.Load(); loadErr == nil {
			prior = &manifest
		} else if !errors.Is(loadErr, ErrInstallationRequired) {
			return loadErr
		}
		for attempt := 0; ; attempt++ {
			plan, err = r.stablePlan(ctx, hash, desired, requirements)
			if err != nil {
				return err
			}
			result.Changed = plan.changed
			if request.Uninstall && prior == nil && !plan.changed {
				return r.append(ctx, request, result, audit.OutcomeNoChange, "", started)
			}
			if !plan.changed && prior != nil && !request.Uninstall && matches(*prior, request.Document, host, schedules) {
				result.Generation = prior.Generation
				return r.append(ctx, request, result, audit.OutcomeNoChange, "", started)
			}
			backup, backupErr := transaction.Backup(plan.before.Bytes, plan.present, prior)
			result.Backup = backup
			if backupErr != nil {
				return backupErr
			}
			observed, observeErr := r.read(ctx)
			if observeErr != nil {
				return observeErr
			}
			if plan.present == observed.present && reflect.DeepEqual(plan.before, observed.after) {
				break
			}
			if attempt+1 == tableRechecks {
				return errors.New("maintenance cron table changed during bounded backup admission")
			}
		}
		result.Changed = true
		generation, err := NewGeneration()
		if err != nil {
			return err
		}
		result.Generation = generation
		source := request.Document.Source
		if request.Uninstall && prior != nil {
			source = prior.Source
			host = prior.Host
		}
		transition := InstallationManifest{SchemaVersion: SchemaVersion, Generation: generation, State: StateInstalling, Repository: r.Store.Repository(), Source: source, Host: host, Schedules: schedules}
		startedTransition := !request.Uninstall || prior != nil
		if startedTransition {
			if err = transaction.Publish(transition); err != nil {
				return r.fail(transaction, request, result, prior, plan, false, transition, err, started)
			}
		}
		wrote := false
		if plan.changed {
			wrote = true
			if err = r.install(ctx, plan.before.Profile, plan.after.Bytes); err != nil {
				return r.fail(transaction, request, result, prior, plan, wrote, transition, err, started)
			}
		}
		verified, verifyErr := r.read(ctx)
		if verifyErr != nil || !bytes.Equal(verified.after.Bytes, plan.after.Bytes) || verified.after.Profile != plan.after.Profile {
			err = errors.Join(errors.New("maintenance cron read-back verification failed"), verifyErr)
			return r.fail(transaction, request, result, prior, plan, wrote, transition, err, started)
		}
		if _, err = maintenancecron.Admit(verified.after, requirements); err != nil {
			return r.fail(transaction, request, result, prior, plan, wrote, transition, err, started)
		}
		if request.Uninstall {
			err = transaction.Restore(ctx, nil)
		} else {
			transition.State = StateReady
			err = transaction.Publish(transition)
		}
		if err != nil {
			return r.fail(transaction, request, result, prior, plan, wrote, transition, err, started)
		}
		if err = r.append(ctx, request, result, audit.OutcomeSucceeded, "", started); err != nil {
			return r.fail(transaction, request, result, prior, plan, wrote, transition, err, started)
		}
		return nil
	})
	if err != nil && !FailureOwned(err) {
		err = r.report(request, result, err, started)
	}
	return result, err
}

func matches(manifest InstallationManifest, document jobs.Document, host HostBinding, schedules []ScheduleFingerprint) bool {
	return manifest.State == StateReady && manifest.Source == document.Source && manifest.Host == host && reflect.DeepEqual(manifest.Schedules, schedules)
}

func (r Reconciler) stablePlan(ctx context.Context, hash string, desired []maintenancecron.CronEntry, requirements []maintenancecron.EntryRequirement) (observedPlan, error) {
	for range tableRechecks {
		first, err := r.read(ctx)
		if err != nil {
			return observedPlan{}, err
		}
		result, err := maintenancecron.Reconcile(first.after.Bytes, hash, desired)
		if err != nil {
			return observedPlan{}, err
		}
		after := first.after
		if result.Changed {
			after.Bytes = result.Replacement
		}
		if _, err := maintenancecron.Admit(after, requirements); err != nil {
			return observedPlan{}, err
		}
		second, err := r.read(ctx)
		if err != nil {
			return observedPlan{}, err
		}
		if first.present == second.present && reflect.DeepEqual(first.after, second.after) {
			return observedPlan{before: first.after, after: after, present: first.present, changed: result.Changed}, nil
		}
	}
	return observedPlan{}, errors.New("maintenance cron table changed during bounded admission")
}

func (r Reconciler) read(ctx context.Context) (observedPlan, error) {
	call, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()
	table, err := r.Cron.Read(call)
	if errors.Is(err, maintenancecron.ErrNoUserTable) {
		return observedPlan{after: table}, nil
	}
	return observedPlan{after: table, present: err == nil}, err
}

func (r Reconciler) install(ctx context.Context, profile maintenancecron.Profile, table []byte) error {
	call, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()
	return r.Cron.Install(call, profile, table)
}

func (r Reconciler) fail(transaction *Transaction, request Request, result ReconcileResult, prior *InstallationManifest, plan observedPlan, wrote bool, unavailable InstallationManifest, cause error, started time.Time) error {
	withhold, stopWithhold := context.WithTimeout(context.Background(), providerTimeout)
	unavailable.State = StateUnavailable
	var withheld error
	if prior != nil || !request.Uninstall {
		withheld = transaction.Restore(withhold, &unavailable)
	}
	stopWithhold()
	timeout := r.recoveryTimeout
	if timeout <= 0 {
		timeout = providerTimeout
	}
	recovery, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var rollback error
	if wrote {
		if !plan.present {
			rollback = errors.New("prior crontab absence cannot be restored by installing an empty table")
		} else if err := r.Cron.Install(recovery, plan.before.Profile, plan.before.Bytes); err != nil {
			rollback = err
		} else if restored, err := r.Cron.Read(recovery); err != nil || !bytes.Equal(restored.Bytes, plan.before.Bytes) || restored.Profile != plan.before.Profile {
			rollback = errors.Join(errors.New("restored crontab did not verify"), err)
		}
	}
	if rollback == nil && prior != nil {
		rollback = r.verifyPrior(recovery, plan, *prior)
	}
	if rollback == nil {
		rollback = transaction.Restore(recovery, prior)
	}
	if rollback != nil && (prior != nil || !request.Uninstall) {
		withhold, stopWithhold := context.WithTimeout(context.Background(), providerTimeout)
		rollback = errors.Join(rollback, transaction.Restore(withhold, &unavailable))
		stopWithhold()
	}
	failed := errors.Join(cause, withheld, rollback)
	if err := r.append(recovery, request, result, audit.OutcomeFailed, failed.Error(), started); err != nil {
		return &reconciliationFailure{error: fmt.Errorf("maintenance installation failed (backup %q): %w", result.Backup, errors.Join(failed, fmt.Errorf("append maintenance failure audit: %w", err)))}
	}
	return &reconciliationFailure{error: fmt.Errorf("maintenance installation failed (backup %q): %w", result.Backup, failed), audited: true}
}

func (r Reconciler) verifyPrior(ctx context.Context, plan observedPlan, prior InstallationManifest) error {
	observation, err := r.read(ctx)
	restored := observation.after
	if err != nil || observation.present != plan.present || !bytes.Equal(restored.Bytes, plan.before.Bytes) || restored.Profile != plan.before.Profile {
		return errors.Join(errors.New("restored crontab did not verify"), err)
	}
	owned, err := maintenancecron.OwnedEntries(restored.Bytes)
	if err != nil {
		return err
	}
	wanted := make(map[string]string, len(prior.Schedules))
	for _, schedule := range prior.Schedules {
		wanted[schedule.JobID] = schedule.SHA256
	}
	requirements := make([]maintenancecron.EntryRequirement, 0, len(wanted))
	for _, entry := range owned {
		if entry.RepositoryHash != prior.Repository.SHA256 {
			continue
		}
		if wanted[entry.JobID] != scheduleFingerprint(entry.JobID, entry.Cron, restored.HostTimezone).SHA256 {
			return fmt.Errorf("%w: restored entry %q does not match the prior schedule/timezone binding", maintenancecron.ErrAdmissionRefused, entry.JobID)
		}
		delete(wanted, entry.JobID)
		requirements = append(requirements, maintenancecron.EntryRequirement{RepositoryHash: entry.RepositoryHash, JobID: entry.JobID, Timezone: restored.HostTimezone})
	}
	if len(wanted) != 0 {
		return errors.New("restored crontab omits authority required by the prior manifest")
	}
	_, err = maintenancecron.Admit(restored, requirements)
	return err
}

func (r Reconciler) append(ctx context.Context, request Request, result ReconcileResult, outcome audit.Outcome, cause string, started time.Time) error {
	changed, owned, elapsed := result.Changed, result.Owned, time.Since(started).Milliseconds()
	level := audit.LevelInfo
	if outcome == audit.OutcomeFailed {
		level = audit.LevelError
	}
	references := []audit.Reference{}
	if result.Backup != "" {
		references = append(references, audit.Reference{Kind: "prior_state_backup", Location: result.Backup})
	}
	causeCode := "install_" + string(outcome)
	if request.Uninstall {
		causeCode = "uninstall_" + string(outcome)
	}
	_, err := r.Journal.Append(ctx, audit.Event{Level: level, Event: audit.EventInstallation, Phase: audit.PhaseInstallation, TraceID: request.Identity.TraceID, InvocationID: request.Identity.InvocationID, InstallationGeneration: string(result.Generation), Outcome: outcome, Changed: &changed, OwnedEntries: &owned, ElapsedMS: &elapsed, CauseCode: causeCode, Cause: cause, References: references}, audit.Consequential)
	return err
}

func (r Reconciler) report(request Request, result ReconcileResult, cause error, started time.Time) error {
	recovery, cancel := context.WithTimeout(context.Background(), providerTimeout)
	defer cancel()
	if err := r.append(recovery, request, result, audit.OutcomeFailed, cause.Error(), started); err != nil {
		return &reconciliationFailure{error: errors.Join(cause, fmt.Errorf("append maintenance failure audit: %w", err))}
	}
	return &reconciliationFailure{error: cause, audited: true}
}
