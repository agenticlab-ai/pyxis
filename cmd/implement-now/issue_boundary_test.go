package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/issueentry"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
)

type issueReadFixture struct {
	port.WorkTracker
	snapshot port.IssueSnapshot
	err      error
	reads    int
}

func (f *issueReadFixture) ReadIssue(context.Context, int) (port.IssueSnapshot, error) {
	f.reads++
	return f.snapshot, f.err
}

func TestIssueRecoveryUsesAdmittedContractAfterPublication(t *testing.T) {
	for _, change := range []string{"edited", "closed", "new blocker", "unreadable"} {
		t.Run(change, func(t *testing.T) {
			repo := implementReviewGitFixture(t)
			binding := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "owner/repository"}
			snapshot := port.IssueSnapshot{Number: 17, Open: true, Executable: true, Title: "Original issue", Body: "Original scope"}
			input := issueCharter(t, binding, snapshot)
			path := filepath.Join(t.TempDir(), "input.json")
			writeJSONFile(t, path, input)
			now := func() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }
			first, _, err := admitIssue(t.Context(), &issueReadFixture{snapshot: snapshot}, binding, 17, path, repo, "origin", now(), now)
			if err != nil {
				t.Fatal(err)
			}
			live := &issueReadFixture{snapshot: snapshot}
			switch change {
			case "edited":
				live.snapshot.Title, live.snapshot.Body = "New title", "New scope"
			case "closed":
				live.snapshot.Open = false
			case "new blocker":
				live.snapshot.Dependencies = []port.IssueDependency{{Number: 18}}
			case "unreadable":
				live.err = errors.New("tracker unavailable")
			}
			// Before publication, the same changes must still block dispatch.
			if _, pending, err := admitIssue(t.Context(), live, binding, 17, path, repo, "origin", now(), now); err != nil {
				t.Fatal(err)
			} else if _, err := pending.Frontier(t.Context()); err == nil {
				t.Fatal("changed source authorized implementation before publication")
			}
			p := port.Proposal{Number: 77, Branch: "feat/issue"}
			if err := first.store.Associate(t.Context(), first.contract, p); err != nil {
				t.Fatal(err)
			}
			live.reads = 0
			resumed, tracker, err := admitIssue(t.Context(), live, binding, 17, path, repo, "origin", now(), now)
			if err != nil {
				t.Fatalf("published contract could not resume after %s: %v", change, err)
			}
			if resumed.contract.ID != first.contract.ID || !reflect.DeepEqual(resumed.task, first.task) || live.reads != 0 {
				t.Fatal("recovery changed admitted authority or consulted mutable source")
			}
			if tasks, err := tracker.Frontier(t.Context()); err != nil || len(tasks) != 0 || live.reads != 0 {
				t.Fatalf("associated proposal reentered implementation: %+v %v", tasks, err)
			}
			input.Goal = "A different implementation"
			writeJSONFile(t, path, input)
			if _, _, err := admitIssue(t.Context(), live, binding, 17, path, repo, "origin", now(), now); err == nil || live.reads != 0 {
				t.Fatal("changed intake reused a published contract")
			}
		})
	}
}

func issueCharter(t *testing.T, binding integrations.ResolvedRepository, snapshot port.IssueSnapshot) issueentry.Input {
	t.Helper()
	inspection, err := issueentry.Inspect(string(binding.Provider), binding.Key(), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	requirements, err := taskrequirements.Parse([]byte(requirementstest.Input))
	if err != nil {
		t.Fatal(err)
	}
	return issueentry.Input{Schema: issueentry.InputSchema, Source: inspection.Source, NativeInput: taskspec.NativeInput{Title: "Fix output", Goal: "Return expected output", Scope: "Only the output", Acceptance: []string{"Output matches fixture"}, Requirements: requirements}}
}

func TestIssueAdmissionAndInspectionFailClosed(t *testing.T) {
	repo := implementReviewGitFixture(t)
	binding := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "owner/repository"}
	snapshot := port.IssueSnapshot{Number: 17, Open: true, Executable: true, Title: "Fix output"}
	input := issueCharter(t, binding, snapshot)
	path := filepath.Join(t.TempDir(), "input.json")
	writeJSONFile(t, path, input)
	now := func() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }
	for _, tc := range []struct {
		name, path, repo, remote string
		tracker                  port.WorkTracker
	}{
		{"missing exact reader", path, repo, "origin", &struct{ port.WorkTracker }{}},
		{"missing input", path + "-missing", repo, "origin", &issueReadFixture{snapshot: snapshot}},
		{"input directory", filepath.Dir(path), repo, "origin", &issueReadFixture{snapshot: snapshot}},
		{"unreadable source", path, repo, "origin", &issueReadFixture{err: errors.New("denied")}},
		{"invalid exact source", path, repo, "origin", &issueReadFixture{snapshot: port.IssueSnapshot{Number: 17}}},
		{"invalid evidence binding", path, repo, "", &issueReadFixture{snapshot: snapshot}},
		{"unreachable evidence", path, repo, "missing", &issueReadFixture{snapshot: snapshot}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := admitIssue(t.Context(), tc.tracker, binding, 17, tc.path, tc.repo, tc.remote, now(), now); err == nil {
				t.Fatal("admitted an incomplete source/contract/evidence boundary")
			}
		})
	}
	if err := os.WriteFile(path, []byte("{} {}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := admitIssue(t.Context(), &issueReadFixture{snapshot: snapshot}, binding, 17, path, repo, "origin", now(), now); err == nil {
		t.Fatal("admitted invalid intake JSON")
	}
	for _, tc := range []struct {
		name, remote string
		tracker      port.WorkTracker
		out          io.Writer
	}{
		{"missing reader", "origin", &struct{ port.WorkTracker }{}, io.Discard},
		{"source denied", "origin", &issueReadFixture{err: errors.New("denied")}, io.Discard},
		{"unknown identity", "origin", &issueReadFixture{}, io.Discard},
		{"invalid evidence binding", "", &issueReadFixture{snapshot: snapshot}, io.Discard},
		{"unreachable evidence", "missing", &issueReadFixture{snapshot: snapshot}, io.Discard},
		{"lost inspection output", "origin", &issueReadFixture{snapshot: snapshot}, issueBrokenWriter{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := inspectIssue(t.Context(), tc.tracker, binding, 17, repo, tc.remote, tc.out); err == nil {
				t.Fatal("reported a complete inspection through a failed boundary")
			}
		})
	}
	// A Carina binding never publishes its root through the task source ref.
	binding = integrations.ResolvedRepository{Provider: integrations.Carina, Root: repo}
	writeJSONFile(t, path, issueCharter(t, binding, snapshot))
	r, tracker, err := admitIssue(t.Context(), &issueReadFixture{snapshot: snapshot}, binding, 17, path, repo, "origin", now(), now)
	if err != nil || tracker == nil || strings.Contains(r.task.SourceRef, repo) || !strings.HasPrefix(r.task.SourceRef, "carina:") {
		t.Fatalf("opaque source: %+v %v", r, err)
	}
}

type issueBrokenWriter struct{}

func (issueBrokenWriter) Write([]byte) (int, error) { return 0, errors.New("sink unavailable") }

type issueReviewRows struct {
	reviewobligation.Store
	rows []reviewobligation.State
	err  error
}

func (r *issueReviewRows) Open(context.Context, string) ([]reviewobligation.State, error) {
	return r.rows, r.err
}

type issueProofHost struct {
	executionHost
	head, after      string
	landing          port.Landing
	checks           port.CheckState
	readErr, openErr error
	headErr          error
	opened           func()
	readErrAt, reads int
}

func (h *issueProofHost) InspectProposal(context.Context, port.Proposal) (port.Landing, string, port.CheckState, error) {
	h.reads++
	if h.readErr != nil && (h.readErrAt == 0 || h.reads == h.readErrAt) {
		return port.LandingUnknown, "", port.CheckPending, h.readErr
	}
	head := h.head
	if h.reads > 1 && h.after != "" {
		head = h.after
	}
	return h.landing, head, h.checks, nil
}
func (h *issueProofHost) OpenProposal(context.Context, port.Task, port.Workdir, prdescription.Description) (port.Proposal, error) {
	if h.opened != nil {
		h.opened()
	}
	return port.Proposal{Number: 77, Branch: "feat/issue"}, h.openErr
}

func TestIssueProposalRetainsProofBeforePostPublicationFailures(t *testing.T) {
	for _, failure := range []string{"head read", "gate unavailable"} {
		t.Run(failure, func(t *testing.T) {
			repo := implementReviewGitFixture(t)
			if err := os.MkdirAll(filepath.Join(repo, ".pyxis"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repo, ".pyxis/config.yaml"), []byte("code-gate:\n  ready:\n    command: test -f .git/local-gate-ready\n"), 0600); err != nil {
				t.Fatal(err)
			}
			runRoutingGit(t, repo, "add", ".pyxis/config.yaml")
			runRoutingGit(t, repo, "commit", "-qm", "test: configure local gate")
			shaBytes, err := localGit(t.Context(), repo, "rev-parse", "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			sha := strings.TrimSpace(string(shaBytes))
			ready := filepath.Join(repo, ".git/local-gate-ready")
			if err := os.WriteFile(ready, nil, 0600); err != nil {
				t.Fatal(err)
			}
			binding := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "owner/repository"}
			snapshot := port.IssueSnapshot{Number: 17, Open: true, Executable: true}
			path := filepath.Join(t.TempDir(), "input.json")
			writeJSONFile(t, path, issueCharter(t, binding, snapshot))
			now := func() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }
			session, _, err := admitIssue(t.Context(), &issueReadFixture{snapshot: snapshot}, binding, 17, path, repo, "origin", now(), now)
			if err != nil {
				t.Fatal(err)
			}
			session.publisher = &issueSummarySink{}
			host := &issueProofHost{head: sha}
			if failure == "head read" {
				host.headErr = errors.New("temporary head read failure")
			} else {
				host.opened = func() {
					if err := os.Remove(ready); err != nil {
						t.Fatal(err)
					}
				}
			}
			wrapped := &issueHost{executionHost: host, session: session}
			p, err := wrapped.OpenProposal(t.Context(), session.task, &healWorkdir{dir: repo}, prdescription.Description{})
			if err != nil {
				t.Fatalf("post-publication %s prevented retaining verified proposal: %v", failure, err)
			}
			fresh, err := issueentry.NewStore(repo, "origin")
			if err != nil {
				t.Fatal(err)
			}
			if known, ok, err := fresh.Proposal(t.Context(), session.contract); err != nil || !ok || known != p {
				t.Fatalf("lost proposal association: %+v %v", known, err)
			}
			if verified, err := fresh.LocalVerified(t.Context(), session.contract, p, sha); err != nil || !verified {
				t.Fatalf("lost recoverable local proof: %v", err)
			}
		})
	}
}
func (h *issueProofHost) ProposalHead(context.Context, port.Proposal) (string, error) {
	return h.head, h.headErr
}

type issueSummarySink struct {
	err     error
	summary issueentry.Summary
}

func (s *issueSummarySink) PublishIssueSummary(_ context.Context, _ port.Proposal, summary issueentry.Summary) error {
	s.summary = summary
	return s.err
}

func TestIssueCompletionRequiresExactLocalCIAndAuthenticatedReview(t *testing.T) {
	repo := implementReviewGitFixture(t)
	binding := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "owner/repository"}
	snapshot := port.IssueSnapshot{Number: 17, Open: true, Executable: true}
	c, task, err := issueentry.Prepare(issueCharter(t, binding, snapshot), string(binding.Provider), binding.Key(), 17, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	task.SourceRef = "https://github.com/owner/repository/issues/17"
	store, err := issueentry.NewStore(repo, "origin")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Admit(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	sink := &issueSummarySink{}
	now := func() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }
	r := &issueRun{contract: c, task: task, store: store, publisher: sink, now: now, started: now()}
	p := port.Proposal{Number: 77, Branch: "feat/issue"}
	var out, diagnostic bytes.Buffer
	if code := r.finish(t.Context(), nil, &issueReviewRows{}, "owner/repository", &out, &diagnostic); code != 1 || !strings.Contains(diagnostic.String(), "no retained proposal") {
		t.Fatalf("unpublished completion: %d %s", code, &diagnostic)
	}
	if rows, err := r.states(t.Context(), &issueReviewRows{}, "owner/repository"); err != nil || len(rows) != 0 {
		t.Fatalf("no proposal read: %+v %v", rows, err)
	}
	if _, err := r.states(t.Context(), &issueReviewRows{rows: []reviewobligation.State{{Status: "handoff_pending"}}}, "owner/repository"); err == nil {
		t.Fatal("unassociated scope authorized review recovery")
	}
	if err := store.Associate(t.Context(), c, p); err != nil {
		t.Fatal(err)
	}
	head := strings.Repeat("a", 40)
	if err := store.RecordLocal(t.Context(), c, p, head); err != nil {
		t.Fatal(err)
	}
	selected := reviewobligation.State{Obligation: reviewobligation.Obligation{Repository: "owner/repository", TaskID: task.ID, TaskNumber: task.Number, DemandID: task.DemandID, ProposalRef: "github:owner/repository#77", Revision: head}, Status: "verified", Pair: &reviewobligation.PublicationPair{Revision: head}}
	scoped := issueReviewStore{Store: &issueReviewRows{rows: []reviewobligation.State{selected}}, session: r, repository: "owner/repository"}
	if _, err := scoped.Open(t.Context(), "other/001"); err == nil {
		t.Fatal("unselected review scope read")
	}
	if rows, err := scoped.Open(t.Context(), r.scope()); err != nil || len(rows) != 1 {
		t.Fatalf("selected scope: %+v %v", rows, err)
	}
	wrong := selected
	wrong.Obligation.TaskID = "other-task"
	scoped.Store = &issueReviewRows{rows: []reviewobligation.State{wrong}}
	repairer := reviewHandoffRepairer{store: scoped, scope: r.scope(), repository: "owner/repository"}
	if _, err := repairer.Pending(t.Context()); err == nil {
		t.Fatal("same scope label widened recovery task authority")
	}
	if _, err := repairer.RepairPairs(t.Context()); err == nil {
		t.Fatal("same scope label widened publication repair authority")
	}
	for _, tc := range []struct {
		name, terminal string
		change         func(*issueProofHost, *issueReviewRows)
	}{
		{"native read denied", "verification_pending", func(h *issueProofHost, _ *issueReviewRows) { h.readErr = errors.New("denied") }},
		{"closed proposal", "verification_pending", func(h *issueProofHost, _ *issueReviewRows) { h.landing = port.LandingClosedUnlanded }},
		{"pending CI", "verification_pending", func(h *issueProofHost, _ *issueReviewRows) { h.checks = port.CheckPending }},
		{"malformed head", "verification_pending", func(h *issueProofHost, _ *issueReviewRows) { h.head = "invalid" }},
		{"new head lacks local proof", "verification_pending", func(h *issueProofHost, _ *issueReviewRows) { h.head = strings.Repeat("b", 40) }},
		{"unreadable review metadata", "review_pending", func(_ *issueProofHost, rows *issueReviewRows) { rows.err = errors.New("denied") }},
		{"wrong proposal", "review_pending", func(_ *issueProofHost, rows *issueReviewRows) {
			rows.rows[0].Obligation.ProposalRef = "github:owner/repository#78"
		}},
		{"old review revision", "review_pending", func(_ *issueProofHost, rows *issueReviewRows) {
			rows.rows[0].Obligation.Revision = strings.Repeat("b", 40)
		}},
		{"process success lacks verified pair", "review_pending", func(_ *issueProofHost, rows *issueReviewRows) { rows.rows[0].Pair = nil }},
		{"pair not lifecycle-verified", "review_pending", func(_ *issueProofHost, rows *issueReviewRows) { rows.rows[0].Status = "pair_verified" }},
		{"missing review", "review_pending", func(_ *issueProofHost, rows *issueReviewRows) { rows.rows = nil }},
		{"duplicate obligation", "review_pending", func(_ *issueProofHost, rows *issueReviewRows) { rows.rows = append(rows.rows, selected) }},
		{"final read denied", "verification_pending", func(h *issueProofHost, _ *issueReviewRows) { h.readErr, h.readErrAt = errors.New("denied"), 2 }},
		{"head moved during verification", "verification_pending", func(h *issueProofHost, _ *issueReviewRows) { h.after = strings.Repeat("b", 40) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := &issueProofHost{head: head, landing: port.LandingOpen, checks: port.CheckGreen}
			reviews := &issueReviewRows{rows: []reviewobligation.State{selected}}
			tc.change(host, reviews)
			if _, terminal, err := r.completion(t.Context(), host, reviews, "owner/repository", p); err == nil || terminal != tc.terminal {
				t.Fatalf("terminal=%s err=%v", terminal, err)
			}
		})
	}
	host := &issueProofHost{head: head, landing: port.LandingOpen, checks: port.CheckGreen}
	reviews := &issueReviewRows{rows: []reviewobligation.State{selected}}
	r.work = &healWorkdir{dir: t.TempDir()}
	diagnostic.Reset()
	if code := r.finish(t.Context(), host, reviews, "owner/repository", &out, &diagnostic); code != 1 || sink.summary.Terminal != "verification_pending" {
		t.Fatal("a currently invalid local worktree borrowed its retained proof")
	}
	r.work = nil
	sink.err = errors.New("publication denied")
	diagnostic.Reset()
	if code := r.finish(t.Context(), host, reviews, "owner/repository", &out, &diagnostic); code != 1 || !strings.Contains(diagnostic.String(), "summary_publication_failed") {
		t.Fatal("metadata failure reported success")
	}
	h := &issueHost{executionHost: host, session: r}
	if _, err := h.OpenProposal(t.Context(), port.Task{ID: "unselected"}, nil, prdescription.Description{}); err == nil {
		t.Fatal("unselected publication")
	}
	host.openErr = errors.New("native publication denied")
	if _, err := h.OpenProposal(t.Context(), task, nil, prdescription.Description{}); err == nil {
		t.Fatal("lost native publication failure")
	}
	// The stored observations are closed JSON, independent of the public sink.
	wire, err := json.Marshal(sink.summary)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := issueentry.DecodeSummary(wire); err != nil {
		t.Fatal(err)
	}
}

func (h *issueProofHost) DescribeProposal(context.Context, port.Task, port.Workdir, prdescription.Description) (bool, error) {
	return false, nil
}
