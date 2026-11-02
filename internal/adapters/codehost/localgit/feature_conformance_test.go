package localgit

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/conformancetest"
)

func TestFeatureProposalPortConformance(t *testing.T) {
	t.Parallel()
	conformancetest.RunProposals(t, func(t *testing.T) conformancetest.ProposalFixture {
		t.Parallel()
		o, drive, port := featureProposalFixture(t)
		base := fixtureGit(t, o.Root, "rev-parse", "HEAD")
		fixtureGit(t, o.Root, "commit", "--allow-empty", "-m", "task work")
		head := fixtureGit(t, o.Root, "rev-parse", "HEAD")
		fixtureGit(t, o.Root, "commit", "--allow-empty", "-m", "task feedback")
		next := fixtureGit(t, o.Root, "rev-parse", "HEAD")
		return conformancetest.ProposalFixture{
			Port: port, Integration: drive.IntegrationRef, Mainline: drive.TargetMainRef,
			Trust: codehost.Trust{SharedAccount: true}, UpdatedHead: next,
			Task:  codehost.ProposalChange{ID: "task-widget-t1", Feature: drive.FeatureID, Task: "T1", Branch: "refs/heads/task/widget-t1", Head: head, State: "open", ExpectedTargetOID: base},
			Final: codehost.ProposalChange{ID: "final-widget", Feature: drive.FeatureID, Branch: "refs/heads/final/widget", Head: base, State: "open", ExpectedTargetOID: base},
			Clone: func(t *testing.T) featuredrive.LocalProposals {
				port, err := NewFeatureProposals(t.Context(), proposalClone(t, o), drive)
				if err != nil {
					t.Fatal(err)
				}
				return port
			},
			Targets: func(t *testing.T) [2]string {
				return [2]string{fixtureGit(t, o.Origin, "rev-parse", drive.IntegrationRef), fixtureGit(t, o.Origin, "rev-parse", drive.TargetMainRef)}
			},
			State: func(t *testing.T) string { return fixtureGit(t, o.Origin, "show-ref") },
			AuthorityState: func(t *testing.T) string {
				return fixtureGit(t, o.Origin, "for-each-ref", "--format=%(refname)", "refs/pyxis/receipts/", "refs/pyxis/review-requests/", "refs/pyxis/proposal-archives/")
			},
			SeedForeign: func(t *testing.T, change codehost.ProposalChange, final bool) {
				options := ProposalOptions{Git: o, Kind: "implementation", IntegrationRef: drive.IntegrationRef}
				if final {
					options.IntegrationRef, options.HeadRef = drive.TargetMainRef, drive.IntegrationRef
				}
				if _, err := NewProposals(options).Publish(t.Context(), change); err != nil {
					t.Fatal(err)
				}
			},
		}
	})
}
