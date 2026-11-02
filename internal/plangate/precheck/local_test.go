package precheck

import (
	"context"
	"errors"
	"strings"
	"testing"

	localgit "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit"
	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit/localgittest"
	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/draft/lock"
)

func localLockedFixture(t *testing.T) (string, string, codehost.DraftLandings, codehost.ReviewBinding, codehost.ReceiptRecord) {
	t.Helper()
	root := lockedFixture(t)
	seed, err := lock.LoadContract(t.Context(), root, "origin", "widget", 1, draftshell.Run)
	if err != nil {
		t.Fatal(err)
	}
	origin := git(t, root, "remote", "get-url", "origin")
	head, base := git(t, root, "rev-parse", "HEAD"), git(t, root, "rev-parse", "HEAD^")
	git(t, root, "tag", "-d", "lock/widget/v1")
	git(t, origin, "update-ref", "-d", "refs/tags/lock/widget/v1")
	git(t, origin, "update-ref", "refs/heads/main", base)
	options := localgit.Options{Root: root, Origin: origin, Remote: "origin", Mainline: "main"}
	proposals := localgit.NewProposals(localgit.ProposalOptions{Git: options, Kind: "draft"})
	first, err := proposals.Publish(t.Context(), codehost.ProposalChange{ID: "draft-widget-v1", Feature: "widget", Branch: "refs/heads/draft/widget", Head: head, State: "open", ExpectedTargetOID: base})
	if err != nil {
		t.Fatal(err)
	}
	request, err := codehost.NewDraftReviewRequest(seed.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := proposals.Publish(t.Context(), codehost.ProposalChange{ID: first.Proposal.ID, Feature: "widget", Branch: first.Proposal.Branch, Head: head, State: "prepared", ExpectedTargetOID: base, ExpectedBranchOID: head, ExpectedProposalOID: first.OID, ReviewRequest: &request}); err != nil {
		t.Fatal(err)
	}
	receipts := localgit.NewReceipts(options)
	binding, err := receipts.Binding(t.Context(), first.Proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := receipts.Record(t.Context(), binding, "operator", "")
	if err != nil {
		t.Fatal(err)
	}
	landings := localgit.NewDraftLandings(options)
	if _, err := landings.LandDraft(t.Context(), codehost.DraftLandingRequest{Binding: binding, Version: 1, ProposalsSinceOpened: 1}); err != nil {
		t.Fatal(err)
	}
	return root, origin, landings, binding, receipt
}

func TestPlanningRequiresCompleteRetainedLocalLock(t *testing.T) {
	localgittest.Skip(t)
	root, origin, landings, binding, receipt := localLockedFixture(t)
	base := binding.TargetOID
	basis, defects := RunWithLocalLock(t.Context(), root, "widget", origin, draftshell.RunLocalGit, landings)
	if basis != "lock/widget/v1" || len(defects) != 0 {
		t.Fatalf("verified local planning basis=%q defects=%v", basis, defects)
	}
	if git(t, root, "rev-parse", "refs/tags/"+basis) != git(t, origin, "rev-parse", "refs/tags/"+basis) || git(t, root, "rev-parse", "refs/tags/"+basis+"^{commit}") != git(t, origin, "rev-parse", "refs/heads/main") {
		t.Fatal("successful precheck left Basis absent or unrelated to the verified lock")
	}
	if basis, defects := Run(t.Context(), root, "widget", draftshell.Run); basis != "" || len(defects) == 0 {
		t.Fatal("hosted planning accepted an unauthenticated local lock")
	}
	git(t, root, "update-ref", "refs/tags/lock/widget/v1", base)
	if basis, defects := RunWithLocalLock(t.Context(), root, "widget", origin, draftshell.RunLocalGit, landings); basis != "" || len(defects) == 0 || !strings.Contains(defects[0].Reason, "conflicting local tag") {
		t.Fatalf("conflicting cached Basis was reported usable: %q %v", basis, defects)
	}
	if git(t, root, "rev-parse", "refs/tags/lock/widget/v1") != base {
		t.Fatal("local planning overwrote a conflicting cached tag")
	}
	git(t, root, "tag", "-d", "lock/widget/v1")
	git(t, origin, "update-ref", "-d", "refs/pyxis/receipts/"+binding.ProposalID+"/"+binding.Nonce)
	if basis, defects := RunWithLocalLock(t.Context(), root, "widget", origin, draftshell.RunLocalGit, landings); basis != "" || len(defects) == 0 || !strings.Contains(defects[0].Reason, "receipt") {
		t.Fatalf("missing retained receipt accepted: basis=%q defects=%v", basis, defects)
	}
	git(t, origin, "update-ref", "refs/pyxis/receipts/"+binding.ProposalID+"/"+binding.Nonce, receipt.OID)
	if basis, defects := RunWithLocalLock(t.Context(), root, "widget", origin, draftshell.RunLocalGit, landings); basis != "lock/widget/v1" || len(defects) != 0 {
		t.Fatalf("restored exact receipt did not recover planning: %q %v", basis, defects)
	}
	if _, defects := RunWithLocalLock(t.Context(), root, "widget", "", draftshell.RunLocalGit, landings); len(defects) == 0 {
		t.Fatal("missing verified origin accepted")
	}
	if _, defects := RunWithLocalLock(t.Context(), root, "widget", origin, draftshell.RunLocalGit, nil); len(defects) == 0 {
		t.Fatal("missing local verifier accepted")
	}
}

type finalPlanningLockReader struct {
	codehost.DraftLockReader
	calls int
	fault string
}

func (r *finalPlanningLockReader) VerifyDraftLock(ctx context.Context, feature string, version int) (codehost.LocalDraftLock, error) {
	verified, err := r.DraftLockReader.VerifyDraftLock(ctx, feature, version)
	r.calls++
	if r.calls == 3 {
		if r.fault == "final lock unavailable" {
			return verified, errors.New(r.fault)
		}
		verified.MergeCommit = strings.Repeat("9", len(verified.MergeCommit))
	}
	return verified, err
}

func TestLocalPlanningRefusesUnusableBasisBeforeSuccess(t *testing.T) {
	localgittest.Skip(t)
	for _, fault := range []string{"tag fetch unavailable", "local tag unreadable", "local tag changed", "final lock unavailable", "final lock changed"} {
		t.Run(fault, func(t *testing.T) {
			t.Parallel()
			root, origin, landings, _, _ := localLockedFixture(t)
			var reader codehost.DraftLockReader = landings
			if strings.HasPrefix(fault, "final lock") {
				reader = &finalPlanningLockReader{DraftLockReader: landings, fault: fault}
			}
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				joined := strings.Join(args, " ")
				if strings.Contains(joined, "refs/tags/lock/widget/v1:refs/tags/lock/widget/v1") && fault == "tag fetch unavailable" || strings.Contains(joined, "--verify refs/tags/lock/widget/v1^{commit}") && fault == "local tag unreadable" {
					return nil, errors.New(fault)
				}
				if strings.Contains(joined, "--verify refs/tags/lock/widget/v1^{commit}") && fault == "local tag changed" {
					return []byte(strings.Repeat("9", 40)), nil
				}
				return draftshell.RunLocalGit(ctx, dir, env, name, args...)
			}
			before := git(t, origin, "show-ref")
			if basis, defects := RunWithLocalLock(t.Context(), root, "widget", origin, run, reader); basis != "" || len(defects) != 1 || git(t, origin, "show-ref") != before {
				t.Fatalf("unusable Basis reported success or changed origin: %q %v", basis, defects)
			}
		})
	}
}

type movingLockReader struct {
	codehost.DraftLockReader
	advance func()
}

func (r *movingLockReader) VerifyDraftLock(ctx context.Context, feature string, version int) (codehost.LocalDraftLock, error) {
	verified, err := r.DraftLockReader.VerifyDraftLock(ctx, feature, version)
	if err == nil && r.advance != nil {
		advance := r.advance
		r.advance = nil
		advance()
	}
	return verified, err
}

func TestLocalPlanningRefusesMainlineAndVersionMovementDuringVerification(t *testing.T) {
	localgittest.Skip(t)
	for _, change := range []string{"mainline", "latest version"} {
		t.Run(change, func(t *testing.T) {
			root, origin, landings, _, _ := localLockedFixture(t)
			reader := &movingLockReader{DraftLockReader: landings, advance: func() {
				main := git(t, origin, "rev-parse", "refs/heads/main")
				if change == "latest version" {
					// A new published version must be selected and verified on
					// retry, even if its mainline happens to be unchanged.
					git(t, origin, "update-ref", "refs/tags/lock/widget/v2", main)
					return
				}
				tree := git(t, root, "rev-parse", main+"^{tree}")
				next := git(t, root, "commit-tree", tree, "-p", main, "-m", "ordinary mainline update")
				git(t, root, "push", "origin", next+":refs/heads/main")
			}}
			basis, defects := RunWithLocalLock(t.Context(), root, "widget", origin, draftshell.RunLocalGit, reader)
			if basis != "" || len(defects) != 1 || !strings.Contains(defects[0].Reason, "changed during readiness verification") {
				t.Fatalf("stale readiness accepted: basis=%q defects=%v", basis, defects)
			}
			basis, defects = RunWithLocalLock(t.Context(), root, "widget", origin, draftshell.RunLocalGit, landings)
			if change == "mainline" {
				if basis != "lock/widget/v1" || len(defects) != 0 {
					t.Fatalf("fresh ordinary mainline did not recover: %q %v", basis, defects)
				}
			} else if basis != "" || len(defects) == 0 {
				t.Fatal("retry skipped verification of the newly published version")
			}
		})
	}
}

func TestLocalPlanningRequiresFinalOriginRead(t *testing.T) {
	localgittest.Skip(t)
	for _, command := range []struct{ name, argument string }{{"fetch", "--no-recurse-submodules"}, {"ls-remote", "--refs"}, {"ls-remote", "--tags"}} {
		t.Run(command.name+"/"+command.argument, func(t *testing.T) {
			root, origin, landings, _, _ := localLockedFixture(t)
			calls := 0
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) > 1 && args[0] == command.name && strings.Contains(strings.Join(args, " "), command.argument) {
					calls++
					if calls == 2 {
						return nil, errors.New("final origin read unavailable")
					}
				}
				return draftshell.RunLocalGit(ctx, dir, env, name, args...)
			}
			basis, defects := RunWithLocalLock(t.Context(), root, "widget", origin, run, landings)
			if basis != "" || len(defects) != 1 || !strings.Contains(defects[0].Reason, "final origin read unavailable") {
				t.Fatalf("unavailable final origin accepted: basis=%q defects=%v", basis, defects)
			}
		})
	}
}

func TestLocalPlanningIgnoresConcurrentFetchHead(t *testing.T) {
	localgittest.Skip(t)
	for _, change := range []string{"unrelated retired source", "concealed mainline advance"} {
		t.Run(change, func(t *testing.T) {
			root, origin, landings, _, _ := localLockedFixture(t)
			main := git(t, origin, "rev-parse", "refs/heads/main")
			tree := git(t, root, "rev-parse", main+"^{tree}")
			unrelated := git(t, root, "commit-tree", tree, "-p", main, "-m", "unrelated branch")
			git(t, root, "checkout", "--detach", main)
			write(t, root, "docs/prds/other/effective.source.json", []byte(`{"units":{"FR-9":{"status":"retired"}}}`))
			git(t, root, "add", "--", "docs/prds/other/effective.source.json")
			retired := git(t, root, "commit-tree", git(t, root, "write-tree"), "-p", main, "-m", "retire cited source")
			var reader codehost.DraftLockReader = landings
			if change == "unrelated retired source" {
				unrelated = retired
			} else {
				git(t, root, "push", "origin", retired+":refs/heads/next-mainline")
				reader = &movingLockReader{DraftLockReader: landings, advance: func() {
					git(t, origin, "update-ref", "refs/heads/main", retired, main)
				}}
			}
			git(t, root, "push", "origin", unrelated+":refs/heads/unrelated")
			interference := 0
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				out, err := draftshell.RunLocalGit(ctx, dir, env, name, args...)
				if err == nil && name == "git" && len(args) >= 3 && args[0] == "fetch" && args[len(args)-2] == origin && !strings.Contains(args[len(args)-1], ":") {
					// Another process fetches a different descendant between
					// our fetch and the old FETCH_HEAD read, on both passes.
					git(t, root, "fetch", "--no-tags", origin, "refs/heads/unrelated")
					interference++
				}
				return out, err
			}
			basis, defects := RunWithLocalLock(t.Context(), root, "widget", origin, run, reader)
			if change == "unrelated retired source" {
				if basis != "lock/widget/v1" || len(defects) != 0 {
					t.Fatalf("another fetch changed planning readiness: %q %v", basis, defects)
				}
			} else if basis != "" || len(defects) != 1 || !strings.Contains(defects[0].Reason, "changed during readiness verification") {
				t.Fatalf("another fetch hid mainline movement: %q %v", basis, defects)
			}
			if interference != 2 || git(t, root, "rev-parse", "FETCH_HEAD") != unrelated {
				t.Fatalf("concurrent fetch was not exercised on both origin reads: %d", interference)
			}
		})
	}
}
