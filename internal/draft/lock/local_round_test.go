package lock

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	localgit "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit"
	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit/localgittest"
	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
)

type localRoundStore struct {
	codehost.DraftProposals
	failure string
}

func (s localRoundStore) Mainline(ctx context.Context) (codehost.Readiness, error) {
	if s.failure == "mainline" {
		return codehost.Readiness{}, errors.New("mainline unavailable")
	}
	return s.DraftProposals.Mainline(ctx)
}
func (s localRoundStore) List(ctx context.Context) ([]codehost.ProposalGeneration, error) {
	if s.failure == "list" {
		return nil, errors.New("proposals unavailable")
	}
	if s.failure == "absent" {
		return nil, nil
	}
	all, err := s.DraftProposals.List(ctx)
	if err == nil && len(all) > 0 {
		if s.failure == "duplicate" {
			all = append(all, all[0])
		}
		if s.failure == "digest" {
			all[0].Proposal.ReviewRequest = &codehost.DraftReviewRequest{Digest: "wrong"}
		}
		if s.failure == "version" {
			all[0].Proposal.ID = "draft-widget-v01"
		}
	}
	return all, err
}
func (s localRoundStore) Publish(ctx context.Context, c codehost.ProposalChange) (codehost.ProposalGeneration, error) {
	if s.failure == "publish" {
		return codehost.ProposalGeneration{}, errors.New("proposal moved")
	}
	return s.DraftProposals.Publish(ctx, c)
}

func TestLocalPreparedRoundRestoresExactBytesAndInvalidatesRequest(t *testing.T) {
	localgittest.Skip(t)
	for _, failure := range []string{"", "mainline", "list", "absent", "duplicate", "digest", "version", "publish"} {
		t.Run(failure, func(t *testing.T) {
			work, cfg, host := prepareFixture(t, true)
			prepared, err := Prepare(t.Context(), work, cfg, "widget", host.run)
			if err != nil {
				t.Fatal(err)
			}
			origin := gitFixture(t, work, "remote", "get-url", "origin")
			gitFixture(t, origin, "update-ref", "-d", "refs/heads/draft/widget")
			base := gitFixture(t, origin, "rev-parse", "refs/heads/main")
			store := localgit.NewProposals(localgit.ProposalOptions{Git: localgit.Options{Root: work, Origin: origin, Remote: "origin", Mainline: "main"}, Kind: "draft"})
			request, err := codehost.NewDraftReviewRequest(prepared.Verdict.DraftDigest)
			if err != nil {
				t.Fatal(err)
			}
			g, err := store.Publish(t.Context(), codehost.ProposalChange{ID: "draft-widget-v1", Feature: "widget", Branch: "refs/heads/draft/widget", Head: prepared.Head, State: "prepared", ExpectedTargetOID: base, ReviewRequest: &request})
			if err != nil {
				t.Fatal(err)
			}
			cfg.Draft.AgentLogin, cfg.Draft.AgentTokenEnv = "", ""
			out, err := RoundWithProposals(t.Context(), work, cfg, "widget", draftshell.RunLocalGit, localRoundStore{store, failure})
			if failure != "" {
				if err == nil {
					t.Fatal("inconsistent prepared state accepted")
				}
				if gitFixture(t, origin, "rev-parse", "refs/heads/draft/widget") != prepared.Head {
					t.Fatal("refused round advanced branch")
				}
				if failure == "publish" {
					clone := filepath.Join(t.TempDir(), "clone")
					gitFixture(t, filepath.Dir(clone), "clone", origin, clone)
					// Clones do not inherit the fixture's local configuration.
					// Keep evidence fetches from launching detached maintenance
					// that can outlive this test's TempDir cleanup.
					gitFixture(t, clone, "config", "gc.auto", "0")
					gitFixture(t, clone, "config", "maintenance.auto", "false")
					pending, pendingErr := events.OpenPending(clone, "origin", "widget", 1)
					if pendingErr != nil {
						t.Fatal(pendingErr)
					}
					retained, pendingErr := pending.Read(t.Context())
					if pendingErr != nil || len(retained) != 1 || retained[0].Name != events.DraftStepFailed || retained[0].Step != "draft-round" || retained[0].VersionDigest != prepared.Verdict.DraftDigest || !strings.Contains(err.Error(), "proposal moved") {
						t.Fatalf("restored round lost failure history: %+v %v (round: %v)", retained, pendingErr, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			next, err := store.Read(t.Context(), "draft-widget-v1")
			if err != nil {
				t.Fatal(err)
			}
			if next.Proposal.State != "open" || next.Proposal.Sequence != 2 || next.Proposal.PreviousOID != g.OID || next.Proposal.ReviewRequest != nil || next.Proposal.Head != out.Head {
				t.Fatalf("restored=%+v", next)
			}
			if out.Digest != prepared.Verdict.DraftDigest || !strings.Contains(gitFixture(t, work, "show", "HEAD:drafts/widget/prd.md"), "Widget") {
				t.Fatal("restored bytes changed their reviewed digest")
			}
			if gitFixture(t, origin, "rev-parse", "refs/heads/main") != base {
				t.Fatal("round changed mainline")
			}
		})
	}
	if _, err := RoundWithProposals(t.Context(), "", config.Defaults(), "widget", nil, nil); err == nil {
		t.Fatal("missing store accepted")
	}
}
