package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	claimgit "github.com/agenticlab-ai/pyxis/internal/adapters/claim/git"
	nativecron "github.com/agenticlab-ai/pyxis/internal/adapters/cron/native"
	"github.com/agenticlab-ai/pyxis/internal/cliguard"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/admission"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/audit"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/claim"
	maintenancecron "github.com/agenticlab-ai/pyxis/internal/maintenance/cron"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/execution"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/installation"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/policy"
	maintenanceworkspace "github.com/agenticlab-ai/pyxis/internal/maintenance/workspace"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
)

const maintenanceUsage = "pyxis maintenance install --repo <absolute-root> [--preview] | run --repo <absolute-root> --job <id> | status --repo <absolute-root> | uninstall --repo <absolute-root> [--preview]"

var maintenanceCronPort maintenancecron.Port = nativecron.System()
var maintenanceExecutable = os.Executable
var maintenanceProcessAccess = executorruntime.ProbeProcessAccess
var maintenanceNow = time.Now

func init() {
	mustRegisterRootCommand(rootCommand{key: "maintenance", usage: "  maintenance install|run|status|uninstall …\n                           inspect or manage daily repository maintenance policy", handler: maintenanceVerb})
}

func maintenanceVerb(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return maintenanceRefuse(stderr, "a subcommand is required")
	}
	verb, rest := args[0], args[1:]
	if verb != "install" && verb != "run" && verb != "status" && verb != "uninstall" {
		return maintenanceRefuse(stderr, fmt.Sprintf("unknown subcommand %q", verb))
	}
	fs := flag.NewFlagSet("maintenance "+verb, flag.ContinueOnError)
	repo := fs.String("repo", "", "absolute Git worktree root")
	var preview *bool
	var jobID *string
	if verb == "install" || verb == "uninstall" {
		preview = fs.Bool("preview", false, "report without mutation")
	}
	if verb == "run" {
		jobID = fs.String("job", "", "maintenance job id")
	}
	positionals, err := cliguard.Positionals(fs, rest)
	if err != nil {
		return maintenanceMisparse(stderr, err)
	}
	if len(positionals) != 0 {
		return maintenanceRefuse(stderr, fmt.Sprintf("takes no positional arguments (got %q)", positionals[0]))
	}
	if *repo == "" {
		return maintenanceRefuse(stderr, "--repo is required")
	}
	if verb == "run" && (*jobID == "" || !jobs.ValidID(*jobID)) {
		return maintenanceRefuse(stderr, "--job requires a valid lowercase job id")
	}
	var started time.Time
	if verb == "run" {
		started = maintenanceNow().UTC()
	}
	root, err := maintenanceRepository(ctx, *repo)
	if err != nil {
		terminalPrintf(stderr, "pyxis: maintenance %s: %v — nothing was run\n", verb, err)
		return 1
	}
	if verb == "install" || verb == "uninstall" {
		return maintenanceInstall(ctx, verb, root, *preview, stdout, stderr)
	}
	if verb == "run" {
		return maintenanceRun(ctx, root, *jobID, started, stdout, stderr)
	}
	output, err := policy.Status(ctx, root, maintenanceCronPort)
	if err != nil {
		terminalPrintf(stderr, "pyxis: maintenance status: desired policy unavailable: %v\n", err)
		return 1
	}
	terminalPrintf(stdout, "%s", output)
	return 0
}

func maintenanceRun(ctx context.Context, root, jobID string, started time.Time, stdout, stderr io.Writer) int {
	identity, err := audit.NewIdentity()
	if err != nil {
		terminalPrintf(stderr, "pyxis: maintenance run: %v — nothing was run\n", err)
		return 1
	}
	journal, err := audit.Open(ctx, root, nil)
	if err != nil {
		terminalPrintf(stderr, "pyxis: maintenance run: audit admission failed: %v — nothing was run\n", err)
		return 1
	}
	store, err := installation.Open(root)
	if err != nil {
		return maintenanceRunRefusal(ctx, journal, identity, root, jobID, admission.Result{}, err, stderr)
	}
	authority := admission.LockedAuthority{Store: store}
	common, err := maintenanceGit(ctx, root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return maintenanceRunRefusal(ctx, journal, identity, root, jobID, admission.Result{}, fmt.Errorf("resolve enrolled Git common directory: %w", err), stderr)
	}
	service := admission.Service{Authority: authority, Publisher: admission.ControlSource{Repository: root, GitCommon: common, ResolveRemote: func(ctx context.Context, remote string) (string, error) {
		return maintenanceGit(ctx, root, "remote", "get-url", remote)
	}},
		Registry: executorruntime.LiveRegistry(), ProcessSupport: maintenanceProcessAccess}
	result, err := service.Prepare(ctx, admission.Request{JobID: jobID, StartedUTC: started})
	if err != nil {
		return maintenanceRunRefusal(ctx, journal, identity, root, jobID, result, err, stderr)
	}
	if result.Kind == admission.ResultSkipped {
		if _, err := journal.Append(ctx, maintenanceAdmissionEvent(identity, result, audit.OutcomeSkipped, string(result.SkipReason), ""), audit.Consequential); err != nil {
			terminalPrintf(stderr, "pyxis: maintenance run: record skip: %v — nothing was run\n", err)
			return 1
		}
		terminalPrintf(stdout, "maintenance run skipped: job %q is %s\n", jobID, strings.TrimPrefix(string(result.SkipReason), "job_"))
		return 0
	}
	prepared := result.Prepared
	defer func() { _ = prepared.Close() }() // The retained control checkout contains no execution work.
	if _, err := store.LoadRefusals(ctx); err != nil && !errors.Is(err, installation.ErrRefusalsAbsent) {
		return maintenanceCounterFailure(ctx, journal, identity, jobID, stderr)
	}
	claimStore, err := claimgit.New(prepared.Control, jobID)
	if err != nil {
		return maintenanceRunRefusal(ctx, journal, identity, root, jobID, result, err, stderr)
	}
	claims := claim.Service{Store: claimStore, Now: maintenanceNow}
	executed, runErr := (execution.Service{
		Claims: claims, Workspaces: maintenanceworkspace.Preparer{}, Lifecycle: maintenanceExecutionLifecycle{Journal: journal, Identity: identity, Admitted: result, Refusals: store}, Now: maintenanceNow,
	}).Run(ctx, *prepared)
	for _, cause := range maintenanceExecutionCauses(executed, runErr) {
		if countErr := maintenanceRecordCounter(ctx, store, identity.InvocationID, jobID, cause); countErr != nil {
			return maintenanceCounterFailure(ctx, journal, identity, jobID, stderr)
		}
	}
	if runErr != nil {
		terminalPrintf(stderr, "pyxis: maintenance run: %s — no retry was started\n", maintenanceRunFailureCode(executed, runErr))
		return 1
	}
	if executed.Outcome == execution.OutcomeConsumed {
		terminalPrintf(stdout, "maintenance run skipped: job %q date %s is already consumed\n", jobID, prepared.JobDate)
		return 0
	}
	if executed.Outcome != execution.OutcomeSucceeded {
		terminalPrintf(stderr, "pyxis: maintenance run: harness execution ended %s — retained work was not published\n", executed.Outcome)
		return 1
	}
	terminalPrintf(stdout, "maintenance run completed: job %q date %s; process tree quiescence proved\n", jobID, prepared.JobDate)
	return 0
}

type maintenanceAuditAppender interface {
	Append(context.Context, audit.Event, audit.Durability) (audit.Result, error)
}

type maintenanceRefusalResetter interface {
	ResetRefusals(context.Context, string) error
}

type maintenanceExecutionLifecycle struct {
	Journal  maintenanceAuditAppender
	Identity audit.Identity
	Admitted admission.Result
	Refusals maintenanceRefusalResetter
}

func (l maintenanceExecutionLifecycle) Observe(ctx context.Context, observed execution.Observation) error {
	event := maintenanceAdmissionEvent(l.Identity, l.Admitted, "", observed.CauseCode, "")
	event.Event, event.Phase = audit.EventLifecycle, audit.PhaseAdmission
	switch observed.Boundary {
	case execution.BoundaryClaim:
		event.Event, event.Phase, event.Outcome = audit.EventClaim, audit.PhaseClaim, audit.OutcomeUnproved
		switch observed.Result.Claim.Disposition {
		case claim.DispositionOwned:
			event.Outcome = audit.OutcomeWon
		case claim.DispositionConsumed:
			event.Outcome = audit.OutcomeLost
		}
	case execution.BoundaryWorkspace:
		event.Phase, event.Outcome = audit.PhaseWorkspace, audit.OutcomeWon
	case execution.BoundaryAgent:
		event.Phase, event.Outcome = audit.PhaseAgent, audit.OutcomeWon
	case execution.BoundaryTerminal:
		event.Event, event.Phase, event.Outcome = audit.EventRun, audit.PhaseTerminal, maintenanceAuditOutcome(observed.Result.Outcome)
		event.ElapsedMS, event.Usage = observed.Result.ElapsedMS, maintenanceAuditUsage(observed.Result)
		if observed.Result.Retained && observed.Result.Workspace != nil && observed.Result.Claim.Record.OwnerNonce != "" {
			event.References = []audit.Reference{{Kind: "retained_workspace", Location: "maintenance-attempt:" + observed.Result.Claim.Record.OwnerNonce}}
		}
	}
	if record := observed.Result.Claim.Record; record.JobID != "" {
		event.Claim = &audit.Claim{Owner: record.OwnerNonce, Ref: "refs/pyxis/maintenance/" + record.JobID}
	}
	if capture := observed.Result.Capture; capture != nil {
		if event.Route == nil {
			event.Route = &audit.Route{}
		}
		event.Launch = &audit.Route{Harness: observed.Result.Harness.String(), Profile: event.Route.Profile, Model: capture.NativeModel, Effort: capture.NativeEffort,
			Policy: event.Route.Policy, Qualification: event.Route.Qualification}
	}
	if observed.Cause != nil {
		event.Level, event.Cause = audit.LevelError, observed.CauseCode
	} else if event.Outcome == audit.OutcomeIncomplete || event.Outcome == audit.OutcomeUnproved {
		event.Level = audit.LevelWarn
	}
	if l.Refusals != nil && observed.Boundary == execution.BoundaryClaim && (observed.Result.Claim.Disposition == claim.DispositionOwned || observed.Result.Claim.Disposition == claim.DispositionConsumed) {
		if err := l.Refusals.ResetRefusals(ctx, event.JobID); err != nil {
			recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			event.Level, event.Event, event.Phase, event.Outcome, event.CauseCode = audit.LevelError, audit.EventLifecycle, audit.PhaseAudit, audit.OutcomeRefused, "counter_write_failed"
			_, auditErr := l.Journal.Append(recovery, event, audit.Consequential)
			return errors.Join(fmt.Errorf("reset refusal counters after coordination: %w", err), auditErr)
		}
	}
	_, err := l.Journal.Append(ctx, event, audit.Consequential)
	return err
}

func maintenanceRunFailureCode(result execution.Result, err error) string {
	metadata, lifecycle := errors.Is(err, execution.ErrTerminalMetadata), errors.Is(err, execution.ErrLifecycle)
	if metadata && lifecycle {
		return "terminal_metadata_failed; audit_failed"
	}
	if metadata {
		return "terminal_metadata_failed"
	}
	if lifecycle {
		return "audit_failed"
	}
	return "execution_" + string(result.Outcome)
}

func maintenanceAuditOutcome(outcome execution.Outcome) audit.Outcome {
	switch outcome {
	case execution.OutcomeSucceeded:
		return audit.OutcomeSucceeded
	case execution.OutcomeTimedOut:
		return audit.OutcomeTimedOut
	case execution.OutcomeIncomplete:
		return audit.OutcomeIncomplete
	case execution.OutcomeConsumed:
		return audit.OutcomeSkipped
	case execution.OutcomeUnproved:
		return audit.OutcomeUnproved
	case execution.OutcomeRefused:
		return audit.OutcomeRefused
	default:
		return audit.OutcomeFailed
	}
}

func maintenanceAuditUsage(result execution.Result) *audit.Usage {
	usage := &audit.Usage{}
	if result.Usage.TokensObserved {
		usage.Tokens = &result.Usage.Tokens
	}
	if result.Usage.TurnsObserved {
		usage.Turns = &result.Usage.Turns
	}
	if result.Usage.ToolCallsObserved {
		usage.ToolCalls = &result.Usage.ToolCalls
	}
	if usage.Tokens == nil && usage.Turns == nil && usage.ToolCalls == nil {
		return nil
	}
	return usage
}

func maintenanceRunRefusal(ctx context.Context, journal *audit.Writer, identity audit.Identity, root, jobID string, result admission.Result, cause error, stderr io.Writer) int {
	event := maintenanceAdmissionEvent(identity, result, audit.OutcomeRefused, "admission_refused", "")
	event.JobID = jobID
	recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	_, auditErr := journal.Append(recovery, event, audit.Consequential)
	if store, err := installation.Open(root); err == nil {
		if countErr := store.RecordRefusal(recovery, identity.InvocationID, jobID, installation.CauseAdmissionRefused, maintenanceNow().UTC()); countErr != nil {
			return maintenanceCounterFailure(ctx, journal, identity, jobID, stderr)
		}
	}
	terminalPrintf(stderr, "pyxis: maintenance run: %v — nothing was launched\n", errors.Join(cause, auditErr))
	return 1
}

func maintenanceAdmissionEvent(identity audit.Identity, result admission.Result, outcome audit.Outcome, code, cause string) audit.Event {
	event := audit.Event{Level: audit.LevelInfo, Event: audit.EventLifecycle, Phase: audit.PhaseConfiguration, TraceID: identity.TraceID, InvocationID: identity.InvocationID,
		JobID: result.JobID, JobDate: result.JobDate, Timezone: result.Timezone, InstallationGeneration: string(result.Ready.Generation),
		ConfigCommit: result.ConfigCommit, Outcome: outcome, CauseCode: code, Cause: cause}
	if outcome == audit.OutcomeRefused {
		event.Level = audit.LevelError
	}
	selection := result.Selection
	if selection == nil && result.Prepared != nil {
		prepared := result.Prepared
		event.JobDate, event.Timezone, event.ConfigCommit = prepared.JobDate, prepared.Job.Schedule.Timezone, prepared.ConfigCommit
		selection = &prepared.Selection
	}
	if selection != nil {
		event.Route = &audit.Route{Harness: selection.Harness.String(), Profile: selection.NativeProfileID, Model: selection.NativeModel,
			Effort: selection.NativeEffort, Policy: selection.PolicyIdentity, Qualification: selection.QualificationID}
	}
	return event
}

func maintenanceInstall(ctx context.Context, verb, root string, preview bool, stdout, stderr io.Writer) int {
	identity, err := audit.NewIdentity()
	if err != nil {
		terminalPrintf(stderr, "pyxis: maintenance %s: %v\n", verb, err)
		return 1
	}
	var journal *audit.Writer
	if !preview {
		journal, err = audit.Open(ctx, root, nil)
		if err != nil {
			terminalPrintf(stderr, "pyxis: maintenance %s: audit admission failed: %v\n", verb, err)
			return 1
		}
	}
	var document jobs.Document
	if verb == "install" {
		store, storeErr := jobs.New(root)
		if storeErr == nil {
			document, storeErr = store.Load(ctx)
		}
		if storeErr != nil {
			return maintenanceInstallError(ctx, verb, root, identity, journal, storeErr, stderr)
		}
	}
	var binding maintenancecron.InstalledBinding
	if verb == "install" {
		binding, err = maintenanceHostBinding(ctx, root, document)
		if err == nil {
			err = maintenanceProcessAccess(ctx)
		}
		if err != nil {
			return maintenanceInstallError(ctx, verb, root, identity, journal, err, stderr)
		}
	}
	manifest, err := installation.Open(root)
	if err != nil {
		return maintenanceInstallError(ctx, verb, root, identity, journal, err, stderr)
	}
	result, err := (installation.Reconciler{Cron: maintenanceCronPort, Store: manifest, Journal: journal}).Reconcile(ctx, installation.Request{SchemaVersion: installation.RequestSchemaVersion, Document: document, Binding: binding, Identity: identity, Uninstall: verb == "uninstall", Preview: preview})
	if err != nil {
		if !preview && !installation.FailureOwned(err) {
			return maintenanceInstallError(ctx, verb, root, identity, journal, err, stderr)
		}
		if !preview {
			if countErr := maintenanceRecordCounter(ctx, manifest, identity.InvocationID, "installation", installation.CauseInstallationFailed); countErr != nil {
				return maintenanceCounterFailure(ctx, journal, identity, "installation", stderr)
			}
		}
		terminalPrintf(stderr, "pyxis: maintenance %s: %v\n", verb, err)
		return 1
	}
	action := "unchanged"
	if result.Changed {
		action = "changed"
	}
	mode := "complete"
	if preview {
		mode = "preview"
	}
	terminalPrintf(stdout, "maintenance %s %s: %s; owned entries: %d\n", verb, mode, action, result.Owned)
	return 0
}

func maintenanceInstallError(ctx context.Context, verb, root string, identity audit.Identity, journal *audit.Writer, cause error, stderr io.Writer) int {
	if journal != nil {
		recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		changed, owned := false, 0
		_, auditErr := journal.Append(recovery, audit.Event{Level: audit.LevelError, Event: audit.EventInstallation, Phase: audit.PhaseAdmission, TraceID: identity.TraceID, InvocationID: identity.InvocationID, Outcome: audit.OutcomeRefused, Changed: &changed, OwnedEntries: &owned, CauseCode: "admission_refused", Cause: cause.Error()}, audit.Consequential)
		cause = errors.Join(cause, auditErr)
		if store, err := installation.Open(root); err == nil {
			if countErr := store.RecordRefusal(recovery, identity.InvocationID, "installation", installation.CauseInstallationFailed, maintenanceNow().UTC()); countErr != nil {
				return maintenanceCounterFailure(ctx, journal, identity, "installation", stderr)
			}
		}
	}
	terminalPrintf(stderr, "pyxis: maintenance %s: %v — operation failed\n", verb, cause)
	return 1
}

func maintenanceRecordCounter(ctx context.Context, store *installation.Store, invocation, jobID string, cause installation.RefusalCause) error {
	recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	return store.RecordRefusal(recovery, invocation, jobID, cause, maintenanceNow().UTC())
}

var maintenanceRefusalOutcomes = map[execution.Outcome]installation.RefusalCause{
	execution.OutcomeUnproved:   installation.CauseClaimUnproved,
	execution.OutcomeRefused:    installation.CauseExecutionRefused,
	execution.OutcomeTimedOut:   installation.CauseExecutionTimedOut,
	execution.OutcomeIncomplete: installation.CauseExecutionIncomplete,
	execution.OutcomeFailed:     installation.CauseExecutionFailed,
}

func maintenanceExecutionCauses(result execution.Result, err error) []installation.RefusalCause {
	var causes []installation.RefusalCause
	if errors.Is(err, execution.ErrTerminalMetadata) {
		causes = append(causes, installation.CauseTerminalFailed)
	}
	if errors.Is(err, execution.ErrLifecycle) {
		causes = append(causes, installation.CauseAuditFailed)
	}
	if cause := maintenanceRefusalOutcomes[result.Outcome]; cause != "" {
		if result.Outcome == execution.OutcomeFailed && result.Claim.Disposition == claim.DispositionOwned && result.Workspace == nil {
			cause = installation.CauseWorkspaceFailed
		}
		causes = append(causes, cause)
	}
	return causes
}

func maintenanceCounterFailure(ctx context.Context, journal maintenanceAuditAppender, identity audit.Identity, jobID string, stderr io.Writer) int {
	recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	event := audit.Event{Level: audit.LevelError, Event: audit.EventLifecycle, Phase: audit.PhaseAudit, TraceID: identity.TraceID, InvocationID: identity.InvocationID, JobID: jobID, Outcome: audit.OutcomeRefused, CauseCode: "counter_write_failed"}
	_, _ = journal.Append(recovery, event, audit.Consequential)
	terminalPrintf(stderr, "pyxis: maintenance: refusal accounting unavailable; launch withheld\n")
	return 1
}

func maintenanceHostBinding(ctx context.Context, root string, document jobs.Document) (maintenancecron.InstalledBinding, error) {
	executable, err := maintenanceExecutable()
	if err == nil {
		executable, err = filepath.EvalSymlinks(executable)
	}
	if err != nil {
		return maintenancecron.InstalledBinding{}, fmt.Errorf("resolve installed pyxis executable: %w", err)
	}
	executable = filepath.Clean(executable)
	info, err := os.Stat(executable)
	if err != nil || !filepath.IsAbs(executable) || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 || executable == root || strings.HasPrefix(executable, root+string(filepath.Separator)) || strings.Contains(executable, string(filepath.Separator)+"go-build") || strings.HasSuffix(executable, ".test") {
		return maintenancecron.InstalledBinding{}, errors.New("maintenance install requires a stable installed core pyxis executable outside the repository")
	}
	toolPath := os.Getenv("PATH")
	for _, path := range filepath.SplitList(toolPath) {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return maintenancecron.InstalledBinding{}, errors.New("maintenance install requires an absolute, normalized PATH")
		}
	}
	for _, tool := range preflight.Prerequisites(ctx, config.Knobs{}, nil) {
		if tool.Name == "git" && (!tool.Present || tool.BelowFloor) {
			return maintenancecron.InstalledBinding{}, errors.New("maintenance install requires supported git on the validated PATH")
		}
	}
	for _, job := range document.Jobs {
		tool := job.Runner.Harness.String()
		if tool == "cursor" {
			tool = "cursor-agent"
		}
		if job.Enabled {
			tools := []string{tool}
			if tool == "cursor-agent" {
				tools = append(tools, "script")
			}
			for _, dependency := range tools {
				if _, err := exec.LookPath(dependency); err != nil {
					return maintenancecron.InstalledBinding{}, fmt.Errorf("maintenance install requires %s on the validated PATH", dependency)
				}
			}
		}
	}
	return maintenancecron.InstalledBinding{Executable: executable, ToolPath: toolPath, StableExecutable: true}, nil
}

func maintenanceRepository(ctx context.Context, operand string) (string, error) {
	if !filepath.IsAbs(operand) {
		return "", errors.New("--repo must be an absolute path")
	}
	root, err := filepath.EvalSymlinks(operand)
	if err != nil {
		return "", fmt.Errorf("canonicalize --repo: %w", err)
	}
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", errors.New("--repo is not an existing directory")
	}
	top, err := maintenanceGit(ctx, root, "rev-parse", "--path-format=absolute", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("--repo is not a Git worktree: %w", err)
	}
	bare, err := maintenanceGit(ctx, root, "rev-parse", "--is-bare-repository")
	if err != nil || bare != "false" {
		return "", errors.New("--repo must be a non-bare Git worktree")
	}
	canonicalTop, err := filepath.EvalSymlinks(top)
	if err != nil || filepath.Clean(canonicalTop) != root {
		return "", errors.New("--repo must name the Git worktree root itself")
	}
	return root, nil
}

func maintenanceGit(ctx context.Context, root string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"--no-optional-locks", "-C", root}, args...)...)
	command.Env = maintenanceGitEnvironment()
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(output), "\n"), nil
}

func maintenanceGitEnvironment() []string {
	result := make([]string, 0, len(os.Environ()))
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		upper := strings.ToUpper(key)
		if strings.HasPrefix(upper, "GIT_CONFIG_KEY_") || strings.HasPrefix(upper, "GIT_CONFIG_VALUE_") || strings.HasPrefix(upper, "GIT_TRACE") || maintenanceGitRedirect[upper] {
			continue
		}
		result = append(result, item)
	}
	return result
}

var maintenanceGitRedirect = map[string]bool{
	"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_COMMON_DIR": true,
	"GIT_INDEX_FILE": true, "GIT_OBJECT_DIRECTORY": true,
	"GIT_ALTERNATE_OBJECT_DIRECTORIES": true, "GIT_NAMESPACE": true,
	"GIT_CEILING_DIRECTORIES": true, "GIT_CONFIG": true, "GIT_CONFIG_COUNT": true,
	"GIT_CONFIG_GLOBAL": true, "GIT_CONFIG_SYSTEM": true, "GIT_CONFIG_PARAMETERS": true,
	"GIT_EXEC_PATH": true, "GIT_TEMPLATE_DIR": true, "GIT_REPLACE_REF_BASE": true,
}

func maintenanceMisparse(stderr io.Writer, err error) int {
	if errors.Is(err, flag.ErrHelp) {
		terminalPrintf(stderr, "usage: %s\n", maintenanceUsage)
		return 2
	}
	var usageError *cliguard.UsageError
	if errors.As(err, &usageError) {
		return maintenanceRefuse(stderr, usageError.Reason)
	}
	return maintenanceRefuse(stderr, err.Error())
}

func maintenanceRefuse(stderr io.Writer, reason string) int {
	terminalPrintf(stderr, "pyxis: maintenance: %s — nothing was run\nusage: %s\n", reason, maintenanceUsage)
	return 2
}
