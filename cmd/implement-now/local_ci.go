package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
	"github.com/agenticlab-ai/pyxis/internal/executor/drivetogreen"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
)

const localGateContext = "pyxis/local-gate"

type statusHost interface {
	SetStatus(context.Context, string, string, string, string, string) error
	ReadStatus(context.Context, string, string) (port.CheckState, error)
}

// localVerifiedHost persists an actual local gate result on its exact commit.
// The read side therefore survives process restarts and never treats an absent
// hosted CI run or an empty local check set as successful verification.
type localVerifiedHost struct {
	executionHost
	status statusHost
	mode   string
	run    drivetogreen.CommandRunner
	git    func(context.Context, string, ...string) ([]byte, error)
}

func withLocalCI(host executionHost, mode string) (executionHost, error) {
	if mode != "local" && mode != "both" {
		return host, nil
	}
	status, ok := host.(statusHost)
	if !ok {
		return nil, errors.New("selected code host cannot publish local verification")
	}
	return &localVerifiedHost{executionHost: host, status: status, mode: mode, run: codeGateRunner, git: func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		return localGit(ctx, dir, args...)
	}}, nil
}

func localGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).Output()
}

func (h *localVerifiedHost) localRevisionAt(ctx context.Context, dir string) (string, error) {
	status, err := h.git(ctx, dir, "status", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("inspect local verification worktree: %w", err)
	}
	if len(status) != 0 {
		return "", errors.New("local verification worktree has uncommitted changes")
	}
	sha, err := h.git(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("read local verification revision: %w", err)
	}
	if strings.TrimSpace(string(sha)) == "" {
		return "", errors.New("local verification revision is empty")
	}
	return strings.TrimSpace(string(sha)), nil
}

func (h *localVerifiedHost) verifyLocal(ctx context.Context, work port.Workdir) (string, error) {
	if work == nil {
		return "", errors.New("local verification requires a worktree")
	}
	return h.verifyLocalAt(ctx, work.Dir())
}

func (h *localVerifiedHost) verifyLocalAt(ctx context.Context, dir string) (string, error) {
	sha, err := h.localRevisionAt(ctx, dir)
	if err != nil {
		return "", err
	}
	cfg, err := codegatemirror.Load(dir)
	if err != nil {
		return "", err
	}
	required := false
	for _, check := range cfg.Checks {
		required = required || check.Class == codegatemirror.ClassRequired
	}
	if !required {
		return "", errors.New("local CI requires at least one required code-gate check")
	}
	result := h.run(ctx, dir, nil)
	if result.Err != nil {
		return "", result.Err
	}
	if !result.Passed {
		return "", fmt.Errorf("local verification failed: %s", result.Output)
	}
	after, err := h.localRevisionAt(ctx, dir)
	if err != nil {
		return "", err
	}
	if after != sha {
		return "", errors.New("worktree changed during local verification")
	}
	return sha, nil
}

func (h *localVerifiedHost) publishLocal(ctx context.Context, work port.Workdir, want string) error {
	sha, err := h.verifyLocal(ctx, work)
	if err != nil {
		return err
	}
	if sha != want {
		return errors.New("local verification does not cover the published revision")
	}
	return h.status.SetStatus(ctx, sha, localGateContext, "success", "", "Required local code-gate checks passed for this exact commit")
}

func (h *localVerifiedHost) OpenProposal(ctx context.Context, task port.Task, work port.Workdir, description prdescription.Description) (port.Proposal, error) {
	sha, err := h.verifyLocal(ctx, work)
	if err != nil {
		return port.Proposal{}, err
	}
	proposal, err := h.executionHost.OpenProposal(ctx, task, work, description)
	if err != nil {
		return proposal, err
	}
	remote, err := h.ProposalHead(ctx, proposal)
	if err != nil {
		return proposal, err
	}
	if remote != sha {
		return proposal, errors.New("proposal head differs from locally verified revision")
	}
	if err := h.status.SetStatus(ctx, sha, localGateContext, "success", "", "Required local code-gate checks passed for this exact commit"); err != nil {
		return proposal, err
	}
	return proposal, nil
}

func (h *localVerifiedHost) Checks(ctx context.Context, proposal port.Proposal) (port.CheckState, error) {
	sha, err := h.ProposalHead(ctx, proposal)
	if err != nil {
		return port.CheckPending, err
	}
	local, err := h.status.ReadStatus(ctx, sha, localGateContext)
	if err != nil {
		return port.CheckPending, err
	}
	if local != port.CheckGreen {
		return local, nil
	}
	if h.mode == "both" {
		local, err = h.executionHost.Checks(ctx, proposal)
		if err != nil {
			return port.CheckPending, err
		}
	}
	after, err := h.ProposalHead(ctx, proposal)
	if err != nil {
		return port.CheckPending, err
	}
	if after != sha {
		return port.CheckPending, errors.New("proposal changed during local verification read")
	}
	return local, nil
}

func (h *localVerifiedHost) InspectProposal(ctx context.Context, proposal port.Proposal) (port.Landing, string, port.CheckState, error) {
	landing, sha, _, err := h.executionHost.InspectProposal(ctx, proposal)
	if err != nil {
		return landing, sha, port.CheckPending, err
	}
	state, err := h.Checks(ctx, proposal)
	if err != nil {
		return landing, sha, state, err
	}
	after, err := h.ProposalHead(ctx, proposal)
	if err == nil && after != sha {
		err = errors.New("proposal changed during inspection")
	}
	return landing, sha, state, err
}

func (h *localVerifiedHost) FetchFailedLogs(ctx context.Context, proposal port.Proposal) (string, error) {
	if source, ok := h.executionHost.(drivetogreen.LogFetcher); ok {
		return source.FetchFailedLogs(ctx, proposal)
	}
	return "", nil
}
