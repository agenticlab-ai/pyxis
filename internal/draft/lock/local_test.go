package lock

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	localgit "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit"
	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit/localgittest"
	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	memorycli "github.com/agenticlab-ai/pyxis/internal/memory/cli"
)

func localPreparationFixture(t *testing.T, reviews bool) (string, config.Config, localgit.Options, LocalPreparation) {
	t.Helper()
	root, cfg, _ := prepareFixture(t, reviews)
	// The production gate requires an initial trusted debt inventory. Build it
	// from the empty mainline through the public generator, before any draft
	// records exist; preparation may never invent its own allowance inventory.
	gitFixture(t, root, "switch", "main")
	var baseline, diagnostic bytes.Buffer
	env := memorycli.OSEnv()
	env.Dir, env.Stdout, env.Stderr = root, &baseline, &diagnostic
	env.Getenv = func(string) string { return "" }
	if code := memorycli.Run(t.Context(), []string{"init"}, env); code != 0 {
		t.Fatalf("memory init: %d %s", code, &diagnostic)
	}
	gitFixture(t, root, "add", "--", "memory/policy.json")
	gitFixture(t, root, "commit", "-m", "initialize memory with default policy")
	baseline.Reset()
	if code := memorycli.Run(t.Context(), []string{"reconcile", "--baseline-generate"}, env); code != 0 {
		t.Fatalf("initial inventory: %d %s", code, &diagnostic)
	}
	fixtureFile(t, root, ".github/memory-reconciliation-baseline.json", baseline.Bytes())
	gitFixture(t, root, "add", "--", ".github/memory-reconciliation-baseline.json")
	gitFixture(t, root, "commit", "-m", "initial trusted memory inventory")
	gitFixture(t, root, "push", "origin", "main")
	base := gitFixture(t, root, "rev-parse", "HEAD")
	baseline.Reset()
	if code := memorycli.Run(t.Context(), []string{"reconcile", "--baseline-sync", "--migrate-v2", "--baseline-ref", base}, env); code != 0 {
		t.Fatalf("data-only policy migration: %d %s", code, &diagnostic)
	}
	gitFixture(t, root, "add", "--", ".github/memory-reconciliation-baseline.json", ".github/memory-reconciliation-baseline")
	gitFixture(t, root, "commit", "-m", "migrate initial inventory without changing debt")
	gitFixture(t, root, "push", "origin", "main")
	gitFixture(t, root, "switch", "draft/widget")
	gitFixture(t, root, "rebase", "main")
	origin := gitFixture(t, root, "remote", "get-url", "origin")
	gitFixture(t, origin, "update-ref", "-d", "refs/heads/draft/widget")
	options := localgit.Options{Root: root, Origin: origin, Remote: "origin", Mainline: "main"}
	store := localgit.NewProposals(localgit.ProposalOptions{Git: options, Kind: "draft"})
	if _, err := store.Publish(t.Context(), codehost.ProposalChange{ID: "draft-widget-v1", Feature: "widget", Branch: "refs/heads/draft/widget", Head: gitFixture(t, root, "rev-parse", "HEAD"), State: "open", ExpectedTargetOID: gitFixture(t, origin, "rev-parse", "refs/heads/main")}); err != nil {
		t.Fatal(err)
	}
	cfg.Draft.AgentLogin, cfg.Draft.AgentTokenEnv = "", ""
	local := LocalPreparation{Proposals: store, Memory: func(context.Context, string, string) error { return nil }}
	local.Locked = func(ctx context.Context, root, _, feature string, version int, _ lifecycle.RunCommand) (lifecycle.LockedContract, error) {
		return LoadLocalContract(ctx, root, feature, version, localgit.NewDraftLandings(options))
	}
	return root, cfg, options, local
}

func TestLocalPreparationAndLockPreserveExactReviewedContract(t *testing.T) {
	localgittest.Skip(t)
	root, cfg, options, local := localPreparationFixture(t, true)
	prd, err := os.ReadFile(filepath.Join(root, "drafts/widget/prd.md"))
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareLocal(t.Context(), root, cfg, "widget", draftshell.RunLocalGit, local)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.PullRequest != 0 || prepared.Proposal == nil || !strings.Contains(prepared.Request, "## PRD §0") || !strings.Contains(prepared.Request, "git show ") || strings.Contains(prepared.Request, "https://github.com") {
		t.Fatalf("local packet incomplete: %+v", prepared)
	}
	receipts := localgit.NewReceipts(options)
	binding, err := receipts.Binding(t.Context(), "draft-widget-v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := receipts.Record(t.Context(), binding, "operator", ""); err != nil {
		t.Fatal(err)
	}
	landings := localgit.NewDraftLandings(options)
	locked, err := LockLocal(t.Context(), root, cfg, binding, draftshell.RunLocalGit, local, receipts, landings)
	if err != nil {
		t.Fatal(err)
	}
	contract, err := LoadLocalContract(t.Context(), root, "widget", 1, landings)
	if err != nil || string(contract.PRD) != string(prd) || contract.Digest != binding.Digest || !locked.Lock.Lineless {
		t.Fatalf("locked text changed: %+v %v", contract, err)
	}
	if retry, err := LockLocal(t.Context(), root, cfg, binding, draftshell.RunLocalGit, local, receipts, landings); err != nil || retry.Created || retry.Lock != locked.Lock {
		t.Fatalf("retry: %+v %v", retry, err)
	}
}

func TestLocalPreparationAndLockRunRealInstalledMemoryPredicates(t *testing.T) {
	localgittest.Skip(t)
	root, cfg, options, local := localPreparationFixture(t, true)
	local.Memory = func(ctx context.Context, root, base string) error {
		var output strings.Builder
		env := memorycli.OSEnv()
		env.Dir, env.Stdout, env.Stderr = root, &output, &output
		env.Getenv = func(string) string { return "" }
		if code := memorycli.RunGate(ctx, []string{"--base", base}, env); code != 0 {
			return errors.New(output.String())
		}
		return nil
	}
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
	if _, err := LockLocal(t.Context(), root, cfg, binding, draftshell.RunLocalGit, local, receipts, localgit.NewDraftLandings(options)); err != nil {
		t.Fatal(err)
	}
}

func TestLocalPrepareResumesPublishedGenerationWithoutReissuingRequest(t *testing.T) {
	localgittest.Skip(t)
	root, cfg, options, local := localPreparationFixture(t, true)
	memoryCalls := 0
	local.Memory = func(context.Context, string, string) error { memoryCalls++; return nil }
	interrupted := func(ctx context.Context, root string, env []string, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) > 0 && args[0] == "reset" {
			return nil, errors.New("exit before aligning the published preparation")
		}
		return draftshell.RunLocalGit(ctx, root, env, name, args...)
	}
	first, err := PrepareLocal(t.Context(), root, cfg, "widget", interrupted, local)
	if err == nil || first.Proposal == nil || first.Request == "" || memoryCalls != 1 {
		t.Fatalf("fixture missed publication before reset: %+v %v", first, err)
	}
	receipts := localgit.NewReceipts(options)
	binding, err := receipts.Binding(t.Context(), "draft-widget-v1")
	if err != nil {
		t.Fatal(err)
	}
	before := gitFixture(t, options.Origin, "show-ref")
	for range 2 {
		resumed, err := PrepareLocal(t.Context(), root, cfg, "widget", draftshell.RunLocalGit, local)
		if err != nil || resumed.Request != first.Request || resumed.Head != first.Head || resumed.Proposal == nil || resumed.Proposal.OID != first.Proposal.OID || memoryCalls != 1 {
			t.Fatalf("retry changed the prepared packet or reran preparation: %+v %v", resumed, err)
		}
		if gitFixture(t, root, "rev-parse", "HEAD") != first.Head || gitFixture(t, options.Origin, "show-ref") != before {
			t.Fatal("retry changed retained origin state or failed to align the checkout")
		}
	}
	clone := filepath.Join(t.TempDir(), "clone")
	gitFixture(t, filepath.Dir(clone), "clone", "--branch", "draft/widget", options.Origin, clone)
	options.Root = clone
	local.Proposals = localgit.NewProposals(localgit.ProposalOptions{Git: options, Kind: "draft"})
	resumed, err := PrepareLocal(t.Context(), clone, cfg, "widget", draftshell.RunLocalGit, local)
	if err != nil || resumed.Request != first.Request || resumed.Proposal == nil || resumed.Proposal.OID != first.Proposal.OID || memoryCalls != 1 {
		t.Fatalf("new checkout cannot render the retained complete packet: %+v %v", resumed, err)
	}
	seen, err := localgit.NewReceipts(options).Binding(t.Context(), "draft-widget-v1")
	if err != nil || seen != binding || gitFixture(t, options.Origin, "show-ref") != before {
		t.Fatal("packet resumption replaced its generation, nonce, or origin state")
	}
}

func TestLocalPreparedPacketRefusesChangedBindingsAndPreservesCheckout(t *testing.T) {
	localgittest.Skip(t)
	for _, failure := range []string{"different checkout", "advanced target", "dirty during recovery", "head read interrupted", "reset interrupted", "generation moved"} {
		t.Run(failure, func(t *testing.T) {
			root, cfg, options, local := localPreparationFixture(t, true)
			first, err := PrepareLocal(t.Context(), root, cfg, "widget", draftshell.RunLocalGit, local)
			if err != nil {
				t.Fatal(err)
			}
			g, p := *first.Proposal, first.Proposal.Proposal
			if failure == "different checkout" {
				gitFixture(t, root, "commit", "--allow-empty", "-m", "keep my new work")
			}
			if failure == "advanced target" {
				next := gitFixture(t, root, "commit-tree", p.TargetOID+"^{tree}", "-p", p.TargetOID, "-m", "another mainline writer")
				gitFixture(t, root, "push", "origin", next+":"+p.TargetRef)
			}
			if failure == "dirty during recovery" || failure == "head read interrupted" || failure == "reset interrupted" {
				gitFixture(t, root, "reset", "--hard", p.Head+"^")
			}
			before, head := localLandingRefs(t, options.Origin), gitFixture(t, root, "rev-parse", "HEAD")
			cause := errors.New("preparation recovery interrupted")
			reads, moved, memoryCalls := 0, false, 0
			local.Memory = func(context.Context, string, string) error { memoryCalls++; return nil }
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if strings.Join(args, " ") == "rev-parse HEAD" {
					reads++
					if failure == "head read interrupted" && reads == 2 {
						return nil, cause
					}
				}
				if failure == "reset interrupted" && len(args) > 0 && args[0] == "reset" {
					return nil, cause
				}
				output, err := draftshell.RunLocalGit(ctx, dir, env, name, args...)
				if err == nil && len(args) > 1 && args[0] == "rev-list" && args[1] == "--parents" {
					if failure == "dirty during recovery" {
						fixtureFile(t, root, "keep-this.txt", []byte("uncommitted work\n"))
					}
					if failure == "generation moved" && !moved {
						request, requestErr := codehost.NewDraftReviewRequest(p.ReviewRequest.Digest)
						if requestErr != nil {
							t.Fatal(requestErr)
						}
						if _, publishErr := local.Proposals.Publish(ctx, codehost.ProposalChange{ID: p.ID, Feature: p.Feature, Branch: p.Branch, Head: p.Head, State: "prepared", ExpectedProposalOID: g.OID, ExpectedBranchOID: p.Head, ExpectedTargetOID: p.TargetOID, ReviewRequest: &request}); publishErr != nil {
							t.Fatal(publishErr)
						}
						moved = true
						before = localLandingRefs(t, options.Origin)
					}
				}
				return output, err
			}
			if _, err := PrepareLocal(t.Context(), root, cfg, "widget", run, local); err == nil || (failure == "head read interrupted" || failure == "reset interrupted") && !errors.Is(err, cause) {
				t.Fatalf("unsafe packet recovery accepted or lost its cause: %v", err)
			}
			if localLandingRefs(t, options.Origin) != before || gitFixture(t, root, "rev-parse", "HEAD") != head || memoryCalls != 0 {
				t.Fatal("refused packet recovery changed a landing ref, overwrote work, or reran preparation")
			}
			if failure == "dirty during recovery" {
				body, err := os.ReadFile(filepath.Join(root, "keep-this.txt"))
				if err != nil || string(body) != "uncommitted work\n" {
					t.Fatal("packet recovery lost uncommitted work")
				}
			}
			if failure == "generation moved" && !moved {
				t.Fatal("fixture did not move the prepared generation during rendering")
			}
		})
	}
}

func TestLocalReadinessAndMemoryRefusalsPreserveProposal(t *testing.T) {
	localgittest.Skip(t)
	for _, failure := range []string{"missing reviews", "preparation memory", "lock memory", "missing receipt", "stale target"} {
		t.Run(failure, func(t *testing.T) {
			root, cfg, options, local := localPreparationFixture(t, failure != "missing reviews")
			before := localLandingRefs(t, options.Origin)
			if failure == "preparation memory" {
				local.Memory = func(context.Context, string, string) error { return errors.New("integrity failed") }
			}
			prepared, err := PrepareLocal(t.Context(), root, cfg, "widget", draftshell.RunLocalGit, local)
			if failure == "missing reviews" || failure == "preparation memory" {
				if err == nil {
					t.Fatal("unready preparation published")
				}
				if localLandingRefs(t, options.Origin) != before {
					t.Fatal("refused preparation changed proposal refs")
				}
				return
			}
			if err != nil || prepared.Proposal == nil {
				t.Fatalf("preparation: %v", err)
			}
			receipts := localgit.NewReceipts(options)
			binding, err := receipts.Binding(t.Context(), "draft-widget-v1")
			if err != nil {
				t.Fatal(err)
			}
			if failure != "missing receipt" {
				if _, err := receipts.Record(t.Context(), binding, "operator", ""); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "stale target" {
				next := gitFixture(t, root, "commit-tree", binding.TargetOID+"^{tree}", "-p", binding.TargetOID, "-m", "ordinary mainline advance after review")
				gitFixture(t, root, "push", options.Origin, next+":"+binding.TargetRef)
			}
			before = localLandingRefs(t, options.Origin)
			if failure == "lock memory" {
				local.Memory = func(context.Context, string, string) error { return errors.New("integrity changed") }
			}
			if _, err := LockLocal(t.Context(), root, cfg, binding, draftshell.RunLocalGit, local, receipts, localgit.NewDraftLandings(options)); err == nil {
				t.Fatal("refused lock published")
			}
			if localLandingRefs(t, options.Origin) != before {
				t.Fatal("refused lock partially published")
			}
			queue, err := events.OpenPending(root, options.Origin, "widget", 1)
			if err != nil {
				t.Fatal(err)
			}
			pending, err := queue.Read(t.Context())
			if err != nil || len(pending) != 1 || pending[0].Name != events.DraftStepFailed || pending[0].Step != "lock" || pending[0].VersionDigest != binding.Digest {
				t.Fatalf("lock failure is not retained: %+v %v", pending, err)
			}
		})
	}
}

func TestLocalPreparedPacketRequiresReadableSourceAndRetainedBindings(t *testing.T) {
	localgittest.Skip(t)
	root, cfg, options, local := localPreparationFixture(t, true)
	first, err := PrepareLocal(t.Context(), root, cfg, "widget", draftshell.RunLocalGit, local)
	if err != nil {
		t.Fatal(err)
	}
	before, head := localLandingRefs(t, options.Origin), gitFixture(t, root, "rev-parse", "HEAD")
	for _, failure := range []string{"missing request", "invalid request", "fetch interrupted", "parents interrupted", "ancestry interrupted", "digest changed", "lock version changed"} {
		t.Run(failure, func(t *testing.T) {
			p, version := first.Proposal.Proposal, 1
			request := *p.ReviewRequest
			p.ReviewRequest = &request
			switch failure {
			case "missing request":
				p.ReviewRequest = nil
			case "invalid request":
				p.ReviewRequest.Nonce = ""
			case "digest changed":
				p.ReviewRequest.Digest = "sha256:" + strings.Repeat("a", 64)
			case "lock version changed":
				version = 2
			}
			cause := errors.New("prepared source read interrupted")
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if len(args) > 0 && (failure == "fetch interrupted" && args[0] == "fetch" || failure == "parents interrupted" && args[0] == "rev-list" || failure == "ancestry interrupted" && args[0] == "merge-base") {
					return nil, cause
				}
				return draftshell.RunLocalGit(ctx, dir, env, name, args...)
			}
			if _, err := inspectPreparedLocal(t.Context(), root, cfg, p, version, run, local); err == nil || strings.HasSuffix(failure, "interrupted") && !errors.Is(err, cause) {
				t.Fatalf("incomplete prepared source accepted or lost its cause: %v", err)
			}
			if localLandingRefs(t, options.Origin) != before || gitFixture(t, root, "rev-parse", "HEAD") != head {
				t.Fatal("refused packet inspection changed the retained proposal or checkout")
			}
		})
	}
}

func TestLocalPreparedPacketRefusesInterruptedSourceAndFinalOriginReads(t *testing.T) {
	localgittest.Skip(t)
	root, cfg, options, local := localPreparationFixture(t, true)
	prepared, err := PrepareLocal(t.Context(), root, cfg, "widget", draftshell.RunLocalGit, local)
	if err != nil {
		t.Fatal(err)
	}
	before, head := localLandingRefs(t, options.Origin), gitFixture(t, root, "rev-parse", "HEAD")
	for _, failure := range []string{"source", "read", "mainline"} {
		t.Run(failure, func(t *testing.T) {
			ports, run := local, lifecycle.RunCommand(draftshell.RunLocalGit)
			if failure == "source" {
				run = refuseLocalCommand([]string{"fetch"}, "")
			} else {
				ports.Proposals = faultyLocalProposals{DraftProposals: local.Proposals, failure: failure}
			}
			if _, err := resumePreparedLocal(t.Context(), root, cfg, *prepared.Proposal, 1, head, prepared.Branch, run, ports); err == nil {
				t.Fatal("packet rendered despite an unreadable retained source or origin")
			}
			if localLandingRefs(t, options.Origin) != before || gitFixture(t, root, "rev-parse", "HEAD") != head {
				t.Fatal("failed final packet read changed origin state or checkout")
			}
		})
	}
}

func localLandingRefs(t *testing.T, origin string) string {
	t.Helper()
	return gitFixture(t, origin, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/", "refs/tags/lock/", "refs/pyxis/proposals/", "refs/pyxis/proposal-archives/")
}
