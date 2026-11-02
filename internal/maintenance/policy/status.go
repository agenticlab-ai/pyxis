// Package policy provides read-only inspection of desired maintenance policy.
package policy

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/audit"
	maintenancecron "github.com/agenticlab-ai/pyxis/internal/maintenance/cron"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/installation"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

// Status observes desired policy and already-persisted local evidence. It never
// opens a writer, transaction, remote, or native runtime.
func Status(ctx context.Context, root string, cron maintenancecron.Port) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	jobStore, err := jobs.New(root)
	if err != nil {
		return "", fmt.Errorf("open desired policy: %w", err)
	}
	document, err := jobStore.Load(ctx)
	if err != nil {
		return "", fmt.Errorf("load desired policy: %w", err)
	}
	var output strings.Builder
	_, _ = fmt.Fprintf(&output, "repository: %q\ndesired policy: valid %q\nsource: remote=%q branch=%q\ndesired jobs: %d\n", jobStore.Root(), document.SchemaVersion, document.Source.Remote, document.Source.Branch, len(document.Jobs))
	for _, job := range document.Jobs {
		_, _ = fmt.Fprintf(&output, "job: id=%q enabled=%t cron=%q timezone=%q timeout=%q harness=%q capability=%q deliberation=%q context=%q assurance=%q", job.ID, job.Enabled, job.Schedule.Cron, job.Schedule.Timezone, job.Timeout, job.Runner.Harness, job.Runner.Routing.Capability, job.Runner.Routing.Deliberation, job.Runner.Routing.Context, job.Runner.Routing.Assurance)
		if job.Delivery == nil {
			_, _ = output.WriteString(" delivery=none")
		} else {
			_, _ = fmt.Fprintf(&output, " delivery_mode=%q delivery_branch=%q", job.Delivery.Mode, job.Delivery.Branch)
		}
		_, _ = output.WriteString(" prompt=configured\n")
	}
	store, openErr := installation.Open(jobStore.Root())
	manifest := installationObservation(ctx, store, openErr, document, &output)
	cronObservation(ctx, jobStore.Root(), cron, document, manifest, &output)
	journalObservation(ctx, jobStore.Root(), document, &output)
	refusalObservation(ctx, store, openErr, &output)
	_, _ = output.WriteString("shared outcome observation: unavailable (no mutation-free registered claim reader)\n")
	return output.String(), nil
}

func installationObservation(ctx context.Context, store *installation.Store, openErr error, document jobs.Document, output *strings.Builder) installation.InstallationManifest {
	if openErr != nil {
		_, _ = output.WriteString("installation observation: unavailable (state location invalid)\n")
		return installation.InstallationManifest{}
	}
	manifest, err := store.Load(ctx)
	if errors.Is(err, installation.ErrInstallationRequired) {
		_, _ = output.WriteString("installation observation: absent\nmanifest drift: unavailable (manifest absent)\n")
		return installation.InstallationManifest{}
	}
	if err != nil {
		_, _ = output.WriteString("installation observation: unavailable (manifest invalid or unreadable)\nmanifest drift: unavailable\n")
		return installation.InstallationManifest{}
	}
	want, _ := installation.ScheduleFingerprints(document)
	source, schedule := manifest.Source == document.Source, reflect.DeepEqual(manifest.Schedules, want)
	_, _ = fmt.Fprintf(output, "installation observation: present state=%q generation=%q\nmanifest drift: source=%t schedule=%t ready=%t\n", manifest.State, manifest.Generation, !source, !schedule, manifest.State == installation.StateReady)
	return manifest
}

func cronObservation(ctx context.Context, root string, port maintenancecron.Port, document jobs.Document, manifest installation.InstallationManifest, output *strings.Builder) {
	if port == nil {
		_, _ = output.WriteString("cron observation: unavailable (provider absent)\n")
		return
	}
	table, err := port.Read(ctx)
	if errors.Is(err, maintenancecron.ErrNoUserTable) {
		_, _ = output.WriteString("cron observation: absent; owned entries: 0\n")
		return
	}
	if err != nil {
		_, _ = output.WriteString("cron observation: unavailable (provider read failed)\n")
		return
	}
	owned, err := maintenancecron.OwnedEntries(table.Bytes)
	if err != nil {
		_, _ = output.WriteString("cron observation: malformed owned entries\n")
		return
	}
	hash, _ := maintenancecron.RepositoryHash(root)
	count := 0
	for _, entry := range owned {
		if entry.RepositoryHash == hash {
			count++
			_, _ = fmt.Fprintf(output, "owned cron: job=%q cron=%q\n", entry.JobID, entry.Cron)
		}
	}
	drift := "unavailable (manifest absent or invalid)"
	if manifest.SchemaVersion == installation.SchemaVersion {
		desired, renderErr := maintenancecron.Render(root, maintenancecron.InstalledBinding{Executable: manifest.Host.ExecutablePath, ToolPath: manifest.Host.ToolSearchPath, StableExecutable: true}, document.Jobs)
		if comparison, compareErr := maintenancecron.Reconcile(table.Bytes, hash, desired); renderErr == nil && compareErr == nil {
			drift = fmt.Sprintf("%t", comparison.Changed)
		}
	}
	_, _ = fmt.Fprintf(output, "cron observation: present; owned entries: %d; drift: %s\n", count, drift)
}

func journalObservation(ctx context.Context, root string, document jobs.Document, output *strings.Builder) {
	inspection, err := audit.Inspect(ctx, root)
	renderJournalObservation(document, inspection, err, output)
}

func renderJournalObservation(document jobs.Document, inspection audit.Inspection, err error, output *strings.Builder) {
	health := "readable"
	if inspection.Missing {
		health = "missing"
	}
	if err != nil {
		health = "unavailable (reader error)"
	}
	_, _ = fmt.Fprintf(output, "journal health: %s malformed_complete=%d incomplete_tail=%t\n", health, inspection.Malformed, inspection.IncompleteTail)
	for _, job := range document.Jobs {
		var latest, activity, decision *audit.Event
		for index := range inspection.Events {
			event := &inspection.Events[index]
			if event.JobID != job.ID {
				continue
			}
			for _, reference := range event.References {
				if reference.Kind == "retained_workspace" && strings.HasPrefix(reference.Location, "maintenance-attempt:") {
					_, _ = fmt.Fprintf(output, "retained work: job=%q kind=%q reference=%q\n", job.ID, reference.Kind, reference.Location)
				}
			}
			// These observations do not start a new local execution attempt.
			if event.Phase != audit.PhaseTerminal && (event.Outcome == audit.OutcomeSkipped || event.Outcome == audit.OutcomeRefused || event.Outcome == audit.OutcomeLost) {
				decision = event
				continue
			}
			activity = event
			if event.Event == audit.EventRun && event.Phase == audit.PhaseTerminal {
				latest = event
			}
		}
		if decision != nil {
			_, _ = fmt.Fprintf(output, "decision: job=%q outcome=%q cause=%q at=%q\n", job.ID, decision.Outcome, decision.CauseCode, decision.Timestamp)
		}
		if latest != nil && latest.InvocationID == activity.InvocationID {
			_, _ = fmt.Fprintf(output, "attempt: job=%q outcome=%q phase=%q cause=%q at=%q\n", job.ID, latest.Outcome, latest.Phase, latest.CauseCode, latest.Timestamp)
		} else if activity != nil {
			_, _ = fmt.Fprintf(output, "attempt: job=%q outcome=incomplete/unproved phase=%q cause=%q at=%q\n", job.ID, activity.Phase, activity.CauseCode, activity.Timestamp)
		} else if err != nil || inspection.Missing || inspection.Malformed != 0 || inspection.IncompleteTail {
			_, _ = fmt.Fprintf(output, "attempt: job=%q outcome=unavailable (journal incomplete)\n", job.ID)
		} else {
			_, _ = fmt.Fprintf(output, "attempt: job=%q outcome=no known attempt\n", job.ID)
		}
	}
}

func refusalObservation(ctx context.Context, store *installation.Store, openErr error, output *strings.Builder) {
	if openErr != nil {
		_, _ = output.WriteString("refusal counters: unavailable\n")
		return
	}
	state, err := store.LoadRefusals(ctx)
	if errors.Is(err, installation.ErrRefusalsAbsent) {
		_, _ = output.WriteString("refusal counters: absent\n")
		return
	}
	if err != nil {
		_, _ = output.WriteString("refusal counters: unavailable (invalid or unreadable)\n")
		return
	}
	_, _ = fmt.Fprintf(output, "refusal counters: present entries=%d\n", len(state.Counters))
	for _, counter := range state.Counters {
		_, _ = fmt.Fprintf(output, "refusal: job=%q cause=%q lifetime=%d consecutive=%d first=%q last=%q\n", counter.JobID, counter.Cause, counter.Lifetime, counter.Consecutive, counter.FirstObserved, counter.LastObserved)
	}
}
