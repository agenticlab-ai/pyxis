package admission

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/installation"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type nativeRuntime interface {
	port.AgentRuntime
	port.CatalogProvider
	port.LaunchBuilder
	port.NativeLaunchCapturer
	nativerouting.NativeCapability
}

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

func (s Service) Prepare(ctx context.Context, request Request) (Result, error) {
	if s.Authority == nil || s.Publisher == nil || s.Registry == nil || s.ProcessSupport == nil || request.JobID == "" || request.StartedUTC.IsZero() {
		return Result{}, errors.New("maintenance admission requires authority, publisher, registry, process support, job, and captured start")
	}
	request.StartedUTC = request.StartedUTC.UTC()
	for read := range maxPublicationReads {
		ready, err := s.Authority.Ready(ctx)
		if err != nil {
			return Result{}, fmt.Errorf("read ready installation: %w", err)
		}
		result, err := s.prepareOnce(ctx, request, ready)
		if err != nil {
			return result, err
		}
		if err := s.Authority.ValidateReady(ctx, ready); err != nil {
			if result.Prepared != nil {
				_ = result.Prepared.Close() // The generation refusal governs even if private cleanup also fails.
				result.Prepared = nil
			}
			if errors.Is(err, installation.ErrGenerationReplaced) && read+1 < maxPublicationReads {
				continue
			}
			return result, fmt.Errorf("reprove ready installation: %w", err)
		}
		return result, nil
	}
	return Result{}, installation.ErrGenerationReplaced
}

func (s Service) prepareOnce(ctx context.Context, request Request, ready installation.ReadyBinding) (Result, error) {
	result := Result{JobID: request.JobID, Ready: ready}
	published, err := s.Publisher.Capture(ctx, ready.Source)
	if err != nil {
		return result, fmt.Errorf("capture published maintenance policy: %w", err)
	}
	result.ConfigCommit = published.Commit
	keep := false
	defer func() {
		if !keep && published.Control != nil {
			_ = published.Control.Close() // Preserve the admission error; this handle owns only its private temporary directory.
		}
	}()
	document, err := jobs.Decode(published.Jobs)
	if err != nil {
		return result, fmt.Errorf("decode published maintenance policy: %w", err)
	}
	if document.Source != ready.Source {
		return result, fmt.Errorf("published source drift: got %s/%s, installed %s/%s", document.Source.Remote, document.Source.Branch, ready.Source.Remote, ready.Source.Branch)
	}
	job, found := findJob(document.Jobs, request.JobID)
	if !found || !job.Enabled {
		reason := SkipRemoved
		if found {
			reason = SkipDisabled
		}
		result.Kind, result.SkipReason = ResultSkipped, reason
		return result, nil
	}
	if err := matchingSchedule(document, ready, job.ID); err != nil {
		return result, err
	}
	// jobs.Decode already validated both values, so these conversions cannot fail.
	location, _ := time.LoadLocation(job.Schedule.Timezone)
	duration, _ := time.ParseDuration(job.Timeout)
	result.JobDate, result.Timezone = request.StartedUTC.In(location).Format(time.DateOnly), job.Schedule.Timezone
	result.DailyIdentity = DailyIdentity{JobID: job.ID, JobDate: result.JobDate}
	configured, err := harnessconfig.Parse(published.Config)
	if err != nil {
		return result, fmt.Errorf("parse published harness catalog: %w", err)
	}
	runtime, err := s.Registry.ResolveConfigured(job.Runner.Harness, configured.Profiles(job.Runner.Harness))
	if err != nil {
		return result, fmt.Errorf("resolve configured %s runtime: %w", job.Runner.Harness, err)
	}
	catalog, selection, nativeLaunch, launchCapture, err := resolveRoute(request, job, runtime)
	if err != nil {
		return result, err
	}
	result.Selection = &selection
	authenticate := s.Authenticate
	if authenticate == nil {
		authenticate = func(ctx context.Context, runtime port.AgentRuntime) error { return runtime.AuthCheck(ctx) }
	}
	if err := authenticate(ctx, runtime); err != nil {
		return result, fmt.Errorf("authenticate %s runtime: %w", job.Runner.Harness, err)
	}
	if err := s.ProcessSupport(ctx); err != nil {
		return result, fmt.Errorf("maintenance process support: %w", err)
	}
	prepared := &Prepared{ConfigCommit: published.Commit, Job: job, Catalog: catalog, Selection: selection, Ready: ready,
		DailyIdentity: result.DailyIdentity, StartedUTC: request.StartedUTC, JobDate: result.JobDate,
		DeadlineUTC: request.StartedUTC.Add(duration), Control: published.Control, authority: s.Authority,
		Route: RuntimeRoute{Runtime: runtime, Launch: nativeLaunch, Capture: launchCapture}}
	keep = true
	result.Kind, result.Prepared = ResultPrepared, prepared
	return result, nil
}

func findJob(all []jobs.Job, id string) (jobs.Job, bool) {
	for _, job := range all {
		if job.ID == id {
			return job, true
		}
	}
	return jobs.Job{}, false
}

func matchingSchedule(document jobs.Document, ready installation.ReadyBinding, jobID string) error {
	want, err := installation.ScheduleFingerprints(document)
	if err != nil {
		return err
	}
	for _, fingerprint := range want {
		if fingerprint.JobID == jobID && slices.Contains(ready.Schedules, fingerprint) {
			return nil
		}
	}
	return fmt.Errorf("published schedule drift for job %q; reinstall maintenance jobs", jobID)
}

func resolveRoute(request Request, job jobs.Job, runtime port.AgentRuntime) (nativerouting.ValidatedCatalog, nativerouting.SelectedProfile, port.NativeLaunchSpec, port.LaunchCapture, error) {
	native, ok := runtime.(nativeRuntime)
	if !ok {
		return nativerouting.ValidatedCatalog{}, nativerouting.SelectedProfile{}, port.NativeLaunchSpec{}, port.LaunchCapture{}, errors.New("configured runtime lacks native catalog, validation, or launch fidelity support")
	}
	catalog, qualifications, err := native.Catalog(nativerouting.RoleImplementation)
	if err != nil {
		return nativerouting.ValidatedCatalog{}, nativerouting.SelectedProfile{}, port.NativeLaunchSpec{}, port.LaunchCapture{}, fmt.Errorf("read implementation catalog: %w", err)
	}
	validated, diagnostics := nativerouting.ValidateCatalog(catalog, qualifications, native, fixedClock{request.StartedUTC}, nativerouting.ReleasedCatalogAuthorization{})
	if len(diagnostics) != 0 {
		return nativerouting.ValidatedCatalog{}, nativerouting.SelectedProfile{}, port.NativeLaunchSpec{}, port.LaunchCapture{}, fmt.Errorf("validate implementation catalog: %v", diagnostics)
	}
	resolution := (nativerouting.Resolver{}).Resolve(nativerouting.Request{SchemaVersion: nativerouting.RequestSchema,
		AttemptID: "maintenance-" + job.ID + "-" + request.StartedUTC.Format("20060102T150405Z"), DemandID: executiondemand.DemandID("maintenance-" + job.ID),
		Harness: entity.Harness(job.Runner.Harness), Role: nativerouting.RoleImplementation, RequestedProfile: job.Runner.Routing, Mode: nativerouting.ModeNormal}, validated)
	if resolution.NativeProfile == nil {
		return nativerouting.ValidatedCatalog{}, nativerouting.SelectedProfile{}, port.NativeLaunchSpec{}, port.LaunchCapture{}, fmt.Errorf("maintenance route unsupported: class=%s reason=%s defects=%v unmet=%v", resolution.Class, resolution.Reason, resolution.PolicyDefects, resolution.UnmetComponents)
	}
	selected := *resolution.NativeProfile
	spec, err := native.Build(selected)
	if err != nil {
		return nativerouting.ValidatedCatalog{}, nativerouting.SelectedProfile{}, port.NativeLaunchSpec{}, port.LaunchCapture{}, fmt.Errorf("build selected native route: %w", err)
	}
	capture, err := native.CaptureNativeLaunch(spec)
	if err != nil {
		return nativerouting.ValidatedCatalog{}, nativerouting.SelectedProfile{}, port.NativeLaunchSpec{}, port.LaunchCapture{}, fmt.Errorf("capture selected native route: %w", err)
	}
	if !capture.MatchesSelected(selected) {
		return nativerouting.ValidatedCatalog{}, nativerouting.SelectedProfile{}, port.NativeLaunchSpec{}, port.LaunchCapture{}, errors.New("selected native route differs from adapter launch capture")
	}
	return validated, selected, spec, capture, nil
}
