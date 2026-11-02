package policy

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/audit"
	maintenancecron "github.com/agenticlab-ai/pyxis/internal/maintenance/cron"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/installation"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

func TestStatusRendersValidatedPolicyWithoutMutation(t *testing.T) {
	root := t.TempDir()
	document := jobs.Document{
		SchemaVersion: jobs.SchemaVersion, Source: jobs.SourceBinding{Remote: "origin", Branch: "main"},
		Jobs: []jobs.Job{{
			ID: "daily", Enabled: false, Prompt: "line\n\x1b[31mred", Timeout: "1h",
			Schedule: jobs.Schedule{Cron: "0 2 * * *", Timezone: "UTC"},
			Runner: jobs.Runner{Harness: entity.OpenCode, Routing: executiondemand.ProfileMinimum{
				Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
				Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
			}},
			Delivery: &jobs.Delivery{Mode: "draft-pr", Branch: "maintenance/daily"},
		}},
	}
	data, err := jobs.Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".pyxis/maintenance/jobs.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis/maintenance/logs.jsonl"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	output, err := Status(t.Context(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`id="daily" enabled=false`, `harness="opencode"`, `delivery_mode="draft-pr" delivery_branch="maintenance/daily"`, "prompt=configured", "installation observation: absent", "cron observation: unavailable", "journal health: readable", "attempt: job=\"daily\" outcome=no known attempt", "refusal counters: absent"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q in status:\n%s", want, output)
		}
	}
	if strings.ContainsRune(output, '\x1b') {
		t.Fatal("raw terminal control in status")
	}
	if after, err := os.ReadFile(path); err != nil || !bytes.Equal(after, data) {
		t.Fatalf("policy changed: %v", err)
	}
}

type statusCron struct {
	table    maintenancecron.Table
	err      error
	reads    int
	installs int
}

func (c *statusCron) Read(context.Context) (maintenancecron.Table, error) {
	c.reads++
	return c.table, c.err
}
func (c *statusCron) Install(context.Context, maintenancecron.Profile, []byte) error {
	c.installs++
	return nil
}

func TestStatusObservesDriftJournalOutcomesAndRetainedWorkWithoutWrites(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	ids := []string{"success", "failure", "timeout", "incomplete", "none"}
	document := jobs.Document{SchemaVersion: jobs.SchemaVersion, Source: jobs.SourceBinding{Remote: "origin", Branch: "main"}}
	for _, id := range ids {
		document.Jobs = append(document.Jobs, jobs.Job{ID: id, Enabled: true, Prompt: "private prompt " + id, Timeout: "1h", Schedule: jobs.Schedule{Cron: "0 2 * * *", Timezone: "UTC"}, Runner: jobs.Runner{Harness: entity.Codex, Routing: executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum, Context: executiondemand.ContextLarge, Assurance: executiondemand.AssuranceOrdinary}}})
	}
	data, _ := jobs.Encode(document)
	path := filepath.Join(root, ".pyxis/maintenance/jobs.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("/.pyxis/maintenance/logs.jsonl\n/.pyxis/state/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stateRoot := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateRoot)
	store, err := installation.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	schedules, _ := installation.ScheduleFingerprints(document)
	manifest := installation.InstallationManifest{SchemaVersion: installation.SchemaVersion, Generation: installation.InstallationGeneration(strings.Repeat("a", 32)), State: installation.StateReady, Repository: installation.Repository(root), Source: jobs.SourceBinding{Remote: "origin", Branch: "drifted"}, Host: installation.HostBinding{ExecutablePath: "/opt/pyxis", ToolSearchPath: "/usr/bin"}, Schedules: schedules}
	if err := store.WithTransaction(t.Context(), func(tx *installation.Transaction) error { return tx.Publish(manifest) }); err != nil {
		t.Fatal(err)
	}
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	entries, _ := maintenancecron.Render(root, maintenancecron.InstalledBinding{Executable: "/opt/pyxis", ToolPath: "/usr/bin", StableExecutable: true}, document.Jobs[:4])
	var table []byte
	for _, entry := range entries {
		table = append(table, entry.Text...)
	}
	cron := &statusCron{table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Bytes: table, Profile: profile, HostTimezone: "UTC"}}
	journal, err := audit.Open(t.Context(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	identity, _ := audit.NewIdentity()
	outcomes := []audit.Outcome{audit.OutcomeSucceeded, audit.OutcomeFailed, audit.OutcomeTimedOut, audit.OutcomeIncomplete}
	for index, outcome := range outcomes {
		event := audit.Event{Level: audit.LevelInfo, Event: audit.EventRun, Phase: audit.PhaseTerminal, TraceID: identity.TraceID, InvocationID: identity.InvocationID, JobID: ids[index], Outcome: outcome, CauseCode: "fixture"}
		if index == 0 {
			event.References = []audit.Reference{{Kind: "retained_workspace", Location: "maintenance-attempt:opaque"}, {Kind: "diagnostic", Location: "/private/worktree/output"}}
		}
		if _, err := journal.Append(t.Context(), event, audit.Consequential); err != nil {
			t.Fatal(err)
		}
	}
	logPath := filepath.Join(root, ".pyxis/maintenance/logs.jsonl")
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.WriteString("{malformed}\n{incomplete")
	_ = file.Close()
	beforeRepo, beforeState := treeSnapshot(t, root), treeSnapshot(t, stateRoot)
	output, err := Status(t.Context(), root, cron)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"manifest drift: source=true schedule=false ready=true", "owned entries: 4; drift: true", "malformed_complete=1 incomplete_tail=true", `job="success" outcome="succeeded"`, `job="failure" outcome="failed"`, `job="timeout" outcome="timed_out"`, `job="incomplete" outcome="incomplete"`, `job="none" outcome=unavailable`, `reference="maintenance-attempt:opaque"`, "shared outcome observation: unavailable"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q in status:\n%s", want, output)
		}
	}
	if strings.Contains(output, "private prompt") || strings.Contains(output, "/private/worktree") || cron.installs != 0 || cron.reads != 1 || !reflect.DeepEqual(beforeRepo, treeSnapshot(t, root)) || !reflect.DeepEqual(beforeState, treeSnapshot(t, stateRoot)) {
		t.Fatal("status leaked prompt or mutated repository, local state, or cron")
	}
	t.Run("ordinary skip retains known terminal facts", func(t *testing.T) {
		identity := audit.Identity{TraceID: strings.Repeat("c", 32), InvocationID: strings.Repeat("d", 32)}
		for _, decision := range []struct {
			event   audit.EventName
			phase   audit.Phase
			outcome audit.Outcome
			cause   string
		}{
			{audit.EventLifecycle, audit.PhaseAdmission, audit.OutcomeSkipped, "job_disabled"},
			{audit.EventClaim, audit.PhaseClaim, audit.OutcomeLost, "date_consumed_succeeded"},
		} {
			event := audit.Event{Level: audit.LevelInfo, Event: decision.event, Phase: decision.phase, TraceID: identity.TraceID, InvocationID: identity.InvocationID, JobID: "success", Outcome: decision.outcome, CauseCode: decision.cause}
			if _, err := journal.Append(t.Context(), event, audit.Consequential); err != nil {
				t.Fatal(err)
			}
			observed, err := Status(t.Context(), root, cron)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(observed, `attempt: job="success" outcome="succeeded"`) {
				t.Fatalf("ordinary %s decision erased a known completed attempt:\n%s", decision.cause, observed)
			}
		}
	})
	t.Run("later success preserves earlier retained work", func(t *testing.T) {
		event := audit.Event{Level: audit.LevelInfo, Event: audit.EventRun, Phase: audit.PhaseTerminal, TraceID: strings.Repeat("4", 32), InvocationID: strings.Repeat("5", 32), JobID: "success", Outcome: audit.OutcomeSucceeded, CauseCode: "later_success"}
		if _, err := journal.Append(t.Context(), event, audit.Consequential); err != nil {
			t.Fatal(err)
		}
		observed, err := Status(t.Context(), root, cron)
		if err != nil || !strings.Contains(observed, `cause="later_success"`) || !strings.Contains(observed, `reference="maintenance-attempt:opaque"`) {
			t.Fatalf("later attempt hid retained work: %v\n%s", err, observed)
		}
	})
	t.Run("foreign repository cannot supply success", func(t *testing.T) {
		original, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.WriteFile(logPath, original, 0o600); err != nil {
				t.Error(err)
			}
		}()
		first := bytes.SplitN(original, []byte("\n"), 2)[0]
		record := append(append([]byte(nil), first...), '\n')
		own := installation.Repository(root).SHA256
		foreign := strings.Repeat("e", 64)
		if foreign == own {
			foreign = strings.Repeat("f", 64)
		}
		record = bytes.Replace(record, []byte(`"repository_id":"`+own+`"`), []byte(`"repository_id":"`+foreign+`"`), 1)
		record = bytes.Replace(record, []byte(`"job_id":"success"`), []byte(`"job_id":"none"`), 1)
		if _, err := audit.Decode(record); err != nil {
			t.Fatalf("foreign fixture is not a valid closed record: %v", err)
		}
		if err := os.WriteFile(logPath, append(original, record...), 0o600); err != nil {
			t.Fatal(err)
		}
		observed, err := Status(t.Context(), root, cron)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(observed, `attempt: job="none" outcome="succeeded"`) {
			t.Fatalf("foreign repository fabricated an execution result:\n%s", observed)
		}
	})
	if err := store.RecordRefusal(t.Context(), strings.Repeat("b", 32), "success", installation.CauseExecutionFailed, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	refusalPath := filepath.Join(filepath.Dir(store.ManifestPath()), "refusals.json")
	if err := os.WriteFile(refusalPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	beforeMalformed := treeSnapshot(t, stateRoot)
	if output, err := Status(t.Context(), root, cron); err != nil || !strings.Contains(output, "refusal counters: unavailable (invalid or unreadable)") || !reflect.DeepEqual(beforeMalformed, treeSnapshot(t, stateRoot)) {
		t.Fatalf("malformed counters were repaired or hidden: %v\n%s", err, output)
	}
	if err := os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	privateJournal := filepath.Join(t.TempDir(), "private-journal")
	if err := os.WriteFile(privateJournal, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(privateJournal, logPath); err != nil {
		t.Fatal(err)
	}
	if output, err := Status(t.Context(), root, cron); err != nil || !strings.Contains(output, "journal health: unavailable (reader error)") {
		t.Fatalf("journal reader error was hidden: %v\n%s", err, output)
	}
}

func treeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			relative, _ := filepath.Rel(root, path)
			result[relative] = string(data)
		}
		return err
	})
	return result
}

func TestStatusReturnsNamedErrorsAndHonorsCancellation(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, test := range []struct {
		name string
		ctx  context.Context
		root string
		want error
	}{
		{"canceled", ctx, root, context.Canceled},
		{"invalid-root", t.Context(), filepath.Join(root, "missing"), jobs.ErrInvalid},
		{"missing-policy", t.Context(), root, jobs.ErrNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			if output, err := Status(test.ctx, test.root, nil); output != "" || !errors.Is(err, test.want) {
				t.Fatalf("Status = %q, %v; want %v", output, err, test.want)
			}
		})
	}
}

func TestStatusNamesUnavailableObservationsWithoutRepair(t *testing.T) {
	root := t.TempDir()
	document := jobs.Document{
		SchemaVersion: jobs.SchemaVersion,
		Source:        jobs.SourceBinding{Remote: "origin", Branch: "main"},
		Jobs: []jobs.Job{{
			ID: "daily", Enabled: true, Prompt: "private", Timeout: "1h",
			Schedule: jobs.Schedule{Cron: "0 2 * * *", Timezone: "UTC"},
			Runner: jobs.Runner{Harness: entity.Codex, Routing: executiondemand.ProfileMinimum{
				Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum,
				Context: executiondemand.ContextLarge, Assurance: executiondemand.AssuranceOrdinary,
			}},
		}},
	}
	data, err := jobs.Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".pyxis/maintenance/jobs.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_STATE_HOME", "relative-state")
	output, err := Status(t.Context(), root, nil)
	if err != nil || !strings.Contains(output, "installation observation: unavailable (state location invalid)") || !strings.Contains(output, "journal health: missing") || !strings.Contains(output, "refusal counters: unavailable") {
		t.Fatalf("invalid state observation = %v\n%s", err, output)
	}

	stateRoot := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateRoot)
	store, err := installation.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordRefusal(t.Context(), strings.Repeat("d", 32), "daily", installation.CauseExecutionFailed, time.Date(2026, 10, 28, 2, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.ManifestPath(), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	beforeRoot, beforeState := treeSnapshot(t, root), treeSnapshot(t, stateRoot)
	for _, test := range []struct {
		name string
		cron *statusCron
		want string
	}{
		{"absent cron", &statusCron{err: maintenancecron.ErrNoUserTable}, "cron observation: absent"},
		{"failed cron", &statusCron{err: errors.New("injected read failure")}, "cron observation: unavailable (provider read failed)"},
		{"malformed cron", &statusCron{table: maintenancecron.Table{Bytes: []byte("# pyxis-maintenance/v1 broken\n")}}, "cron observation: malformed owned entries"},
	} {
		t.Run(test.name, func(t *testing.T) {
			output, err := Status(t.Context(), root, test.cron)
			if err != nil || !strings.Contains(output, "installation observation: unavailable (manifest invalid or unreadable)") || !strings.Contains(output, "refusal counters: present entries=1") || !strings.Contains(output, `refusal: job="daily" cause="execution_failed"`) || !strings.Contains(output, test.want) {
				t.Fatalf("status = %v\n%s", err, output)
			}
		})
	}
	if !reflect.DeepEqual(beforeRoot, treeSnapshot(t, root)) || !reflect.DeepEqual(beforeState, treeSnapshot(t, stateRoot)) {
		t.Fatal("status repaired or changed observed state")
	}
}
