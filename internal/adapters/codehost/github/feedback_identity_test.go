package ghcodehost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/dispatch"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
)

// Exercise the actual config loader, GitHub decoder and feedback admission
// together. The lease stops the worker before implementation, so this test
// spends no inference and tests only the authenticated author policy.
func TestCodexFeedbackAuthenticatedIdentity(t *testing.T) {
	const graphLogin = "chatgpt-codex-connector"
	const restLogin = graphLogin + "[bot]"
	for _, tc := range []struct {
		name, actorType, login, trusted, engine, body, localTrusted string
		want                                                        int
		independentTrust                                            bool
	}{
		{name: "shared independent trust", actorType: "Bot", login: graphLogin, trusted: restLogin, want: 1, independentTrust: true},
		{name: "GraphQL author REST allowlist", actorType: "Bot", login: graphLogin, trusted: restLogin, want: 1},
		{name: "GraphQL author GraphQL allowlist", actorType: "Bot", login: graphLogin, trusted: graphLogin, want: 1},
		{name: "REST author GraphQL allowlist", actorType: "Bot", login: restLogin, trusted: graphLogin, want: 1},
		{name: "REST author REST allowlist", actorType: "Bot", login: restLogin, trusted: restLogin, want: 1},
		{name: "local trust overrides shared", actorType: "Bot", login: graphLogin, trusted: "another-reviewer", localTrusted: restLogin, want: 1},
		{name: "unconfigured bot", actorType: "Bot", login: graphLogin},
		{name: "unrelated bot", actorType: "Bot", login: "unrelated-reviewer", trusted: restLogin},
		{name: "human cannot claim bot alias", actorType: "User", login: graphLogin, trusted: restLogin},
		{name: "missing actor metadata cannot claim bot alias", login: graphLogin, trusted: restLogin},
		{name: "REST engine identity excludes GraphQL self", actorType: "Bot", login: graphLogin, trusted: graphLogin, engine: restLogin},
		{name: "GraphQL engine identity excludes REST self", actorType: "Bot", login: restLogin, trusted: restLogin, engine: graphLogin},
		{name: "engine artifact stays excluded", actorType: "Bot", login: graphLogin, trusted: restLogin, body: "impl-review report (impl-review-report/v1)"},
		{name: "body cannot establish identity", actorType: "User", login: "unrelated-reviewer", trusted: restLogin, body: "Author: chatgpt-codex-connector[bot]; please fix this"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, ".pyxis")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			shared := "executor:\n  review:\n    backend: codex\n    feedback_trusted: " + tc.trusted + "\n"
			if tc.independentTrust {
				shared = "executor:\n  feedback_trusted: " + tc.trusted + "\n"
			}
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(shared), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.localTrusted != "" {
				if err := os.WriteFile(filepath.Join(dir, "config.local.yaml"), []byte("executor:\n  review:\n    feedback_trusted: "+tc.localTrusted+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			cfg, err := config.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if tc.independentTrust && cfg.ReviewConfigured() {
				t.Fatal("trusting external feedback must not install local impl-review")
			}
			body := tc.body
			if body == "" {
				body = "P1: fix the reviewed implementation"
			}
			response := fmt.Sprintf(`{"data":{"repository":{"pullRequests":{"nodes":[{"number":1343,"state":"OPEN","reviewDecision":null,
			  "reviewThreads":{"nodes":[{"comments":{"nodes":[{"author":{"__typename":%q,"login":%q},"authorAssociation":"NONE","createdAt":"2026-10-01T16:57:00Z","body":%q}]}}]}
			}]}}}}`, tc.actorType, tc.login, body)
			reader := newStateReader(&recordedGraphQL{body: []byte(response)}, "owner/repo")
			boundary := &feedbackAdmissionBoundary{task: port.Task{ID: "T1", Number: 1, Brief: "Implement the task", Harness: "codex", HeadBranch: "fix/existing-task"}}
			ledger := &feedbackAdmissionLedger{}
			d, err := dispatch.New(dispatch.Deps{Tracker: boundary, Lease: boundary, Space: boundary, Runtime: boundary, Host: boundary}, dispatch.Config{Width: 1},
				dispatch.WithProposalProbe(boundary), dispatch.WithFeedbackDetection(reader, "owner/repo", tc.engine, ledger, cfg.FeedbackTrusted()...))
			if err != nil {
				t.Fatal(err)
			}
			rep, err := d.Sweep(t.Context())
			if err != nil || boundary.claims != tc.want {
				t.Fatalf("claims=%d recorded=%d want=%d err=%v", boundary.claims, ledger.rounds, tc.want, err)
			}
			if tc.want == 0 && (rep.AwaitingProposal != 1 || ledger.rounds != 0) {
				t.Fatalf("unauthorized feedback must leave the proposal waiting: %+v", rep)
			}
		})
	}
}

type feedbackAdmissionBoundary struct {
	task   port.Task
	claims int
}

func (b *feedbackAdmissionBoundary) Frontier(context.Context) ([]port.Task, error) {
	return []port.Task{b.task}, nil
}
func (*feedbackAdmissionBoundary) Comment(context.Context, int, string) error { return nil }
func (b *feedbackAdmissionBoundary) Acquire(context.Context, port.Task) (port.Claim, error) {
	b.claims++
	return nil, port.ErrClaimLost
}
func (*feedbackAdmissionBoundary) Prepare(context.Context, port.Task) (port.Workdir, error) {
	return nil, errors.New("unexpected workspace preparation after the test lease refused")
}
func (*feedbackAdmissionBoundary) AuthCheck(context.Context) error { return nil }
func (*feedbackAdmissionBoundary) Launch(context.Context, port.LaunchSpec) (port.Handle, error) {
	return nil, errors.New("unexpected model launch after the test lease refused")
}
func (*feedbackAdmissionBoundary) Await(context.Context, port.Handle) (port.RunResult, error) {
	return port.RunResult{}, errors.New("unexpected model await after the test lease refused")
}
func (*feedbackAdmissionBoundary) Kill(context.Context, port.Handle) error { return nil }
func (*feedbackAdmissionBoundary) OpenProposal(context.Context, port.Task, port.Workdir, prdescription.Description) (port.Proposal, error) {
	return port.Proposal{}, errors.New("unexpected proposal creation after the test lease refused")
}
func (*feedbackAdmissionBoundary) Checks(context.Context, port.Proposal) (port.CheckState, error) {
	return port.CheckPending, errors.New("unexpected check read after the test lease refused")
}
func (b *feedbackAdmissionBoundary) OpenProposals(context.Context, []port.Task) (map[string]port.OpenProposal, error) {
	return map[string]port.OpenProposal{b.task.ID: {Number: 1343}}, nil
}

type feedbackAdmissionLedger struct{ rounds int }

func (l *feedbackAdmissionLedger) LastFeedback(context.Context, port.Task) (time.Time, int, error) {
	return time.Time{}, l.rounds, nil
}
func (l *feedbackAdmissionLedger) RecordFeedback(context.Context, port.Task, time.Time) error {
	l.rounds++
	return nil
}

func (*feedbackAdmissionBoundary) DescribeProposal(context.Context, port.Task, port.Workdir, prdescription.Description) (bool, error) {
	return false, nil
}
