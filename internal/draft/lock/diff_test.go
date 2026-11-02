package lock

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/packet"
	"github.com/agenticlab-ai/pyxis/internal/memory/policy"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
	"github.com/agenticlab-ai/pyxis/internal/memory/view"
)

func TestPrepareReopenedLockWithOneChangedContractDocument(t *testing.T) {
	work, firstRequest, host := preparedMerge(t)
	firstApproval := &recordApproval{result: approval.Approval{Operator: "mdarabi", Head: firstRequest.ApprovedHead, ReviewID: 27}}
	if _, err := RecordMerge(t.Context(), work, firstRequest, firstApproval, host.run); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, work, "switch", "-c", "draft/widget-v2", "main")
	prd, tech := readyPRD(t), []byte("# Widget — tech spec\n\n### D-1 — Open widget\n\n**Decision:** Open and queue the widget.\n\n**Serves:** FR-1 · UB-1\n")
	fixtureFile(t, work, "drafts/widget/prd.md", prd)
	fixtureFile(t, work, "drafts/widget/tech-spec.md", tech)
	gitFixture(t, work, "add", "--", "drafts/widget/prd.md", "drafts/widget/tech-spec.md")
	gitFixture(t, work, "commit", "-m", "seed reopened draft")
	changed := []byte(strings.Replace(string(prd), "The widget shall open.", "The widget shall open durably.", 1))
	fixtureFile(t, work, "drafts/widget/prd.md", changed)
	fixtureFile(t, work, "drafts/widget/problems.md", []byte("# Problems\n\n### P-1 — FR-1 — Opening durability\n\n**Raised by:** operator\n**Problem:** Opening must survive retries.\n**Ruling:** operator · retries need durability · Reword FR-1 for durable opening.\n"))
	draft, err := compile.Compile("widget", "main", map[string][]byte{"prd.md": changed, "tech-spec.md": tech})
	if err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, work, "drafts/widget/reviews/003.md", []byte("# PRD review\n\n**Review subject:** prd\n**Draft digest:** "+draft.Digest+"\n"))
	fixtureFile(t, work, "drafts/widget/reviews/004.md", []byte("# Tech-spec review\n\n**Review subject:** tech-spec\n**Draft digest:** "+draft.Digest+"\n"))
	gitFixture(t, work, "add", "--", "drafts/widget")
	gitFixture(t, work, "commit", "-m", "reword opening requirement")
	gitFixture(t, work, "push", "-u", "origin", "draft/widget-v2")
	cfg := config.Defaults()
	cfg.Draft.AgentLogin = "agent"
	cfg.Draft.AgentTokenEnv = "PYXIS_DRAFT_TEST_TOKEN"
	prepared, err := Prepare(t.Context(), work, cfg, "widget", host.run)
	if err != nil || !prepared.Verdict.Ready {
		t.Fatalf("prepare reopened lock: %+v %v", prepared, err)
	}
	set, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(work, store.Dir)})
	if err != nil || len(set.Problems) > 0 || len(set.Dangling) > 0 {
		t.Fatalf("prepared memory: %v problems=%v dangling=%v", err, set.Problems, set.Dangling)
	}
	projected := resolve.New(set)
	if len(projected.Violations) > 0 || len(projected.Conflicts()) > 0 || len(projected.AnswerConflicts()) > 0 {
		t.Fatalf("prepared projection: violations=%v conflicts=%v answers=%v", projected.Violations, projected.Conflicts(), projected.AnswerConflicts())
	}
	baseSet := mustLockedSet(t, work, "refs/tags/lock/widget/v1")
	check := packet.Compute(packet.Input{Base: baseSet, Head: set, Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "draft/widget-v2", BaseRef: "lock/widget/v1"})
	if !check.OK() {
		t.Fatalf("prepared memory packet has blockers: %+v", check.Blockers)
	}
	if projected.Head("design/widget", "design").ID != activeAt(resolve.New(baseSet), "design/widget", "design").ID {
		t.Fatal("unchanged tech spec Document was reissued")
	}
	gitFixture(t, work, "switch", "main")
	gitFixture(t, work, "merge", "--squash", "draft/widget-v2")
	gitFixture(t, work, "commit", "-m", "Lock widget v2 (#18)")
	gitFixture(t, work, "push", "origin", "main")
	second := RecordRequest{Feature: "widget", Remote: "origin", ExpectedVersion: 2, MergeCommit: gitFixture(t, work, "rev-parse", "HEAD"), PullRequest: 18, ApprovedHead: prepared.Head, ReadinessDigest: prepared.Verdict.DraftDigest, ProposalsSinceOpened: 1}
	secondApproval := &recordApproval{result: approval.Approval{Operator: "mdarabi", Head: prepared.Head, ReviewID: 28}}
	if out, err := RecordMerge(t.Context(), work, second, secondApproval, host.run); err != nil || out.Lock == nil || out.Lock.Version != 2 {
		t.Fatalf("record reopened lock: %+v %v", out, err)
	}
	for _, tc := range []struct {
		version int
		want    string
	}{{1, "The widget shall open."}, {2, "The widget shall open durably."}} {
		ref := "refs/tags/lock/widget/v" + strconv.Itoa(tc.version)
		locked := resolve.New(mustLockedSet(t, work, ref))
		text, err := view.Render(locked, "prd/widget", "prd", ref)
		if err != nil || !strings.Contains(text, tc.want) {
			t.Fatalf("render %s: %v %s", ref, err, text)
		}
	}
}

func TestPrepareReopenedLockRequiresReadableBindingVersion(t *testing.T) {
	for _, tc := range []struct {
		name, want    string
		unreadableTag bool
	}{
		{name: "missing", want: "no binding lock exists"},
		{name: "unreadable", want: "load binding version", unreadableTag: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work, cfg, host := prepareFixture(t, true)
			gitFixture(t, work, "branch", "-m", "draft/widget-v2")
			gitFixture(t, work, "push", "-u", "origin", "draft/widget-v2")
			if tc.unreadableTag {
				gitFixture(t, work, "tag", "lock/widget/v1", "main")
				gitFixture(t, work, "push", "origin", "refs/tags/lock/widget/v1")
			}
			head := gitFixture(t, work, "rev-parse", "HEAD")
			if _, err := Prepare(t.Context(), work, cfg, "widget", host.run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("unreadable binding lock was accepted: %v", err)
			}
			if gitFixture(t, work, "rev-parse", "HEAD") != head || gitFixture(t, work, "status", "--porcelain") != "" {
				t.Fatal("refused reopened preparation changed the working tree")
			}
		})
	}
}

func mustLockedSet(t *testing.T, root, ref string) *store.Set {
	t.Helper()
	repo, err := gitsrc.Open(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	set, err := store.Load(t.Context(), gitsrc.TreeSource{Repo: repo, Ref: ref, Prefix: store.Dir})
	if err != nil || len(set.Problems) > 0 {
		t.Fatalf("load %s: %v %+v", ref, err, set.Problems)
	}
	return set
}

func diffUnit(kind, subject, family, statement string, links ...compile.Link) compile.Record {
	return compile.Record{Draft: reconcile.Draft{Kind: kind, Subject: subject, Family: family, Statement: statement}, Links: links}
}

func diffDocument(subject, family, text string) compile.Record {
	return compile.Record{Draft: reconcile.Draft{Kind: record.Document, Subject: subject, Family: family, Statement: "Draft " + family, LiteralBody: true, Body: []byte(text)}}
}

func TestReopenedLockWritesDifferencesAndRetainsBindingHistory(t *testing.T) {
	root := t.TempDir()
	first := compile.Result{Feature: "widget", Records: []compile.Record{
		diffUnit(record.Promise, "prd/widget", "FR-1", "Queue a widget."),
		diffUnit(record.Promise, "prd/widget", "FR-2", "Open a widget."),
		diffUnit(record.Promise, "prd/widget", "FR-3", "Close a widget."),
		diffUnit(record.Question, "prd/widget", "Q-1", "Who approves?"),
		diffUnit(record.Answer, "prd/widget", "", "The operator.", compile.Link{Type: record.Answers, Target: "prd/widget#Q-1"}),
		diffUnit(record.Question, "prd/widget", "P-1", "Why queue?"),
		diffUnit(record.Answer, "prd/widget", "", "For retries.", compile.Link{Type: record.Answers, Target: "prd/widget#P-1"}),
		diffUnit(record.Decision, "design/widget", "D-1", "Use a queue.", compile.Link{Type: record.Serves, Target: "prd/widget#FR-1"}),
		diffUnit(record.Decision, "design/widget", "D-2", "Open directly.", compile.Link{Type: record.Serves, Target: "prd/widget#FR-2"}),
		diffDocument("prd/widget", "prd", "# Widget PRD\nFR-1 queue. FR-2 open. FR-3 close.\n"),
		diffDocument("design/widget", "design", "# Widget design\nD-1 queue. D-2 open.\n"),
		diffDocument("prd/widget", "problems", "# Problems\nP-1 why queue?\n"),
		diffDocument("brainstorm/widget", "record-001", "# Brainstorm\n"),
		diffDocument("review/widget", "report-001", "# Review\n"),
		diffDocument("feature/widget", "events", "# Events\n"),
	}}
	if _, _, err := writeRecords(root, "agent:test", first, readiness.Sources{}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(root, store.Dir)})
	if err != nil || len(before.Problems) > 0 {
		t.Fatalf("load first lock: %v %v", err, before.Problems)
	}
	binding := resolve.New(before)
	second := compile.Result{Feature: "widget", Records: []compile.Record{
		diffUnit(record.Promise, "prd/widget", "FR-1", "Queue widgets durably."),
		diffUnit(record.Promise, "prd/widget", "FR-3", "Close a widget."),
		diffUnit(record.Promise, "prd/widget", "FR-4", "Retry a widget."),
		diffUnit(record.Decision, "design/widget", "D-1", "Use a queue.", compile.Link{Type: record.Serves, Target: "prd/widget#FR-1"}),
		diffDocument("prd/widget", "prd", "# Widget PRD\nFR-1 durable. FR-3 close. FR-4 retry.\n"),
		diffDocument("design/widget", "design", "# Widget design\nD-1 queue. D-2 open.\n"),
		diffDocument("feature/widget", "events", "# Events v2\n"),
	}}
	docs, paths, err := writeRecordsWithBinding(root, "agent:test", second, readiness.Sources{Mainline: binding}, binding, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 11 || docs["prd"] == "" || docs["tech-spec"] == "" {
		t.Fatalf("new paths=%d: %v docs=%v", len(paths), paths, docs)
	}
	after, err := store.Load(context.Background(), store.DirSource{Root: filepath.Join(root, store.Dir)})
	if err != nil || len(after.Problems) > 0 || len(after.Dangling) > 0 {
		t.Fatalf("load reopened lock: %v problems=%v dangling=%v", err, after.Problems, after.Dangling)
	}
	current := resolve.New(after)
	if len(current.Violations) > 0 || len(current.Conflicts()) > 0 || len(current.AnswerConflicts()) > 0 {
		t.Fatalf("invalid reopened projection: violations=%v conflicts=%v answer conflicts=%v", current.Violations, current.Conflicts(), current.AnswerConflicts())
	}
	if current.Head("prd/widget", "FR-3").ID != binding.Head("prd/widget", "FR-3").ID || current.Head("design/widget", "design").ID != binding.Head("design/widget", "design").ID {
		t.Fatal("unchanged unit was reissued")
	}
	if current.Head("prd/widget", "FR-1").ID == binding.Head("prd/widget", "FR-1").ID || current.Head("prd/widget", "FR-2") != nil || current.Head("prd/widget", "FR-4") == nil {
		t.Fatal("changed, removed, or new requirement has wrong state")
	}
	if decision := current.Head("design/widget", "D-1"); decision == nil || decision.ID == binding.Head("design/widget", "D-1").ID || current.Targets(decision, record.Serves)[0] != current.Head("prd/widget", "FR-1").ID {
		t.Fatalf("unchanged decision text did not follow changed requirement: %+v", decision)
	}
	if len(current.Undesigned("prd/widget")) != 2 { // FR-3 and FR-4 have no design in this focused fixture.
		t.Fatalf("serving projection=%v", current.Undesigned("prd/widget"))
	}
	var problemQuestion string
	for _, old := range binding.Active("prd/widget") {
		if old.Kind == record.Question && old.Family == "P-1" {
			problemQuestion = old.ID
		}
	}
	if problemQuestion == "" {
		t.Fatal("binding problem question absent")
	}
	for _, historical := range []string{problemQuestion, binding.Head("prd/widget", "problems").ID, binding.Head("brainstorm/widget", "record-001").ID, binding.Head("review/widget", "report-001").ID, binding.Head("feature/widget", "events").ID} {
		if current.Status(historical) != resolve.Active {
			t.Fatalf("historical record %s retired", historical)
		}
	}
	if current.Head("feature/widget", "events-v2") == nil {
		t.Fatal("new version's events did not get a distinct address")
	}
	oldText, err := view.Render(binding, "prd/widget", "prd", "lock/widget/v1")
	if err != nil || !strings.Contains(oldText, "FR-2 open") {
		t.Fatalf("binding render: %v %s", err, oldText)
	}
	newText, err := view.Render(current, "prd/widget", "prd", "lock/widget/v2")
	if err != nil || !strings.Contains(newText, "FR-4 retry") || strings.Contains(newText, "FR-2 open") {
		t.Fatalf("reopened render: %v %s", err, newText)
	}
}

func TestRewordedQuestionReissuesItsAnswer(t *testing.T) {
	root := t.TempDir()
	old := compile.Result{Feature: "widget", Records: []compile.Record{
		diffUnit(record.Question, "prd/widget", "Q-1", "Who approves?"),
		diffUnit(record.Answer, "prd/widget", "", "The operator.", compile.Link{Type: record.Answers, Target: "prd/widget#Q-1"}),
		diffDocument("prd/widget", "prd", "# PRD\nWho approves?\n"),
		diffDocument("design/widget", "design", "# Design\n"),
	}}
	if _, _, err := writeRecords(root, "agent:test", old, readiness.Sources{}); err != nil {
		t.Fatal(err)
	}
	set, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(root, store.Dir)})
	if err != nil {
		t.Fatal(err)
	}
	binding := resolve.New(set)
	oldQuestion := activeAt(binding, "prd/widget", "Q-1")
	oldAnswer := binding.Answer(oldQuestion.ID)
	next := compile.Result{Feature: "widget", Records: []compile.Record{
		diffUnit(record.Question, "prd/widget", "Q-1", "Who signs approval?"),
		diffUnit(record.Answer, "prd/widget", "", "The operator.", compile.Link{Type: record.Answers, Target: "prd/widget#Q-1"}),
		diffDocument("prd/widget", "prd", "# PRD\nWho signs approval?\n"),
		diffDocument("design/widget", "design", "# Design\n"),
	}}
	if _, _, err := writeRecordsWithBinding(root, "agent:test", next, readiness.Sources{Mainline: binding}, binding, 2); err != nil {
		t.Fatal(err)
	}
	set, err = store.Load(t.Context(), store.DirSource{Root: filepath.Join(root, store.Dir)})
	if err != nil || len(set.Problems) > 0 {
		t.Fatalf("reopened memory: %v %+v", err, set.Problems)
	}
	current := resolve.New(set)
	newQuestion := activeAt(current, "prd/widget", "Q-1")
	if len(current.Violations) > 0 || newQuestion == nil || newQuestion.ID == oldQuestion.ID || current.Status(oldQuestion.ID) != resolve.Superseded || current.Status(oldAnswer.ID) != resolve.Revoked {
		t.Fatalf("question transition: question=%+v violations=%v old question=%s old answer=%s", newQuestion, current.Violations, current.Status(oldQuestion.ID), current.Status(oldAnswer.ID))
	}
	if answer := current.Answer(newQuestion.ID); answer == nil || answer.ID == oldAnswer.ID {
		t.Fatalf("reworded question lacks new answer: %+v", answer)
	}
}

func TestReopenedDiffRejectsUnsafeAddresses(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*compile.Result)
		want string
	}{
		{"history reuse", func(r *compile.Result) {
			r.Records = append(r.Records, diffDocument("review/widget", "report-001", "# Reused review\n"))
		}, "already used"},
		{"kind change", func(r *compile.Result) { r.Records[0].Draft.Kind = record.Need }, "changes kind"},
		{"orphaned serves", func(r *compile.Result) { r.Records = r.Records[1:] }, "row absent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			first := compile.Result{Feature: "widget", Records: []compile.Record{
				diffUnit(record.Promise, "prd/widget", "FR-1", "Open widgets."),
				diffUnit(record.Decision, "design/widget", "D-1", "Open directly.", compile.Link{Type: record.Serves, Target: "prd/widget#FR-1"}),
				diffDocument("prd/widget", "prd", "# PRD\n"),
				diffDocument("design/widget", "design", "# Design\n"),
				diffDocument("review/widget", "report-001", "# Old review\n"),
			}}
			if _, _, err := writeRecords(root, "agent:test", first, readiness.Sources{}); err != nil {
				t.Fatal(err)
			}
			set, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(root, store.Dir)})
			if err != nil {
				t.Fatal(err)
			}
			binding := resolve.New(set)
			next := compile.Result{Feature: "widget", Records: append([]compile.Record(nil), first.Records[:4]...)}
			tc.edit(&next)
			if _, _, err := writeRecordsWithBinding(root, "agent:test", next, readiness.Sources{Mainline: binding}, binding, 2); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestReopenedDiffRevokesRemovedAnswerAndUpdatesMetadata(t *testing.T) {
	root := t.TempDir()
	claim := diffUnit(record.Promise, "prd/widget", "FR-1", "Open widgets.")
	claim.Draft.FitCriterion = "Opens once."
	first := compile.Result{Feature: "widget", Records: []compile.Record{
		claim,
		diffUnit(record.Question, "prd/widget", "Q-1", "Who approves?"),
		diffUnit(record.Answer, "prd/widget", "", "The operator.", compile.Link{Type: record.Answers, Target: "prd/widget#Q-1"}),
		diffDocument("prd/widget", "prd", "# PRD\n"),
		diffDocument("design/widget", "design", "# Design\n"),
	}}
	if _, _, err := writeRecords(root, "agent:test", first, readiness.Sources{}); err != nil {
		t.Fatal(err)
	}
	set, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(root, store.Dir)})
	if err != nil {
		t.Fatal(err)
	}
	binding := resolve.New(set)
	oldQuestion := activeAt(binding, "prd/widget", "Q-1")
	oldAnswer := binding.Answer(oldQuestion.ID)
	claim.Draft.FitCriterion = "Opens after retry."
	next := compile.Result{Feature: "widget", Records: []compile.Record{
		claim,
		diffUnit(record.Question, "prd/widget", "Q-1", "Who approves?"),
		diffDocument("prd/widget", "prd", "# PRD\n"),
		diffDocument("design/widget", "design", "# Design\n"),
	}}
	if _, _, err := writeRecordsWithBinding(root, "agent:test", next, readiness.Sources{Mainline: binding}, binding, 2); err != nil {
		t.Fatal(err)
	}
	set, err = store.Load(t.Context(), store.DirSource{Root: filepath.Join(root, store.Dir)})
	if err != nil || len(set.Problems) > 0 {
		t.Fatalf("memory after change: %v %+v", err, set.Problems)
	}
	current := resolve.New(set)
	if current.Head("prd/widget", "FR-1").ID == binding.Head("prd/widget", "FR-1").ID || current.Status(oldAnswer.ID) != resolve.Revoked || current.Status(oldQuestion.ID) != resolve.Active || len(current.Violations) > 0 {
		t.Fatalf("metadata and answer removal: violations=%v old answer=%s", current.Violations, current.Status(oldAnswer.ID))
	}
}
