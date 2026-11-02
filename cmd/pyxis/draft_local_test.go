package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	localcodehost "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit"
	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit/localgittest"
	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/localfs"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func TestLocalDraftCommandsPublishAndObserveWithoutHostedCalls(t *testing.T) {
	localgittest.Skip(t)
	root, origin := localGitCLIFixture(t, true)
	for file, body := range map[string]string{"prd-template.md": "# <Feature or capability name> — PRD\n", "draft-tech-spec-template.md": "# <Feature> — tech spec\n", "draft-problems-template.md": "# Requirement problems\n", "draft-events-template.md": "# Draft events\n"} {
		p := filepath.Join(root, ".pyxis/templates", file)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	localGitCLI(t, root, "add", "--", ".pyxis/templates")
	localGitCLI(t, root, "commit", "-m", "draft templates")
	localGitCLI(t, root, "push", "upstream", "trunk")
	base := localGitCLI(t, origin, "rev-parse", "refs/heads/trunk")
	swap(t, &draftRepoRoot, func() (string, error) { return root, nil })
	swap(t, &draftCommand, func(context.Context, string, []string, string, ...string) ([]byte, error) {
		t.Fatal("entered hosted lifecycle")
		return nil, nil
	})
	calls := 0
	swap(t, &localDraftCommand, func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		calls++
		if name != "git" {
			t.Fatalf("hosted command %s", name)
		}
		return draftshell.RunLocalGit(ctx, dir, env, name, args...)
	})
	for _, args := range [][]string{{"status"}, {"open", "widget"}, {"status", "widget"}, {"round", "widget"}, {"abandon", "widget", "--reason", "This was only a test."}, {"status", "widget"}} {
		var out, diagnostic bytes.Buffer
		if exit := draftVerb(args, &out, &diagnostic); exit != 0 {
			t.Fatalf("%v: exit=%d out=%s err=%s", args, exit, &out, &diagnostic)
		}
		if args[0] != "status" && !strings.Contains(out.String(), "refs/pyxis/proposals/draft-widget-v1") {
			t.Fatalf("missing typed local identity: %s", &out)
		}
		if strings.Contains(out.String(), "https://") || !strings.Contains(diagnostic.String(), "unauthenticated") {
			t.Fatal("local proposal claims hosted identity")
		}
	}
	if calls == 0 || localGitCLI(t, origin, "rev-parse", "refs/heads/trunk") != base {
		t.Fatal("draft changed mainline")
	}
	for _, args := range [][]string{{"status", "missing"}, {"open", "Bad"}, {"reopen", "missing"}, {"round", "missing"}, {"abandon", "missing", "--reason", "test"}} {
		var out, diagnostic bytes.Buffer
		if exit := draftVerb(args, &out, &diagnostic); exit != 1 {
			t.Fatalf("%v unexpectedly accepted: %d %s", args, exit, &out)
		}
	}
}

func TestLocalDraftSelectionDefersHostBindingsAndRefusesInvalidConfiguration(t *testing.T) {
	for _, state := range []string{"outside repository", "invalid YAML", "hosted binding", "invalid binding"} {
		t.Run(state, func(t *testing.T) {
			root, origin := localGitCLIFixture(t, true)
			body := ""
			switch state {
			case "invalid YAML":
				body = "[invalid YAML"
			case "hosted binding":
				body = "integrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: github\n      repository: o/r\n"
			case "invalid binding":
				body = "integrations:\n  schema_version: unknown\n"
			}
			if state != "outside repository" {
				if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			swap(t, &draftRepoRoot, func() (string, error) {
				if state == "outside repository" {
					return "", errors.New("outside repository")
				}
				return root, nil
			})
			before := localGitCLI(t, origin, "show-ref")
			var out, diagnostic bytes.Buffer
			exit, handled := localDraftVerb([]string{"status"}, &out, &diagnostic)
			if state == "invalid binding" {
				if exit != 1 || !handled || diagnostic.Len() == 0 {
					t.Fatal("invalid local selection was not refused")
				}
			} else if handled {
				t.Fatalf("non-local selection handled: %d %s", exit, &diagnostic)
			}
			if localGitCLI(t, origin, "show-ref") != before {
				t.Fatal("selection changed origin refs")
			}
		})
	}
}

func TestLocalDraftStatusAndRoundRefuseUnreadableOriginRecords(t *testing.T) {
	localgittest.Skip(t)
	root, origin := localGitCLIFixture(t, true)
	base := localGitCLI(t, origin, "rev-parse", "refs/heads/trunk")
	localGitCLI(t, origin, "update-ref", "refs/pyxis/proposals/broken", base)
	swap(t, &draftRepoRoot, func() (string, error) { return root, nil })
	before := localGitCLI(t, origin, "show-ref")
	for _, args := range [][]string{{"status"}, {"round", "widget"}} {
		var out, diagnostic bytes.Buffer
		if exit := draftVerb(args, &out, &diagnostic); exit != 1 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "readable record") {
			t.Fatalf("unreadable proposal accepted: %d %s", exit, &diagnostic)
		}
	}
	if localGitCLI(t, origin, "show-ref") != before {
		t.Fatal("read refusal mutated origin")
	}
}

func TestLocalDraftStatusFiltersNamedFeatureAndRejectsUnsupportedVerb(t *testing.T) {
	localgittest.Skip(t)
	root, origin := localGitCLIFixture(t, true)
	localGitCLI(t, root, "tag", "lock/widget/v1")
	localGitCLI(t, root, "push", "upstream", "refs/tags/lock/widget/v1")
	swap(t, &draftRepoRoot, func() (string, error) { return root, nil })
	before := localGitCLI(t, origin, "show-ref")
	var out, diagnostic bytes.Buffer
	if exit := draftVerb([]string{"status", "another"}, &out, &diagnostic); exit != 1 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "no contract state") {
		t.Fatalf("unrelated feature presented: %d %s", exit, &out)
	}
	if exit, handled := localDraftVerb([]string{"unknown"}, &out, &diagnostic); exit != 1 || !handled || !strings.Contains(diagnostic.String(), "not implemented") {
		t.Fatal("unsupported local verb was admitted")
	}
	if localGitCLI(t, origin, "show-ref") != before {
		t.Fatal("status mutated origin")
	}
}

func TestLocalDraftStatusShowsAbandonmentAlongsideActiveFeatures(t *testing.T) {
	localgittest.Skip(t)
	root, origin := localGitCLIFixture(t, true)
	base := localGitCLI(t, origin, "rev-parse", "refs/heads/trunk")
	store := localcodehost.NewProposals(localcodehost.ProposalOptions{Git: localcodehost.Options{Root: root, Origin: origin, Remote: "upstream", Mainline: "trunk"}, Kind: "draft"})
	for _, feature := range []string{"active", "cancelled"} {
		g, err := store.Publish(t.Context(), codehost.ProposalChange{ID: "draft-" + feature + "-v1", Feature: feature, Branch: "refs/heads/draft/" + feature, Head: base, State: "open", ExpectedTargetOID: base})
		if err != nil {
			t.Fatal(err)
		}
		if feature == "cancelled" {
			if _, err := store.Publish(t.Context(), codehost.ProposalChange{ID: g.Proposal.ID, Feature: feature, Branch: g.Proposal.Branch, Head: base, State: "abandoned", AbandonmentReason: "A test-only idea was cancelled.", ExpectedTargetOID: base, ExpectedBranchOID: base, ExpectedProposalOID: g.OID}); err != nil {
				t.Fatal(err)
			}
		}
	}
	swap(t, &draftRepoRoot, func() (string, error) { return root, nil })
	before := localGitCLI(t, origin, "show-ref")
	for _, feature := range []string{"", "active", "cancelled"} {
		args := []string{"status"}
		if feature != "" {
			args = append(args, feature)
		}
		var out, diagnostic bytes.Buffer
		if code := draftVerb(args, &out, &diagnostic); code != 0 {
			t.Fatalf("%v: %d %s", args, code, &diagnostic)
		}
		if strings.Contains(out.String(), "active:") != (feature != "cancelled") || strings.Contains(out.String(), "cancelled: abandoned") != (feature != "active") {
			t.Fatalf("mixed states or feature filter lost: %v %s", args, &out)
		}
		if feature != "active" && !strings.Contains(out.String(), "A test-only idea was cancelled.") {
			t.Fatalf("retained abandonment reason hidden: %s", &out)
		}
	}
	if localGitCLI(t, origin, "show-ref") != before {
		t.Fatal("status changed origin refs")
	}
}

func TestLocalDraftRoundRecoversPublicationBeforeLocalReset(t *testing.T) {
	localgittest.Skip(t)
	root, origin := localGitCLIFixture(t, true)
	prd, err := os.ReadFile("../../internal/draft/compile/testdata/prd/working.md")
	if err != nil {
		t.Fatal(err)
	}
	tech := []byte("# Widget — tech spec\n\n### D-1 — Open widget\n\n**Decision:** Open and queue the widget.\n\n**Serves:** FR-1 · UB-1\n")
	for file, body := range map[string][]byte{"prd-template.md": prd, "draft-tech-spec-template.md": tech, "draft-problems-template.md": []byte("# Requirement problems\n"), "draft-events-template.md": []byte("# Draft events\n")} {
		path := filepath.Join(root, ".pyxis/templates", file)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0644); err != nil {
			t.Fatal(err)
		}
	}
	localGitCLI(t, root, "add", "--", ".pyxis/templates")
	localGitCLI(t, root, "commit", "-m", "draft templates")
	localGitCLI(t, root, "push", "upstream", "trunk")
	base := localGitCLI(t, origin, "rev-parse", "refs/heads/trunk")
	swap(t, &draftRepoRoot, func() (string, error) { return root, nil })
	var out, diagnostic bytes.Buffer
	if code := draftVerb([]string{"open", "widget"}, &out, &diagnostic); code != 0 {
		t.Fatalf("open: %d %s", code, &diagnostic)
	}
	store := localcodehost.NewProposals(localcodehost.ProposalOptions{Git: localcodehost.Options{Root: root, Origin: origin, Remote: "upstream", Mainline: "trunk"}, Kind: "draft"})
	opened, err := store.Read(t.Context(), "draft-widget-v1")
	if err != nil {
		t.Fatal(err)
	}
	prd, err = os.ReadFile(filepath.Join(root, "drafts/widget/prd.md"))
	if err != nil {
		t.Fatal(err)
	}
	tech, err = os.ReadFile(filepath.Join(root, "drafts/widget/tech-spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	digest := compile.ContractDigest(prd, tech)
	if err := os.RemoveAll(filepath.Join(root, "drafts/widget")); err != nil {
		t.Fatal(err)
	}
	localGitCLI(t, root, "add", "-u", "--", "drafts/widget")
	localGitCLI(t, root, "commit", "-m", "docs(draft): prepare lock widget", "-m", "Draft-Digest: "+digest)
	preparedHead := localGitCLI(t, root, "rev-parse", "HEAD")
	request, err := codehost.NewDraftReviewRequest(digest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Publish(t.Context(), codehost.ProposalChange{ID: opened.Proposal.ID, Feature: "widget", Branch: opened.Proposal.Branch, Head: preparedHead, State: "prepared", ReviewRequest: &request, ExpectedProposalOID: opened.OID, ExpectedBranchOID: opened.Proposal.Head, ExpectedTargetOID: base}); err != nil {
		t.Fatal(err)
	}
	swap(t, &localDraftCommand, func(ctx context.Context, root string, env []string, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) > 0 && args[0] == "reset" {
			return nil, errors.New("simulated exit before local reset")
		}
		return draftshell.RunLocalGit(ctx, root, env, name, args...)
	})
	out.Reset()
	diagnostic.Reset()
	if code := draftVerb([]string{"round", "widget"}, &out, &diagnostic); code != 1 || !strings.Contains(diagnostic.String(), "align restored working tree") {
		t.Fatalf("interrupted round: %d %s", code, &diagnostic)
	}
	published, err := store.Read(t.Context(), "draft-widget-v1")
	if err != nil || published.Proposal.State != "open" || published.Proposal.ReviewRequest != nil || localGitCLI(t, root, "rev-parse", "HEAD") != preparedHead {
		t.Fatalf("fixture missed publication/reset window: %+v %v", published, err)
	}
	before := localGitCLI(t, origin, "show-ref")
	swap(t, &localDraftCommand, func(ctx context.Context, root string, env []string, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) > 0 && args[0] == "show" {
			return nil, errors.New("unreadable local preparation metadata")
		}
		return draftshell.RunLocalGit(ctx, root, env, name, args...)
	})
	out.Reset()
	diagnostic.Reset()
	if code := draftVerb([]string{"round", "widget"}, &out, &diagnostic); code != 1 || !strings.Contains(diagnostic.String(), "unreadable local preparation metadata") {
		t.Fatalf("unreadable local metadata accepted: %d %s", code, &diagnostic)
	}
	if localGitCLI(t, root, "rev-parse", "HEAD") != preparedHead || localGitCLI(t, origin, "show-ref") != before {
		t.Fatal("metadata refusal changed local or origin state")
	}
	swap(t, &localDraftCommand, lifecycle.RunCommand(draftshell.RunLocalGit))
	unsaved := filepath.Join(root, "unsaved.txt")
	if err := os.WriteFile(unsaved, []byte("keep this work\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	diagnostic.Reset()
	if code := draftVerb([]string{"round", "widget"}, &out, &diagnostic); code != 1 || !strings.Contains(diagnostic.String(), "clean working tree") {
		t.Fatalf("dirty recovery accepted: %d %s", code, &diagnostic)
	}
	if body, err := os.ReadFile(unsaved); err != nil || string(body) != "keep this work\n" || localGitCLI(t, root, "rev-parse", "HEAD") != preparedHead {
		t.Fatal("recovery lost uncommitted work")
	}
	if err := os.Remove(unsaved); err != nil {
		t.Fatal(err)
	}
	// The dirty retry retains a failure event; successful recovery must then
	// preserve that pending history as well as the published round.
	before = localGitCLI(t, origin, "show-ref")
	for range 2 {
		out.Reset()
		diagnostic.Reset()
		if code := draftVerb([]string{"round", "widget"}, &out, &diagnostic); code != 0 {
			t.Fatalf("retry: %d %s", code, &diagnostic)
		}
		if localGitCLI(t, root, "rev-parse", "HEAD") != published.Proposal.Head || localGitCLI(t, origin, "show-ref") != before {
			t.Fatal("recovery changed the published round or failed to align the checkout")
		}
	}
	if _, err := os.Stat(filepath.Join(root, "drafts/widget/events.md")); err != nil {
		t.Fatal("recovery did not restore the working event log")
	}
	// A recovery observes its completed generation; it must not publish the
	// old checkout over another writer's subsequent round.
	localGitCLI(t, root, "reset", "--hard", preparedHead)
	competingHead := ""
	swap(t, &localDraftCommand, func(ctx context.Context, root string, env []string, name string, args ...string) ([]byte, error) {
		output, err := draftshell.RunLocalGit(ctx, root, env, name, args...)
		if err == nil && name == "git" && len(args) > 0 && args[0] == "reset" {
			competingHead = localGitCLI(t, root, "commit-tree", "HEAD^{tree}", "-p", published.Proposal.Head, "-m", "another writer's next round")
			if _, err := store.Publish(ctx, codehost.ProposalChange{ID: published.Proposal.ID, Feature: "widget", Branch: published.Proposal.Branch, Head: competingHead, State: "open", ExpectedProposalOID: published.OID, ExpectedBranchOID: published.Proposal.Head, ExpectedTargetOID: base}); err != nil {
				t.Fatal(err)
			}
		}
		return output, err
	})
	out.Reset()
	diagnostic.Reset()
	if code := draftVerb([]string{"round", "widget"}, &out, &diagnostic); code != 1 || !strings.Contains(diagnostic.String(), "proposal moved") || competingHead == "" {
		t.Fatalf("concurrent round was not detected: %d %s", code, &diagnostic)
	}
	if localGitCLI(t, origin, "rev-parse", published.Proposal.Branch) != competingHead {
		t.Fatal("recovery overwrote the competing round")
	}
}

func TestLocalDraftReopenRejectsHostedOrInvalidCarinaOperand(t *testing.T) {
	localgittest.Skip(t)
	for _, state := range []string{"hosted tasks", "missing operand", "invalid operand configuration"} {
		t.Run(state, func(t *testing.T) {
			root, origin := localGitCLIFixture(t, true)
			configPath := filepath.Join(root, ".pyxis/config.yaml")
			body, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if state == "hosted tasks" {
				body = bytes.Replace(body, []byte("tracker: carina"), []byte("tracker: gh"), 1)
				body = bytes.Replace(body, []byte("    tasks:\n      provider: carina\n"), []byte("    tasks:\n      provider: github\n      repository: o/r\n"), 1)
			} else {
				operand := filepath.Join(t.TempDir(), "tracker")
				if state == "invalid operand configuration" {
					localGitCLI(t, filepath.Dir(operand), "init", operand)
					if err := os.Mkdir(filepath.Join(operand, ".pyxis"), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(operand, ".pyxis/config.yaml"), []byte("[invalid YAML"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				body = bytes.Replace(body, []byte("    tasks:\n      provider: carina\n"), []byte("    tasks:\n      provider: carina\n      root: "+operand+"\n"), 1)
			}
			if err := os.WriteFile(configPath, body, 0644); err != nil {
				t.Fatal(err)
			}
			swap(t, &draftRepoRoot, func() (string, error) { return root, nil })
			before := localGitCLI(t, origin, "show-ref")
			var out, diagnostic bytes.Buffer
			if exit := draftVerb([]string{"reopen", "widget"}, &out, &diagnostic); exit != 1 || out.Len() != 0 || diagnostic.Len() == 0 {
				t.Fatalf("invalid operand admitted: %d %s", exit, &diagnostic)
			}
			if localGitCLI(t, origin, "show-ref") != before {
				t.Fatal("operand refusal mutated origin")
			}
		})
	}
}

func TestLocalDraftReopenUsesCarinaOperandConfiguredStore(t *testing.T) {
	localgittest.Skip(t)
	root, origin := localGitCLIFixture(t, true)
	configPath := filepath.Join(root, ".pyxis/config.yaml")
	configBody, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	configBody = bytes.Replace(configBody, []byte("    tasks:\n      provider: carina\n"), []byte("    tasks:\n      provider: carina\n      root: .\n"), 1)
	if err := os.WriteFile(configPath, configBody, 0644); err != nil {
		t.Fatal(err)
	}
	for file, body := range map[string]string{"prd-template.md": "# <Feature or capability name> — PRD\n", "draft-tech-spec-template.md": "# <Feature> — tech spec\n", "draft-problems-template.md": "# Requirement problems\n", "draft-events-template.md": "# Draft events\n"} {
		p := filepath.Join(root, ".pyxis/templates", file)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := os.ReadFile("../../internal/draft/lifecycle/testdata/reopen/001-plan.md")
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(root, "docs/plans/widget/001-plan.md")
	if err := os.MkdirAll(filepath.Dir(planPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, plan, 0644); err != nil {
		t.Fatal(err)
	}
	store := localfs.New(filepath.Join(root, ".workitems"), time.Now)
	for _, id := range []string{"T1", "T2"} {
		item, defects := store.Create(t.Context(), model.CreateSpec{Title: id, Body: "<!-- pyxis:plan-task widget/001/" + id + " -->"})
		if len(defects) > 0 {
			t.Fatal(defects)
		}
		if _, defects = store.Transition(t.Context(), item.ID, model.StateClosed, model.CloseCompleted, "operator"); len(defects) > 0 {
			t.Fatal(defects)
		}
	}
	localGitCLI(t, root, "add", "--", ".pyxis/config.yaml", ".pyxis/templates", "docs/plans/widget/001-plan.md", ".workitems")
	localGitCLI(t, root, "commit", "-m", "closed plan tasks")
	localGitCLI(t, root, "push", "upstream", "trunk")
	localGitCLI(t, root, "tag", "lock/widget/v1")
	localGitCLI(t, root, "push", "upstream", "refs/tags/lock/widget/v1")
	swap(t, &draftRepoRoot, func() (string, error) { return root, nil })
	seed := lifecycle.LockedContract{Version: 1, PRD: []byte("exact PRD"), TechSpec: []byte("exact design")}
	seed.Digest = compile.ContractDigest(seed.PRD, seed.TechSpec)
	canonicalOrigin, err := filepath.EvalSymlinks(origin)
	if err != nil {
		t.Fatal(err)
	}
	swap(t, &draftLocked, func(_ context.Context, _, remote, feature string, version int, _ lifecycle.RunCommand) (lifecycle.LockedContract, error) {
		if remote != canonicalOrigin || feature != "widget" || version != 1 {
			t.Fatal("wrong lock selection")
		}
		return seed, nil
	})
	var out, diagnostic bytes.Buffer
	if exit := draftVerb([]string{"reopen", "widget", "--cause", "new-need", "--cause-ref", "request:1"}, &out, &diagnostic); exit != 0 || !strings.Contains(out.String(), "draft-widget-v2") {
		t.Fatalf("reopen: exit=%d out=%s err=%s", exit, &out, &diagnostic)
	}
}
