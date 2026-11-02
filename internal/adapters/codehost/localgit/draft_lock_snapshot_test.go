package localgit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func TestRetainedDraftLockRefusesEvidenceMovementDuringFinalRead(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ evidence, operation string }{
		{"tag", "delete"}, {"tag", "replace"},
		{"proposal", "delete"}, {"proposal", "replace"},
		{"archive", "delete"}, {"archive", "replace"},
		{"receipt", "delete"}, {"receipt", "replace"},
		{"receipt", "busy"},
		{"branch", "replace"},
		{"mainline", "after snapshot"},
	} {
		t.Run(tc.evidence+" "+tc.operation, func(t *testing.T) {
			t.Parallel()
			o, store, request := landingFixture(t, true)
			if _, err := store.LandDraft(t.Context(), request); err != nil {
				t.Fatal(err)
			}
			final, err := NewProposals(ProposalOptions{Git: o, Kind: "draft"}).Read(t.Context(), request.Binding.ProposalID)
			if err != nil {
				t.Fatal(err)
			}
			ref := map[string]string{
				"tag":      "refs/tags/lock/widget/v1",
				"proposal": proposalPrefix + request.Binding.ProposalID,
				"archive":  final.Proposal.ArchiveRef,
				"receipt":  receiptRef(request.Binding),
				"branch":   request.Binding.Branch,
				"mainline": request.Binding.TargetRef,
			}[tc.evidence]
			inspections, changedRefs := 0, ""
			store.options.Run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
				if root == o.Root && len(args) > 2 && args[2] == "check-ref-format" {
					inspections++
					if inspections == 3 && tc.operation != "after snapshot" {
						switch tc.operation {
						case "delete":
							fixtureGit(t, o.Origin, "update-ref", "-d", ref)
						case "busy":
							if err := os.WriteFile(filepath.Join(o.Origin, ref+".lock"), nil, 0o600); err != nil {
								t.Fatal(err)
							}
						default:
							fixtureGit(t, o.Origin, "update-ref", ref, request.Binding.Head)
						}
						changedRefs = fixtureGit(t, o.Origin, "show-ref")
					}
				}
				out, err := runGit(ctx, root, args...)
				if root == o.Origin && len(args) > 2 && args[2] == "for-each-ref" && inspections == 3 && tc.operation == "after snapshot" {
					// Return the old snapshot after another process resets mainline.
					fixtureGit(t, o.Origin, "update-ref", ref, request.Binding.TargetOID)
					changedRefs = fixtureGit(t, o.Origin, "show-ref")
				}
				return out, err
			}
			_, err = store.VerifyDraftLock(t.Context(), "widget", 1)
			if !errors.Is(err, codehost.ErrReviewInvalid) || changedRefs == "" {
				t.Fatalf("changed retained %s certified: %v", tc.evidence, err)
			}
			if fixtureGit(t, o.Origin, "show-ref") != changedRefs {
				t.Fatal("verification repaired changed evidence implicitly")
			}
			if tc.operation == "busy" {
				if err := os.Remove(filepath.Join(o.Origin, ref+".lock")); err != nil {
					t.Fatal(err)
				}
				store.options.Run = nil
				if _, err := store.VerifyDraftLock(t.Context(), "widget", 1); err != nil {
					t.Fatalf("failed snapshot left verification locks behind: %v", err)
				}
			}
		})
	}
}

func TestRetainedDraftLockRefusesMissingTagObject(t *testing.T) {
	t.Parallel()
	o, store, request := landingFixture(t, true)
	if _, err := store.LandDraft(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	ref := filepath.Join(o.Origin, "refs/tags/lock/widget/v1")
	// A damaged origin can retain the pointer after its object disappears.
	if err := os.WriteFile(ref, []byte(strings.Repeat("f", len(request.Binding.Head))+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := store.VerifyDraftLock(t.Context(), "widget", 1)
	if !errors.Is(err, codehost.ErrReviewInvalid) || !strings.Contains(err.Error(), "tag is unreadable") {
		t.Fatalf("missing retained tag object certified: %v", err)
	}
}
