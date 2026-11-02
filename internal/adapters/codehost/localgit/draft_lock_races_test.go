package localgit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func TestLocalDraftLockRechecksEvidenceAfterPreparingObjects(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		at   int
	}{
		{"unreadable mainline", 1},
		{"unreadable generation", 2},
		{"unreadable publication snapshot", 7},
		{"target moved before publication", 7},
		{"receipt deleted before publication", 8},
		{"receipt replaced before publication", 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, store, request := landingFixture(t, true)
			before := fixtureGit(t, o.Origin, "show-ref")
			receiptRef := receiptRef(request.Binding)
			originalReceipt := fixtureGit(t, o.Origin, "rev-parse", receiptRef)
			altered := fixtureGit(t, o.Root, "commit-tree", request.Binding.TargetOID+"^{tree}", "-p", request.Binding.TargetOID, "-m", "competing mainline work")
			fixtureGit(t, o.Origin, "fetch", "--no-write-fetch-head", o.Root, altered)
			if tc.name == "receipt replaced before publication" {
				receipt, err := NewReceipts(o).Verify(t.Context(), request.Binding)
				if err != nil {
					t.Fatal(err)
				}
				receipt.Receipt.Reviewer = "another declared reviewer"
				body, err := json.Marshal(receipt.Receipt)
				if err != nil {
					t.Fatal(err)
				}
				altered, err = prepareReceipt(t.Context(), o.Root, request.Binding.ProposalOID, append(body, '\n'))
				if err != nil {
					t.Fatal(err)
				}
				fixtureGit(t, o.Origin, "fetch", "--no-write-fetch-head", o.Root, altered)
			}
			inspections := 0
			store.options.Run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
				if root == o.Root && len(args) > 2 && args[2] == "check-ref-format" {
					inspections++
					if inspections == tc.at {
						switch tc.name {
						case "target moved before publication":
							fixtureGit(t, o.Origin, "update-ref", request.Binding.TargetRef, altered)
						case "receipt deleted before publication":
							fixtureGit(t, o.Origin, "update-ref", "-d", receiptRef)
						case "receipt replaced before publication":
							fixtureGit(t, o.Origin, "update-ref", receiptRef, altered)
						default:
							return nil, errors.New("simulated unreadable local origin")
						}
					}
				}
				return runGit(ctx, root, args...)
			}
			result, err := store.LandDraft(t.Context(), request)
			if err == nil || result.Created || inspections < tc.at {
				t.Fatalf("changed or unreadable proof accepted: %+v %v (%d inspections)", result, err, inspections)
			}
			fixtureGit(t, o.Origin, "update-ref", request.Binding.TargetRef, request.Binding.TargetOID)
			fixtureGit(t, o.Origin, "update-ref", receiptRef, originalReceipt)
			if fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("refusal partially published a lock")
			}
		})
	}
}

func TestLocalDraftLockHoldsReceiptAcrossAtomicPublication(t *testing.T) {
	for _, operation := range []string{"delete", "replace"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			o, store, request := landingFixture(t, true)
			ref := receiptRef(request.Binding)
			receiptOID := fixtureGit(t, o.Origin, "rev-parse", ref)
			before := fixtureGit(t, o.Origin, "show-ref")
			called := false
			store.push = func(ctx context.Context, root string, args ...string) error {
				called = true
				update := []string{"update-ref", "-d", ref, receiptOID}
				if operation == "replace" {
					update = []string{"update-ref", ref, request.Binding.TargetOID, receiptOID}
				}
				if _, err := proposalGit(ctx, o.Origin, nil, nil, update...); err == nil || !strings.Contains(err.Error(), "cannot lock ref") {
					t.Fatalf("receipt %s was not blocked during publication: %v", operation, err)
				}
				if fixtureGit(t, o.Origin, "show-ref") != before {
					t.Fatal("receipt mutation changed origin before publication")
				}
				_, err := proposalGit(ctx, root, nil, nil, args...)
				return err
			}
			result, err := store.LandDraft(t.Context(), request)
			if err != nil || !result.Created || !called {
				t.Fatalf("guarded landing failed: %+v %v (push called: %t)", result, err, called)
			}
			if fixtureGit(t, o.Origin, "rev-parse", ref) != receiptOID {
				t.Fatal("landing changed the reviewed receipt")
			}
			verified, err := NewDraftLandings(proposalClone(t, o)).VerifyDraftLock(t.Context(), "widget", 1)
			if err != nil || verified != result.Lock {
				t.Fatalf("fresh clone could not verify the guarded landing: %+v %v", verified, err)
			}
		})
	}
}

func TestLocalRetainedLockRejectsFalseGitProof(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"wrong landing parent", "wrong landing tree", "different receipt"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			o, store, request := landingFixture(t, true)
			result, err := store.LandDraft(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			if failure == "different receipt" {
				receipt := result.Lock.Review.Receipt
				receipt.Reviewer = "another declared reviewer"
				body, err := json.Marshal(receipt)
				if err != nil {
					t.Fatal(err)
				}
				oid, err := prepareReceipt(t.Context(), o.Root, request.Binding.ProposalOID, append(body, '\n'))
				if err != nil {
					t.Fatal(err)
				}
				fixtureGit(t, o.Origin, "fetch", "--no-write-fetch-head", o.Root, oid)
				fixtureGit(t, o.Origin, "update-ref", receiptRef(request.Binding), oid)
			} else {
				parent, tree := request.Binding.TargetOID, request.Binding.Head+"^{tree}"
				if failure == "wrong landing parent" {
					parent = request.Binding.Head
				} else {
					blob, err := proposalGit(t.Context(), o.Root, []byte("unreviewed content\n"), nil, "hash-object", "-w", "--stdin")
					if err != nil {
						t.Fatal(err)
					}
					wrongTree, err := proposalGit(t.Context(), o.Root, []byte("100644 blob "+strings.TrimSpace(string(blob))+"\tunreviewed.txt\n"), nil, "mktree")
					if err != nil {
						t.Fatal(err)
					}
					tree = strings.TrimSpace(string(wrongTree))
				}
				wrong := fixtureGit(t, o.Root, "commit-tree", tree, "-p", parent, "-m", "false declared landing")
				g, err := NewProposals(ProposalOptions{Git: o, Kind: "draft"}).Read(t.Context(), request.Binding.ProposalID)
				if err != nil {
					t.Fatal(err)
				}
				lock := result.Lock
				lock.MergeCommit = wrong
				g.Proposal.DraftLock = &lock
				oid, err := prepareProposal(t.Context(), o.Root, g.Proposal)
				if err != nil {
					t.Fatal(err)
				}
				tagRef := "refs/tags/lock/widget/v1"
				tag, err := prepareLockTag(t.Context(), o.Root, tagRef, lock)
				if err != nil {
					t.Fatal(err)
				}
				fixtureGit(t, o.Origin, "fetch", "--no-write-fetch-head", o.Root, oid, tag)
				for ref, head := range map[string]string{request.Binding.TargetRef: wrong, proposalPrefix + request.Binding.ProposalID: oid, g.Proposal.ArchiveRef: oid, tagRef: tag} {
					fixtureGit(t, o.Origin, "update-ref", ref, head)
				}
			}
			before := fixtureGit(t, o.Origin, "show-ref")
			if _, err := NewDraftLandings(proposalClone(t, o)).VerifyDraftLock(t.Context(), "widget", 1); err == nil || !strings.Contains(err.Error(), "review") {
				t.Fatalf("false retained proof verified: %v", err)
			}
			if fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("verification repaired false proof implicitly")
			}
		})
	}
}

func TestRetainedDraftLockRechecksCurrentMainline(t *testing.T) {
	t.Parallel()
	for _, movement := range []string{"mainline reset", "descendant mainline", "unreadable final mainline"} {
		t.Run(movement, func(t *testing.T) {
			o, store, request := landingFixture(t, true)
			landed, err := store.LandDraft(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			next := request.Binding.TargetOID
			if movement == "descendant mainline" {
				next = fixtureGit(t, o.Root, "commit-tree", landed.Lock.MergeCommit+"^{tree}", "-p", landed.Lock.MergeCommit, "-m", "ordinary mainline advance")
				fixtureGit(t, o.Origin, "fetch", "--no-write-fetch-head", o.Root, next)
			}
			inspections := 0
			store.options.Run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
				if root == o.Root && len(args) > 2 && args[2] == "check-ref-format" {
					inspections++
					if inspections == 2 && movement != "unreadable final mainline" {
						// The initial snapshot already contains the landing. Move
						// origin before its retained proposal is read and certified.
						fixtureGit(t, o.Origin, "update-ref", request.Binding.TargetRef, next)
					}
					if inspections == 3 && movement == "unreadable final mainline" {
						return nil, errors.New("final mainline unavailable")
					}
				}
				return runGit(ctx, root, args...)
			}
			verified, err := store.VerifyDraftLock(t.Context(), "widget", 1)
			switch movement {
			case "mainline reset":
				if !errors.Is(err, codehost.ErrReviewInvalid) || !strings.Contains(err.Error(), "absent from current mainline") {
					t.Fatalf("lock certified after its landing left mainline: %+v %v", verified, err)
				}
			case "unreadable final mainline":
				if err == nil || !strings.Contains(err.Error(), "final mainline unavailable") {
					t.Fatalf("unreadable final snapshot certified: %+v %v", verified, err)
				}
			case "descendant mainline":
				if err != nil || verified != landed.Lock {
					t.Fatalf("valid descendant refused: %+v %v", verified, err)
				}
			}
			if movement != "unreadable final mainline" && fixtureGit(t, o.Origin, "rev-parse", request.Binding.TargetRef) != next {
				t.Fatal("verification changed the mainline observation")
			}
		})
	}
}

func TestRetainedDraftLockRejectsOriginChange(t *testing.T) {
	t.Parallel()
	o, store, request := landingFixture(t, true)
	landed, err := store.LandDraft(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	copied := filepath.Join(dir, "copied.git")
	fixtureGit(t, o.Root, "clone", "--mirror", o.Origin, copied)
	alias := filepath.Join(dir, "origin.git")
	if err := os.Symlink(o.Origin, alias); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, o.Root, "remote", "set-url", o.Remote, alias)
	store.options.Origin = alias
	before := fixtureGit(t, o.Origin, "show-ref")
	inspections := 0
	store.options.Run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
		if root == o.Root && len(args) > 2 && args[2] == "check-ref-format" {
			inspections++
			if inspections == 3 {
				if err := os.Remove(alias); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(copied, alias); err != nil {
					t.Fatal(err)
				}
			}
		}
		return runGit(ctx, root, args...)
	}
	if _, err := store.VerifyDraftLock(t.Context(), "widget", 1); !errors.Is(err, codehost.ErrReviewInvalid) || !strings.Contains(err.Error(), "configured origin changed") {
		t.Fatalf("changed origin certified: %v", err)
	}
	for _, origin := range []string{o.Origin, copied} {
		if fixtureGit(t, origin, "show-ref") != before {
			t.Fatal("verification changed retained origin refs")
		}
	}
	// A stable copy remains consistent under the shared-account trust model.
	store.options.Run = nil
	verified, err := store.VerifyDraftLock(t.Context(), "widget", 1)
	if err != nil || verified != landed.Lock {
		t.Fatalf("stable origin copy refused: %+v %v", verified, err)
	}
}

func TestDraftLockIgnoresReplacementObjects(t *testing.T) {
	// The adapter must retain this protection after removing ambient Git vars.
	t.Setenv("GIT_NO_REPLACE_OBJECTS", "1")
	for _, location := range []string{"working repository", "bare origin"} {
		t.Run(location, func(t *testing.T) {
			o, store, request := landingFixture(t, true)
			reviewedTree := fixtureGit(t, o.Root, "--no-replace-objects", "rev-parse", request.Binding.Head+"^{tree}")
			blob, err := proposalGit(t.Context(), o.Root, []byte("unreviewed replacement\n"), nil, "hash-object", "-w", "--stdin")
			if err != nil {
				t.Fatal(err)
			}
			tree, err := proposalGit(t.Context(), o.Root, []byte("100644 blob "+strings.TrimSpace(string(blob))+"\tunreviewed.txt\n"), nil, "mktree")
			if err != nil {
				t.Fatal(err)
			}
			replacement := fixtureGit(t, o.Root, "commit-tree", strings.TrimSpace(string(tree)), "-p", request.Binding.TargetOID, "-m", "unreviewed replacement")
			replaced := o.Root
			if location == "bare origin" {
				fixtureGit(t, o.Origin, "fetch", "--no-write-fetch-head", o.Root, replacement)
				replaced = o.Origin
			}
			fixtureGit(t, replaced, "replace", request.Binding.Head, replacement)
			result, err := store.LandDraft(t.Context(), request)
			actualTree := fixtureGit(t, o.Origin, "--no-replace-objects", "rev-parse", request.Binding.TargetRef+"^{tree}")
			if err != nil || !result.Created || actualTree != reviewedTree {
				t.Fatalf("replacement changed reviewed landing: %v (raw tree %s, reviewed %s)", err, actualTree, reviewedTree)
			}
			if fixtureGit(t, replaced, "--no-replace-objects", "rev-parse", "refs/replace/"+request.Binding.Head) != replacement {
				t.Fatal("landing modified the unrelated replacement ref")
			}
			verified, err := NewDraftLandings(proposalClone(t, o)).VerifyDraftLock(t.Context(), "widget", 1)
			if err != nil || verified != result.Lock {
				t.Fatalf("raw reviewed landing did not verify from a clone: %v", err)
			}
		})
	}
}
