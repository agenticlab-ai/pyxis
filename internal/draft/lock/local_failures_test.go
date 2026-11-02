package lock

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	localgit "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit"
	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit/localgittest"
	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
)

type faultyLocalProposals struct {
	codehost.DraftProposals
	failure string
}

func (s faultyLocalProposals) Read(ctx context.Context, id string) (codehost.ProposalGeneration, error) {
	if s.failure == "read" {
		return codehost.ProposalGeneration{}, errors.New("simulated unreadable proposal")
	}
	g, err := s.DraftProposals.Read(ctx, id)
	switch s.failure {
	case "open":
		g.Proposal.State = "open"
	case "generation":
		g.OID = strings.Repeat("f", 40)
	case "version":
		g.Proposal.ID = "draft-widget-v01"
	case "branch":
		g.Proposal.Branch = "refs/heads/manual-draft"
	}
	return g, err
}

func (s faultyLocalProposals) Mainline(ctx context.Context) (codehost.Readiness, error) {
	if s.failure == "mainline" {
		return codehost.Readiness{}, errors.New("simulated unreadable mainline")
	}
	r, err := s.DraftProposals.Mainline(ctx)
	if s.failure == "target" {
		r.MainlineOID = strings.Repeat("f", 40)
	}
	return r, err
}

func (s faultyLocalProposals) List(ctx context.Context) ([]codehost.ProposalGeneration, error) {
	if s.failure == "list" {
		return nil, errors.New("simulated unreadable proposal inventory")
	}
	return s.DraftProposals.List(ctx)
}

func refuseLocalCommand(prefix []string, output string) lifecycle.RunCommand {
	return func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) >= len(prefix) && slices.Equal(args[:len(prefix)], prefix) {
			if output != "" {
				return []byte(output), nil
			}
			return nil, errors.New("simulated interrupted Git operation")
		}
		return draftshell.RunLocalGit(ctx, dir, env, name, args...)
	}
}

type contradictoryLocalLandings struct{ codehost.DraftLandings }

func (s contradictoryLocalLandings) VerifyDraftLock(ctx context.Context, feature string, version int) (codehost.LocalDraftLock, error) {
	lock, err := s.DraftLandings.VerifyDraftLock(ctx, feature, version)
	lock.ProposalsSinceOpened++
	return lock, err
}

func TestLocalLockRefusesIncompletePortsAndInterruptedEvidenceReads(t *testing.T) {
	localgittest.Skip(t)
	root, cfg, options, local := localPreparationFixture(t, true)
	if _, err := PrepareLocal(t.Context(), root, cfg, "widget", draftshell.RunLocalGit, local); err != nil {
		t.Fatal(err)
	}
	receipts := localgit.NewReceipts(options)
	binding, err := receipts.Binding(t.Context(), "draft-widget-v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := receipts.Record(t.Context(), binding, "operator", ""); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, port, output, want string
		command                  []string
	}{
		{name: "missing memory", port: "nil", want: "requires repository"},
		{name: "unreadable proposal", port: "read", want: "simulated"},
		{name: "unprepared proposal", port: "open", want: "review"},
		{name: "wrong generation", port: "generation", want: "review"},
		{name: "unreadable mainline", port: "mainline", want: "simulated"},
		{name: "stale target", port: "target", want: "proposal moved"},
		{name: "noncanonical version", port: "version", want: "review"},
		{name: "wrong branch", port: "branch", want: "review"},
		{name: "fetch interrupted", command: []string{"fetch"}, want: "simulated"},
		{name: "parent read interrupted", command: []string{"rev-list", "--parents"}, want: "simulated"},
		{name: "missing source parent", command: []string{"rev-list", "--parents"}, output: binding.Head + "\n", want: "one source parent"},
		{name: "wrong source ancestry", command: []string{"merge-base", "--is-ancestor"}, want: "not rebased"},
		{name: "source stamps unreadable", command: []string{"log"}, want: "source commits"},
		{name: "source contract absent", command: []string{"rev-list", "--parents"}, output: binding.Head + " " + binding.TargetOID + "\n", want: "recompile prepared source"},
		{name: "lock inventory interrupted", command: []string{"ls-remote", "--tags"}, want: "inspect latest lock"},
		{name: "worktree creation interrupted", command: []string{"worktree", "add"}, want: "simulated"},
		{name: "proposal inventory interrupted", port: "list", want: "simulated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := localLandingRefs(t, options.Origin)
			ports := local
			ports.Proposals = faultyLocalProposals{DraftProposals: local.Proposals, failure: tc.port}
			if tc.port == "nil" {
				ports.Memory = nil
			}
			run := draftshell.RunLocalGit
			if tc.command != nil {
				run = refuseLocalCommand(tc.command, tc.output)
			}
			result, err := LockLocal(t.Context(), root, cfg, binding, run, ports, receipts, localgit.NewDraftLandings(options))
			if err == nil || result.Created || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("incomplete lock accepted: %+v %v; want %q", result, err, tc.want)
			}
			if localLandingRefs(t, options.Origin) != before {
				t.Fatal("refusal published some of the landing refs")
			}
		})
	}
}

func TestLocalLockCleanupFailureRetainsACompleteRecoverableLanding(t *testing.T) {
	localgittest.Skip(t)
	root, cfg, options, local := localPreparationFixture(t, true)
	if _, err := PrepareLocal(t.Context(), root, cfg, "widget", draftshell.RunLocalGit, local); err != nil {
		t.Fatal(err)
	}
	receipts := localgit.NewReceipts(options)
	binding, err := receipts.Binding(t.Context(), "draft-widget-v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := receipts.Record(t.Context(), binding, "operator", ""); err != nil {
		t.Fatal(err)
	}
	run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) > 1 && slices.Equal(args[:2], []string{"worktree", "remove"}) {
			if _, ok := ctx.Deadline(); !ok || ctx.Err() != nil {
				t.Fatal("cleanup did not use its bounded independent context")
			}
			if _, err := draftshell.RunLocalGit(ctx, dir, env, name, args...); err != nil {
				t.Fatal(err)
			}
			return nil, errors.New("simulated cleanup reply lost")
		}
		return draftshell.RunLocalGit(ctx, dir, env, name, args...)
	}
	landings := localgit.NewDraftLandings(options)
	result, err := LockLocal(t.Context(), root, cfg, binding, run, local, receipts, landings)
	if err == nil || !result.Created || !strings.Contains(err.Error(), "cleanup reply lost") {
		t.Fatalf("cleanup obscured publication state: %+v %v", result, err)
	}
	before := localLandingRefs(t, options.Origin)
	if retry, err := LockLocal(t.Context(), root, cfg, binding, draftshell.RunLocalGit, local, receipts, landings); err != nil || retry.Created || retry.Lock != result.Lock {
		t.Fatalf("complete landing did not recover: %+v %v", retry, err)
	}
	if localLandingRefs(t, options.Origin) != before {
		t.Fatal("recovery published a second landing")
	}
	if _, err := LockLocal(t.Context(), root, cfg, binding, draftshell.RunLocalGit, local, receipts, contradictoryLocalLandings{landings}); !errors.Is(err, codehost.ErrReviewInvalid) {
		t.Fatalf("retry accepted contradictory retained lock metadata: %v", err)
	}
	if localLandingRefs(t, options.Origin) != before {
		t.Fatal("contradictory verification changed the completed landing")
	}
}

func TestLocalPreparationRefusesUnavailablePortsAndOrigin(t *testing.T) {
	localgittest.Skip(t)
	root, cfg, options, local := localPreparationFixture(t, true)
	before, head := localLandingRefs(t, options.Origin), gitFixture(t, root, "rev-parse", "HEAD")
	for _, failure := range []string{"missing memory", "mainline", "read"} {
		t.Run(failure, func(t *testing.T) {
			ports := local
			ports.Proposals = faultyLocalProposals{DraftProposals: local.Proposals, failure: failure}
			ports.Memory = func(context.Context, string, string) error {
				t.Fatal("unavailable origin reached the memory gate")
				return nil
			}
			if failure == "missing memory" {
				ports.Memory = nil
			}
			if _, err := PrepareLocal(t.Context(), root, cfg, "widget", draftshell.RunLocalGit, ports); err == nil {
				t.Fatal("incomplete preparation ports accepted")
			}
			if localLandingRefs(t, options.Origin) != before || gitFixture(t, root, "rev-parse", "HEAD") != head || gitFixture(t, root, "status", "--porcelain") != "" {
				t.Fatal("refused preparation changed the proposal or working draft")
			}
		})
	}
	if _, err := LoadLocalContract(t.Context(), root, "widget", 1, nil); err == nil {
		t.Fatal("contract read accepted a missing retained-evidence verifier")
	}
}
