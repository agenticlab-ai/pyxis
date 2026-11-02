package lock

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/memory/packet"
	"github.com/agenticlab-ai/pyxis/internal/memory/policy"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestGuardReopenedRefusesMissingOrUnreadableFirstCommit(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  lifecycle.RunCommand
		want string
	}{
		{"unreadable", func(context.Context, string, []string, string, ...string) ([]byte, error) {
			return nil, errors.New("history unavailable")
		}, "read reopened draft history"},
		{"missing", func(context.Context, string, []string, string, ...string) ([]byte, error) {
			return nil, nil
		}, "no first commit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := guardReopened(t.Context(), "", "", nil, tc.run, strings.Repeat("a", 40), strings.Repeat("b", 40), "drafts", "widget", lifecycle.LockedContract{}, compile.Result{}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestLatestLockedVersionRejectsMalformedRemoteHistory(t *testing.T) {
	for _, tc := range []struct {
		name, listing, want string
		failure             error
	}{
		{name: "transport", failure: errors.New("remote unavailable"), want: "inspect latest lock"},
		{name: "malformed", listing: "missing tab", want: "malformed lock tag listing"},
		{name: "invalid version", listing: strings.Repeat("a", 40) + "\trefs/tags/lock/widget/v02", want: "invalid lock tag version"},
		{name: "unexpected ref", listing: strings.Repeat("a", 40) + "\trefs/tags/lock/other/v2", want: "unexpected lock tag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(_ context.Context, _ string, _ []string, name string, args ...string) ([]byte, error) {
				if name != "git" || len(args) != 4 || args[0] != "ls-remote" {
					t.Fatalf("unexpected command %s %v", name, args)
				}
				return []byte(tc.listing), tc.failure
			}
			if _, err := latestLockedVersion(t.Context(), "", "origin", "widget", run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
	listing := strings.Repeat("a", 40) + "\trefs/tags/lock/widget/v2\n" + strings.Repeat("b", 40) + "\trefs/tags/lock/widget/v2^{}\n"
	run := func(context.Context, string, []string, string, ...string) ([]byte, error) {
		return []byte(listing), nil
	}
	if latest, err := latestLockedVersion(t.Context(), "", "origin", "widget", run); err != nil || latest != 2 {
		t.Fatalf("annotated tag listing: latest=%d err=%v", latest, err)
	}
}

func TestPrepareReopenedLockRefusesHandSeedAndUnchangedContract(t *testing.T) {
	for _, tc := range []struct {
		name, condition, remedy string
		change                  bool
		failHistory             bool
		staleBinding            bool
	}{
		{name: "hand-seeded", condition: "FR-20", remedy: "no earlier locked version", change: true},
		{name: "unchanged", condition: "FR-22", remedy: "abandon this draft"},
		{name: "unreadable history", failHistory: true},
		{name: "stale binding", condition: "FR-20", remedy: "latest binding lock is v2", staleBinding: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work, firstRequest, host := preparedMerge(t)
			reader := &recordApproval{result: approval.Approval{Operator: "mdarabi", Head: firstRequest.ApprovedHead, ReviewID: 27}}
			if _, err := RecordMerge(t.Context(), work, firstRequest, reader, host.run); err != nil {
				t.Fatal(err)
			}
			if tc.staleBinding {
				gitFixture(t, work, "tag", "lock/widget/v2", "main")
				gitFixture(t, work, "push", "origin", "refs/tags/lock/widget/v2")
			}
			gitFixture(t, work, "switch", "-c", "draft/widget-v2", "main")
			prd := readyPRD(t)
			if tc.change {
				prd = []byte(strings.Replace(string(prd), "The widget shall open.", "The widget shall open durably.", 1))
			}
			tech := []byte("# Widget — tech spec\n\n### D-1 — Open widget\n\n**Decision:** Open and queue the widget.\n\n**Serves:** FR-1 · UB-1\n")
			fixtureFile(t, work, "drafts/widget/prd.md", prd)
			fixtureFile(t, work, "drafts/widget/tech-spec.md", tech)
			gitFixture(t, work, "add", "--", "drafts/widget/prd.md", "drafts/widget/tech-spec.md")
			gitFixture(t, work, "commit", "-m", "seed by hand")
			draft, err := compile.Compile("widget", "main", map[string][]byte{"prd.md": prd, "tech-spec.md": tech})
			if err != nil {
				t.Fatal(err)
			}
			fixtureFile(t, work, "drafts/widget/reviews/003.md", []byte("# PRD review\n\n**Review subject:** prd\n**Draft digest:** "+draft.Digest+"\n"))
			fixtureFile(t, work, "drafts/widget/reviews/004.md", []byte("# Tech-spec review\n\n**Review subject:** tech-spec\n**Draft digest:** "+draft.Digest+"\n"))
			gitFixture(t, work, "add", "--", "drafts/widget/reviews/003.md", "drafts/widget/reviews/004.md")
			gitFixture(t, work, "commit", "-m", "review reopened draft")
			gitFixture(t, work, "push", "-u", "origin", "draft/widget-v2")
			cfg := config.Defaults()
			cfg.Draft.AgentLogin = "agent"
			cfg.Draft.AgentTokenEnv = "PYXIS_DRAFT_TEST_TOKEN"
			head := gitFixture(t, work, "rev-parse", "HEAD")
			host.failHistory = tc.failHistory
			prepared, err := Prepare(t.Context(), work, cfg, "widget", host.run)
			if tc.failHistory {
				if err == nil || !strings.Contains(err.Error(), "guard reopened lock") || gitFixture(t, work, "rev-parse", "HEAD") != head || gitFixture(t, work, "status", "--porcelain") != "" {
					t.Fatalf("unreadable history did not fail closed: %+v %v", prepared, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "readiness refused") {
				t.Fatalf("invalid reopen was accepted: %+v %v", prepared, err)
			}
			found := false
			for _, unmet := range prepared.Verdict.Unmet {
				found = found || unmet.Condition == tc.condition && strings.Contains(unmet.Remedy, tc.remedy)
			}
			if !found || gitFixture(t, work, "rev-parse", "HEAD") != head || gitFixture(t, work, "status", "--porcelain") != "" {
				t.Fatalf("missing refusal or changed files: %+v", prepared.Verdict.Unmet)
			}
		})
	}
}

func TestMemoryGateRefusesEditToLockedRecordOutsideReopen(t *testing.T) {
	work, firstRequest, host := preparedMerge(t)
	reader := &recordApproval{result: approval.Approval{Operator: "mdarabi", Head: firstRequest.ApprovedHead, ReviewID: 27}}
	if _, err := RecordMerge(t.Context(), work, firstRequest, reader, host.run); err != nil {
		t.Fatal(err)
	}
	base := mustLockedSet(t, work, "refs/tags/lock/widget/v1")
	locked := resolve.New(base).Head("prd/widget", "FR-1")
	changed := *locked
	changed.Statement = "The widget shall open silently after the lock."
	body, err := record.Marshal(&changed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, store.Dir, locked.Path()), body, 0o644); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, work, "add", "--", filepath.ToSlash(filepath.Join(store.Dir, locked.Path())))
	gitFixture(t, work, "commit", "-m", "edit locked record outside reopen")
	head, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(work, store.Dir)})
	if err != nil {
		t.Fatal(err)
	}
	check := packet.Compute(packet.Input{Base: base, Head: head, Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "other/proposal", BaseRef: "lock/widget/v1"})
	found := false
	for _, blocker := range check.Blockers {
		found = found || blocker.Code == "record-modified" && strings.Contains(blocker.What, "pyxis draft reopen <feature>")
	}
	if check.OK() || !found {
		t.Fatalf("locked record edit escaped memory gate or lacked reopen remedy: %+v", check.Blockers)
	}
}

func TestPrepareRefusesFirstVersionAfterFeatureWasLocked(t *testing.T) {
	work, firstRequest, host := preparedMerge(t)
	reader := &recordApproval{result: approval.Approval{Operator: "mdarabi", Head: firstRequest.ApprovedHead, ReviewID: 27}}
	if _, err := RecordMerge(t.Context(), work, firstRequest, reader, host.run); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, work, "branch", "-D", "draft/widget")
	gitFixture(t, work, "switch", "-c", "draft/widget", "main")
	prd := readyPRD(t)
	tech := []byte("# Widget — tech spec\n\n### D-1 — Open widget\n\n**Decision:** Open and queue the widget.\n\n**Serves:** FR-1 · UB-1\n")
	fixtureFile(t, work, "drafts/widget/prd.md", prd)
	fixtureFile(t, work, "drafts/widget/tech-spec.md", tech)
	gitFixture(t, work, "add", "--", "drafts/widget/prd.md", "drafts/widget/tech-spec.md")
	gitFixture(t, work, "commit", "-m", "attempt another first version")
	gitFixture(t, work, "push", "--force", "origin", "draft/widget")
	cfg := config.Defaults()
	cfg.Draft.AgentLogin = "agent"
	cfg.Draft.AgentTokenEnv = "PYXIS_DRAFT_TEST_TOKEN"
	prepared, err := Prepare(t.Context(), work, cfg, "widget", host.run)
	if err == nil || !strings.Contains(err.Error(), "readiness refused") {
		t.Fatalf("second first version accepted: %+v %v", prepared, err)
	}
	for _, unmet := range prepared.Verdict.Unmet {
		if unmet.Condition == "FR-20" && strings.Contains(unmet.Remedy, "already locked at v1") {
			return
		}
	}
	t.Fatalf("missing first-version refusal: %+v", prepared.Verdict.Unmet)
}

func TestPrepareReopenedLockNamesEarlierVersionSeed(t *testing.T) {
	work, firstRequest, host := preparedMerge(t)
	reader := &recordApproval{result: approval.Approval{Operator: "mdarabi", Head: firstRequest.ApprovedHead, ReviewID: 27}}
	if _, err := RecordMerge(t.Context(), work, firstRequest, reader, host.run); err != nil {
		t.Fatal(err)
	}
	oldPRD := readyPRD(t)
	tech := []byte("# Widget — tech spec\n\n### D-1 — Open widget\n\n**Decision:** Open and queue the widget.\n\n**Serves:** FR-1 · UB-1\n")
	changedPRD := []byte(strings.Replace(string(oldPRD), "The widget shall open.", "The widget shall open durably.", 1))
	binding := resolve.New(mustLockedSet(t, work, "refs/tags/lock/widget/v1"))
	previous := binding.Head("prd/widget", "prd")
	when := time.Now().UTC().Add(time.Minute)
	newDocument := &record.Record{Schema: record.Schema, ID: record.NewID(record.Document, when, [10]byte{9}),
		Kind: record.Document, Subject: "prd/widget", Family: "prd", Statement: "Draft prd.md", LiteralBody: true,
		Issuer: "agent:test", CreatedAt: when.Format(time.RFC3339), Relations: []record.Relation{{Type: record.Supersedes, Target: previous.ID}}}
	if _, err := store.WriteDocument(filepath.Join(work, store.Dir), newDocument, changedPRD); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, work, "add", "--", filepath.ToSlash(filepath.Join(store.Dir, newDocument.Path())), filepath.ToSlash(filepath.Join(store.Dir, newDocument.BodyPath())))
	gitFixture(t, work, "commit", "-m", "fixture binding version two")
	gitFixture(t, work, "push", "origin", "main")
	merge := gitFixture(t, work, "rev-parse", "HEAD")
	second := LockRecord{Schema: lockSchema, Feature: "widget", Version: 2, MergeCommit: merge,
		PullRequest: 18, ApprovedHead: merge, ReadinessDigest: compile.ContractDigest(changedPRD, tech),
		ProductApprover: "mdarabi", EngineeringApprover: "mdarabi", Lineless: true, ReviewID: 28, ProposalsSinceOpened: 1}
	body, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	gitFixture(t, work, "tag", "-a", "lock/widget/v2", merge, "-m", string(body))
	gitFixture(t, work, "push", "origin", "refs/tags/lock/widget/v2")
	gitFixture(t, work, "switch", "-c", "draft/widget-v3", "main")
	fixtureFile(t, work, "drafts/widget/prd.md", oldPRD)
	fixtureFile(t, work, "drafts/widget/tech-spec.md", tech)
	gitFixture(t, work, "add", "--", "drafts/widget/prd.md", "drafts/widget/tech-spec.md")
	gitFixture(t, work, "commit", "-m", "seed from old version")
	draft, err := compile.Compile("widget", "main", map[string][]byte{"prd.md": oldPRD, "tech-spec.md": tech})
	if err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, work, "drafts/widget/reviews/005.md", []byte("# PRD review\n\n**Review subject:** prd\n**Draft digest:** "+draft.Digest+"\n"))
	fixtureFile(t, work, "drafts/widget/reviews/006.md", []byte("# Tech-spec review\n\n**Review subject:** tech-spec\n**Draft digest:** "+draft.Digest+"\n"))
	gitFixture(t, work, "add", "--", "drafts/widget/reviews/005.md", "drafts/widget/reviews/006.md")
	gitFixture(t, work, "commit", "-m", "review stale seed")
	gitFixture(t, work, "push", "-u", "origin", "draft/widget-v3")
	cfg := config.Defaults()
	cfg.Draft.AgentLogin = "agent"
	cfg.Draft.AgentTokenEnv = "PYXIS_DRAFT_TEST_TOKEN"
	prepared, err := Prepare(t.Context(), work, cfg, "widget", host.run)
	if err == nil || !strings.Contains(err.Error(), "readiness refused") {
		t.Fatalf("stale seed accepted: %+v %v", prepared, err)
	}
	found := false
	for _, unmet := range prepared.Verdict.Unmet {
		found = found || unmet.Condition == "FR-20" && strings.Contains(unmet.Remedy, "lock v1") && strings.Contains(unmet.Remedy, "binding lock v2")
	}
	if !found {
		t.Fatalf("stale seed refusal did not name versions: %+v", prepared.Verdict.Unmet)
	}
}
