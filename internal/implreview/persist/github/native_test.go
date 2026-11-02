package github

import (
	"context"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
)

type nativeFake struct{ *hostFakeClient }

func (c nativeFake) PostComment(ctx context.Context, n int, s string) (string, error) {
	return c.postComment(ctx, n, s)
}
func (c nativeFake) ReadComment(ctx context.Context, s string) (string, error) {
	return c.readComment(ctx, s)
}
func (c nativeFake) CurrentLogin(ctx context.Context) (string, error) { return c.currentLogin(ctx) }
func (c nativeFake) RepositoryIdentity(ctx context.Context) (string, error) {
	return c.repositoryNodeID(ctx)
}
func (c nativeFake) CanWriteComments(ctx context.Context) (bool, error) {
	return c.canWriteProposalComments(ctx)
}

func TestNativePublicationUsesSameDurableFenceAndQualifiedReceipts(t *testing.T) {
	for _, ambiguous := range []bool{false, true} {
		t.Run(strings.TrimSpace(map[bool]string{true: "ambiguous acknowledgement", false: "verified"}[ambiguous]), func(t *testing.T) {
			repo := "forgejo:https://forgejo.test:o/r"
			client := &hostFakeClient{comments: map[string]string{}, login: "engineer", node: "encoded-origin-and-repository-id", writable: true, postErrAfterStore: ambiguous}
			store := &memoryPublicationStore{}
			adapter, err := NewNativeHostAdapter(repo, persist.MediumForgejo, nativeFake{client}, store)
			if err != nil {
				t.Fatal(err)
			}
			if err := adapter.Preflight(t.Context(), repo); err != nil {
				t.Fatal(err)
			}
			binding := hostTestBinding()
			binding.ProposalID = repo + "#7"
			handoff := hostTestHandoff(t)
			handoff.Aggregate.ProposalID = binding.ProposalID
			handoff.Aggregate.Report.ProposalRef = binding.ProposalID
			terminal, err := adapter.Complete(t.Context(), binding, handoff)
			if err != nil {
				t.Fatal(err)
			}
			want := implreview.PublicationVerified
			if ambiguous {
				want = implreview.PublicationFailed
			}
			if terminal.Status != want || terminal.Medium != "forgejo" {
				t.Fatalf("terminal=%+v", terminal)
			}
			replayed, err := adapter.Complete(t.Context(), binding, handoff)
			if err != nil || replayed != terminal || client.posts != 1 {
				t.Fatalf("replay=%+v posts=%d err=%v", replayed, client.posts, err)
			}
			if !ambiguous {
				ref := persist.Ref{Medium: persist.MediumForgejo, Locator: terminal.NativeReceipt}
				if _, err := adapter.Read(t.Context(), ref); err != nil {
					t.Fatal(err)
				}
				ref.Locator = "71"
				if _, err := adapter.Read(t.Context(), ref); err == nil {
					t.Fatal("unqualified receipt accepted")
				}
				ref.Locator = "forgejo-comment:another-server:71"
				if _, err := adapter.Read(t.Context(), ref); err == nil {
					t.Fatal("foreign receipt accepted")
				}
			}
			for _, proposal := range []string{"github:o/r#7", repo + "#0", repo + "#bad"} {
				if _, err := adapter.proposalNumber(proposal); err == nil {
					t.Fatalf("invalid proposal accepted: %s", proposal)
				}
			}
		})
	}
	if _, err := NewNativeHostAdapter("o/r", persist.MediumGitHub, nativeFake{}, &memoryPublicationStore{}); err == nil {
		t.Fatal("unsupported medium accepted")
	}
	if _, err := NewNativeHostAdapter("invalid", persist.MediumForgejo, nativeFake{}, &memoryPublicationStore{}); err == nil {
		t.Fatal("invalid repository accepted")
	}
}
