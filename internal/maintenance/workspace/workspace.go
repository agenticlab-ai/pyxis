// Package workspace prepares private independent maintenance execution clones.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/admission"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/claim"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/installation"
	"github.com/agenticlab-ai/pyxis/internal/statedir"
)

const capturedRemote = "captured"
const attemptMarker = ".pyxis-maintenance-attempt"

type AccessProbe func(context.Context, *executorruntime.CommandAccess) error

type Preparer struct {
	StateRoot string
	Probe     AccessProbe
}

type Prepared struct {
	AttemptRoot     string
	Workdir         string
	GitDir          string
	RuntimeStateDir string
	HeadBranch      string
	Access          *executorruntime.CommandAccess
}

func (p Preparer) Prepare(ctx context.Context, admitted admission.Prepared, owned claim.ClaimResult) (Prepared, error) {
	if err := validateAuthority(admitted, owned); err != nil {
		return Prepared{}, err
	}
	source, err := admitted.ExecutionSource()
	if err != nil {
		return Prepared{}, err
	}
	protected, err := validateSource(admitted, source)
	if err != nil {
		return Prepared{}, err
	}
	root := p.StateRoot
	if root == "" {
		root, err = statedir.Root()
		if err != nil {
			return Prepared{}, err
		}
	}
	root, err = canonicalDirectory(root)
	if err != nil {
		return Prepared{}, fmt.Errorf("maintenance attempt state root: %w", err)
	}
	if err := validateStateRoot(ctx, root); err != nil {
		return Prepared{}, err
	}
	children := []string{"maintenance", "attempts", admitted.Ready.Repository.SHA256, admitted.Job.ID, admitted.JobDate}
	candidate := filepath.Join(append([]string{root}, children...)...)
	overlap, err := overlapsProtected(ctx, candidate, protected)
	if err != nil {
		return Prepared{}, err
	}
	if overlap {
		return Prepared{}, errors.New("maintenance attempt storage overlaps enrolled or control storage")
	}
	parent, err := ensurePrivateChildren(root, children...)
	if err != nil {
		return Prepared{}, err
	}
	attempt := filepath.Join(parent, owned.Record.OwnerNonce)
	if err := os.Mkdir(attempt, 0o700); err != nil {
		return Prepared{}, fmt.Errorf("allocate fresh maintenance attempt: %w", err)
	}
	// This supervisor-owned marker is outside both harness-writable roots.
	if err := os.WriteFile(filepath.Join(attempt, attemptMarker), []byte("maintenance-attempt/v1\n"), 0o600); err != nil {
		return Prepared{}, fmt.Errorf("mark maintenance attempt ownership: %w", err)
	}
	work := filepath.Join(attempt, "source")
	if err := os.Mkdir(work, 0o700); err != nil {
		return Prepared{}, fmt.Errorf("create maintenance attempt workdir: %w", err)
	}
	state, _, err := executorruntime.PrepareTaskState(ctx, work, admitted.Job.Runner.Harness)
	if err != nil {
		return Prepared{}, fmt.Errorf("prepare maintenance runtime state: %w", err)
	}
	branch := "maintenance/attempt-" + owned.Record.OwnerNonce
	if admitted.Job.Delivery != nil {
		branch = admitted.Job.Delivery.Branch
	}
	spec := port.LaunchSpec{Purpose: port.LaunchPurposeImplementation, Workdir: work, RuntimeStateDir: state, HeadBranch: branch}
	env := executorruntime.LaunchEnvironment(spec)
	if _, err := git(ctx, "", env, "clone", "--quiet", "--no-checkout", "--no-local", "--no-hardlinks", "--origin", capturedRemote, source.ControlRepository, work); err != nil {
		return Prepared{}, fmt.Errorf("clone captured maintenance source: %w", err)
	}
	if _, err := git(ctx, work, env, "fetch", "--quiet", "--no-tags", "--no-write-fetch-head", capturedRemote, "+refs/pyxis/maintenance-source:refs/pyxis/maintenance-source"); err != nil {
		return Prepared{}, fmt.Errorf("copy captured maintenance objects: %w", err)
	}
	if _, err := git(ctx, work, env, "remote", "remove", capturedRemote); err != nil {
		return Prepared{}, fmt.Errorf("remove maintenance control remote: %w", err)
	}
	if _, err := git(ctx, work, env, "checkout", "--quiet", "--no-recurse-submodules", "-b", spec.HeadBranch, admitted.ConfigCommit, "--"); err != nil {
		return Prepared{}, fmt.Errorf("checkout captured maintenance source: %w", err)
	}
	gitDir, err := validateClone(ctx, work, state, admitted.ConfigCommit, spec.HeadBranch, env, protected)
	if err != nil {
		return Prepared{}, err
	}
	access := executorruntime.LaunchAccess(spec)
	probe := p.Probe
	if probe == nil {
		probe = probeAccess
	}
	if err := probe(ctx, access); err != nil {
		return Prepared{}, fmt.Errorf("probe maintenance execution access: %w", err)
	}
	return Prepared{AttemptRoot: attempt, Workdir: work, GitDir: gitDir, RuntimeStateDir: state, HeadBranch: spec.HeadBranch, Access: access}, nil
}

func validateAuthority(admitted admission.Prepared, owned claim.ClaimResult) error {
	if owned.Disposition != claim.DispositionOwned || owned.Record.Validate() != nil || owned.Record.Outcome != claim.OutcomeClaimed {
		return errors.New("maintenance workspace requires a positively owned active claim")
	}
	if owned.Record.JobID != admitted.Job.ID || owned.Record.JobDate != admitted.JobDate || owned.Record.ConfigCommit != admitted.ConfigCommit ||
		owned.Record.Timezone != admitted.Job.Schedule.Timezone || !reflect.DeepEqual(owned.Record.Route, admitted.Selection) {
		return errors.New("maintenance owned claim does not match captured admission")
	}
	if admitted.DailyIdentity.JobID != admitted.Job.ID || admitted.DailyIdentity.JobDate != admitted.JobDate {
		return errors.New("maintenance admission daily identity is inconsistent")
	}
	if admitted.Job.Delivery != nil && strings.TrimSpace(admitted.Job.Delivery.Branch) == "" {
		return errors.New("maintenance workspace requires a configured attempt branch")
	}
	return nil
}

func validateSource(admitted admission.Prepared, source admission.ExecutionSource) ([]string, error) {
	if admitted.Ready.Repository != installation.Repository(admitted.Ready.Repository.Root) {
		return nil, errors.New("maintenance admission has an invalid repository identity")
	}
	paths := []string{source.ControlRepository, source.EnrolledRepository, source.EnrolledGitCommon}
	for index, path := range paths {
		canonical, err := canonicalDirectory(path)
		if err != nil {
			return nil, fmt.Errorf("canonicalize maintenance protected storage: %w", err)
		}
		paths[index] = canonical
	}
	if err := privateDirectory(paths[0]); err != nil {
		return nil, fmt.Errorf("maintenance control storage is unsafe: %w", err)
	}
	if paths[1] != admitted.Ready.Repository.Root || anyOverlap(paths[0], paths[1:]) {
		return nil, errors.New("maintenance captured source identity is overlapping or does not match admission")
	}
	return paths, nil
}

func validateClone(ctx context.Context, work, state, commit, branch string, env, protected []string) (string, error) {
	gitDir, err := canonicalDirectory(filepath.Join(work, ".git"))
	if err != nil {
		return "", fmt.Errorf("inspect maintenance clone Git directory: %w", err)
	}
	settings, err := git(ctx, work, env, "config", "--local", "--null", "--list")
	if err != nil {
		return "", errors.New("inspect maintenance clone config")
	}
	for setting := range strings.SplitSeq(strings.TrimSuffix(string(settings), "\x00"), "\x00") {
		key, value, found := strings.Cut(setting, "\n")
		if !found || !safeLocalConfig(strings.ToLower(key), strings.ToLower(value)) {
			return "", fmt.Errorf("maintenance clone contains unsafe local Git setting %q", key)
		}
	}
	out, err := git(ctx, work, env, "rev-parse", "--path-format=absolute", "HEAD", "--absolute-git-dir", "--git-common-dir", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("inspect maintenance clone: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 4 || lines[0] != commit || lines[3] != branch {
		return "", errors.New("maintenance clone does not contain the exact captured revision and branch")
	}
	if lines[1] != gitDir || lines[2] != gitDir || work == gitDir || anyOverlap(work, protected) || anyOverlap(state, append(protected, work, gitDir)) || anyOverlap(gitDir, protected) {
		return "", errors.Join(errors.New("maintenance source, Git, runtime, and protected storage must be distinct"), err)
	}
	for _, path := range []string{work, executorruntime.TaskStateRoot(work), state} {
		if err := privateDirectory(path); err != nil {
			return "", fmt.Errorf("maintenance attempt storage is unsafe: %w", err)
		}
	}
	if _, err := os.Lstat(filepath.Join(gitDir, "objects", "info", "alternates")); !errors.Is(err, os.ErrNotExist) {
		return "", errors.New("maintenance clone uses object alternates")
	}
	return gitDir, nil
}

func safeLocalConfig(key, value string) bool {
	switch key {
	case "core.repositoryformatversion":
		return value == "0" || value == "1"
	case "core.bare":
		return value == "false"
	case "core.filemode", "core.logallrefupdates", "core.ignorecase", "core.precomposeunicode", "core.symlinks", "core.protecthfs", "core.protectntfs":
		return value == "true" || value == "false"
	case "extensions.objectformat":
		return value == "sha1" || value == "sha256"
	default:
		return false
	}
}

func probeAccess(ctx context.Context, access *executorruntime.CommandAccess) error {
	cmd := exec.CommandContext(ctx, "/usr/bin/true")
	cmd.Dir = access.Workdir
	cmd.Env = executorruntime.LaunchEnvironment(port.LaunchSpec{Purpose: access.Purpose, Workdir: access.Workdir, RuntimeStateDir: access.RuntimeStateDir})
	if err := executorruntime.ConfineCommand(ctx, cmd, access); err != nil {
		return err
	}
	return cmd.Run()
}

func git(ctx context.Context, dir string, env []string, args ...string) ([]byte, error) {
	full := append([]string{"--no-replace-objects", "--literal-pathspecs", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null"}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Dir, cmd.Env = dir, env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func canonicalDirectory(path string) (string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", errors.New("expected a canonical absolute directory")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	if resolved != path {
		return "", errors.New("directory path is symlink-redirected")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", errors.Join(errors.New("expected an existing directory"), err)
	}
	return resolved, nil
}

func validateStateRoot(ctx context.Context, root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("inspect maintenance state ownership: %w", err)
	}
	if !info.IsDir() || !ownedByProcess(info) || info.Mode().Perm()&0o022 != 0 {
		return errors.New("maintenance state root must be process-owned without group or world writes")
	}
	for ancestor := root; ; ancestor = filepath.Dir(ancestor) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := os.Lstat(filepath.Join(ancestor, attemptMarker)); err == nil {
			return errors.New("maintenance state root overlaps another maintenance attempt")
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect maintenance attempt ancestry: %w", err)
		}
		if filepath.Dir(ancestor) == ancestor {
			return nil
		}
	}
}

func ensurePrivateChildren(root string, children ...string) (string, error) {
	path := root
	for _, child := range children {
		path = filepath.Join(path, child)
		if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("create maintenance attempt parent: %w", err)
		}
		if err := privateDirectory(path); err != nil {
			return "", fmt.Errorf("maintenance attempt parent is unsafe: %w", err)
		}
	}
	return path, nil
}

func privateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return errors.Join(errors.New("directory must be real, private, and process-owned"), err)
	}
	if !info.IsDir() || info.Mode().Perm()&0o077 != 0 || !ownedByProcess(info) {
		return fmt.Errorf("directory must be real, private, and process-owned (mode %04o)", info.Mode().Perm())
	}
	return nil
}

func overlapsProtected(ctx context.Context, path string, others []string) (bool, error) {
	if anyOverlap(path, others) {
		return true, nil
	}
	for _, other := range others {
		for _, pair := range [][2]string{{other, path}, {path, other}} {
			ancestor, err := os.Stat(pair[0])
			if err != nil {
				if pair[0] == path && errors.Is(err, os.ErrNotExist) {
					continue
				}
				return false, fmt.Errorf("inspect protected maintenance storage: %w", err)
			}
			for directory := pair[1]; ; directory = filepath.Dir(directory) {
				if err := ctx.Err(); err != nil {
					return false, err
				}
				info, err := os.Stat(directory)
				if err == nil && os.SameFile(ancestor, info) {
					return true, nil
				}
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					return false, fmt.Errorf("inspect maintenance storage ancestry: %w", err)
				}
				if filepath.Dir(directory) == directory {
					break
				}
			}
		}
	}
	return false, nil
}

func anyOverlap(path string, others []string) bool {
	for _, other := range others {
		if contains(path, other) || contains(other, path) {
			return true
		}
	}
	return false
}

func contains(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
