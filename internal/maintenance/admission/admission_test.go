package admission

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/claude"
	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/codex"
	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/cursor"
	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/opencode"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig/harnessconfigtest"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/installation"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

type fakeAuthority struct {
	ready       []installation.ReadyBinding
	reads       int
	validations int
	replace     int
	err         error
	validateErr error
}

func (a *fakeAuthority) Ready(context.Context) (installation.ReadyBinding, error) {
	if a.err != nil {
		return installation.ReadyBinding{}, a.err
	}
	index := a.reads
	if index >= len(a.ready) {
		index = len(a.ready) - 1
	}
	a.reads++
	return a.ready[index], nil
}

func (a *fakeAuthority) ValidateReady(context.Context, installation.ReadyBinding) error {
	a.validations++
	if a.validations <= a.replace {
		return installation.ErrGenerationReplaced
	}
	return a.validateErr
}

func (a *fakeAuthority) GuardReady(ctx context.Context, ready installation.ReadyBinding, run func() error) error {
	if err := a.ValidateReady(ctx, ready); err != nil {
		return err
	}
	return run()
}

type fakeControl struct{ closed int }

func (*fakeControl) RefStore(string) (*gitrefstore.Store, error) { return nil, errors.New("unused") }
func (c *fakeControl) Close() error                              { c.closed++; return nil }

type fakePublisher struct {
	documents []jobs.Document
	configs   [][]byte
	rawJobs   [][]byte
	controls  []*fakeControl
	reads     int
	err       error
}

func (p *fakePublisher) Capture(_ context.Context, _ jobs.SourceBinding) (Published, error) {
	if p.err != nil {
		return Published{}, p.err
	}
	index := p.reads
	p.reads++
	var data []byte
	if index < len(p.rawJobs) && p.rawJobs[index] != nil {
		data = p.rawJobs[index]
	} else {
		var err error
		data, err = jobs.Encode(p.documents[index])
		if err != nil {
			return Published{}, err
		}
	}
	control := &fakeControl{}
	p.controls = append(p.controls, control)
	return Published{Commit: fmt.Sprintf("commit-%d", index), Jobs: data, Config: p.configs[index], Control: control}, nil
}

func testJob(harness entity.Harness) jobs.Job {
	return jobs.Job{ID: "daily", Enabled: true, Schedule: jobs.Schedule{Cron: "0 2 * * *", Timezone: "America/Los_Angeles"}, Timeout: "90m", Prompt: "published prompt",
		Runner: jobs.Runner{Harness: harness, Routing: executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum, Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical}}}
}

func testDocument(job jobs.Job) jobs.Document {
	return jobs.Document{SchemaVersion: jobs.SchemaVersion, Source: jobs.SourceBinding{Remote: "origin", Branch: "main"}, Jobs: []jobs.Job{job}}
}

func testReady(t *testing.T, document jobs.Document, generation byte) installation.ReadyBinding {
	t.Helper()
	schedules, err := installation.ScheduleFingerprints(document)
	if err != nil {
		t.Fatal(err)
	}
	return installation.ReadyBinding{Generation: installation.InstallationGeneration(strings.Repeat(fmt.Sprintf("%x", generation), 32)), Source: document.Source, Schedules: schedules}
}

func testService(t *testing.T, documents ...jobs.Document) (Service, *fakeAuthority, *fakePublisher) {
	t.Helper()
	ready := make([]installation.ReadyBinding, len(documents))
	configs := make([][]byte, len(documents))
	for i, document := range documents {
		ready[i] = testReady(t, document, byte(i+1))
		configs[i] = []byte(harnessconfigtest.YAML)
	}
	authority := &fakeAuthority{ready: ready}
	publisher := &fakePublisher{documents: documents, configs: configs}
	return Service{Authority: authority, Publisher: publisher, Registry: runtime.LiveRegistry(), ProcessSupport: func(context.Context) error { return nil }, Authenticate: func(context.Context, port.AgentRuntime) error { return nil }}, authority, publisher
}

func TestPublishedCatalogRoutesAllHarnessesAndFreezesCapturedDate(t *testing.T) {
	starts := []struct {
		name, value, want string
	}{
		{"utc previous date", "2026-10-20T06:59:00Z", "2026-10-19"},
		{"utc boundary", "2026-10-20T07:00:00Z", "2026-10-20"},
		{"same date delayed", "2026-10-21T06:59:00Z", "2026-10-20"},
		{"dst first 0130", "2026-11-01T08:30:00Z", "2026-11-01"},
		{"dst repeated 0130", "2026-11-01T09:30:00Z", "2026-11-01"},
		{"next date", "2026-11-02T09:30:00Z", "2026-11-02"},
	}
	for _, harness := range entity.Harnesses() {
		for _, fixture := range starts {
			t.Run(harness.String()+"/"+fixture.name, func(t *testing.T) {
				document := testDocument(testJob(harness))
				service, authority, publisher := testService(t, document)
				start, err := time.Parse(time.RFC3339, fixture.value)
				if err != nil {
					t.Fatal(err)
				}
				result, err := service.Prepare(t.Context(), Request{JobID: "daily", StartedUTC: start})
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = result.Prepared.Close() }() // Fixture-owned private control cleanup.
				if result.Kind != ResultPrepared || result.Prepared.JobDate != fixture.want || result.Prepared.StartedUTC != start || result.Prepared.DeadlineUTC != start.Add(90*time.Minute) {
					t.Fatalf("prepared = %+v", result.Prepared)
				}
				if result.Prepared.DailyIdentity != (DailyIdentity{JobID: "daily", JobDate: fixture.want}) {
					t.Fatalf("daily identity = %+v", result.Prepared.DailyIdentity)
				}
				if result.Prepared.Selection.Harness != harness || result.Prepared.ConfigCommit != "commit-0" || publisher.reads != 1 || authority.validations != 1 {
					t.Fatalf("selection/read proof = %+v reads=%d validations=%d", result.Prepared.Selection, publisher.reads, authority.validations)
				}
				runtime, native, err := result.Prepared.ExecutionRoute()
				if err != nil || runtime == nil || native.NativeModel != result.Prepared.Selection.NativeModel || native.NativeEffort != result.Prepared.Selection.NativeEffort || !result.Prepared.Route.Capture.MatchesSelected(result.Prepared.Selection) {
					t.Fatalf("execution route runtime=%T native=%+v capture=%+v err=%v", runtime, native, result.Prepared.Route.Capture, err)
				}
				if err := result.Prepared.ValidateReady(t.Context()); err != nil || authority.validations != 2 {
					t.Fatalf("claim-time ready seam = %v validations=%d", err, authority.validations)
				}
				guarded := false
				if err := result.Prepared.GuardReady(t.Context(), func() error { guarded = true; return nil }); err != nil || !guarded || authority.validations != 3 {
					t.Fatalf("guarded ready seam = %v guarded=%v validations=%d", err, guarded, authority.validations)
				}
			})
		}
	}
}

func TestDisabledAndRemovedJobsSkipBeforeCatalogOrAuthentication(t *testing.T) {
	for _, test := range []struct {
		name string
		doc  jobs.Document
		want SkipReason
	}{
		{"disabled", func() jobs.Document { job := testJob(entity.Codex); job.Enabled = false; return testDocument(job) }(), SkipDisabled},
		{"removed", jobs.Document{SchemaVersion: jobs.SchemaVersion, Source: jobs.SourceBinding{Remote: "origin", Branch: "main"}, Jobs: []jobs.Job{}}, SkipRemoved},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, _, publisher := testService(t, test.doc)
			publisher.configs[0] = []byte("harness-config: [malformed")
			result, err := service.Prepare(t.Context(), Request{JobID: "daily", StartedUTC: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)})
			if err != nil || result.Kind != ResultSkipped || result.SkipReason != test.want || publisher.controls[0].closed != 1 {
				t.Fatalf("result=%+v err=%v closed=%d", result, err, publisher.controls[0].closed)
			}
		})
	}
}

func TestGenerationReplacementGetsOneFreshPublishedRead(t *testing.T) {
	first, second := testDocument(testJob(entity.Codex)), testDocument(testJob(entity.Codex))
	second.Jobs[0].Prompt = "replacement prompt"
	service, authority, publisher := testService(t, first, second)
	authority.replace = 1
	result, err := service.Prepare(t.Context(), Request{JobID: "daily", StartedUTC: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = result.Prepared.Close() }() // Fixture-owned private control cleanup.
	if publisher.reads != 2 || authority.reads != 2 || result.Prepared.Job.Prompt != "replacement prompt" || result.Prepared.ConfigCommit != "commit-1" || publisher.controls[0].closed != 1 {
		t.Fatalf("reads=%d authority=%d prepared=%+v firstClosed=%d", publisher.reads, authority.reads, result.Prepared, publisher.controls[0].closed)
	}
	authority.replace = 3
	service, authority, publisher = testService(t, first, second)
	authority.replace = 3
	refused, err := service.Prepare(t.Context(), Request{JobID: "daily", StartedUTC: time.Now()})
	if !errors.Is(err, installation.ErrGenerationReplaced) || publisher.reads != maxPublicationReads || refused.Prepared != nil {
		t.Fatalf("bounded replacement result=%+v err=%v reads=%d", refused, err, publisher.reads)
	}
	service, authority, _ = testService(t, first)
	authority.validateErr = errors.New("reproof unavailable")
	refused, err = service.Prepare(t.Context(), Request{JobID: "daily", StartedUTC: time.Now()})
	if err == nil || !strings.Contains(err.Error(), "reproof unavailable") || refused.Prepared != nil || refused.Selection == nil {
		t.Fatalf("reproof refusal result=%+v err=%v", refused, err)
	}
}

func TestAdmissionRefusesSourceScheduleDeliveryRouteAuthAndProcessDefects(t *testing.T) {
	base := testDocument(testJob(entity.Codex))
	baselineService, _, _ := testService(t, base)
	baseline, err := baselineService.Prepare(t.Context(), Request{JobID: "daily", StartedUTC: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	expectedSelection := *baseline.Selection
	_ = baseline.Prepared.Close() // Fixture-owned private control cleanup.
	weakCatalog := []byte(`harness-config:
  codex:
    implementation:
      economy:
        model: gpt-5.6-luna
        effort: low
        capability: economy
        deliberation: light
        context: standard
        assurance: ordinary
        cost_rank: 1
        strongest: true
`)
	for _, test := range []struct {
		name         string
		edit         func(*Service, *fakeAuthority, *fakePublisher)
		want         string
		wantIdentity bool
		wantRoute    bool
	}{
		{"malformed policy", func(_ *Service, _ *fakeAuthority, p *fakePublisher) {
			p.rawJobs = [][]byte{[]byte("schema_version: [")}
		}, "invalid maintenance jobs document", false, false},
		{"source drift", func(_ *Service, _ *fakeAuthority, p *fakePublisher) { p.documents[0].Source.Branch = "other" }, "source drift", false, false},
		{"schedule drift", func(_ *Service, a *fakeAuthority, _ *fakePublisher) {
			changed := base
			changed.Jobs = append([]jobs.Job(nil), base.Jobs...)
			changed.Jobs[0].Schedule.Cron = "1 2 * * *"
			a.ready[0] = testReady(t, changed, 1)
		}, "schedule drift", false, false},
		{"unsupported delivery", func(_ *Service, _ *fakeAuthority, p *fakePublisher) {
			p.rawJobs = [][]byte{[]byte(`schema_version: pyxis-maintenance-jobs/v1
source:
  remote: origin
  branch: main
jobs:
  - id: daily
    enabled: true
    schedule:
      cron: 0 2 * * *
      timezone: America/Los_Angeles
    timeout: 90m
    runner:
      harness: codex
      routing:
        capability: frontier
        deliberation: maximum
        context: extended
        assurance: critical
    delivery:
      mode: merge
      branch: maintenance/daily
    prompt: published prompt
`)}
		}, "invalid delivery", false, false},
		{"malformed catalog", func(_ *Service, _ *fakeAuthority, p *fakePublisher) {
			p.configs[0] = []byte("harness-config: [")
		}, "parse published harness catalog", true, false},
		{"unsupported route", func(_ *Service, _ *fakeAuthority, p *fakePublisher) { p.configs[0] = weakCatalog }, "route unsupported", true, false},
		{"authentication", func(s *Service, _ *fakeAuthority, _ *fakePublisher) {
			s.Authenticate = func(context.Context, port.AgentRuntime) error { return errors.New("auth unavailable") }
		}, "auth unavailable", true, true},
		{"process support", func(s *Service, _ *fakeAuthority, _ *fakePublisher) {
			s.ProcessSupport = func(context.Context) error { return errors.New("process unsupported") }
		}, "process unsupported", true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, authority, publisher := testService(t, base)
			test.edit(&service, authority, publisher)
			result, err := service.Prepare(t.Context(), Request{JobID: "daily", StartedUTC: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)})
			if err == nil || !strings.Contains(err.Error(), test.want) || publisher.controls[0].closed != 1 {
				t.Fatalf("err=%v closed=%d", err, publisher.controls[0].closed)
			}
			if test.wantIdentity && (result.JobDate != "2026-10-20" || result.Timezone != "America/Los_Angeles" || result.DailyIdentity.JobID != "daily") {
				t.Fatalf("refusal identity = %+v", result)
			}
			if result.Prepared != nil {
				t.Fatalf("refusal exposed Prepared authority: %+v", result.Prepared)
			}
			if test.wantRoute {
				if result.Selection == nil || !reflect.DeepEqual(*result.Selection, expectedSelection) {
					t.Fatalf("refusal selection = %+v, want %+v", result.Selection, expectedSelection)
				}
			} else if result.Selection != nil {
				t.Fatalf("pre-selection refusal fabricated route: %+v", result.Selection)
			}
		})
	}
}

func TestAdmissionDependencyAndValueBoundaries(t *testing.T) {
	if _, _, err := (Prepared{}).ExecutionRoute(); err == nil {
		t.Fatal("empty prepared execution route succeeded")
	}
	request := Request{JobID: "daily", StartedUTC: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)}
	if _, err := (Service{}).Prepare(t.Context(), request); err == nil || !strings.Contains(err.Error(), "requires authority") {
		t.Fatalf("empty service error = %v", err)
	}
	document := testDocument(testJob(entity.Codex))
	service, authority, publisher := testService(t, document)
	authority.err = errors.New("ready unavailable")
	if _, err := service.Prepare(t.Context(), request); err == nil || !strings.Contains(err.Error(), "ready unavailable") {
		t.Fatalf("ready error = %v", err)
	}
	authority.err = nil
	publisher.err = errors.New("publication unavailable")
	if _, err := service.Prepare(t.Context(), request); err == nil || !strings.Contains(err.Error(), "publication unavailable") {
		t.Fatalf("publication error = %v", err)
	}

	if err := (Prepared{}).ValidateReady(t.Context()); !errors.Is(err, installation.ErrInstallationRequired) {
		t.Fatalf("empty prepared validation = %v", err)
	}
	if err := (Prepared{}).GuardReady(t.Context(), func() error { return nil }); !errors.Is(err, installation.ErrInstallationRequired) {
		t.Fatalf("empty prepared guard = %v", err)
	}
	if err := (Prepared{}).Close(); err != nil {
		t.Fatalf("empty prepared close = %v", err)
	}
	locked := LockedAuthority{}
	if _, err := locked.Ready(t.Context()); err == nil {
		t.Fatal("authority without store read succeeded")
	}
	if err := locked.ValidateReady(t.Context(), installation.ReadyBinding{}); err == nil {
		t.Fatal("authority without store validation succeeded")
	}
	if err := locked.GuardReady(t.Context(), installation.ReadyBinding{}, func() error { return nil }); err == nil {
		t.Fatal("authority without store guard succeeded")
	}
}
