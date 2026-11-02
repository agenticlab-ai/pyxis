package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig/harnessconfigtest"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/admission"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/audit"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/claim"
	maintenancecron "github.com/agenticlab-ai/pyxis/internal/maintenance/cron"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/execution"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/installation"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
	maintenanceworkspace "github.com/agenticlab-ai/pyxis/internal/maintenance/workspace"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

func maintenanceRepo(t *testing.T, document *jobs.Document) string {
	t.Helper()
	root := t.TempDir()
	command := exec.Command("git", "init", "-q", root)
	command.Env = maintenanceGitEnvironment()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if document != nil {
		data, err := jobs.Encode(*document)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(root, ".pyxis/maintenance/jobs.yaml"), data)
	}
	return root
}

func maintenanceDocument(id, prompt string, enabled bool) jobs.Document {
	return jobs.Document{
		SchemaVersion: jobs.SchemaVersion,
		Source:        jobs.SourceBinding{Remote: "origin", Branch: "main"},
		Jobs: []jobs.Job{{
			ID: id, Enabled: enabled, Prompt: prompt, Timeout: "90m",
			Schedule: jobs.Schedule{Cron: "0 2 * * *", Timezone: "America/Los_Angeles"},
			Runner: jobs.Runner{Harness: entity.Codex, Routing: executiondemand.ProfileMinimum{
				Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum,
				Context: executiondemand.ContextLarge, Assurance: executiondemand.AssuranceOrdinary,
			}},
		}},
	}
}

func TestMaintenanceStatusUsesExplicitCanonicalRepositoryReadOnly(t *testing.T) {
	oldPort := maintenanceCronPort
	maintenanceCronPort = &maintenanceFakeCron{}
	t.Cleanup(func() { maintenanceCronPort = oldPort })
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	firstDocument := maintenanceDocument("first", "line one\n\x1b[31mred", false)
	secondDocument := maintenanceDocument("second", "other", true)
	first := maintenanceRepo(t, &firstDocument)
	second := maintenanceRepo(t, &secondDocument)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(first, alias); err != nil {
		t.Fatal(err)
	}
	beforeFirst, beforeSecond := snapshot(t, first), snapshot(t, second)
	policyPath := filepath.Join(first, ".pyxis/maintenance/jobs.yaml")
	beforePolicy, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	hostile := t.TempDir()
	t.Chdir(hostile)
	t.Setenv("GIT_DIR", filepath.Join(second, ".git"))
	t.Setenv("GIT_WORK_TREE", second)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.worktree")
	t.Setenv("GIT_CONFIG_VALUE_0", second)
	t.Setenv("GIT_TRACE", filepath.Join(first, ".pyxis/maintenance/git-trace.log"))

	for _, operand := range []string{first, alias} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{"maintenance", "status", "--repo", operand}, &stdout, &stderr); code != 0 {
			t.Fatalf("status(%s) = %d, stderr=%q", operand, code, stderr.String())
		}
		output := stdout.String()
		if !strings.Contains(output, "repository: "+strconvQuote(first)) || !strings.Contains(output, `id="first" enabled=false`) || strings.Contains(output, `id="second"`) {
			t.Fatalf("status selected wrong repository:\n%s", output)
		}
		if strings.Contains(output, "\x1b") || strings.Contains(output, "line one") || !strings.Contains(output, "prompt=configured") {
			t.Fatalf("status exposed raw prompt or terminal controls: %q", output)
		}
		if !strings.Contains(output, "installation observation: absent") || !strings.Contains(output, "journal health: missing") {
			t.Fatalf("status inferred unavailable observations:\n%s", output)
		}
	}
	if got := snapshot(t, first); !reflect.DeepEqual(beforeFirst, got) {
		t.Fatalf("first repository changed:\nbefore=%v\nafter=%v", beforeFirst, got)
	}
	if got := snapshot(t, second); !reflect.DeepEqual(beforeSecond, got) {
		t.Fatalf("second repository changed:\nbefore=%v\nafter=%v", beforeSecond, got)
	}
	if afterPolicy, err := os.ReadFile(policyPath); err != nil || !bytes.Equal(beforePolicy, afterPolicy) {
		t.Fatalf("status changed policy bytes: %v", err)
	}
}

func TestCompiledMaintenanceInspectsTwoRepositoriesFromHostileDirectory(t *testing.T) {
	firstDocument := maintenanceDocument("first", "first prompt", true)
	secondDocument := maintenanceDocument("second", "second prompt", true)
	first := maintenanceRepo(t, &firstDocument)
	second := maintenanceRepo(t, &secondDocument)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(first, alias); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "pyxis")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build compiled CLI: %v: %s", err, output)
	}
	hostile := t.TempDir()
	beforeFirst, beforeSecond := snapshot(t, first), snapshot(t, second)
	for _, test := range []struct{ operand, id string }{{alias, "first"}, {second, "second"}} {
		command := exec.Command(binary, "maintenance", "status", "--repo", test.operand)
		command.Dir = hostile
		command.Env = append(os.Environ(), "GIT_DIR="+filepath.Join(first, ".git"), "GIT_WORK_TREE="+first)
		output, err := command.CombinedOutput()
		if err != nil || !bytes.Contains(output, []byte(`id="`+test.id+`"`)) {
			t.Fatalf("compiled status(%s): %v: %s", test.operand, err, output)
		}
	}
	if !reflect.DeepEqual(beforeFirst, snapshot(t, first)) || !reflect.DeepEqual(beforeSecond, snapshot(t, second)) {
		t.Fatal("compiled status changed a repository")
	}
}

func TestMaintenanceStatusAcceptsEmptyPolicyAndNamesMissingOrMalformed(t *testing.T) {
	oldPort := maintenanceCronPort
	maintenanceCronPort = &maintenanceFakeCron{}
	t.Cleanup(func() { maintenanceCronPort = oldPort })
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	empty := maintenanceDocument("unused", "unused", true)
	empty.Jobs = []jobs.Job{}
	root := maintenanceRepo(t, &empty)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"maintenance", "status", "--repo", root}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "desired jobs: 0") {
		t.Fatalf("empty status = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	missing := maintenanceRepo(t, nil)
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"maintenance", "status", "--repo", missing}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), jobs.ErrNotFound.Error()) {
		t.Fatalf("missing status = %d, stderr=%q", code, stderr.String())
	}
	writeFile(t, filepath.Join(missing, ".pyxis/maintenance/jobs.yaml"), []byte("schema_version: ["))
	stderr.Reset()
	if code := run([]string{"maintenance", "status", "--repo", missing}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), jobs.ErrInvalid.Error()) {
		t.Fatalf("malformed status = %d, stderr=%q", code, stderr.String())
	}
}

func TestMaintenanceArgumentsAndRepositoryAdmissionFailClosed(t *testing.T) {
	root := maintenanceRepo(t, nil)
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	bare := filepath.Join(t.TempDir(), "bare.git")
	command := exec.Command("git", "init", "-q", "--bare", bare)
	command.Env = maintenanceGitEnvironment()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, output)
	}
	cases := [][]string{
		{"maintenance"}, {"maintenance", "unknown"}, {"maintenance", "status"},
		{"maintenance", "status", "--repo", "relative"}, {"maintenance", "status", "--repo", root, "extra"},
		{"maintenance", "status", "--repo", root, "--preview"}, {"maintenance", "status", "--unknown"},
		{"maintenance", "run", "--repo", root}, {"maintenance", "run", "--repo", root, "--job", "INVALID"},
		{"maintenance", "run", "--repo", root, "--job", "valid", "--preview"},
		{"maintenance", "install", "--repo", root, "--job", "valid"},
		{"maintenance", "uninstall", "--repo", root, "--job", "valid"},
		{"maintenance", "status", "--repo", nested}, {"maintenance", "status", "--repo", bare},
	}
	for _, args := range cases {
		t.Run(strings.Join(args[1:], "_"), func(t *testing.T) {
			before := snapshot(t, root)
			var stdout, stderr bytes.Buffer
			if code := run(args, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), "nothing was run") && !strings.Contains(stderr.String(), "desired policy unavailable") {
				t.Fatalf("run(%q) = %d, stderr=%q", args, code, stderr.String())
			}
			if got := snapshot(t, root); !reflect.DeepEqual(before, got) {
				t.Fatalf("refusal changed repository: before=%v after=%v", before, got)
			}
		})
	}
}

func TestMaintenanceAdmissionCoverageBoundaries(t *testing.T) {
	selection := nativerouting.SelectedProfile{Harness: entity.Codex, NativeProfileID: "frontier", NativeModel: "gpt-6.1-sol", NativeEffort: "high", PolicyIdentity: "policy", QualificationID: "qualification"}
	prepared := &admission.Prepared{ConfigCommit: "published-commit", JobDate: "2026-10-20",
		Job:       jobs.Job{Schedule: jobs.Schedule{Timezone: "America/Los_Angeles"}, Runner: jobs.Runner{Harness: entity.Codex}},
		Selection: selection}
	event := maintenanceAdmissionEvent(audit.Identity{TraceID: "trace", InvocationID: "invocation"}, admission.Result{Prepared: prepared}, audit.OutcomeRefused, "fixture", "refused")
	if event.Route == nil || event.Route.Harness != "codex" || event.JobDate != prepared.JobDate || event.Level != audit.LevelError {
		t.Fatalf("prepared admission event = %+v", event)
	}
	event = maintenanceAdmissionEvent(audit.Identity{}, admission.Result{Selection: &selection}, audit.OutcomeRefused, "fixture", "refused")
	if event.Route == nil || event.Route.Profile != selection.NativeProfileID || event.Route.Model != selection.NativeModel || event.Route.Effort != selection.NativeEffort || event.Route.Policy != selection.PolicyIdentity || event.Route.Qualification != selection.QualificationID {
		t.Fatalf("pre-claim refusal route = %+v", event.Route)
	}
	if event := maintenanceAdmissionEvent(audit.Identity{}, admission.Result{}, audit.OutcomeRefused, "fixture", "refused"); event.Route != nil {
		t.Fatalf("pre-selection refusal route = %+v", event.Route)
	}
	if got := maintenanceRunFailureCode(execution.Result{Outcome: execution.OutcomeSucceeded}, errors.Join(execution.ErrTerminalMetadata, execution.ErrLifecycle, errors.New("/private/secret"))); got != "terminal_metadata_failed; audit_failed" || strings.Contains(got, "private") {
		t.Fatalf("sanitized independent terminal failures = %q", got)
	}

	root := maintenanceRepo(t, nil)
	var stdout, stderr bytes.Buffer
	if code := maintenanceRun(t.Context(), root, "daily", time.Now().UTC(), &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "audit admission failed") {
		t.Fatalf("audit refusal = %d, %q", code, stderr.String())
	}
	writeFile(t, filepath.Join(root, ".gitignore"), []byte("/.pyxis/maintenance/logs.jsonl\n"))
	t.Setenv("XDG_STATE_HOME", "relative")
	stderr.Reset()
	if code := maintenanceRun(t.Context(), root, "daily", time.Now().UTC(), &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "XDG_STATE_HOME") {
		t.Fatalf("installation store refusal = %d, %q", code, stderr.String())
	}

	stderr.Reset()
	if code := maintenanceMisparse(&stderr, flag.ErrHelp); code != 2 || !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("help misparse = %d, %q", code, stderr.String())
	}
	stderr.Reset()
	if code := maintenanceMisparse(&stderr, errors.New("bad flags")); code != 2 || !strings.Contains(stderr.String(), "bad flags") {
		t.Fatalf("generic misparse = %d, %q", code, stderr.String())
	}

	file := filepath.Join(t.TempDir(), "file")
	writeFile(t, file, []byte("not a repository"))
	if _, err := maintenanceRepository(t.Context(), file); err == nil || !strings.Contains(err.Error(), "existing directory") {
		t.Fatalf("file repository error = %v", err)
	}
	bare := filepath.Join(t.TempDir(), "bare.git")
	command := exec.Command("git", "init", "-q", "--bare", bare)
	command.Env = maintenanceGitEnvironment()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, output)
	}
	if _, err := maintenanceRepository(t.Context(), bare); err == nil || !strings.Contains(err.Error(), "Git worktree") {
		t.Fatalf("bare repository error = %v", err)
	}
}

func TestMaintenanceTerminalMappingsCoverDistinctFacts(t *testing.T) {
	for _, test := range []struct {
		name    string
		outcome execution.Outcome
		want    audit.Outcome
	}{
		{"succeeded", execution.OutcomeSucceeded, audit.OutcomeSucceeded},
		{"timed out", execution.OutcomeTimedOut, audit.OutcomeTimedOut},
		{"incomplete", execution.OutcomeIncomplete, audit.OutcomeIncomplete},
		{"consumed", execution.OutcomeConsumed, audit.OutcomeSkipped},
		{"unproved", execution.OutcomeUnproved, audit.OutcomeUnproved},
		{"refused", execution.OutcomeRefused, audit.OutcomeRefused},
		{"failed", execution.OutcomeFailed, audit.OutcomeFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := maintenanceAuditOutcome(test.outcome); got != test.want {
				t.Fatalf("maintenanceAuditOutcome(%q) = %q, want %q", test.outcome, got, test.want)
			}
		})
	}

	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{"metadata", execution.ErrTerminalMetadata, "terminal_metadata_failed"},
		{"audit", execution.ErrLifecycle, "audit_failed"},
		{"execution", errors.New("runtime failed"), "execution_failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := maintenanceRunFailureCode(execution.Result{Outcome: execution.OutcomeFailed}, test.err); got != test.want {
				t.Fatalf("maintenanceRunFailureCode() = %q, want %q", got, test.want)
			}
		})
	}

	if got := maintenanceAuditUsage(execution.Result{}); got != nil {
		t.Fatalf("unobserved usage = %+v", got)
	}
	result := execution.Result{Usage: port.RunResult{Turns: 0, TurnsObserved: true, ToolCalls: 0, ToolCallsObserved: true}}
	usage := maintenanceAuditUsage(result)
	if usage == nil || usage.Turns == nil || *usage.Turns != 0 || usage.ToolCalls == nil || *usage.ToolCalls != 0 || usage.Tokens != nil {
		t.Fatalf("observed zero usage = %+v", usage)
	}
}

func TestMaintenancePreclaimRefusalsJournalResolvedRouteWithoutClaimOrLaunch(t *testing.T) {
	selection := nativerouting.SelectedProfile{Harness: entity.Codex, NativeProfileID: "codex-implementation-frontier", NativeModel: "gpt-5.6-sol", NativeEffort: "high", PolicyIdentity: "sha256:policy", QualificationID: "qualification-codex"}
	for _, cause := range []string{"authenticate codex runtime: unavailable", "maintenance process support: unavailable"} {
		t.Run(strings.Fields(cause)[0], func(t *testing.T) {
			root := maintenanceRepo(t, nil)
			writeFile(t, filepath.Join(root, ".gitignore"), []byte("/.pyxis/maintenance/logs.jsonl\n"))
			journal, err := audit.Open(t.Context(), root, nil)
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			result := admission.Result{JobID: "daily", JobDate: "2026-10-20", Timezone: "America/Los_Angeles", ConfigCommit: "published", Selection: &selection}
			identity, err := audit.NewIdentity()
			if err != nil {
				t.Fatal(err)
			}
			if code := maintenanceRunRefusal(t.Context(), journal, identity, root, "daily", result, errors.New(cause), &stderr); code != 1 {
				t.Fatalf("refusal code = %d", code)
			}
			entry, err := os.ReadFile(filepath.Join(root, ".pyxis/maintenance/logs.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{`"profile":"codex-implementation-frontier"`, `"model":"gpt-5.6-sol"`, `"effort":"high"`, `"policy":"sha256:policy"`, `"qualification":"qualification-codex"`} {
				if !bytes.Contains(entry, []byte(want)) {
					t.Errorf("journal %q lacks %s", entry, want)
				}
			}
			if bytes.Contains(entry, []byte(`"claim":`)) || bytes.Contains(entry, []byte(`"launch":`)) {
				t.Fatalf("pre-claim refusal journaled claim or launch: %s", entry)
			}
		})
	}
}

func TestMaintenanceExecutionAuditMapsOnlyBoundedLifecycleFacts(t *testing.T) {
	selection := nativerouting.SelectedProfile{Harness: entity.Codex, NativeProfileID: "profile", NativeModel: "model", NativeEffort: "high", PolicyIdentity: "policy", QualificationID: "qualification"}
	admitted := admission.Result{JobID: "daily", JobDate: "2026-10-20", Timezone: "America/Los_Angeles", ConfigCommit: strings.Repeat("a", 40), Selection: &selection}
	identity, err := audit.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		result   execution.Result
		want     string
		runEvent bool
	}{
		{"consumed", execution.Result{Outcome: execution.OutcomeConsumed, Claim: claim.ClaimResult{Disposition: claim.DispositionConsumed, Record: claim.Record{JobID: "daily", OwnerNonce: strings.Repeat("b", 64)}}}, `"outcome":"lost"`, false},
		{"unproved", execution.Result{Outcome: execution.OutcomeUnproved}, `"outcome":"unproved"`, false},
		{"failed", execution.Result{Outcome: execution.OutcomeFailed, Claim: claim.ClaimResult{Disposition: claim.DispositionOwned, Record: claim.Record{JobID: "daily", OwnerNonce: strings.Repeat("c", 64)}}}, `"outcome":"failed"`, true},
		{"timed out", execution.Result{Outcome: execution.OutcomeTimedOut, Claim: claim.ClaimResult{Disposition: claim.DispositionOwned, Record: claim.Record{JobID: "daily", OwnerNonce: strings.Repeat("d", 64)}}}, `"outcome":"timed_out"`, true},
		{"incomplete", execution.Result{Outcome: execution.OutcomeIncomplete, Claim: claim.ClaimResult{Disposition: claim.DispositionOwned, Record: claim.Record{JobID: "daily", OwnerNonce: strings.Repeat("e", 64)}}}, `"outcome":"incomplete"`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := maintenanceRepo(t, nil)
			writeFile(t, filepath.Join(root, ".gitignore"), []byte("/.pyxis/maintenance/logs.jsonl\n"))
			journal, err := audit.Open(t.Context(), root, nil)
			if err != nil {
				t.Fatal(err)
			}
			lifecycle := maintenanceExecutionLifecycle{Journal: journal, Identity: identity, Admitted: admitted}
			if err := lifecycle.Observe(t.Context(), execution.Observation{Boundary: execution.BoundaryClaim, Result: test.result, CauseCode: "fixture"}); err != nil {
				t.Fatal(err)
			}
			if test.runEvent {
				test.result.Capture = &port.LaunchCapture{NativeModel: "actual-model", NativeEffort: "high"}
				test.result.Usage = port.RunResult{Tokens: 0, TokensObserved: true}
				if err := lifecycle.Observe(t.Context(), execution.Observation{Boundary: execution.BoundaryTerminal, Result: test.result, CauseCode: "fixture"}); err != nil {
					t.Fatal(err)
				}
			}
			data, err := os.ReadFile(filepath.Join(root, ".pyxis/maintenance/logs.jsonl"))
			if err != nil || !bytes.Contains(data, []byte(test.want)) || bytes.Contains(data, []byte("literal prompt")) || bytes.Contains(data, []byte(root)) {
				t.Fatalf("audit=%q err=%v", data, err)
			}
			if bytes.Contains(data, []byte(`"event":"maintenance_run_result"`)) != test.runEvent {
				t.Fatalf("run event presence in %s", data)
			}
			if test.runEvent && (!bytes.Contains(data, []byte(`"tokens":0`)) || !bytes.Contains(data, []byte(`"model":"actual-model"`))) {
				t.Fatalf("observed zero usage or actual launch missing from %s", data)
			}
		})
	}
}

func TestMaintenanceRunCapturesStartBeforeRepositoryDiscovery(t *testing.T) {
	prior := maintenanceNow
	t.Cleanup(func() { maintenanceNow = prior })
	captured := false
	maintenanceNow = func() time.Time {
		captured = true
		return time.Date(2026, 10, 20, 23, 59, 59, 0, time.UTC)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"maintenance", "run", "--repo", missing, "--job", "daily"}, &stdout, &stderr); code != 1 || !captured {
		t.Fatalf("run = %d captured=%v stderr=%q", code, captured, stderr.String())
	}
}

func TestMaintenanceRunRefusesWithoutReadyInstallation(t *testing.T) {
	root := maintenanceRepo(t, nil)
	writeFile(t, filepath.Join(root, ".gitignore"), []byte("/.pyxis/maintenance/logs.jsonl\n/.pyxis/state/\n"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	args := []string{"maintenance", "run", "--repo", root, "--job", "daily"}
	if code := run(args, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "maintenance installation required") || !strings.Contains(stderr.String(), "nothing was launched") {
		t.Fatalf("run(%q) = %d, stderr=%q", args, code, stderr.String())
	}
	journal, err := os.ReadFile(filepath.Join(root, ".pyxis/maintenance/logs.jsonl"))
	if err != nil || !bytes.Contains(journal, []byte(`"cause_code":"admission_refused"`)) {
		t.Fatalf("run refusal audit = %q, %v", journal, err)
	}
	store, err := installation.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.LoadRefusals(t.Context())
	if err != nil || len(state.Counters) != 1 || state.Counters[0].Cause != installation.CauseAdmissionRefused || state.Counters[0].Lifetime != 1 {
		t.Fatalf("run refusal counter = %+v, %v", state, err)
	}
}

func TestMaintenanceRunUsesPublishedDisableAndLeavesLocalPolicyUntouched(t *testing.T) {
	published := maintenanceDocument("daily", "published prompt", false)
	installed := published
	installed.Jobs = append([]jobs.Job(nil), published.Jobs...)
	installed.Jobs[0].Enabled = true
	root := maintenanceRepo(t, nil)
	runPyxisGit(t, root, "branch", "-M", "main")
	runPyxisGit(t, root, "config", "user.name", "fixture")
	runPyxisGit(t, root, "config", "user.email", "fixture@example.test")
	policy, err := jobs.Encode(published)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".gitignore"), []byte("/.pyxis/maintenance/logs.jsonl\n"))
	writeFile(t, filepath.Join(root, ".pyxis/maintenance/jobs.yaml"), policy)
	writeFile(t, filepath.Join(root, ".pyxis/config.yaml"), []byte(harnessconfigtest.YAML))
	runPyxisGit(t, root, "add", ".gitignore", ".pyxis/config.yaml", ".pyxis/maintenance/jobs.yaml")
	runPyxisGit(t, root, "commit", "-m", "published policy")
	remote := filepath.Join(t.TempDir(), "remote.git")
	if output, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, output)
	}
	runPyxisGit(t, root, "remote", "add", "origin", remote)
	runPyxisGit(t, root, "push", "-q", "origin", "main")
	unpublished, err := jobs.Encode(installed)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".pyxis/maintenance/jobs.yaml"), unpublished)
	stateRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", stateRoot)
	store, err := installation.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	schedules, err := installation.ScheduleFingerprints(installed)
	if err != nil {
		t.Fatal(err)
	}
	manifest := installation.InstallationManifest{SchemaVersion: installation.SchemaVersion, Generation: "11111111111111111111111111111111", State: installation.StateReady,
		Repository: store.Repository(), Source: installed.Source, Host: installation.HostBinding{ExecutablePath: "/opt/pyxis", ToolSearchPath: "/usr/bin:/bin"}, Schedules: schedules}
	if err := store.WithTransaction(t.Context(), func(tx *installation.Transaction) error { return tx.Publish(manifest) }); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"maintenance", "run", "--repo", root, "--job", "daily"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "is disabled") {
		t.Fatalf("run = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if local, err := os.ReadFile(filepath.Join(root, ".pyxis/maintenance/jobs.yaml")); err != nil || !bytes.Equal(local, unpublished) {
		t.Fatalf("local unpublished policy changed: %v", err)
	}
	if _, err := store.LoadRefusals(t.Context()); !errors.Is(err, installation.ErrRefusalsAbsent) {
		t.Fatalf("disabled skip changed refusal counters: %v", err)
	}

	writeFile(t, filepath.Join(root, ".pyxis/maintenance/jobs.yaml"), unpublished)
	runPyxisGit(t, root, "add", ".pyxis/maintenance/jobs.yaml")
	runPyxisGit(t, root, "commit", "-m", "enable published job")
	runPyxisGit(t, root, "push", "-q", "origin", "main")
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	gitExecPath, err := exec.CommandContext(t.Context(), gitPath, "--exec-path").Output()
	if err != nil {
		t.Fatal(err)
	}
	toolDir := t.TempDir()
	for tool, target := range map[string]string{
		"git":              gitPath,
		"git-upload-pack":  filepath.Join(strings.TrimSpace(string(gitExecPath)), "git-upload-pack"),
		"git-receive-pack": filepath.Join(strings.TrimSpace(string(gitExecPath)), "git-receive-pack"),
	} {
		if err := os.Symlink(target, filepath.Join(toolDir, tool)); err != nil {
			t.Fatal(err)
		}
	}
	codex := filepath.Join(toolDir, "codex")
	if output, err := exec.CommandContext(t.Context(), "go", "build", "-o", codex, "../../internal/maintenance/execution/testdata/native").CombinedOutput(); err != nil {
		t.Fatalf("build native fixture: %v: %s", err, output)
	}
	// Linux confines the harness with Bubblewrap, found through PATH.
	if bwrap, err := exec.LookPath("bwrap"); err == nil {
		if err := os.Symlink(bwrap, filepath.Join(toolDir, "bwrap")); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", toolDir)
	t.Setenv("OPENAI_API_KEY", "fixture")
	oldProcessAccess := maintenanceProcessAccess
	maintenanceProcessAccess = func(context.Context) error { return nil }
	oldNow := maintenanceNow
	fixedNow := time.Now().UTC()
	maintenanceNow = func() time.Time { return fixedNow }
	t.Cleanup(func() { maintenanceProcessAccess, maintenanceNow = oldProcessAccess, oldNow })
	if err := store.RecordRefusal(t.Context(), strings.Repeat("a", 32), "daily", installation.CauseAdmissionRefused, fixedNow); err != nil {
		t.Fatal(err)
	}
	refusalPath := filepath.Join(filepath.Dir(store.ManifestPath()), "refusals.json")
	refusalBytes, err := os.ReadFile(refusalPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("repeated unproved claims retain consecutive failures", func(t *testing.T) {
		defer func() {
			if err := os.WriteFile(refusalPath, refusalBytes, 0o600); err != nil {
				t.Error(err)
			}
		}()
		refs := filepath.Join(remote, "refs", "pyxis", "maintenance")
		if err := os.MkdirAll(refs, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(refs, 0o500); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Chmod(refs, 0o700); err != nil {
				t.Error(err)
			}
		}()
		for range 2 {
			var stdout, stderr bytes.Buffer
			if code := run([]string{"maintenance", "run", "--repo", root, "--job", "daily"}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "execution_unproved") {
				t.Fatalf("fixture did not reach an unproved claim: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		}
		state, err := store.LoadRefusals(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		for _, counter := range state.Counters {
			if counter.Cause == installation.CauseClaimUnproved {
				if counter.Lifetime != 2 || counter.Consecutive != 2 {
					t.Fatalf("unproved claims lost their consecutive history: %+v", counter)
				}
				return
			}
		}
		t.Fatal("unproved claim counter missing")
	})
	if err := os.Remove(refusalPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(refusalPath, 0o700); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"maintenance", "run", "--repo", root, "--job", "daily"}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "launch withheld") {
		t.Fatalf("counter-write refusal = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if output, err := exec.Command("git", "--git-dir", remote, "show-ref", "--verify", "refs/pyxis/maintenance/daily").CombinedOutput(); err == nil {
		t.Fatalf("counter failure granted a claim or launch: %s", output)
	}
	if err := os.Remove(refusalPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(refusalPath, refusalBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"maintenance", "run", "--repo", root, "--job", "daily"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "process tree quiescence proved") {
		t.Fatalf("enabled run = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	journal, err := os.ReadFile(filepath.Join(root, ".pyxis/maintenance/logs.jsonl"))
	if err != nil || !bytes.Contains(journal, []byte(`"event":"maintenance_claim_result"`)) || !bytes.Contains(journal, []byte(`"event":"maintenance_run_result"`)) || !bytes.Contains(journal, []byte(`"cause_code":"execution_completed"`)) {
		t.Fatalf("execution journal = %q, %v", journal, err)
	}
	if bytes.Contains(journal, []byte("published prompt")) || bytes.Contains(journal, []byte(root)) || bytes.Contains(journal, []byte("total_tokens")) {
		t.Fatalf("execution journal leaked prompt, output, or private path: %s", journal)
	}
	location, _ := time.LoadLocation(published.Jobs[0].Schedule.Timezone)
	date := fixedNow.In(location).Format(time.DateOnly)
	command := exec.Command("git", "--git-dir", remote, "show", "refs/pyxis/maintenance/daily:occurrences/"+date+".json")
	command.Env = maintenanceGitEnvironment()
	shared, err := command.Output()
	if err != nil {
		t.Fatalf("read shared terminal: %v", err)
	}
	record, err := claim.Decode(shared)
	if err != nil || record.Outcome != claim.OutcomeSucceeded || record.DurationMS == nil || record.ExitCode != nil {
		t.Fatalf("shared terminal = %+v, %v", record, err)
	}
}

func TestMaintenanceExecutionRefusalCausesExcludeOrdinaryConsumedSkip(t *testing.T) {
	if got := maintenanceExecutionCauses(execution.Result{Outcome: execution.OutcomeConsumed, Claim: claim.ClaimResult{Disposition: claim.DispositionConsumed}}, nil); len(got) != 0 {
		t.Fatalf("consumed skip counted as malfunction: %v", got)
	}
	if got := maintenanceExecutionCauses(execution.Result{Outcome: execution.OutcomeTimedOut}, context.DeadlineExceeded); !reflect.DeepEqual(got, []installation.RefusalCause{installation.CauseExecutionTimedOut}) {
		t.Fatalf("timeout causes = %v", got)
	}
	for _, test := range []struct {
		name   string
		result execution.Result
		err    error
		want   []installation.RefusalCause
	}{
		{"terminal and audit", execution.Result{Outcome: execution.OutcomeSucceeded}, errors.Join(execution.ErrTerminalMetadata, execution.ErrLifecycle), []installation.RefusalCause{installation.CauseTerminalFailed, installation.CauseAuditFailed}},
		{"claim unproved", execution.Result{Outcome: execution.OutcomeUnproved}, nil, []installation.RefusalCause{installation.CauseClaimUnproved}},
		{"execution refused", execution.Result{Outcome: execution.OutcomeRefused}, nil, []installation.RefusalCause{installation.CauseExecutionRefused}},
		{"execution incomplete", execution.Result{Outcome: execution.OutcomeIncomplete}, nil, []installation.RefusalCause{installation.CauseExecutionIncomplete}},
		{"workspace failed", execution.Result{Outcome: execution.OutcomeFailed, Claim: claim.ClaimResult{Disposition: claim.DispositionOwned}}, nil, []installation.RefusalCause{installation.CauseWorkspaceFailed}},
		{"execution failed", execution.Result{Outcome: execution.OutcomeFailed, Workspace: &maintenanceworkspace.Prepared{}}, nil, []installation.RefusalCause{installation.CauseExecutionFailed}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := maintenanceExecutionCauses(test.result, test.err); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("causes = %v, want %v", got, test.want)
			}
		})
	}
}

func TestMaintenanceRecordCounterUsesBoundedRecoveryContext(t *testing.T) {
	root := maintenanceRepo(t, nil)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	store, err := installation.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	priorNow := maintenanceNow
	maintenanceNow = func() time.Time { return time.Date(2026, 10, 28, 4, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { maintenanceNow = priorNow })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := maintenanceRecordCounter(ctx, store, strings.Repeat("f", 32), "daily", installation.CauseExecutionFailed); err != nil {
		t.Fatal(err)
	}
	state, err := store.LoadRefusals(t.Context())
	if err != nil || len(state.Counters) != 1 || state.Counters[0].Consecutive != 1 {
		t.Fatalf("recovered counter = %+v, %v", state, err)
	}
}

type maintenanceFakeCron struct {
	table    maintenancecron.Table
	installs int
}

func (fake *maintenanceFakeCron) Read(context.Context) (maintenancecron.Table, error) {
	table := fake.table
	table.Bytes = bytes.Clone(table.Bytes)
	return table, nil
}

func (fake *maintenanceFakeCron) Install(_ context.Context, profile maintenancecron.Profile, table []byte) error {
	fake.installs++
	fake.table.Profile, fake.table.Bytes = profile, bytes.Clone(table)
	return nil
}

func TestMaintenanceInstallPreviewNoopAndPolicyFreeUninstall(t *testing.T) {
	document := maintenanceDocument("daily", "inspect safely", true)
	root := maintenanceRepo(t, &document)
	writeFile(t, filepath.Join(root, ".gitignore"), []byte("/.pyxis/maintenance/logs.jsonl\n/.pyxis/state/\n"))
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	other, err := maintenancecron.Render("/srv/other", maintenancecron.InstalledBinding{Executable: "/opt/pyxis", ToolPath: "/usr/bin", StableExecutable: true}, []jobs.Job{{ID: "shared", Enabled: true, Schedule: jobs.Schedule{Cron: "5 4 * * *"}}})
	if err != nil {
		t.Fatal(err)
	}
	initial := append([]byte("MAILTO=ops@example.test\nCRON_TZ=America/Los_Angeles\n"), other[0].Text...)
	fake := &maintenanceFakeCron{table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Bytes: bytes.Clone(initial), Profile: profile, HostTimezone: "America/Los_Angeles"}}
	oldPort, oldExecutable, oldProcessAccess := maintenanceCronPort, maintenanceExecutable, maintenanceProcessAccess
	maintenanceCronPort = fake
	maintenanceProcessAccess = func(context.Context) error { return nil }
	t.Cleanup(func() {
		maintenanceCronPort, maintenanceExecutable, maintenanceProcessAccess = oldPort, oldExecutable, oldProcessAccess
	})
	bin := filepath.Join(t.TempDir(), "pyxis")
	writeFile(t, bin, []byte("binary"))
	if err := os.Chmod(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	maintenanceExecutable = func() (string, error) { return bin, nil }
	toolDir := t.TempDir()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	for tool, target := range map[string]string{"git": gitPath, "codex": "/usr/bin/true"} {
		if err := os.Symlink(target, filepath.Join(toolDir, tool)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", toolDir)
	stateParent := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateParent)
	before := snapshot(t, root)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"maintenance", "install", "--repo", root, "--preview"}, &stdout, &stderr); code != 0 || fake.installs != 0 || !strings.Contains(stdout.String(), "preview: changed") {
		t.Fatalf("preview = %d, writes=%d stdout=%q stderr=%q", code, fake.installs, stdout.String(), stderr.String())
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("preview changed repository")
	}
	if _, err := os.Stat(filepath.Join(stateParent, "pyxis")); !os.IsNotExist(err) {
		t.Fatalf("preview created machine state: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"maintenance", "install", "--repo", root}, &stdout, &stderr); code != 0 || fake.installs != 1 {
		t.Fatalf("install = %d, writes=%d stdout=%q stderr=%q", code, fake.installs, stdout.String(), stderr.String())
	}
	store, err := installation.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Load(t.Context())
	if err != nil || first.State != installation.StateReady {
		t.Fatalf("ready manifest = %+v, %v", first, err)
	}
	if err := store.RecordRefusal(t.Context(), strings.Repeat("a", 32), "daily", installation.CauseExecutionFailed, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"maintenance", "install", "--repo", root}, &stdout, &stderr); code != 0 || fake.installs != 1 || !strings.Contains(stdout.String(), "unchanged") {
		t.Fatalf("reinstall = %d, writes=%d stdout=%q stderr=%q", code, fake.installs, stdout.String(), stderr.String())
	}
	second, err := store.Load(t.Context())
	if err != nil || second.Generation != first.Generation {
		t.Fatalf("no-op generation = %q, %v; want %q", second.Generation, err, first.Generation)
	}
	counters, err := store.LoadRefusals(t.Context())
	if err != nil || counters.Counters[0].Lifetime != 1 || counters.Counters[0].Consecutive != 1 {
		t.Fatalf("reinstall changed execution failure history = %+v, %v", counters, err)
	}
	disabled := maintenanceDocument("daily", "inspect safely", false)
	disabledBytes, err := jobs.Encode(disabled)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".pyxis/maintenance/jobs.yaml"), disabledBytes)
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"maintenance", "install", "--repo", root}, &stdout, &stderr); code != 0 || fake.installs != 2 || !bytes.Equal(fake.table.Bytes, initial) {
		t.Fatalf("disabled reconcile = %d, writes=%d preserved=%v stdout=%q stderr=%q", code, fake.installs, bytes.Equal(fake.table.Bytes, initial), stdout.String(), stderr.String())
	}
	disabledManifest, err := store.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".pyxis/maintenance/jobs.yaml")); err != nil {
		t.Fatal(err)
	}
	beforeUninstall := snapshot(t, root)
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"maintenance", "uninstall", "--repo", root, "--preview"}, &stdout, &stderr); code != 0 || fake.installs != 2 || !strings.Contains(stdout.String(), "preview: changed") {
		t.Fatalf("uninstall preview = %d, writes=%d stdout=%q stderr=%q", code, fake.installs, stdout.String(), stderr.String())
	}
	if !reflect.DeepEqual(beforeUninstall, snapshot(t, root)) {
		t.Fatal("uninstall preview changed repository or audit history")
	}
	previewManifest, err := store.Load(t.Context())
	if err != nil || previewManifest.Generation != disabledManifest.Generation {
		t.Fatalf("uninstall preview changed manifest: %+v, %v", previewManifest, err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"maintenance", "uninstall", "--repo", root}, &stdout, &stderr); code != 0 || fake.installs != 2 || !bytes.Equal(fake.table.Bytes, initial) {
		t.Fatalf("uninstall = %d, writes=%d preserved=%v stdout=%q stderr=%q", code, fake.installs, bytes.Equal(fake.table.Bytes, initial), stdout.String(), stderr.String())
	}
	counters, err = store.LoadRefusals(t.Context())
	if err != nil || counters.Counters[0].Lifetime != 1 || counters.Counters[0].Consecutive != 1 {
		t.Fatalf("disable/uninstall changed execution failure history = %+v, %v", counters, err)
	}
	journal, err := os.ReadFile(filepath.Join(root, ".pyxis/maintenance/logs.jsonl"))
	if err != nil || bytes.Count(journal, []byte("\n")) != 4 {
		t.Fatalf("audit history = %v records, %v", bytes.Count(journal, []byte("\n")), err)
	}
}

func strconvQuote(value string) string { return strconv.Quote(value) }

func TestMaintenanceStatusPreservesWhitespaceInRepositoryPaths(t *testing.T) {
	oldPort := maintenanceCronPort
	maintenanceCronPort = &maintenanceFakeCron{}
	t.Cleanup(func() { maintenanceCronPort = oldPort })
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	for _, suffix := range []string{"space ", "tab\t", "newline\n"} {
		t.Run(strconv.Quote(suffix), func(t *testing.T) {
			document := maintenanceDocument("daily", "inspect policy", true)
			original := maintenanceRepo(t, &document)
			parent, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(parent, suffix)
			if err := os.Rename(original, root); err != nil {
				t.Fatal(err)
			}
			before := snapshot(t, root)
			var stdout, stderr bytes.Buffer
			if code := runContext(t.Context(), []string{"maintenance", "status", "--repo", root}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "repository: "+strconv.Quote(root)) {
				t.Fatalf("status = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("whitespace-path status changed repository")
			}
		})
	}
}

func TestMaintenanceCommandHonorsCanceledBoundary(t *testing.T) {
	root := maintenanceRepo(t, nil)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var stdout, stderr bytes.Buffer
	if code := runContext(ctx, []string{"maintenance", "status", "--repo", root}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), context.Canceled.Error()) {
		t.Fatalf("canceled status = %d, stderr=%q", code, stderr.String())
	}
}

func TestMaintenanceInstallPreviewMissingOrInvalidPolicyCreatesNothing(t *testing.T) {
	root := maintenanceRepo(t, nil)
	state := filepath.Join(t.TempDir(), "state")
	t.Setenv("XDG_STATE_HOME", state)
	for _, contents := range [][]byte{nil, []byte("schema_version: [")} {
		if contents != nil {
			writeFile(t, filepath.Join(root, ".pyxis/maintenance/jobs.yaml"), contents)
		}
		before := snapshot(t, root)
		var stdout, stderr bytes.Buffer
		if code := run([]string{"maintenance", "install", "--repo", root, "--preview"}, &stdout, &stderr); code == 0 {
			t.Fatalf("invalid preview succeeded: stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
		if !reflect.DeepEqual(before, snapshot(t, root)) {
			t.Fatal("invalid preview changed repository")
		}
		if _, err := os.Stat(state); !os.IsNotExist(err) {
			t.Fatalf("invalid preview created state: %v", err)
		}
	}
}

func TestMaintenanceComposesNativeCronPort(t *testing.T) {
	if maintenanceCronPort == nil {
		t.Fatal("maintenance native cron port is not composed")
	}
}

func TestMaintenanceCursorRequiresPTYHelperOnlyWhenEnabled(t *testing.T) {
	root := maintenanceRepo(t, nil)
	bin := filepath.Join(t.TempDir(), "pyxis")
	writeFile(t, bin, []byte("binary"))
	if err := os.Chmod(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	oldExecutable := maintenanceExecutable
	maintenanceExecutable = func() (string, error) { return bin, nil }
	t.Cleanup(func() { maintenanceExecutable = oldExecutable })
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name       string
		harness    entity.Harness
		enabled    bool
		tools      []string
		wantAbsent string
	}{
		{name: "enabled cursor without script", harness: entity.Cursor, enabled: true, tools: []string{"cursor-agent"}, wantAbsent: "script"},
		{name: "enabled cursor with script", harness: entity.Cursor, enabled: true, tools: []string{"cursor-agent", "script"}},
		{name: "disabled cursor", harness: entity.Cursor, enabled: false},
		{name: "other enabled harness", harness: entity.Codex, enabled: true, tools: []string{"codex"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			toolDir := t.TempDir()
			if err := os.Symlink(gitPath, filepath.Join(toolDir, "git")); err != nil {
				t.Fatal(err)
			}
			for _, tool := range test.tools {
				if err := os.Symlink("/usr/bin/true", filepath.Join(toolDir, tool)); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", toolDir)
			document := maintenanceDocument("daily", "inspect", test.enabled)
			document.Jobs[0].Runner.Harness = test.harness
			_, err := maintenanceHostBinding(t.Context(), root, document)
			if test.wantAbsent != "" {
				if err == nil || !strings.Contains(err.Error(), "requires "+test.wantAbsent) {
					t.Fatalf("missing dependency error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("host binding = %v", err)
			}
		})
	}
}

func TestMaintenanceInstallMissingCursorPTYHelperMutatesNoCronOrManifest(t *testing.T) {
	document := maintenanceDocument("daily", "inspect", true)
	document.Jobs[0].Runner.Harness = entity.Cursor
	root := maintenanceRepo(t, &document)
	writeFile(t, filepath.Join(root, ".gitignore"), []byte("/.pyxis/maintenance/logs.jsonl\n"))
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	cron := &maintenanceFakeCron{table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Profile: profile, HostTimezone: "America/Los_Angeles"}}
	oldPort, oldExecutable, oldProcessAccess := maintenanceCronPort, maintenanceExecutable, maintenanceProcessAccess
	maintenanceCronPort = cron
	maintenanceProcessAccess = func(context.Context) error { return nil }
	t.Cleanup(func() {
		maintenanceCronPort, maintenanceExecutable, maintenanceProcessAccess = oldPort, oldExecutable, oldProcessAccess
	})
	bin := filepath.Join(t.TempDir(), "pyxis")
	writeFile(t, bin, []byte("binary"))
	if err := os.Chmod(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	maintenanceExecutable = func() (string, error) { return bin, nil }
	toolDir := t.TempDir()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	for tool, target := range map[string]string{"git": gitPath, "cursor-agent": "/usr/bin/true"} {
		if err := os.Symlink(target, filepath.Join(toolDir, tool)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", toolDir)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := run([]string{"maintenance", "install", "--repo", root}, &stdout, &stderr); code != 1 || cron.installs != 0 || !strings.Contains(stderr.String(), "requires script") {
		t.Fatalf("install=%d writes=%d stdout=%q stderr=%q", code, cron.installs, stdout.String(), stderr.String())
	}
	store, err := installation.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(t.Context()); !errors.Is(err, installation.ErrInstallationRequired) {
		t.Fatalf("missing Cursor prerequisite exposed manifest authority: %v", err)
	}
}
