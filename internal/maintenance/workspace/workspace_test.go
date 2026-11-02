package workspace

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig/harnessconfigtest"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/admission"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/claim"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/installation"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type fixture struct {
	state, enrolled, remote, commit string
	admitted                        admission.Prepared
	owned                           claim.ClaimResult
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	return newJobFixture(t, testJob())
}

func newJobFixture(t *testing.T, job jobs.Job) *fixture {
	t.Helper()
	root := canonical(t, t.TempDir())
	remote := filepath.Join(root, "remote.git")
	enrolled := filepath.Join(root, "enrolled")
	gitOK(t, root, "init", "--bare", "--quiet", remote)
	gitOK(t, root, "init", "--quiet", "-b", "main", enrolled)
	gitOK(t, enrolled, "remote", "add", "origin", remote)
	document := jobs.Document{SchemaVersion: jobs.SchemaVersion, Source: jobs.SourceBinding{Remote: "origin", Branch: "main"}, Jobs: []jobs.Job{job}}
	policy, err := jobs.Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		".pyxis/maintenance/jobs.yaml": policy,
		".pyxis/config.yaml":           []byte(harnessconfigtest.YAML),
		"captured.txt":                 []byte("captured source\n"),
	} {
		path := filepath.Join(enrolled, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gitOK(t, enrolled, "add", ".")
	gitOK(t, enrolled, "commit", "-m", "captured")
	gitOK(t, enrolled, "push", "origin", "main")
	commit := gitOK(t, enrolled, "rev-parse", "HEAD")
	common := canonical(t, gitOK(t, enrolled, "rev-parse", "--path-format=absolute", "--git-common-dir"))
	xdg := canonical(t, t.TempDir())
	state := filepath.Join(xdg, "pyxis")
	if err := os.Mkdir(state, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(state, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", xdg)
	published, err := (admission.ControlSource{Repository: enrolled, GitCommon: common, RemoteURL: remote}).Capture(t.Context(), document.Source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = published.Control.Close() })
	route := nativerouting.SelectedProfile{CatalogID: "catalog", PolicyIdentity: "policy", Harness: entity.Codex, Role: nativerouting.RoleImplementation,
		NativeProfileID: "strong", NativeModel: "model", NativeEffort: "high", Provides: nativerouting.Provides{Capability: executiondemand.CapabilityStrong},
		QualificationID: "qualification", QualificationDigest: "digest", AdapterGeneration: "adapter"}
	ready := installation.ReadyBinding{Repository: installation.Repository(enrolled), Source: document.Source}
	admitted := admission.Prepared{ConfigCommit: published.Commit, DailyIdentity: admission.DailyIdentity{JobID: "daily", JobDate: "2026-10-20"},
		Job: document.Jobs[0], Selection: route, Ready: ready, JobDate: "2026-10-20", Control: published.Control}
	record := claim.Record{SchemaVersion: claim.SchemaVersion, JobID: "daily", JobDate: admitted.JobDate, Timezone: "America/Los_Angeles",
		OwnerNonce: strings.Repeat("a", 64), ConfigCommit: commit, Route: route, ClaimedAt: time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC), Outcome: claim.OutcomeClaimed}
	return &fixture{state: state, enrolled: enrolled, remote: remote, commit: commit, admitted: admitted,
		owned: claim.ClaimResult{Disposition: claim.DispositionOwned, Record: record}}
}

func testJob() jobs.Job {
	return jobs.Job{ID: "daily", Enabled: true, Schedule: jobs.Schedule{Cron: "0 2 * * *", Timezone: "America/Los_Angeles"}, Timeout: "60m",
		Runner: jobs.Runner{Harness: entity.Codex, Routing: executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityStrong,
			Deliberation: executiondemand.DeliberationDeep, Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceElevated}},
		Delivery: &jobs.Delivery{Mode: "draft-pr", Branch: "maintenance/daily"}, Prompt: "maintain this repository"}
}

func TestPrepareIndependentCloneSurvivesControlCloseAndPreservesSources(t *testing.T) {
	f := newFixture(t)
	source, err := f.admitted.ExecutionSource()
	if err != nil {
		t.Fatal(err)
	}
	gitOK(t, source.ControlRepository, "config", "http.extraheader", "Authorization: credential-canary")
	beforeEnrolled, beforeRemote, beforeControl := digestTree(t, f.enrolled), digestTree(t, f.remote), digestTree(t, source.ControlRepository)
	prepared, err := (Preparer{StateRoot: f.state, Probe: allowProbe}).Prepare(t.Context(), f.admitted, f.owned)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Access.Purpose != port.LaunchPurposeImplementation || prepared.Access.Workdir != prepared.Workdir || prepared.Access.RuntimeStateDir != prepared.RuntimeStateDir {
		t.Fatalf("wrong implementation access: %+v", prepared.Access)
	}
	if prepared.Workdir == prepared.RuntimeStateDir || prepared.GitDir == source.ControlRepository || prepared.GitDir == source.EnrolledGitCommon {
		t.Fatalf("storage is not independent: %+v source=%+v", prepared, source)
	}
	for _, path := range []string{prepared.AttemptRoot, prepared.Workdir, executorruntime.TaskStateRoot(prepared.Workdir), prepared.RuntimeStateDir} {
		if err := privateDirectory(path); err != nil {
			t.Fatalf("private attempt path %s: %v", filepath.Base(path), err)
		}
	}
	if got := gitOK(t, prepared.Workdir, "rev-parse", "HEAD"); got != f.commit {
		t.Fatalf("HEAD = %s, want %s", got, f.commit)
	}
	if got := gitOK(t, prepared.Workdir, "branch", "--show-current"); got != "maintenance/daily" {
		t.Fatalf("branch = %q", got)
	}
	if remotes := gitOK(t, prepared.Workdir, "remote"); remotes != "" {
		t.Fatalf("agent clone retained remotes: %q", remotes)
	}
	assertNoSharedObjectFiles(t, source.ControlRepository, prepared.GitDir)
	config, err := os.ReadFile(filepath.Join(prepared.GitDir, "config"))
	if err != nil || strings.Contains(string(config), "credential-canary") {
		t.Fatalf("credential-bearing control config reached clone: %q %v", config, err)
	}
	if after := digestTree(t, f.enrolled); !reflect.DeepEqual(beforeEnrolled, after) {
		t.Fatalf("enrolled checkout changed\nbefore=%v\nafter=%v", beforeEnrolled, after)
	}
	if after := digestTree(t, f.remote); !reflect.DeepEqual(beforeRemote, after) {
		t.Fatalf("remote changed\nbefore=%v\nafter=%v", beforeRemote, after)
	}
	if after := digestTree(t, source.ControlRepository); !reflect.DeepEqual(beforeControl, after) {
		t.Fatalf("control repository changed\nbefore=%v\nafter=%v", beforeControl, after)
	}
	if err := f.admitted.Close(); err != nil {
		t.Fatal(err)
	}
	if got := gitOK(t, prepared.Workdir, "show", "HEAD:captured.txt"); got != "captured source" {
		t.Fatalf("clone unusable after control close: %q", got)
	}
}

func TestPrepareSeparatesAttemptsAndRefusesExistingClaimLocation(t *testing.T) {
	f := newFixture(t)
	first, err := (Preparer{StateRoot: f.state, Probe: allowProbe}).Prepare(t.Context(), f.admitted, f.owned)
	if err != nil {
		t.Fatal(err)
	}
	secondOwned := f.owned
	secondOwned.Record.OwnerNonce = strings.Repeat("b", 64)
	second, err := (Preparer{StateRoot: f.state, Probe: allowProbe}).Prepare(t.Context(), f.admitted, secondOwned)
	if err != nil {
		t.Fatal(err)
	}
	if first.AttemptRoot == second.AttemptRoot || first.Workdir == second.Workdir || first.GitDir == second.GitDir || first.RuntimeStateDir == second.RuntimeStateDir {
		t.Fatalf("attempts share storage: first=%+v second=%+v", first, second)
	}
	marker := filepath.Join(second.Workdir, "other-attempt")
	if err := os.WriteFile(marker, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (Preparer{StateRoot: f.state, Probe: allowProbe}).Prepare(t.Context(), f.admitted, f.owned); err == nil {
		t.Fatal("existing attempt location was resumed or replaced")
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "preserve" {
		t.Fatalf("other attempt changed: %q %v", got, err)
	}
}

func TestPrepareRefusesAuthorityPathsPermissionsAndUnsafeConfig(t *testing.T) {
	t.Run("claim", func(t *testing.T) {
		f := newFixture(t)
		f.owned.Disposition = claim.DispositionConsumed
		if _, err := (Preparer{StateRoot: f.state, Probe: allowProbe}).Prepare(t.Context(), f.admitted, f.owned); err == nil {
			t.Fatal("consumed foreign claim prepared a workspace")
		}
	})
	t.Run("symlink", func(t *testing.T) {
		f := newFixture(t)
		redirect := t.TempDir()
		if err := os.Symlink(redirect, filepath.Join(f.state, "maintenance", "attempts")); err != nil {
			t.Fatal(err)
		}
		if _, err := (Preparer{StateRoot: f.state, Probe: allowProbe}).Prepare(t.Context(), f.admitted, f.owned); err == nil {
			t.Fatal("symlink-redirected attempt parent prepared a workspace")
		}
		entries, err := os.ReadDir(redirect)
		if err != nil || len(entries) != 0 {
			t.Fatalf("refusal wrote through symlink: %v %v", entries, err)
		}
	})
	t.Run("permissions", func(t *testing.T) {
		f := newFixture(t)
		maintenance := filepath.Join(f.state, "maintenance")
		if err := os.Chmod(maintenance, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := (Preparer{StateRoot: f.state, Probe: allowProbe}).Prepare(t.Context(), f.admitted, f.owned); err == nil {
			t.Fatal("public attempt root prepared a workspace")
		}
		info, err := os.Stat(maintenance)
		if err != nil || ownedBy(info, os.Geteuid()^1) {
			t.Fatalf("foreign ownership was accepted: %v", err)
		}
	})
	t.Run("overlap", func(t *testing.T) {
		f := newFixture(t)
		if err := os.Chmod(f.enrolled, 0o700); err != nil {
			t.Fatal(err)
		}
		before := digestTree(t, f.enrolled)
		if _, err := (Preparer{StateRoot: f.enrolled, Probe: allowProbe}).Prepare(t.Context(), f.admitted, f.owned); err == nil {
			t.Fatal("overlapping attempt root prepared a workspace")
		}
		if after := digestTree(t, f.enrolled); !reflect.DeepEqual(before, after) {
			t.Fatalf("overlap refusal changed enrolled checkout\nbefore=%v\nafter=%v", before, after)
		}
	})
	t.Run("config", func(t *testing.T) {
		f := newFixture(t)
		prepared, err := (Preparer{StateRoot: f.state, Probe: allowProbe}).Prepare(t.Context(), f.admitted, f.owned)
		if err != nil {
			t.Fatal(err)
		}
		source, _ := f.admitted.ExecutionSource()
		protected, _ := validateSource(f.admitted, source)
		env := executorruntime.LaunchEnvironment(port.LaunchSpec{Purpose: port.LaunchPurposeImplementation, Workdir: prepared.Workdir, RuntimeStateDir: prepared.RuntimeStateDir})
		for _, setting := range []struct{ key, value string }{
			{"credential.helper", "!credential-canary"}, {"http.extraheader", "Authorization: credential-canary"},
			{"include.path", "/credential-canary"}, {"core.hooksPath", "/credential-canary"}, {"core.symlinks", "credential-canary"},
		} {
			gitOK(t, prepared.Workdir, "config", "--local", setting.key, setting.value)
			_, err := validateClone(t.Context(), prepared.Workdir, prepared.RuntimeStateDir, f.commit, prepared.HeadBranch, env, protected)
			if err == nil || strings.Contains(err.Error(), "credential-canary") {
				t.Fatalf("unsafe %s refusal = %v", setting.key, err)
			}
			gitOK(t, prepared.Workdir, "config", "--local", "--unset-all", setting.key)
		}
	})
}

func TestPrepareRetainsPartialStateAndNamedUnsupportedRefusal(t *testing.T) {
	f := newFixture(t)
	f.admitted.ConfigCommit = strings.Repeat("f", 40)
	f.owned.Record.ConfigCommit = f.admitted.ConfigCommit
	_, err := (Preparer{StateRoot: f.state, Probe: allowProbe}).Prepare(t.Context(), f.admitted, f.owned)
	if err == nil {
		t.Fatal("missing captured commit prepared a workspace")
	}
	attempt := filepath.Join(f.state, "maintenance", "attempts", f.admitted.Ready.Repository.SHA256, "daily", "2026-10-20", f.owned.Record.OwnerNonce)
	if _, err := os.Stat(filepath.Join(attempt, "source-runtime")); err != nil {
		t.Fatalf("partial runtime state was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(attempt, "source", ".git")); err != nil {
		t.Fatalf("partial clone was removed: %v", err)
	}

	other := newFixture(t)
	_, err = (Preparer{StateRoot: other.state, Probe: func(context.Context, *executorruntime.CommandAccess) error {
		return executorruntime.ErrAccessUnsupported
	}}).Prepare(t.Context(), other.admitted, other.owned)
	if !errors.Is(err, executorruntime.ErrAccessUnsupported) {
		t.Fatalf("unsupported confinement refusal = %v", err)
	}
}

func TestPreparedAccessEnforcesAttemptWriteBoundary(t *testing.T) {
	f := newFixture(t)
	prepared, err := (Preparer{StateRoot: f.state}).Prepare(t.Context(), f.admitted, f.owned)
	if errors.Is(err, executorruntime.ErrAccessUnsupported) {
		t.Skipf("OS confinement unsupported: %v", err)
	} else if err != nil {
		t.Fatal(err)
	}
	source, _ := f.admitted.ExecutionSource()
	other := filepath.Join(filepath.Dir(prepared.AttemptRoot), strings.Repeat("c", 64))
	if err := os.Mkdir(other, 0o700); err != nil {
		t.Fatal(err)
	}
	otherFile := filepath.Join(other, "preserve")
	enrolledFile := filepath.Join(f.enrolled, "captured.txt")
	controlFile := filepath.Join(source.ControlRepository, "config")
	for _, path := range []string{otherFile} {
		if err := os.WriteFile(path, []byte("preserve"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestConfinementHelper", "--", prepared.Workdir, prepared.RuntimeStateDir,
		filepath.Join(prepared.GitDir, "config"), enrolledFile, controlFile, otherFile, filepath.Join(prepared.AttemptRoot, attemptMarker))
	cmd.Dir = prepared.Workdir
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "GOCOVERDIR=") && !strings.HasPrefix(value, "TMPDIR=") && !strings.HasPrefix(value, "TMP=") && !strings.HasPrefix(value, "TEMP=") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, "GOCOVERDIR="+prepared.RuntimeStateDir, "TMPDIR="+filepath.Join(prepared.RuntimeStateDir, "tmp"),
		"TMP="+filepath.Join(prepared.RuntimeStateDir, "tmp"), "TEMP="+filepath.Join(prepared.RuntimeStateDir, "tmp"))
	if err := executorruntime.ConfineCommand(t.Context(), cmd, prepared.Access); errors.Is(err, executorruntime.ErrAccessUnsupported) {
		t.Skipf("OS confinement unsupported: %v", err)
	} else if err != nil {
		t.Fatal(err)
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("confined boundary helper: %v: %s", err, output)
	}
	if got, _ := os.ReadFile(otherFile); string(got) != "preserve" {
		t.Fatalf("other attempt changed: %q", got)
	}
	if got, err := os.ReadFile(filepath.Join(prepared.AttemptRoot, attemptMarker)); err != nil || string(got) != "maintenance-attempt/v1\n" {
		t.Fatalf("attempt ownership marker changed: %q %v", got, err)
	}
}

func TestConfinementHelper(t *testing.T) {
	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 {
		return
	}
	paths := os.Args[separator+1:]
	if len(paths) != 7 {
		os.Exit(10)
	}
	for _, path := range []string{filepath.Join(paths[0], "agent-write"), filepath.Join(paths[1], "agent-state")} {
		if err := os.WriteFile(path, []byte("allowed"), 0o600); err != nil {
			os.Exit(11)
		}
	}
	for _, path := range paths[2:] {
		if err := os.WriteFile(path, []byte("forbidden"), 0o600); err == nil {
			os.Exit(12)
		}
	}
}

func allowProbe(context.Context, *executorruntime.CommandAccess) error { return nil }

func canonical(t *testing.T, path string) string {
	t.Helper()
	result, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func gitOK(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func digestTree(t *testing.T, root string) []string {
	t.Helper()
	var result []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		result = append(result, fmt.Sprintf("%s %x", rel, sha256.Sum256(data)))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(result)
	return result
}

func assertNoSharedObjectFiles(t *testing.T, source, cloneGit string) {
	t.Helper()
	sourceObjects := filepath.Join(source, "objects")
	cloneObjects := filepath.Join(cloneGit, "objects")
	if err := filepath.WalkDir(cloneObjects, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(cloneObjects, path)
		if err != nil {
			return err
		}
		sourceInfo, err := os.Stat(filepath.Join(sourceObjects, rel))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		cloneInfo, err := os.Stat(path)
		if err != nil {
			return err
		}
		if os.SameFile(sourceInfo, cloneInfo) {
			return fmt.Errorf("shared object file %s", rel)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareRefusesNestedAttemptStateRoots(t *testing.T) {
	for _, location := range []string{"runtime", "source", "git", "attempt", "runtime child"} {
		t.Run(location, func(t *testing.T) {
			f := newFixture(t)
			first, err := (Preparer{StateRoot: f.state, Probe: allowProbe}).Prepare(t.Context(), f.admitted, f.owned)
			if err != nil {
				t.Fatal(err)
			}
			root := first.RuntimeStateDir
			switch location {
			case "source":
				root = first.Workdir
			case "git":
				root = first.GitDir
			case "attempt":
				root = first.AttemptRoot
			case "runtime child":
				root = filepath.Join(first.RuntimeStateDir, "nested-state")
				if err := os.Mkdir(root, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			next, owned := f.admitted, f.owned
			next.JobDate, next.DailyIdentity.JobDate = "2026-10-21", "2026-10-21"
			owned.Record.JobDate = next.JobDate
			owned.Record.OwnerNonce = strings.Repeat("b", 64)
			owned.Record.ClaimedAt = owned.Record.ClaimedAt.Add(24 * time.Hour)
			before := digestTree(t, first.AttemptRoot)
			if _, err := (Preparer{StateRoot: root, Probe: allowProbe}).Prepare(t.Context(), next, owned); err == nil {
				t.Fatal("another attempt was allocated within prior attempt storage")
			} else if !strings.Contains(err.Error(), "another maintenance attempt") {
				t.Fatalf("unexpected nested-attempt refusal: %v", err)
			}
			if after := digestTree(t, first.AttemptRoot); !reflect.DeepEqual(before, after) {
				t.Fatal("nested-attempt refusal changed prior attempt storage")
			}
		})
	}
}

func TestPrepareChecksStateRootOwnershipPermissions(t *testing.T) {
	for _, mode := range []os.FileMode{0o777, 0o770, 0o755} {
		t.Run(fmt.Sprintf("%04o", mode), func(t *testing.T) {
			f := newFixture(t)
			if err := os.Chmod(f.state, mode); err != nil {
				t.Fatal(err)
			}
			_, err := (Preparer{StateRoot: f.state, Probe: allowProbe}).Prepare(t.Context(), f.admitted, f.owned)
			if mode == 0o755 {
				if err != nil {
					t.Fatalf("process-owned root without shared writes refused: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "without group or world writes") {
				t.Fatalf("unsafe shared-writable root result: %v", err)
			}
		})
	}
}

func TestPrepareRefusesCaseAliasProtectedStorage(t *testing.T) {
	for _, storage := range []string{"enrolled", "git", "control"} {
		t.Run(storage, func(t *testing.T) {
			f := newFixture(t)
			source, err := f.admitted.ExecutionSource()
			if err != nil {
				t.Fatal(err)
			}
			protected := f.enrolled
			switch storage {
			case "git":
				protected = source.EnrolledGitCommon
			case "control":
				protected = source.ControlRepository
			}
			alias := filepath.Join(filepath.Dir(protected), strings.ToUpper(filepath.Base(protected)))
			aliased, err := os.Stat(alias)
			if errors.Is(err, os.ErrNotExist) {
				t.Skip("filesystem is case-sensitive")
			}
			if err != nil {
				t.Fatal(err)
			}
			original, err := os.Stat(protected)
			if err != nil || !os.SameFile(original, aliased) {
				t.Fatalf("case alias does not identify protected storage: %v", err)
			}
			before := digestTree(t, protected)
			if _, err := (Preparer{StateRoot: alias, Probe: allowProbe}).Prepare(t.Context(), f.admitted, f.owned); err == nil {
				t.Fatal("case alias of protected storage accepted")
			}
			if after := digestTree(t, protected); !reflect.DeepEqual(before, after) {
				t.Fatal("case-alias refusal changed protected storage")
			}
		})
	}
}

func TestProtectedOverlapRequiresFilesystemIdentityProof(t *testing.T) {
	root := canonical(t, t.TempDir())
	protected := filepath.Join(root, "protected")
	nested := filepath.Join(protected, "nested")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(protected, alias); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, candidate, protected string
	}{
		{"alias child", filepath.Join(alias, "missing-child"), protected},
		{"alias parent", alias, nested},
	} {
		t.Run(test.name, func(t *testing.T) {
			overlap, err := overlapsProtected(t.Context(), test.candidate, []string{test.protected})
			if err != nil || !overlap {
				t.Fatalf("physical overlap not proved: %v %v", overlap, err)
			}
		})
	}
	if _, err := overlapsProtected(t.Context(), filepath.Join(root, "candidate"), []string{filepath.Join(root, "missing-protected")}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing protection identity did not refuse: %v", err)
	}
	if _, err := overlapsProtected(t.Context(), filepath.Join(root, "invalid\x00child"), []string{protected}); err == nil {
		t.Fatal("unreadable candidate identity did not refuse")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := overlapsProtected(ctx, filepath.Join(root, "candidate"), []string{protected}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled identity proof did not refuse: %v", err)
	}
}

func TestPrepareValidPublishedJobWithoutDelivery(t *testing.T) {
	job := testJob()
	job.Delivery = nil
	f := newJobFixture(t, job)
	before := digestTree(t, f.enrolled)
	prepared, err := (Preparer{StateRoot: f.state, Probe: allowProbe}).Prepare(t.Context(), f.admitted, f.owned)
	if err != nil {
		t.Fatalf("prepare valid published job without delivery: %v", err)
	}
	if prepared.HeadBranch == "" || prepared.HeadBranch == f.admitted.Ready.Source.Branch {
		t.Fatalf("missing private attempt branch: %q", prepared.HeadBranch)
	}
	if got := gitOK(t, prepared.Workdir, "branch", "--show-current"); got != prepared.HeadBranch {
		t.Fatalf("attempt branch %q, want %q", got, prepared.HeadBranch)
	}
	if got := gitOK(t, prepared.Workdir, "rev-parse", "HEAD"); got != f.commit {
		t.Fatalf("captured revision %q, want %q", got, f.commit)
	}
	policy, err := os.ReadFile(filepath.Join(prepared.Workdir, ".pyxis", "maintenance", "jobs.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := jobs.Decode(policy)
	if err != nil || len(document.Jobs) != 1 || document.Jobs[0].Delivery != nil {
		t.Fatalf("captured no-delivery policy changed: %+v %v", document, err)
	}
	if after := digestTree(t, f.enrolled); !reflect.DeepEqual(before, after) {
		t.Fatal("enrolled checkout changed")
	}
}
