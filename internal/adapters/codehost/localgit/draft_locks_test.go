package localgit

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func landingFixture(t *testing.T, approve bool) (Options, *DraftLandingStore, codehost.DraftLandingRequest) {
	t.Helper()
	o, proposals, change := proposalFixture(t)
	change.ID = "draft-widget-v1"
	first, err := proposals.Publish(t.Context(), change)
	if err != nil {
		t.Fatal(err)
	}
	request, err := codehost.NewDraftReviewRequest("sha256:" + strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	prepared := revision(first, first.Proposal.Head)
	prepared.State, prepared.ReviewRequest = "prepared", &request
	if _, err := proposals.Publish(t.Context(), prepared); err != nil {
		t.Fatal(err)
	}
	receipts := NewReceipts(o)
	binding, err := receipts.Binding(t.Context(), change.ID)
	if err != nil {
		t.Fatal(err)
	}
	if approve {
		if _, err := receipts.Record(t.Context(), binding, "declared operator", "reviewed exact request"); err != nil {
			t.Fatal(err)
		}
	}
	return o, NewDraftLandings(o), codehost.DraftLandingRequest{Binding: binding, Version: 1, ProposalsSinceOpened: 1}
}

func TestDraftLockAtomicLandingAndCloneRecovery(t *testing.T) {
	t.Parallel()
	o, landings, request := landingFixture(t, true)
	result, err := landings.LandDraft(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	lock := result.Lock
	if !result.Created || lock.Validate() != nil || lock.Review.Receipt.Binding != request.Binding || lock.Review.Trust != (codehost.Trust{SharedAccount: true}) {
		t.Fatalf("bad lock: %+v", result)
	}
	if fixtureGit(t, o.Origin, "rev-parse", "refs/heads/trunk") != lock.MergeCommit || fixtureGit(t, o.Origin, "rev-parse", lock.MergeCommit+"^") != request.Binding.TargetOID || fixtureGit(t, o.Origin, "rev-parse", lock.MergeCommit+"^{tree}") != fixtureGit(t, o.Origin, "rev-parse", request.Binding.Head+"^{tree}") {
		t.Fatal("landing differs from reviewed tree or base")
	}
	if branches := fixtureGit(t, o.Origin, "for-each-ref", "--format=%(refname)", request.Binding.Branch); branches != "" {
		t.Fatal("proposal branch survived landing")
	}
	clone := proposalClone(t, o)
	if err := os.RemoveAll(o.Root); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, o.Origin, "gc", "--prune=now")
	reader := NewDraftLandings(clone)
	verified, err := reader.VerifyDraftLock(t.Context(), "widget", 1)
	if err != nil || verified != lock {
		t.Fatalf("clone verification: %+v %v", verified, err)
	}
	retry, err := reader.LandDraft(t.Context(), request)
	if err != nil || retry.Created || retry.Lock != lock {
		t.Fatalf("retry created another lock: %+v %v", retry, err)
	}
	fixtureGit(t, clone.Root, "commit", "--allow-empty", "-m", "subsequent mainline work")
	fixtureGit(t, clone.Root, "push", clone.Remote, "HEAD:refs/heads/trunk")
	if _, err := reader.VerifyDraftLock(t.Context(), "widget", 1); err != nil {
		t.Fatal(err)
	}
	wrong := request
	wrong.Binding.Nonce = strings.Repeat("0", 32)
	if _, err := reader.LandDraft(t.Context(), wrong); err == nil {
		t.Fatal("terminal lock authorized another request")
	}
}

func TestDraftLockRefusalsLeaveEveryLandingRefUnchanged(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"missing receipt", "wrong request", "wrong version", "before push", "unsupported atomic", "branch move", "target move", "proposal move", "occupied tag", "occupied archive", "lost reply"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			o, store, request := landingFixture(t, failure != "missing receipt")
			before := fixtureGit(t, o.Origin, "show-ref")
			if failure == "wrong request" {
				request.Binding.Digest = "sha256:" + strings.Repeat("b", 64)
			}
			if failure == "wrong version" {
				request.Version = 2
			}
			if failure == "unsupported atomic" {
				fixtureGit(t, o.Origin, "config", "receive.advertiseAtomic", "false")
			}
			if failure != "missing receipt" && failure != "wrong request" && failure != "wrong version" && failure != "unsupported atomic" {
				store.push = func(ctx context.Context, root string, args ...string) error {
					switch failure {
					case "before push":
						return errors.New("interrupted before publication")
					case "branch move", "target move":
						ref, parent := request.Binding.Branch, request.Binding.Head
						if failure == "target move" {
							ref, parent = request.Binding.TargetRef, request.Binding.TargetOID
						}
						head := fixtureGit(t, root, "commit-tree", parent+"^{tree}", "-p", parent, "-m", "another writer")
						fixtureGit(t, o.Origin, "fetch", "--no-write-fetch-head", root, head)
						fixtureGit(t, o.Origin, "update-ref", ref, head)
					case "proposal move":
						p := NewProposals(ProposalOptions{Git: o, Kind: "draft"})
						g, err := p.Read(ctx, request.Binding.ProposalID)
						if err != nil {
							return err
						}
						nonce, err := codehost.NewDraftReviewRequest(request.Binding.Digest)
						if err != nil {
							return err
						}
						change := revision(g, g.Proposal.Head)
						change.State, change.ReviewRequest = "prepared", &nonce
						if _, err := p.Publish(ctx, change); err != nil {
							return err
						}
					case "occupied tag":
						fixtureGit(t, o.Origin, "update-ref", "refs/tags/lock/widget/v1", request.Binding.Head)
					case "occupied archive":
						fixtureGit(t, o.Origin, "update-ref", "refs/pyxis/proposal-archives/draft-widget-v1/3", request.Binding.ProposalOID)
					}
					_, err := proposalGit(ctx, root, nil, nil, args...)
					if err == nil && failure == "lost reply" {
						return errors.New("reply lost after publication")
					}
					return err
				}
			}
			result, err := store.LandDraft(t.Context(), request)
			if failure == "lost reply" {
				if err != nil || result.Lock.Version != 1 {
					t.Fatalf("completed transaction not recovered: %+v %v", result, err)
				}
				return
			}
			if err == nil {
				t.Fatal("invalid or interrupted landing succeeded")
			}
			if failure == "unsupported atomic" && !errors.Is(err, codehost.ErrAtomicUnavailable) {
				t.Fatalf("atomic support: %v", err)
			}
			if failure == "branch move" {
				fixtureGit(t, o.Origin, "update-ref", request.Binding.Branch, request.Binding.Head)
			}
			if failure == "target move" {
				fixtureGit(t, o.Origin, "update-ref", request.Binding.TargetRef, request.Binding.TargetOID)
			}
			if failure == "proposal move" {
				fixtureGit(t, o.Origin, "update-ref", proposalPrefix+request.Binding.ProposalID, request.Binding.ProposalOID)
			}
			if failure == "occupied tag" {
				fixtureGit(t, o.Origin, "update-ref", "-d", "refs/tags/lock/widget/v1")
			}
			if failure == "occupied archive" {
				fixtureGit(t, o.Origin, "update-ref", "-d", "refs/pyxis/proposal-archives/draft-widget-v1/3")
			}
			if fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("refusal partially published a landing")
			}
		})
	}
}

func TestDraftLockCloneDetectsMissingOrAlteredRetainedEvidence(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"receipt missing", "receipt changed", "archive", "tag", "lightweight tag", "branch resurrected", "mainline reset", "invalid declaration"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			o, store, request := landingFixture(t, true)
			result, err := store.LandDraft(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			switch failure {
			case "receipt missing":
				fixtureGit(t, o.Origin, "update-ref", "-d", receiptRef(request.Binding))
			case "receipt changed":
				fixtureGit(t, o.Origin, "update-ref", receiptRef(request.Binding), request.Binding.ProposalOID)
			case "archive":
				fixtureGit(t, o.Origin, "update-ref", "-d", "refs/pyxis/proposal-archives/draft-widget-v1/3")
			case "tag":
				fixtureGit(t, o.Origin, "update-ref", "-d", "refs/tags/lock/widget/v1")
			case "lightweight tag":
				fixtureGit(t, o.Origin, "update-ref", "refs/tags/lock/widget/v1", result.Lock.MergeCommit)
			case "branch resurrected":
				fixtureGit(t, o.Origin, "update-ref", request.Binding.Branch, request.Binding.Head)
			case "mainline reset":
				fixtureGit(t, o.Origin, "update-ref", request.Binding.TargetRef, request.Binding.TargetOID)
			case "invalid declaration":
				bad := result.Lock
				bad.Review.Trust.ApproverAuthenticated = true
				tag, err := prepareLockTag(t.Context(), o.Root, "refs/tags/lock/widget/v1", bad)
				if err != nil {
					t.Fatal(err)
				}
				fixtureGit(t, o.Origin, "fetch", "--no-write-fetch-head", o.Root, tag)
				fixtureGit(t, o.Origin, "update-ref", "refs/tags/lock/widget/v1", tag)
			}
			if _, err := NewDraftLandings(proposalClone(t, o)).VerifyDraftLock(t.Context(), "widget", 1); err == nil {
				t.Fatal("missing or altered evidence verified")
			}
		})
	}
}
