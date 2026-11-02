package lifecycle

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	localgit "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit"
	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit/localgittest"
	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func localLifecycle(t *testing.T) (ProposalLifecycle, string) {
	t.Helper()
	root, origin := gitFixture(t)
	gitTest(t, root, "branch", "-m", "trunk")
	gitTest(t, root, "push", "origin", "trunk")
	gitTest(t, origin, "symbolic-ref", "HEAD", "refs/heads/trunk")
	cfg := config.Defaults()
	cfg.Branch = "trunk"
	l := ProposalLifecycle{Root: root, Config: cfg, Run: draftshell.RunLocalGit}
	l.Store = localgit.NewProposals(localgit.ProposalOptions{Git: localgit.Options{Root: root, Origin: origin, Remote: "origin", Mainline: "trunk"}, Kind: "draft"})
	return l, origin
}

func writeLocal(t *testing.T, root, path, body string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestProposalLifecyclePinsOriginMainlineAndRetainsTerminalHistory(t *testing.T) {
	localgittest.Skip(t)
	l, origin := localLifecycle(t)
	old := gitTest(t, l.Root, "rev-parse", "origin/trunk")
	other := filepath.Join(t.TempDir(), "other")
	gitTest(t, filepath.Dir(other), "clone", origin, other)
	gitTest(t, other, "config", "user.name", "test")
	gitTest(t, other, "config", "user.email", "test@example.test")
	writeLocal(t, other, ".pyxis/templates/prd-template.md", "# Fresh <Feature or capability name> — PRD\n")
	gitTest(t, other, "add", "--", ".pyxis/templates/prd-template.md")
	gitTest(t, other, "commit", "-m", "new template")
	gitTest(t, other, "push", "origin", "trunk")
	base := gitTest(t, origin, "rev-parse", "refs/heads/trunk")
	opened, err := l.Open(t.Context(), "widget")
	if err != nil {
		t.Fatal(err)
	}
	if opened.Generation.Proposal.TargetOID != base || old == base || gitTest(t, l.Root, "rev-parse", "origin/trunk") != old {
		t.Fatal("draft used or rewrote a cached mainline ref")
	}
	body, err := os.ReadFile(filepath.Join(l.Root, "drafts/widget/prd.md"))
	if err != nil || !bytes.HasPrefix(body, []byte("# Fresh")) {
		t.Fatalf("template=%s error=%v", body, err)
	}
	s, _, err := l.Inspect(t.Context())
	if err != nil || !s.OpenDrafts["widget"] {
		t.Fatalf("snapshot=%+v %v", s, err)
	}
	writeLocal(t, l.Root, "drafts/widget/brainstorm/round.md", "One small change.\n")
	gitTest(t, l.Root, "add", "--", "drafts/widget/brainstorm/round.md")
	gitTest(t, l.Root, "commit", "-m", "draft round")
	round, err := l.PublishRound(t.Context(), "widget")
	if err != nil {
		t.Fatal(err)
	}
	if round.Generation.Proposal.Sequence != 2 || round.Generation.Proposal.PreviousOID != opened.Generation.OID {
		t.Fatal("round did not advance its paired generation")
	}
	retry, err := l.PublishRound(t.Context(), "widget")
	if err != nil || retry.Generation.OID != round.Generation.OID {
		t.Fatal("round retry minted another generation")
	}
	clone := filepath.Join(t.TempDir(), "clone")
	gitTest(t, filepath.Dir(clone), "clone", origin, clone)
	reader := localgit.NewProposals(localgit.ProposalOptions{Git: localgit.Options{Root: clone, Origin: origin, Remote: "origin", Mainline: "trunk"}, Kind: "draft"})
	seen, err := reader.Read(t.Context(), round.Generation.Proposal.ID)
	if err != nil || seen.OID != round.Generation.OID {
		t.Fatal("fresh clone cannot discover round")
	}
	abandoned, err := l.Abandon(t.Context(), "widget", "No longer needed.")
	if err != nil {
		t.Fatal(err)
	}
	p := abandoned.Generation.Proposal
	if p.State != "abandoned" || p.AbandonmentReason != "No longer needed." || p.Head != round.Generation.Proposal.Head || p.ArchiveRef == "" || p.ReviewRequest != nil {
		t.Fatalf("terminal=%+v", p)
	}
	if gitTest(t, origin, "rev-parse", p.ArchiveRef) != abandoned.Generation.OID || gitTest(t, origin, "rev-parse", "refs/heads/trunk") != base {
		t.Fatal("abandonment lost its archive or changed mainline")
	}
	retry, err = l.Abandon(t.Context(), "widget", "No longer needed.")
	if err != nil || retry.Generation.OID != abandoned.Generation.OID {
		t.Fatal("terminal retry changed history")
	}
	if _, err = l.Abandon(t.Context(), "widget", "Different reason"); err == nil {
		t.Fatal("terminal reason was rewritten")
	}
	s, _, err = l.Inspect(t.Context())
	if err != nil || s.OpenDrafts["widget"] {
		t.Fatal("terminal proposal still open")
	}
	if gitTest(t, origin, "tag", "--list", "lock/widget/*") != "" {
		t.Fatal("abandonment created a lock")
	}
}

func TestProposalOpenRefusesAbandonedIdentityBeforeScaffolding(t *testing.T) {
	localgittest.Skip(t)
	l, origin := localLifecycle(t)
	if _, err := l.Open(t.Context(), "widget"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Abandon(t.Context(), "widget", "No longer needed."); err != nil {
		t.Fatal(err)
	}
	clone := filepath.Join(t.TempDir(), "clone")
	gitTest(t, filepath.Dir(clone), "clone", origin, clone)
	l.Root = clone
	l.Store = localgit.NewProposals(localgit.ProposalOptions{Git: localgit.Options{Root: clone, Origin: origin, Remote: "origin", Mainline: "trunk"}, Kind: "draft"})
	before, remote := gitTest(t, clone, "show-ref"), gitTest(t, origin, "show-ref")
	if _, err := l.Open(t.Context(), "widget"); err == nil || !strings.Contains(err.Error(), "abandoned draft v1") {
		t.Fatalf("terminal identity allowed a fresh scaffold: %v", err)
	}
	if gitTest(t, clone, "show-ref") != before || gitTest(t, origin, "show-ref") != remote || gitTest(t, clone, "status", "--porcelain") != "" {
		t.Fatal("refused opening changed local or origin state")
	}
	if _, err := os.Stat(filepath.Join(clone, "drafts/widget")); !os.IsNotExist(err) {
		t.Fatal("refused opening left draft files")
	}
}

func TestProposalStartValidatesActualVersionIdentityBeforeScaffolding(t *testing.T) {
	localgittest.Skip(t)
	for _, tc := range []struct {
		name     string
		length   int
		previous int
	}{
		{"first version", 120, 0},
		{"two-digit version", 119, 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, origin := localLifecycle(t)
			feature := strings.Repeat("a", tc.length)
			if tc.previous > 0 {
				tag := "lock/" + feature + "/v9"
				gitTest(t, l.Root, "tag", tag)
				gitTest(t, l.Root, "push", "origin", "refs/tags/"+tag)
			}
			l.Locked = func(context.Context, string, string, string, int, RunCommand) (LockedContract, error) {
				t.Fatal("invalid proposal identity reached locked-contract seeding")
				return LockedContract{}, nil
			}
			before, remote := gitTest(t, l.Root, "show-ref"), gitTest(t, origin, "show-ref")
			var err error
			if tc.previous == 0 {
				_, err = l.Open(t.Context(), feature)
			} else {
				_, err = l.Reopen(t.Context(), feature, "new-need", "issue:1")
			}
			if err == nil || !strings.Contains(err.Error(), "fitting the proposal identity") {
				t.Fatalf("oversized actual proposal identity accepted: %v", err)
			}
			if gitTest(t, l.Root, "show-ref") != before || gitTest(t, origin, "show-ref") != remote || gitTest(t, l.Root, "status", "--porcelain") != "" {
				t.Fatal("invalid proposal identity changed local or origin state")
			}
			if _, err := os.Stat(filepath.Join(l.Root, "drafts", feature)); !os.IsNotExist(err) {
				t.Fatal("invalid proposal identity left scaffold files")
			}
		})
	}
	l, _ := localLifecycle(t)
	opened, err := l.Open(t.Context(), strings.Repeat("a", 119))
	if err != nil || len(opened.Generation.Proposal.ID) != 128 {
		t.Fatalf("maximum first-version identity refused: %+v %v", opened, err)
	}
}

func TestProposalAbandonRetainsPublicationFailureAcrossClones(t *testing.T) {
	localgittest.Skip(t)
	for _, tc := range []struct {
		name string
		want error
	}{
		{"atomic unavailable", codehost.ErrAtomicUnavailable},
		{"stale branch lease", codehost.ErrProposalConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, origin := localLifecycle(t)
			opened, err := l.Open(t.Context(), "widget")
			if err != nil {
				t.Fatal(err)
			}
			p := opened.Generation.Proposal
			base := gitTest(t, origin, "rev-parse", "refs/heads/trunk")
			branchHead := p.Head
			if tc.name == "atomic unavailable" {
				gitTest(t, origin, "config", "receive.advertiseAtomic", "false")
			} else {
				l.Run = func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
					if strings.Join(args, " ") == "rev-parse HEAD" {
						gitTest(t, origin, "update-ref", p.Branch, base, p.Head)
						branchHead = base
					}
					return draftshell.RunLocalGit(ctx, dir, env, name, args...)
				}
			}
			if _, err := l.Abandon(t.Context(), "widget", "No longer needed."); !errors.Is(err, tc.want) {
				t.Fatalf("publication error lost: %v", err)
			}
			if gitTest(t, origin, "rev-parse", "refs/heads/trunk") != base || gitTest(t, origin, "rev-parse", "refs/pyxis/proposals/"+p.ID) != opened.Generation.OID || gitTest(t, origin, "rev-parse", p.Branch) != branchHead || gitTest(t, l.Root, "rev-parse", "HEAD") != p.Head || gitTest(t, l.Root, "status", "--porcelain") != "" {
				t.Fatal("failed abandonment changed the proposal or checkout")
			}
			if gitTest(t, origin, "for-each-ref", "--format=%(refname)", "refs/pyxis/proposal-archives") != "" {
				t.Fatal("failed abandonment published an archive")
			}
			clone := filepath.Join(t.TempDir(), "clone")
			gitTest(t, filepath.Dir(clone), "clone", origin, clone)
			pending, err := events.OpenPending(clone, "origin", "widget", 1)
			if err != nil {
				t.Fatal(err)
			}
			failures, err := pending.Read(t.Context())
			if err != nil || len(failures) != 1 || failures[0].Name != events.DraftStepFailed || failures[0].Feature != "widget" || failures[0].Step != "draft-abandon" || failures[0].ErrorClass != "step-error" {
				t.Fatalf("fresh clone lost abandonment failure: %+v %v", failures, err)
			}
			gitTest(t, origin, "config", "receive.advertiseAtomic", "true")
			gitTest(t, origin, "update-ref", p.Branch, p.Head, branchHead)
			l.Run = draftshell.RunLocalGit
			abandoned, err := l.Abandon(t.Context(), "widget", "No longer needed.")
			if err != nil {
				t.Fatal(err)
			}
			retry, err := l.Abandon(t.Context(), "widget", "No longer needed.")
			if err != nil || retry.Generation.OID != abandoned.Generation.OID {
				t.Fatal("successful abandonment retry changed its terminal generation")
			}
			failures, err = pending.Read(t.Context())
			if err != nil || len(failures) != 1 {
				t.Fatalf("successful abandonment retry added failure events: %+v %v", failures, err)
			}
		})
	}
}

func TestProposalRoundRetainsPublicationFailureAcrossClones(t *testing.T) {
	localgittest.Skip(t)
	for _, failure := range []string{"atomic unavailable", "stale branch lease"} {
		t.Run(failure, func(t *testing.T) {
			l, origin := localLifecycle(t)
			opened, err := l.Open(t.Context(), "widget")
			if err != nil {
				t.Fatal(err)
			}
			writeLocal(t, l.Root, "drafts/widget/brainstorm/round.md", "A revised draft.\n")
			gitTest(t, l.Root, "add", "--", "drafts/widget/brainstorm/round.md")
			gitTest(t, l.Root, "commit", "-m", "draft round")
			head := gitTest(t, l.Root, "rev-parse", "HEAD")
			p, want := opened.Generation.Proposal, codehost.ErrAtomicUnavailable
			if failure == "atomic unavailable" {
				gitTest(t, origin, "config", "receive.advertiseAtomic", "false")
			} else {
				want = codehost.ErrProposalConflict
				l.Run = func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
					if strings.Join(args, " ") == "rev-parse HEAD" {
						gitTest(t, origin, "update-ref", p.Branch, p.TargetOID, p.Head)
					}
					return draftshell.RunLocalGit(ctx, dir, env, name, args...)
				}
			}
			if _, err := l.PublishRound(t.Context(), "widget"); !errors.Is(err, want) {
				t.Fatalf("round publication error lost: %v", err)
			}
			if gitTest(t, origin, "rev-parse", "refs/heads/trunk") != p.TargetOID || gitTest(t, origin, "rev-parse", "refs/pyxis/proposals/"+p.ID) != opened.Generation.OID || gitTest(t, l.Root, "rev-parse", "HEAD") != head || gitTest(t, l.Root, "status", "--porcelain") != "" {
				t.Fatal("failed round changed mainline, proposal, or working files")
			}
			clone := filepath.Join(t.TempDir(), "clone")
			gitTest(t, filepath.Dir(clone), "clone", origin, clone)
			pending, err := events.OpenPending(clone, "origin", "widget", 1)
			if err != nil {
				t.Fatal(err)
			}
			retained, err := pending.Read(t.Context())
			if err != nil || len(retained) != 1 || retained[0].Name != events.DraftStepFailed || retained[0].Step != "draft-round" || retained[0].ErrorClass != "step-error" {
				t.Fatalf("fresh clone lost round failure: %+v %v", retained, err)
			}
		})
	}
}

type localTasks struct {
	items   []model.WorkItem
	defects []schema.Defect
}

func (t localTasks) List(context.Context, model.Query) ([]model.WorkItem, []schema.Defect) {
	return t.items, t.defects
}

func TestProposalReopenChecksCurrentPlanAndExactLockedBytes(t *testing.T) {
	localgittest.Skip(t)
	l, _ := localLifecycle(t)
	plan, err := os.ReadFile("testdata/reopen/001-plan.md")
	if err != nil {
		t.Fatal(err)
	}
	writeLocal(t, l.Root, "docs/plans/widget/001-plan.md", string(plan))
	gitTest(t, l.Root, "add", "--", "docs/plans/widget/001-plan.md")
	gitTest(t, l.Root, "commit", "-m", "plan")
	gitTest(t, l.Root, "push", "origin", "trunk")
	gitTest(t, l.Root, "tag", "lock/widget/v1")
	gitTest(t, l.Root, "push", "origin", "refs/tags/lock/widget/v1")
	seed := LockedContract{Version: 1, PRD: []byte("# Locked PRD\n"), TechSpec: []byte("# Locked design\n")}
	seed.Digest = compile.ContractDigest(seed.PRD, seed.TechSpec)
	reads := 0
	l.Locked = func(_ context.Context, _, remote, feature string, v int, _ RunCommand) (LockedContract, error) {
		reads++
		if remote != "origin" || feature != "widget" || v != 1 {
			t.Fatal("wrong lock selector")
		}
		return seed, nil
	}
	l.Tracker = localTasks{}
	if _, err := l.Reopen(t.Context(), "widget", "dependency-change", "issue:1"); err == nil || !strings.Contains(err.Error(), "not projected") || reads != 0 {
		t.Fatalf("unprojected=%v reads=%d", err, reads)
	}
	t1 := model.WorkItem{ID: 1, State: model.StateClosed, Body: "<!-- pyxis:plan-task widget/001/T1 -->"}
	t2 := model.WorkItem{ID: 2, State: model.StateOpen, Body: "<!-- pyxis:plan-task widget/001/T2 -->"}
	l.Tracker = localTasks{items: []model.WorkItem{t1, t2}}
	if _, err := l.Reopen(t.Context(), "widget", "other", ""); err == nil || !strings.Contains(err.Error(), "T2") {
		t.Fatal("unfinished task allowed reopen")
	}
	t2.State = model.StateClosed
	l.Tracker = localTasks{items: []model.WorkItem{t1, t2}}
	if _, err := l.Open(t.Context(), "widget"); err == nil {
		t.Fatal("locked feature opened as v1")
	}
	opened, err := l.Reopen(t.Context(), "widget", "other", "")
	if err != nil {
		t.Fatal(err)
	}
	if opened.Version != 2 || opened.Generation.Proposal.Branch != "refs/heads/draft/widget-v2" {
		t.Fatalf("reopened=%+v", opened)
	}
	for name, want := range map[string][]byte{"prd.md": seed.PRD, "tech-spec.md": seed.TechSpec} {
		got, err := os.ReadFile(filepath.Join(l.Root, "drafts/widget", name))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatal("reopen changed binding bytes")
		}
	}
	logBody, err := os.ReadFile(filepath.Join(l.Root, "drafts/widget/events.md"))
	if err != nil {
		t.Fatal(err)
	}
	log, err := events.Parse(logBody)
	if err != nil || len(log) < 2 || log[len(log)-2].Origin != "reopen" || log[len(log)-1].CauseRef != "lock/widget/v1" {
		t.Fatalf("events=%+v %v", log, err)
	}
	if _, err := l.Reopen(t.Context(), "widget", "other", ""); err == nil {
		t.Fatal("duplicate reopen allowed")
	}
}

type brokenDraftStore struct {
	codehost.DraftProposals
	fail string
}

func (b brokenDraftStore) Mainline(ctx context.Context) (codehost.Readiness, error) {
	if b.fail == "mainline" {
		return codehost.Readiness{}, errors.New("unavailable mainline")
	}
	return b.DraftProposals.Mainline(ctx)
}
func (b brokenDraftStore) List(ctx context.Context) ([]codehost.ProposalGeneration, error) {
	if b.fail == "list" {
		return nil, errors.New("unreadable proposals")
	}
	return b.DraftProposals.List(ctx)
}
func (b brokenDraftStore) Read(ctx context.Context, id string) (codehost.ProposalGeneration, error) {
	if b.fail == "read" {
		return codehost.ProposalGeneration{}, errors.New("unreadable generation")
	}
	return b.DraftProposals.Read(ctx, id)
}
func (b brokenDraftStore) Publish(ctx context.Context, c codehost.ProposalChange) (codehost.ProposalGeneration, error) {
	if b.fail == "publish" {
		return codehost.ProposalGeneration{}, errors.New("publication interrupted")
	}
	if b.fail == "after publish" {
		_, err := b.DraftProposals.Publish(ctx, c)
		return codehost.ProposalGeneration{}, errors.Join(err, errors.New("publication response lost"))
	}
	return b.DraftProposals.Publish(ctx, c)
}

func TestProposalOpeningRecoversInterruptedPublicationAndRollsBackEarlierFailures(t *testing.T) {
	localgittest.Skip(t)
	for _, fail := range []string{"mainline", "list", "ls-tree", "ls-remote --tags", "show", "branch --show-current", "rev-parse HEAD", "switch", "add", "commit", "publish", "after publish"} {
		t.Run(fail, func(t *testing.T) {
			l, origin := localLifecycle(t)
			base := gitTest(t, origin, "rev-parse", "refs/heads/trunk")
			realStore := l.Store
			l.Store = brokenDraftStore{realStore, fail}
			l.Run = func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if strings.HasPrefix(strings.Join(args, " "), fail) {
					return nil, errors.New("injected Git failure")
				}
				return draftshell.RunLocalGit(ctx, dir, env, name, args...)
			}
			if _, err := l.Open(t.Context(), "widget"); err == nil {
				t.Fatal("injected failure accepted")
			}
			if gitTest(t, origin, "rev-parse", "refs/heads/trunk") != base {
				t.Fatal("failure changed mainline")
			}
			if fail == "publish" || fail == "after publish" {
				l.Store = realStore
				l.Run = draftshell.RunLocalGit
				recovered, err := l.PublishRound(t.Context(), "widget")
				if err != nil || recovered.Generation.Proposal.Sequence != 1 {
					t.Fatalf("recovery=%+v %v", recovered, err)
				}
			} else if gitTest(t, l.Root, "branch", "--show-current") != "trunk" || gitTest(t, l.Root, "status", "--porcelain") != "" {
				t.Fatal("pre-publication failure left scaffold changes")
			}
		})
	}
}

func TestProposalInspectionAndRoundRefuseUnreadableOrForeignState(t *testing.T) {
	localgittest.Skip(t)
	if _, _, err := (ProposalLifecycle{}).Inspect(t.Context()); err == nil {
		t.Fatal("missing composition accepted")
	}
	l, _ := localLifecycle(t)
	real := l.Store
	l.Store = brokenDraftStore{real, "mainline"}
	if _, _, err := l.Inspect(t.Context()); err == nil {
		t.Fatal("mainline failure hidden")
	}
	l.Store = real
	for _, failure := range []string{"ledger", "tags", "malformed tags"} {
		l.Run = func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
			if args[0] == "ls-tree" && failure == "ledger" || args[0] == "ls-remote" && failure == "tags" {
				return nil, errors.New("read unavailable")
			}
			if args[0] == "ls-remote" && failure == "malformed tags" {
				return []byte("not a valid tag advertisement"), nil
			}
			return draftshell.RunLocalGit(ctx, dir, env, name, args...)
		}
		if _, _, err := l.Inspect(t.Context()); err == nil {
			t.Fatal("unreadable contracts accepted")
		}
	}
	l.Run = draftshell.RunLocalGit
	if _, err := l.Open(t.Context(), "widget"); err != nil {
		t.Fatal(err)
	}
	for _, failure := range []string{"mainline", "list", "read"} {
		l.Store = brokenDraftStore{real, failure}
		if _, err := l.PublishRound(t.Context(), "widget"); err == nil {
			t.Fatalf("%s accepted", failure)
		}
		if _, err := l.Abandon(t.Context(), "widget", "test"); err == nil {
			t.Fatalf("abandon %s accepted", failure)
		}
	}
	l.Store = real
	writeLocal(t, l.Root, "drafts/widget/events.md", "broken events")
	gitTest(t, l.Root, "add", "--", "drafts/widget/events.md")
	gitTest(t, l.Root, "commit", "-m", "invalid log")
	if _, err := l.PublishRound(t.Context(), "widget"); err == nil {
		t.Fatal("malformed working history published")
	}
	writeLocal(t, l.Root, "drafts/widget/events.md", "# Draft events\n")
	gitTest(t, l.Root, "add", "--", "drafts/widget/events.md")
	gitTest(t, l.Root, "commit", "-m", "empty log")
	if _, err := l.PublishRound(t.Context(), "widget"); err == nil {
		t.Fatal("missing opening event published")
	}
	event, err := events.New(events.DraftOpened, "other", "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	event.Origin = "new"
	body, _, err := events.Append(nil, event)
	if err != nil {
		t.Fatal(err)
	}
	writeLocal(t, l.Root, "drafts/widget/events.md", string(body))
	gitTest(t, l.Root, "add", "--", "drafts/widget/events.md")
	gitTest(t, l.Root, "commit", "-m", "foreign log")
	if _, err := l.PublishRound(t.Context(), "widget"); err == nil {
		t.Fatal("foreign feature events published")
	}
}

func TestProposalReopenRefusesMissingOrCorruptLockReader(t *testing.T) {
	localgittest.Skip(t)
	for _, failure := range []string{"nil", "read", "digest", "version"} {
		t.Run(failure, func(t *testing.T) {
			l, _ := localLifecycle(t)
			gitTest(t, l.Root, "tag", "lock/widget/v1")
			gitTest(t, l.Root, "push", "origin", "refs/tags/lock/widget/v1")
			if failure != "nil" {
				l.Locked = func(context.Context, string, string, string, int, RunCommand) (LockedContract, error) {
					if failure == "read" {
						return LockedContract{}, errors.New("lock unavailable")
					}
					seed := LockedContract{Version: 1, PRD: []byte("p"), TechSpec: []byte("d")}
					seed.Digest = compile.ContractDigest(seed.PRD, seed.TechSpec)
					if failure == "digest" {
						seed.Digest = "wrong"
					}
					if failure == "version" {
						seed.Version = 2
					}
					return seed, nil
				}
			}
			if _, err := l.Reopen(t.Context(), "widget", "other", ""); err == nil {
				t.Fatal("unverified seed accepted")
			}
			if gitTest(t, l.Root, "branch", "--show-current") != "trunk" || gitTest(t, l.Root, "status", "--porcelain") != "" {
				t.Fatal("refused reopen wrote working files")
			}
		})
	}
}

func TestProposalTaskReaderRefusesAmbiguousOrUnreadableProjection(t *testing.T) {
	localgittest.Skip(t)
	root := mergedPlanFixture(t)
	cfg := config.Defaults()
	base := gitTest(t, root, "rev-parse", "origin/main")
	for _, tracker := range []localTasks{{defects: []schema.Defect{{Reason: "unreadable item"}}}, {items: []model.WorkItem{{Body: "<!-- pyxis:plan-task widget/001/T1 -->"}, {Body: "<!-- pyxis:plan-task widget/001/T1 -->"}}}} {
		if _, err := UnfinishedTasksWithTracker(t.Context(), root, cfg, "widget", base, tracker, draftshell.RunLocalGit); err == nil {
			t.Fatal("ambiguous projection accepted")
		}
	}
	if _, err := UnfinishedTasksWithTracker(t.Context(), root, cfg, "widget", base, nil, draftshell.RunLocalGit); err == nil {
		t.Fatal("missing tracker accepted")
	}
	if _, err := UnfinishedTasksWithTracker(t.Context(), root, cfg, "Bad", base, nil, draftshell.RunLocalGit); err == nil {
		t.Fatal("invalid feature accepted")
	}
	for _, failure := range []string{"ls-tree", "show", "parse"} {
		run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
			if args[0] == failure {
				return nil, errors.New("unavailable plan")
			}
			if failure == "parse" && args[0] == "show" {
				return []byte("invalid plan"), nil
			}
			return draftshell.RunLocalGit(ctx, dir, env, name, args...)
		}
		if _, err := UnfinishedTasksWithTracker(t.Context(), root, cfg, "widget", base, localTasks{}, run); err == nil {
			t.Fatal("unreadable merged plan accepted")
		}
	}
}

func TestProposalLifecycleRefusesAbsentMainlineAndUnpublishedEdits(t *testing.T) {
	localgittest.Skip(t)
	l, origin := localLifecycle(t)
	gitTest(t, origin, "update-ref", "-d", "refs/heads/trunk")
	before := gitTest(t, origin, "show-ref")
	if _, err := l.Open(t.Context(), "widget"); err == nil || !strings.Contains(err.Error(), "first configured-mainline push") {
		t.Fatalf("missing mainline=%v", err)
	}
	if gitTest(t, origin, "show-ref") != before || gitTest(t, l.Root, "status", "--porcelain") != "" {
		t.Fatal("missing-mainline refusal wrote state")
	}
	gitTest(t, l.Root, "push", "origin", "trunk")
	if _, err := l.Open(t.Context(), "Bad"); err == nil {
		t.Fatal("invalid feature accepted")
	}
	if _, err := l.Reopen(t.Context(), "widget", "invalid", ""); err == nil {
		t.Fatal("invalid cause accepted")
	}
	if _, err := l.Reopen(t.Context(), "widget", "dependency-change", ""); err == nil {
		t.Fatal("missing evidence accepted")
	}
	if _, err := l.Reopen(t.Context(), "widget", "other", ""); err == nil {
		t.Fatal("missing lock accepted")
	}
	if _, err := l.Open(t.Context(), "widget"); err != nil {
		t.Fatal(err)
	}
	writeLocal(t, l.Root, "drafts/widget/brainstorm/unsaved.md", "edit")
	if _, err := l.PublishRound(t.Context(), "widget"); err == nil {
		t.Fatal("dirty round published")
	}
	gitTest(t, l.Root, "add", "--", "drafts/widget/brainstorm/unsaved.md")
	gitTest(t, l.Root, "commit", "-m", "unsaved round")
	if _, err := l.Abandon(t.Context(), "widget", "test"); err == nil {
		t.Fatal("unpublished edits abandoned")
	}
}
